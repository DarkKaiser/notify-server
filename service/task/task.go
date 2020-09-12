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

const (
	TaskCtxKeyErrorOccurred = "ErrorOccurred"

	TaskCtxKeyTaskID         = "Task.TaskID"
	TaskCtxKeyTaskCommandID  = "Task.TaskCommandID"
	TaskCtxKeyTaskInstanceID = "Task.TaskInstanceID"
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
var supportedTasks = make(map[TaskID]*supportedTaskConfig)

type newTaskFunction func(TaskInstanceID, *taskRunData) taskHandler

type supportedTaskConfig struct {
	commandConfigs []*supportedTaskCommandConfig

	newTaskFunc newTaskFunction
}

type supportedTaskCommandConfig struct {
	taskCommandID TaskCommandID

	allowMultipleIntances bool
}

func findConfigFromSupportedTask(taskID TaskID, taskCommandID TaskCommandID) (*supportedTaskConfig, *supportedTaskCommandConfig, error) {
	taskConfig, exists := supportedTasks[taskID]
	if exists == true {
		for _, commandConfig := range taskConfig.commandConfigs {
			if commandConfig.taskCommandID == taskCommandID {
				return taskConfig, commandConfig, nil
			}
		}

		return nil, nil, errors.New("지원하지 않는 Command가 입력되었습니다")
	}

	return nil, nil, errors.New("입력된 Task를 찾을 수 없습니다")
}

//
// task
//
type task struct {
	id         TaskID
	commandID  TaskCommandID
	instanceID TaskInstanceID

	notifierID string

	runFunc func(TaskNotificationSender) bool

	cancel bool
}

type taskHandler interface {
	ID() TaskID
	CommandID() TaskCommandID
	InstanceID() TaskInstanceID

	NotifierID() string

	Run(taskNotificationSender TaskNotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceID)

	Cancel()
	IsCanceled() bool
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

func (t *task) Run(taskNotificationSender TaskNotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceID) {
	defer taskStopWaiter.Done()
	defer func() {
		taskDoneC <- t.instanceID
	}()

	if t.runFunc == nil {
		log.Panicf("'%s::%s' Task 객체의 runFunc이 할당되지 않았습니다.", t.ID(), t.CommandID())
	}

	if t.runFunc(taskNotificationSender) == false {
		taskCtx := context.Background()
		taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskID, t.ID())
		taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskCommandID, t.CommandID())
		taskCtx = context.WithValue(taskCtx, TaskCtxKeyErrorOccurred, true)

		m := fmt.Sprintf("'%s' Task의 '%s' 명령은 등록되지 않았습니다.", t.ID(), t.CommandID())

		log.Error(m)
		taskNotificationSender.Notify(t.NotifierID(), m, taskCtx)
	}
}

func (t *task) Cancel() {
	t.cancel = true
}

func (t *task) IsCanceled() bool {
	return t.cancel
}

func (t *task) dataFileName() string {
	filename := fmt.Sprintf("%s-task-%s-%s.json", g.AppName, utils.ToSnakeCase(string(t.ID())), utils.ToSnakeCase(string(t.CommandID())))
	return strings.ReplaceAll(filename, "_", "-")
}

func (t *alganicMallTask) readDataFromFile(v interface{}) error {
	data, err := ioutil.ReadFile(t.dataFileName())
	if err != nil {
		return err
	}

	return json.Unmarshal(data, v)
}

func (t *alganicMallTask) writeDataToFile(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(t.dataFileName(), data, os.FileMode(0644))
}

//
// taskRunData
//
type taskRunData struct {
	taskID        TaskID
	taskCommandID TaskCommandID

	taskCtx context.Context

	notifierID string

	notificationOfRequestResult bool
}

//
// TaskRunner
//
type TaskRunner interface {
	TaskRun(taskID TaskID, taskCommandID TaskCommandID, notifierID string, notificationOfRequestResult bool) (succeeded bool)
	TaskRunWithContext(taskID TaskID, taskCommandID TaskCommandID, taskCtx context.Context, notifierID string, notificationOfRequestResult bool) (succeeded bool)
	TaskCancel(taskInstanceID TaskInstanceID) (succeeded bool)
}

