package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutils"
	log "github.com/sirupsen/logrus"
)

type TaskID string
type TaskCommandID string
type TaskInstanceID string
type TaskRunBy int

// TaskCommandID의 마지막에 들어가는 특별한 문자
// 이 문자는 환경설정 파일(JSON)에서는 사용되지 않으며 오직 소스코드 상에서만 사용한다.
const taskCommandIDAnyString string = "*"

type taskContextKey string

const (
	TaskCtxKeyTitle         taskContextKey = "Title"
	TaskCtxKeyErrorOccurred taskContextKey = "ErrorOccurred"

	TaskCtxKeyTaskID              taskContextKey = "Task.TaskID"
	TaskCtxKeyTaskCommandID       taskContextKey = "Task.TaskCommandID"
	TaskCtxKeyTaskInstanceID      taskContextKey = "Task.TaskInstanceID"
	TaskCtxKeyElapsedTimeAfterRun taskContextKey = "Task.ElapsedTimeAfterRun"
)

const (
	TaskRunByUser TaskRunBy = iota
	TaskRunByScheduler
)

var (
	ErrNotSupportedTask               = apperrors.New(apperrors.ErrInvalidInput, "지원되지 않는 작업입니다")
	ErrNotSupportedCommand            = apperrors.New(apperrors.ErrInvalidInput, "지원되지 않는 작업 커맨드입니다")
	ErrNoImplementationForTaskCommand = apperrors.New(apperrors.ErrInternal, "작업 커맨드에 대한 구현이 없습니다")
)

// taskInstanceIDGenerator
type taskInstanceIDGenerator struct {
}

func (g *taskInstanceIDGenerator) New() TaskInstanceID {
	return TaskInstanceID(g.toRadixNotation62String(time.Now().UnixNano()))
}

func (g *taskInstanceIDGenerator) toRadixNotation62String(value int64) string {
	if value < 0 {
		return ""
	}

	var digits = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
		"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}

	var s []string
	var radix = int64(len(digits))

	for value > 0 {
		s = append(s, digits[value%radix])
		value = value / radix
		if value == 0 {
			break
		}
	}

	return strings.Join(g.reverse(s), "")
}

func (g *taskInstanceIDGenerator) reverse(s []string) []string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// supportedTasks
type NewTaskFunc func(TaskInstanceID, *TaskRunData, *config.AppConfig) (TaskHandler, error)
type NewTaskResultDataFunc func() interface{}

var supportedTasks = make(map[TaskID]*TaskConfig)

func RegisterTask(taskID TaskID, config *TaskConfig) {
	supportedTasks[taskID] = config
}

type TaskConfig struct {
	CommandConfigs []*TaskCommandConfig

	NewTaskFn NewTaskFunc
}

type TaskCommandConfig struct {
	TaskCommandID TaskCommandID

	AllowMultipleInstances bool

	NewTaskResultDataFn NewTaskResultDataFunc
}

func (c *TaskCommandConfig) equalsTaskCommandID(taskCommandID TaskCommandID) bool {
	if strings.HasSuffix(string(c.TaskCommandID), taskCommandIDAnyString) == true {
		compareLength := len(c.TaskCommandID) - len(taskCommandIDAnyString)
		return len(c.TaskCommandID) <= len(taskCommandID) && c.TaskCommandID[:compareLength] == taskCommandID[:compareLength]
	}

	return c.TaskCommandID == taskCommandID
}

func findConfigFromSupportedTask(taskID TaskID, taskCommandID TaskCommandID) (*TaskConfig, *TaskCommandConfig, error) {
	taskConfig, exists := supportedTasks[taskID]
	if exists == true {
		for _, commandConfig := range taskConfig.CommandConfigs {
			if commandConfig.equalsTaskCommandID(taskCommandID) == true {
				return taskConfig, commandConfig, nil
			}
		}

		return nil, nil, ErrNotSupportedCommand
	}

	return nil, nil, ErrNotSupportedTask
}

// TaskRunFunc
type TaskRunFunc func(interface{}, bool) (string, interface{}, error)

