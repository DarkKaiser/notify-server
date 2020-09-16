package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

type TaskID string
type TaskCommandID string
type TaskInstanceID uint64
type TaskRunBy int

const (
	TaskCtxKeyErrorOccurred = "ErrorOccurred"

	TaskCtxKeyTaskID         = "Task.TaskID"
	TaskCtxKeyTaskCommandID  = "Task.TaskCommandID"
	TaskCtxKeyTaskInstanceID = "Task.TaskInstanceID"
)

const (
	TaskRunByUser TaskRunBy = iota
	TaskRunByScheduler
)

var (
	ErrNotSupportedTask               = errors.New("not supported task")
	ErrNotSupportedCommand            = errors.New("not supported command")
	ErrNoImplementationForTaskCommand = errors.New("no implementation for task command")
)

//
// taskInstanceIDGenerator
//
type taskInstanceIDGenerator struct {
	id TaskInstanceID
}

func (g *taskInstanceIDGenerator) New() TaskInstanceID {
	return TaskInstanceID(atomic.AddUint64((*uint64)(&g.id), 1))
}

//
// supportedTasks
//
type newTaskFunc func(TaskInstanceID, *taskRunData) taskHandler
type newTaskDataFunc func() interface{}

var supportedTasks = make(map[TaskID]*supportedTaskConfig)

type supportedTaskConfig struct {
	commandConfigs []*supportedTaskCommandConfig

	newTaskFn newTaskFunc
}

type supportedTaskCommandConfig struct {
	taskCommandID TaskCommandID

	allowMultipleIntances bool

	newTaskDataFn newTaskDataFunc
}

func findConfigFromSupportedTask(taskID TaskID, taskCommandID TaskCommandID) (*supportedTaskConfig, *supportedTaskCommandConfig, error) {
	taskConfig, exists := supportedTasks[taskID]
	if exists == true {
		for _, commandConfig := range taskConfig.commandConfigs {
			if commandConfig.taskCommandID == taskCommandID {
				return taskConfig, commandConfig, nil
			}
		}

		return nil, nil, ErrNotSupportedCommand
	}

	return nil, nil, ErrNotSupportedTask
}

//
// task
//
type runFunc func(interface{}, TaskNotificationSender, TaskContext) (string, interface{}, error)

type task struct {
	id         TaskID
	commandID  TaskCommandID
	instanceID TaskInstanceID

	notifierID string

	canceled bool

	runBy TaskRunBy

	runFn runFunc
}

type taskHandler interface {
	ID() TaskID
	CommandID() TaskCommandID
	InstanceID() TaskInstanceID

	NotifierID() string

	Cancel()
	IsCanceled() bool

	Run(taskNotificationSender TaskNotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceID)
}

func (t *task) ID() TaskID {
	return t.id
}

func (t *task) CommandID() TaskCommandID {
	return t.commandID
}

func (t *task) InstanceID() TaskInstanceID {
	return t.instanceID
}

func (t *task) NotifierID() string {
	return t.notifierID
}

func (t *task) Cancel() {
	t.canceled = true
}

func (t *task) IsCanceled() bool {
	return t.canceled
}