//
// TaskNotificationSender
//
type TaskNotificationSender interface {
	Notify(notifierID string, message string, taskCtx context.Context) bool
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
	// @@@@@ 서비스 시작시 allowMultipleIntances 값을 config에서 읽어와서 초기화할까????
	// 여러가지 정보중에서 위 변수 하나만 초기화하는것도 이상함!!!

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

			taskConfig, commandConfig, err := findConfigFromSupportedTask(taskRunData.taskID, taskRunData.taskCommandID)
			if err != nil {
				m := fmt.Sprintf("'%s::%s'는 등록되지 않은 Task입니다.", taskRunData.taskID, taskRunData.taskCommandID)

				log.Error(m)

				taskRunData.taskCtx = context.WithValue(taskRunData.taskCtx, TaskCtxKeyErrorOccurred, true)
				s.taskNotificationSender.Notify(taskRunData.notifierID, m, taskRunData.taskCtx)

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
					taskRunData.taskCtx = context.WithValue(taskRunData.taskCtx, TaskCtxKeyTaskInstanceID, alreadyRunTaskHandler.InstanceID())
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

			h := taskConfig.newTaskFunc(instanceID, taskRunData)
			if h == nil {
				m := fmt.Sprintf("'%s::%s'는 등록되지 않은 Task입니다.", taskRunData.taskID, taskRunData.taskCommandID)

				log.Error(m)

				taskRunData.taskCtx = context.WithValue(taskRunData.taskCtx, TaskCtxKeyErrorOccurred, true)
				s.taskNotificationSender.Notify(taskRunData.notifierID, m, taskRunData.taskCtx)

				continue
			}

			s.runningMu.Lock()
			s.taskHandlers[instanceID] = h
			s.runningMu.Unlock()

			s.taskStopWaiter.Add(1)
			go h.Run(s.taskNotificationSender, s.taskStopWaiter, s.taskDoneC)

			if taskRunData.notificationOfRequestResult == true {
				taskRunData.taskCtx = context.WithValue(taskRunData.taskCtx, TaskCtxKeyTaskInstanceID, instanceID)
				s.taskNotificationSender.Notify(taskRunData.notifierID, "작업 진행중입니다. 잠시만 기다려 주세요.", taskRunData.taskCtx)
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

				taskCtx := context.Background()
				taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskID, taskHandler.ID())
				taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskCommandID, taskHandler.CommandID())

				s.taskNotificationSender.Notify(taskHandler.NotifierID(), "사용자 요청에 의해 작업이 취소되었습니다.", taskCtx)
			} else {
				log.Warnf("등록되지 않은 Task에 대한 작업취소 요청 메시지가 수신되었습니다.(TaskInstanceID:%d)", instanceID)

				s.taskNotificationSender.NotifyWithDefault(fmt.Sprintf("해당 작업에 대한 정보를 찾을 수 없습니다. 취소 요청이 실패하였습니다.(ID:%d)", instanceID))
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

func (s *TaskService) TaskRun(taskID TaskID, taskCommandID TaskCommandID, notifierID string, notificationOfRequestResult bool) (succeeded bool) {
	return s.TaskRunWithContext(taskID, taskCommandID, nil, notifierID, notificationOfRequestResult)
}

func (s *TaskService) TaskRunWithContext(taskID TaskID, taskCommandID TaskCommandID, taskCtx context.Context, notifierID string, notificationOfRequestResult bool) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("'%s::%s' Task 실행 요청중에 panic이 발생하였습니다.(panic:%s", taskID, taskCommandID, r)
		}
	}()

	if taskCtx == nil {
		taskCtx = context.Background()
	}

	taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskID, taskID)
	taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskCommandID, taskCommandID)

	s.taskRunC <- &taskRunData{
		taskID:        taskID,
		taskCommandID: taskCommandID,

		taskCtx: taskCtx,

		notifierID: notifierID,

		notificationOfRequestResult: notificationOfRequestResult,
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