type Task struct {
	ID         TaskID
	CommandID  TaskCommandID
	InstanceID TaskInstanceID

	NotifierID string

	Canceled bool

	RunBy   TaskRunBy
	RunTime time.Time

	RunFn TaskRunFunc

	Fetcher Fetcher
}

type TaskHandler interface {
	GetID() TaskID
	GetCommandID() TaskCommandID
	GetInstanceID() TaskInstanceID

	GetNotifierID() string

	Cancel()
	IsCanceled() bool

	ElapsedTimeAfterRun() int64

	Run(taskNotificationSender TaskNotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceID)
}

func (t *Task) GetID() TaskID {
	return t.ID
}

func (t *Task) GetCommandID() TaskCommandID {
	return t.CommandID
}

func (t *Task) GetInstanceID() TaskInstanceID {
	return t.InstanceID
}

func (t *Task) GetNotifierID() string {
	return t.NotifierID
}

func (t *Task) Cancel() {
	t.Canceled = true
}

func (t *Task) IsCanceled() bool {
	return t.Canceled
}

func (t *Task) ElapsedTimeAfterRun() int64 {
	return int64(time.Since(t.RunTime).Seconds())
}

func (t *Task) Run(taskNotificationSender TaskNotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceID) {
	const errString = "작업 진행중 오류가 발생하여 작업이 실패하였습니다.😱"

	defer taskStopWaiter.Done()
	defer func() {
		taskDoneC <- t.InstanceID
	}()

	t.RunTime = time.Now()

	var taskCtx = NewContext().WithTask(t.GetID(), t.GetCommandID())

	if t.RunFn == nil {
		m := fmt.Sprintf("%s\n\n☑ runFn()이 초기화되지 않았습니다.", errString)

		applog.WithComponentAndFields("task.executor", log.Fields{
			"task_id":    t.GetID(),
			"command_id": t.GetCommandID(),
		}).Error(m)

		t.notifyError(taskNotificationSender, m, taskCtx)

		return
	}

	// TaskResultData를 초기화하고 읽어들인다.
	var taskResultData interface{}
	if taskConfig, exists := supportedTasks[t.GetID()]; exists == true {
		for _, commandConfig := range taskConfig.CommandConfigs {
			if commandConfig.equalsTaskCommandID(t.GetCommandID()) == true {
				taskResultData = commandConfig.NewTaskResultDataFn()
				break
			}
		}
	}
	if taskResultData == nil {
		m := fmt.Sprintf("%s\n\n☑ 작업결과데이터 생성이 실패하였습니다.", errString)

		applog.WithComponentAndFields("task.executor", log.Fields{
			"task_id":    t.GetID(),
			"command_id": t.GetCommandID(),
		}).Error(m)

		t.notifyError(taskNotificationSender, m, taskCtx)

		return
	}
	err := t.readTaskResultDataFromFile(taskResultData)
	if err != nil {
		m := fmt.Sprintf("이전 작업결과데이터 로딩이 실패하였습니다.😱\n\n☑ %s\n\n빈 작업결과데이터를 이용하여 작업을 계속 진행합니다.", err)

		applog.WithComponentAndFields("task.executor", log.Fields{
			"task_id":    t.GetID(),
			"command_id": t.GetCommandID(),
			"error":      err,
		}).Warn(m)

		t.notify(taskNotificationSender, m, taskCtx)
	}

	if message, changedTaskResultData, err := t.RunFn(taskResultData, taskNotificationSender.SupportsHTMLMessage(t.NotifierID)); t.IsCanceled() == false {
		if err == nil {
			if len(message) > 0 {
				t.notify(taskNotificationSender, message, taskCtx)
			}

			if changedTaskResultData != nil {
				if err := t.writeTaskResultDataToFile(changedTaskResultData); err != nil {
					m := fmt.Sprintf("작업이 끝난 작업결과데이터의 저장이 실패하였습니다.😱\n\n☑ %s", err)

					applog.WithComponentAndFields("task.executor", log.Fields{
						"task_id":    t.GetID(),
						"command_id": t.GetCommandID(),
						"error":      err,
					}).Warn(m)

					t.notifyError(taskNotificationSender, m, taskCtx)
				}
			}
		} else {
			m := fmt.Sprintf("%s\n\n☑ %s", errString, err)

			applog.WithComponentAndFields("task.executor", log.Fields{
				"task_id":    t.GetID(),
				"command_id": t.GetCommandID(),
				"error":      err,
			}).Error(m)

			t.notifyError(taskNotificationSender, m, taskCtx)

			return
		}
	}
}