func (t *task) Run(taskNotificationSender TaskNotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceID) {
	defer taskStopWaiter.Done()
	defer func() {
		taskDoneC <- t.instanceID
	}()

	var taskCtx = NewContext().WithTask(t.ID(), t.CommandID())

	if t.runFn == nil {
		log.Errorf("'%s::%s' Task의 runFn()이 초기화되지 않아 작업이 실패하였습니다.", t.ID(), t.CommandID())
		t.notifyError(taskNotificationSender, "작업 진행중 오류가 발생하여 작업이 실패하였습니다.\n\n- runFn()이 초기화되지 않았습니다.", taskCtx)
		return
	}

	// TaskData를 초기화하고 읽어들인다.
	var taskData interface{}
	if taskConfig, exists := supportedTasks[t.ID()]; exists == true {
		for _, commandConfig := range taskConfig.commandConfigs {
			if commandConfig.taskCommandID == t.CommandID() {
				taskData = commandConfig.newTaskDataFn()
				break
			}
		}
	}
	if taskData == nil {
		log.Errorf("'%s::%s' Task의 TaskData 생성이 실패하여 작업이 실패하였습니다.", t.ID(), t.CommandID())
		t.notifyError(taskNotificationSender, "작업 진행중 오류가 발생하여 작업이 실패하였습니다.\n\n- 작업데이터 생성이 실패하였습니다.", taskCtx)
		return
	}
	err := t.readTaskDataFromFile(taskData)
	if err != nil {
		log.Warnf("'%s::%s' Task의 TaskData 로딩이 실패하였습니다. 빈 TaskData를 이용하여 Task를 계속 실행합니다.(error:%s)", t.ID(), t.CommandID(), err.Error())
		t.notify(taskNotificationSender, fmt.Sprintf("작업데이터 로딩이 실패하였습니다.\n\n- %s\n\n빈 작업데이터를 이용하여 작업을 계속 진행합니다.", err.Error()), taskCtx)
	}

	message, changedTaskData, err := t.runFn(taskData, taskNotificationSender, taskCtx)
	if err == nil {
		if len(message) > 0 {
			t.notify(taskNotificationSender, message, taskCtx)
		}

		if changedTaskData != nil {
			if err := t.writeTaskDataToFile(changedTaskData); err != nil {
				log.Warnf("'%s::%s' Task의 TaskData 저장이 실패하였습니다.(error:%s)", t.ID(), t.CommandID(), err.Error())
				t.notifyError(taskNotificationSender, fmt.Sprintf("작업이 끝난 작업데이터의 저장이 실패하였습니다.\n\n- %s", err.Error()), taskCtx)
			}
		}
	} else {
		log.Errorf("'%s::%s' Task를 실행하는 중에 오류가 발생하여 작업이 실패하였습니다.(error:%s)", t.ID(), t.CommandID(), err.Error())
		t.notifyError(taskNotificationSender, fmt.Sprintf("작업 진행중 오류가 발생하여 작업이 실패하였습니다.\n\n- %s", err.Error()), taskCtx)
		return
	}
}

func (t *task) notify(taskNotificationSender TaskNotificationSender, m string, taskCtx TaskContext) bool {
	return taskNotificationSender.Notify(t.NotifierID(), m, taskCtx)
}

func (t *task) notifyError(taskNotificationSender TaskNotificationSender, m string, taskCtx TaskContext) bool {
	return taskNotificationSender.Notify(t.NotifierID(), m, taskCtx.WithError())
}

func (t *task) dataFileName() string {
	filename := fmt.Sprintf("%s-task-%s-%s.json", g.AppName, utils.ToSnakeCase(string(t.ID())), utils.ToSnakeCase(string(t.CommandID())))
	return strings.ReplaceAll(filename, "_", "-")
}

func (t *task) readTaskDataFromFile(v interface{}) error {
	data, err := ioutil.ReadFile(t.dataFileName())
	if err != nil {
		// 아직 데이터 파일이 생성되기 전이라면 nil을 반환한다.
		if _, ok := err.(*os.PathError); ok {
			return nil
		}

		return err
	}

	return json.Unmarshal(data, v)
}

func (t *task) writeTaskDataToFile(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(t.dataFileName(), data, os.FileMode(0644))
}

//
// TaskContext
//
type TaskContext interface {
	With(key, val interface{}) TaskContext
	WithTask(taskID TaskID, taskCommandID TaskCommandID) TaskContext
	WithInstanceID(taskInstanceID TaskInstanceID) TaskContext
	WithError() TaskContext
	Value(key interface{}) interface{}
}

type taskContext struct {
	ctx context.Context
}

func NewContext() TaskContext {
	return &taskContext{
		ctx: context.Background(),
	}
}

func (c *taskContext) With(key, val interface{}) TaskContext {
	c.ctx = context.WithValue(c.ctx, key, val)
	return c
}

func (c *taskContext) WithTask(taskID TaskID, taskCommandID TaskCommandID) TaskContext {
	c.ctx = context.WithValue(c.ctx, TaskCtxKeyTaskID, taskID)
	c.ctx = context.WithValue(c.ctx, TaskCtxKeyTaskCommandID, taskCommandID)
	return c
}

func (c *taskContext) WithInstanceID(taskInstanceID TaskInstanceID) TaskContext {
	c.ctx = context.WithValue(c.ctx, TaskCtxKeyTaskInstanceID, taskInstanceID)
	return c
}

func (c *taskContext) WithError() TaskContext {
	c.ctx = context.WithValue(c.ctx, TaskCtxKeyErrorOccurred, true)
	return c
}

func (c *taskContext) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}

//
// taskRunData
//
type taskRunData struct {
	taskID        TaskID
	taskCommandID TaskCommandID

	taskCtx TaskContext

	notifierID string

	notificationOfRequestResult bool // @@@@@ 변수명 한번 더 고민

	taskRunBy TaskRunBy
}

