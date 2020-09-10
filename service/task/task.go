package task

import (
	"context"
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
)

type TaskID string
type TaskCommandID string
type TaskInstanceID uint64

const (
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
var supportedTasks = make(map[TaskID]*supportedTaskData)

type supportedTaskData struct {
	supportedCommands []TaskCommandID

	newTaskFunc func(TaskInstanceID, *taskRunData) taskHandler
}

func isSupportedTask(taskID TaskID, taskCommandID TaskCommandID) bool {
	taskData, exists := supportedTasks[taskID]
	if exists == true {
		for _, command := range taskData.supportedCommands {
			if command == taskCommandID {
				return true
			}
		}
	}

	return false
}

//
// task
//
type task struct {
	id         TaskID
	commandID  TaskCommandID
	instanceID TaskInstanceID

	notifierID string

	runFunc func(TaskNotificationSender)

	cancel bool
}

type taskHandler interface {
	ID() TaskID
	CommandID() TaskCommandID
	InstanceID() TaskInstanceID

	NotifierID() string

	Run(taskNotificationSender TaskNotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceID)

	Cancel()
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

	t.runFunc(taskNotificationSender)
}

func (t *task) Cancel() {
	t.cancel = true
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

			if isSupportedTask(taskRunData.taskID, taskRunData.taskCommandID) == false {
				m := fmt.Sprintf("'%s::%s'는 등록되지 않은 Task입니다.", taskRunData.taskID, taskRunData.taskCommandID)

				log.Error(m)
				s.taskNotificationSender.Notify(taskRunData.notifierID, m, taskRunData.taskCtx)

				continue
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

			h := supportedTasks[taskRunData.taskID].newTaskFunc(instanceID, taskRunData)
			if h == nil {
				m := fmt.Sprintf("'%s::%s'는 등록되지 않은 Task입니다.", taskRunData.taskID, taskRunData.taskCommandID)

				log.Error(m)
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

				taskCtx := context.Background()
				taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskID, taskHandler.ID())
				taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskCommandID, taskHandler.CommandID())
				s.taskNotificationSender.Notify(taskHandler.NotifierID(), "사용자 요청에 의해 작업이 취소되었습니다.", taskCtx)

				log.Debugf("'%s::%s' Task의 작업이 취소되었습니다.(TaskInstanceID:%d)", taskHandler.ID(), taskHandler.CommandID(), instanceID)
			} else {
				s.taskNotificationSender.NotifyWithDefault(fmt.Sprintf("해당 작업에 대한 정보를 찾을 수 없어 취소 요청이 실패하였습니다.(ID:%d)", instanceID))

				log.Warnf("등록되지 않은 Task에 대한 작업취소요청 메시지가 수신되었습니다.(TaskInstanceID:%d)", instanceID)
			}
			s.runningMu.Unlock()

		case <-serviceStopCtx.Done():
			log.Debug("Task 서비스 중지중...")

			// Task 스케쥴러를 중지한다.
			s.scheduler.Stop()

			// @@@@@ 아래 블럭도 mutex를 감싸야하나?
			// 어차피 채널을 닫기 때문에 닫은 서비스는 재사용하지 못한다. new로 무조건 재생성해야된다.
			/////////////////
			s.runningMu.Lock()
			for _, handler := range s.taskHandlers {
				handler.Cancel()
			}
			s.runningMu.Unlock()
			close(s.taskRunC)
			close(s.taskCancelC)
			close(s.taskDoneC)
			s.taskStopWaiter.Wait()

			s.runningMu.Lock()
			s.running = false
			s.taskHandlers = nil
			s.taskNotificationSender = nil //@@@@@
			s.runningMu.Unlock()
			/////////////////

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

			log.Errorf("Task 취소 요청중에 panic이 발생하였습니다.(TaskInstanceID:%s, panic:%s", taskInstanceID, r)
		}
	}()

	s.taskCancelC <- taskInstanceID

	return true
}

func (s *TaskService) SetTaskNotificationSender(taskNotificiationSender TaskNotificationSender) {
	s.taskNotificationSender = taskNotificiationSender
}