func (t *Task) notify(taskNotificationSender TaskNotificationSender, m string, taskCtx TaskContext) bool {
	return taskNotificationSender.NotifyWithTaskContext(t.GetNotifierID(), m, taskCtx)
}

func (t *Task) notifyError(taskNotificationSender TaskNotificationSender, m string, taskCtx TaskContext) bool {
	return taskNotificationSender.NotifyWithTaskContext(t.GetNotifierID(), m, taskCtx.WithError())
}

func (t *Task) dataFileName() string {
	filename := fmt.Sprintf("%s-task-%s-%s.json", config.AppName, strutils.ToSnakeCase(string(t.GetID())), strutils.ToSnakeCase(string(t.GetCommandID())))
	return strings.ReplaceAll(filename, "_", "-")
}

func (t *Task) readTaskResultDataFromFile(v interface{}) error {
	data, err := os.ReadFile(t.dataFileName())
	if err != nil {
		// 아직 데이터 파일이 생성되기 전이라면 nil을 반환한다.
		var pathError *os.PathError
		if errors.As(err, &pathError) == true {
			return nil
		}

		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 파일을 읽는데 실패했습니다")
	}

	return json.Unmarshal(data, v)
}

func (t *Task) writeTaskResultDataToFile(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 마샬링에 실패했습니다")
	}

	if err := os.WriteFile(t.dataFileName(), data, os.FileMode(0644)); err != nil {
		return apperrors.Wrap(err, apperrors.ErrInternal, "작업 결과 데이터 파일 쓰기에 실패했습니다")
	}

	return nil
}

// TaskContext
type TaskContext interface {
	With(key, val interface{}) TaskContext
	WithTask(taskID TaskID, taskCommandID TaskCommandID) TaskContext
	WithInstanceID(taskInstanceID TaskInstanceID, elapsedTimeAfterRun int64) TaskContext
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

func (c *taskContext) WithInstanceID(taskInstanceID TaskInstanceID, elapsedTimeAfterRun int64) TaskContext {
	c.ctx = context.WithValue(c.ctx, TaskCtxKeyTaskInstanceID, taskInstanceID)
	c.ctx = context.WithValue(c.ctx, TaskCtxKeyElapsedTimeAfterRun, elapsedTimeAfterRun)
	return c
}

func (c *taskContext) WithError() TaskContext {
	c.ctx = context.WithValue(c.ctx, TaskCtxKeyErrorOccurred, true)
	return c
}

func (c *taskContext) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}

// TaskRunData
type TaskRunData struct {
	TaskID        TaskID
	TaskCommandID TaskCommandID

	TaskCtx TaskContext

	NotifierID string

	NotifyResultOfTaskRunRequest bool

	TaskRunBy TaskRunBy
}

// TaskExecutor
type TaskExecutor interface {
	TaskRun(taskID TaskID, taskCommandID TaskCommandID, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) (succeeded bool)
	TaskRunWithContext(taskID TaskID, taskCommandID TaskCommandID, taskCtx TaskContext, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) (succeeded bool)
}

// TaskCanceler
type TaskCanceler interface {
	TaskCancel(taskInstanceID TaskInstanceID) (succeeded bool)
}

// TaskRunner
type TaskRunner interface {
	TaskExecutor
	TaskCanceler
}

// TaskNotificationSender
type TaskNotificationSender interface {
	NotifyToDefault(message string) bool
	NotifyWithTaskContext(notifierID string, message string, taskCtx TaskContext) bool

	SupportsHTMLMessage(notifierID string) bool
}