//
// TaskRunner
//
type TaskRunner interface {
	TaskRun(taskID TaskID, taskCommandID TaskCommandID, notifierID string, notificationOfRequestResult bool, taskRunBy TaskRunBy) (succeeded bool)
	TaskRunWithContext(taskID TaskID, taskCommandID TaskCommandID, taskCtx TaskContext, notifierID string, notificationOfRequestResult bool, taskRunBy TaskRunBy) (succeeded bool)
	TaskCancel(taskInstanceID TaskInstanceID) (succeeded bool)
}

//
// TaskNotificationSender
//
type TaskNotificationSender interface {
	Notify(notifierID string, message string, taskCtx TaskContext) bool
	NotifyWithDefault(message string) bool
}

//
// TaskService
//
type TaskService struct {
	config *g.AppConfig

	running   bool
	runningMu sync.Mutex

	scheduler scheduler

	taskHandlers map[TaskInstanceID]taskHandler

	taskInstanceIDGenerator taskInstanceIDGenerator

	taskNotificationSender TaskNotificationSender

	taskRunC    chan *taskRunData
	taskDoneC   chan TaskInstanceID
	taskCancelC chan TaskInstanceID

	taskStopWaiter *sync.WaitGroup
}

func NewService(config *g.AppConfig) *TaskService {
	return &TaskService{
		config: config,

		running:   false,
		runningMu: sync.Mutex{},

		scheduler: scheduler{},

		taskHandlers: make(map[TaskInstanceID]taskHandler),

		taskInstanceIDGenerator: taskInstanceIDGenerator{id: 0},

		taskNotificationSender: nil,

		taskRunC:    make(chan *taskRunData, 10),
		taskDoneC:   make(chan TaskInstanceID, 10),
		taskCancelC: make(chan TaskInstanceID, 10),

		taskStopWaiter: &sync.WaitGroup{},
	}
}

func (s *TaskService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Debug("Task 서비스 시작중...")

	if s.taskNotificationSender == nil {
		log.Panicf("TaskNotificationSender 객체가 초기화되지 않았습니다.")
	}

	if s.running == true {
		defer serviceStopWaiter.Done()

		log.Warn("Task 서비스가 이미 시작됨!!!")

		return
	}

	// Task 스케쥴러를 시작한다.
	s.scheduler.Start(s.config, s, s.taskNotificationSender)

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	log.Debug("Task 서비스 시작됨")
}