// TaskService
type TaskService struct {
	appConfig *config.AppConfig

	running   bool
	runningMu sync.Mutex

	scheduler scheduler

	taskHandlers map[TaskInstanceID]TaskHandler

	taskInstanceIDGenerator taskInstanceIDGenerator

	taskNotificationSender TaskNotificationSender

	taskRunC    chan *TaskRunData
	taskDoneC   chan TaskInstanceID
	taskCancelC chan TaskInstanceID

	taskStopWaiter *sync.WaitGroup
}

func NewService(appConfig *config.AppConfig) *TaskService {
	return &TaskService{
		appConfig: appConfig,

		running:   false,
		runningMu: sync.Mutex{},

		scheduler: scheduler{},

		taskHandlers: make(map[TaskInstanceID]TaskHandler),

		taskInstanceIDGenerator: taskInstanceIDGenerator{},

		taskNotificationSender: nil,

		taskRunC:    make(chan *TaskRunData, 10),
		taskDoneC:   make(chan TaskInstanceID, 10),
		taskCancelC: make(chan TaskInstanceID, 10),

		taskStopWaiter: &sync.WaitGroup{},
	}
}

func (s *TaskService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent("task.service").Info("Task 서비스 시작중...")

	if s.taskNotificationSender == nil {
		defer serviceStopWaiter.Done()

		return apperrors.New(apperrors.ErrInternal, "TaskNotificationSender 객체가 초기화되지 않았습니다")
	}

	if s.running == true {
		defer serviceStopWaiter.Done()

		applog.WithComponent("task.service").Warn("Task 서비스가 이미 시작됨!!!")

		return nil
	}

	// Task 스케쥴러를 시작한다.
	s.scheduler.Start(s.appConfig, s, s.taskNotificationSender)

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	applog.WithComponent("task.service").Info("Task 서비스 시작됨")

	return nil
}

func (s *TaskService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	for {
		select {
		case taskRunData := <-s.taskRunC:
			applog.WithComponentAndFields("task.service", log.Fields{
				"task_id":    taskRunData.TaskID,
				"command_id": taskRunData.TaskCommandID,
				"run_by":     taskRunData.TaskRunBy,
			}).Debug("새로운 Task 실행 요청 수신")

			if taskRunData.TaskCtx == nil {
				taskRunData.TaskCtx = NewContext()
			}
			taskRunData.TaskCtx.WithTask(taskRunData.TaskID, taskRunData.TaskCommandID)

			taskConfig, commandConfig, err := findConfigFromSupportedTask(taskRunData.TaskID, taskRunData.TaskCommandID)
			if err != nil {
				m := "등록되지 않은 작업입니다.😱"

				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":    taskRunData.TaskID,
					"command_id": taskRunData.TaskCommandID,
					"error":      err,
				}).Error(m)

				s.taskNotificationSender.NotifyWithTaskContext(taskRunData.NotifierID, m, taskRunData.TaskCtx.WithError())

				continue
			}

			// 다중 인스턴스의 생성이 허용되지 않는 Task인 경우, 이미 실행중인 동일한 Task가 있는지 확인한다.
			if commandConfig.AllowMultipleInstances == false {
				var alreadyRunTaskHandler TaskHandler

				s.runningMu.Lock()
				for _, handler := range s.taskHandlers {
					if handler.GetID() == taskRunData.TaskID && handler.GetCommandID() == taskRunData.TaskCommandID && handler.IsCanceled() == false {
						alreadyRunTaskHandler = handler
						break
					}
				}
				s.runningMu.Unlock()

				if alreadyRunTaskHandler != nil {
					taskRunData.TaskCtx.WithInstanceID(alreadyRunTaskHandler.GetInstanceID(), alreadyRunTaskHandler.ElapsedTimeAfterRun())
					s.taskNotificationSender.NotifyWithTaskContext(taskRunData.NotifierID, "요청하신 작업은 이미 진행중입니다.\n이전 작업을 취소하시려면 아래 명령어를 클릭하여 주세요.", taskRunData.TaskCtx)
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

			h, err := taskConfig.NewTaskFn(instanceID, taskRunData, s.appConfig)
			if h == nil {
				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":    taskRunData.TaskID,
					"command_id": taskRunData.TaskCommandID,
					"error":      err,
				}).Error(err)

				s.taskNotificationSender.NotifyWithTaskContext(taskRunData.NotifierID, err.Error(), taskRunData.TaskCtx.WithError())

				continue
			}

			s.runningMu.Lock()
			s.taskHandlers[instanceID] = h
			s.runningMu.Unlock()

			s.taskStopWaiter.Add(1)
			go h.Run(s.taskNotificationSender, s.taskStopWaiter, s.taskDoneC)

			if taskRunData.NotifyResultOfTaskRunRequest == true {
				s.taskNotificationSender.NotifyWithTaskContext(taskRunData.NotifierID, "작업 진행중입니다. 잠시만 기다려 주세요.", taskRunData.TaskCtx.WithInstanceID(instanceID, 0))
			}

		case instanceID := <-s.taskDoneC:
			s.runningMu.Lock()
			if taskHandler, exists := s.taskHandlers[instanceID]; exists == true {
				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":     taskHandler.GetID(),
					"command_id":  taskHandler.GetCommandID(),
					"instance_id": instanceID,
				}).Debug("Task 작업 완료")

				delete(s.taskHandlers, instanceID)
			} else {
				applog.WithComponentAndFields("task.service", log.Fields{
					"instance_id": instanceID,
				}).Warn("등록되지 않은 Task에 대한 작업완료 메시지 수신")
			}
			s.runningMu.Unlock()

		case instanceID := <-s.taskCancelC:
			s.runningMu.Lock()
			if taskHandler, exists := s.taskHandlers[instanceID]; exists == true {
				taskHandler.Cancel()

				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":     taskHandler.GetID(),
					"command_id":  taskHandler.GetCommandID(),
					"instance_id": instanceID,
				}).Debug("Task 작업 취소")

				s.taskNotificationSender.NotifyWithTaskContext(taskHandler.GetNotifierID(), "사용자 요청에 의해 작업이 취소되었습니다.", NewContext().WithTask(taskHandler.GetID(), taskHandler.GetCommandID()))
			} else {
				applog.WithComponentAndFields("task.service", log.Fields{
					"instance_id": instanceID,
				}).Warn("등록되지 않은 Task에 대한 작업취소 요청 메시지 수신")

				s.taskNotificationSender.NotifyToDefault(fmt.Sprintf("해당 작업에 대한 정보를 찾을 수 없습니다.😱\n취소 요청이 실패하였습니다.(ID:%s)", instanceID))
			}
			s.runningMu.Unlock()

		case <-serviceStopCtx.Done():
			applog.WithComponent("task.service").Info("Task 서비스 중지중...")

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

			applog.WithComponent("task.service").Info("Task 서비스 중지됨")

			return
		}
	}
}

func (s *TaskService) TaskRun(taskID TaskID, taskCommandID TaskCommandID, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) (succeeded bool) {
	return s.TaskRunWithContext(taskID, taskCommandID, nil, notifierID, notifyResultOfTaskRunRequest, taskRunBy)
}

func (s *TaskService) TaskRunWithContext(taskID TaskID, taskCommandID TaskCommandID, taskCtx TaskContext, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			applog.WithComponentAndFields("task.service", log.Fields{
				"task_id":    taskID,
				"command_id": taskCommandID,
				"panic":      r,
			}).Error("Task 실행 요청중에 panic 발생")
		}
	}()

	s.taskRunC <- &TaskRunData{
		TaskID:        taskID,
		TaskCommandID: taskCommandID,

		TaskCtx: taskCtx,

		NotifierID: notifierID,

		NotifyResultOfTaskRunRequest: notifyResultOfTaskRunRequest,

		TaskRunBy: taskRunBy,
	}

	return true
}

func (s *TaskService) TaskCancel(taskInstanceID TaskInstanceID) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			applog.WithComponentAndFields("task.service", log.Fields{
				"instance_id": taskInstanceID,
				"panic":       r,
			}).Error("Task 취소 요청중에 panic 발생")
		}
	}()

	s.taskCancelC <- taskInstanceID

	return true
}

func (s *TaskService) SetTaskNotificationSender(taskNotificiationSender TaskNotificationSender) {
	s.taskNotificationSender = taskNotificiationSender
}