func (s *TaskService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	for {
		select {
		case taskRunData := <-s.taskRunC:
			log.Debugf("새로운 '%s::%s' Task 실행 요청 수신", taskRunData.taskID, taskRunData.taskCommandID)

			if taskRunData.taskCtx == nil {
				taskRunData.taskCtx = NewContext()
			}
			taskRunData.taskCtx.WithTask(taskRunData.taskID, taskRunData.taskCommandID)

			taskConfig, commandConfig, err := findConfigFromSupportedTask(taskRunData.taskID, taskRunData.taskCommandID)
			if err != nil {
				m := "등록되지 않은 작업입니다."

				log.Error(m)

				s.taskNotificationSender.Notify(taskRunData.notifierID, m, taskRunData.taskCtx.WithError())

				continue
			}

			// 다중 인스턴스의 생성이 허용되지 않는 Task인 경우, 이미 실행중인 동일한 Task가 있는지 확인한다.
			if commandConfig.allowMultipleIntances == false {
				var alreadyRunTaskHandler taskHandler

				s.runningMu.Lock()
				for _, handler := range s.taskHandlers {
					if handler.ID() == taskRunData.taskID && handler.CommandID() == taskRunData.taskCommandID && handler.IsCanceled() == false {
						alreadyRunTaskHandler = handler
						break
					}
				}
				s.runningMu.Unlock()

				if alreadyRunTaskHandler != nil {
					taskRunData.taskCtx.WithInstanceID(alreadyRunTaskHandler.InstanceID())
					s.taskNotificationSender.Notify(taskRunData.notifierID, "요청하신 작업은 이미 진행중입니다.\n이전 작업을 취소하시려면 아래 명령어를 클릭하여 주세요.", taskRunData.taskCtx)
					continue
				}
			}

			var instanceID TaskInstanceID

			s.runningMu.Lock()
			for {
				instanceID = s.taskInstanceIDGenerator.New()
				if _, exists := s.taskHandlers[instanceID]; exists == false {
					break
				}
			}
			s.runningMu.Unlock()

			h := taskConfig.newTaskFn(instanceID, taskRunData)
			if h == nil {
				m := "등록되지 않은 작업입니다."

				log.Error(m)

				s.taskNotificationSender.Notify(taskRunData.notifierID, m, taskRunData.taskCtx.WithError())

				continue
			}

			s.runningMu.Lock()
			s.taskHandlers[instanceID] = h
			s.runningMu.Unlock()

			s.taskStopWaiter.Add(1)
			go h.Run(s.taskNotificationSender, s.taskStopWaiter, s.taskDoneC)

			if taskRunData.notificationOfRequestResult == true {
				s.taskNotificationSender.Notify(taskRunData.notifierID, "작업 진행중입니다. 잠시만 기다려 주세요.", taskRunData.taskCtx.WithInstanceID(instanceID))
			}

		case instanceID := <-s.taskDoneC:
			s.runningMu.Lock()
			if taskHandler, exists := s.taskHandlers[instanceID]; exists == true {
				log.Debugf("'%s::%s' Task의 작업이 완료되었습니다.(TaskInstanceID:%d)", taskHandler.ID(), taskHandler.CommandID(), instanceID)

				delete(s.taskHandlers, instanceID)
			} else {
				log.Warnf("등록되지 않은 Task에 대한 작업완료 메시지가 수신되었습니다.(TaskInstanceID:%d)", instanceID)
			}
			s.runningMu.Unlock()

		case instanceID := <-s.taskCancelC:
			s.runningMu.Lock()
			if taskHandler, exists := s.taskHandlers[instanceID]; exists == true {
				taskHandler.Cancel()

				log.Debugf("'%s::%s' Task의 작업이 취소되었습니다.(TaskInstanceID:%d)", taskHandler.ID(), taskHandler.CommandID(), instanceID)

				s.taskNotificationSender.Notify(taskHandler.NotifierID(), "사용자 요청에 의해 작업이 취소되었습니다.", NewContext().WithTask(taskHandler.ID(), taskHandler.CommandID()))
			} else {
				log.Warnf("등록되지 않은 Task에 대한 작업취소 요청 메시지가 수신되었습니다.(TaskInstanceID:%d)", instanceID)

				s.taskNotificationSender.NotifyWithDefault(fmt.Sprintf("해당 작업 정보를 찾을 수 없습니다. 취소 요청이 실패하였습니다.(ID:%d)", instanceID))
			}
			s.runningMu.Unlock()

		case <-serviceStopCtx.Done():
			log.Debug("Task 서비스 중지중...")

			// Task 스케쥴러를 중지한다.
			s.scheduler.Stop()

			s.runningMu.Lock()
			// 현재 작업중인 Task의 작업을 모두 취소한다.
			for _, handler := range s.taskHandlers {
				handler.Cancel()
			}
			s.runningMu.Unlock()

			close(s.taskRunC)
			close(s.taskCancelC)

			// Task의 작업이 모두 취소될 때까지 대기한다.
			s.taskStopWaiter.Wait()

			close(s.taskDoneC)

			s.runningMu.Lock()
			s.running = false
			s.taskHandlers = nil
			s.taskNotificationSender = nil
			s.runningMu.Unlock()

			log.Debug("Task 서비스 중지됨")

			return
		}
	}
}

func (s *TaskService) TaskRun(taskID TaskID, taskCommandID TaskCommandID, notifierID string, notificationOfRequestResult bool, taskRunBy TaskRunBy) (succeeded bool) {
	return s.TaskRunWithContext(taskID, taskCommandID, nil, notifierID, notificationOfRequestResult, taskRunBy)
}

func (s *TaskService) TaskRunWithContext(taskID TaskID, taskCommandID TaskCommandID, taskCtx TaskContext, notifierID string, notificationOfRequestResult bool, taskRunBy TaskRunBy) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("'%s::%s' Task 실행 요청중에 panic이 발생하였습니다.(panic:%s", taskID, taskCommandID, r)
		}
	}()

	s.taskRunC <- &taskRunData{
		taskID:        taskID,
		taskCommandID: taskCommandID,

		taskCtx: taskCtx,

		notifierID: notifierID,

		notificationOfRequestResult: notificationOfRequestResult,

		taskRunBy: taskRunBy,
	}

	return true
}

func (s *TaskService) TaskCancel(taskInstanceID TaskInstanceID) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("Task 취소 요청중에 panic이 발생하였습니다.(TaskInstanceID:%d, panic:%s", taskInstanceID, r)
		}
	}()

	s.taskCancelC <- taskInstanceID

	return true
}

func (s *TaskService) SetTaskNotificationSender(taskNotificiationSender TaskNotificationSender) {
	s.taskNotificationSender = taskNotificiationSender
}
