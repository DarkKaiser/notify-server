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
type TaskContextKey string

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

// @@@@@
func validTaskCommand(taskID TaskID, taskCommandID TaskCommandID) bool {
	ids, exists := supportedTasks[taskID]
	if exists == false {
		return false
	}

	for _, id := range ids.supportedCommands {
		if id == taskCommandID {
			return true
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

	cancel bool

	runFunc func(TaskNotificationSender)
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
	id        TaskID
	commandID TaskCommandID
	ctx       context.Context

	notifierID string

	notificationOfRequestResult bool
}

//
// TaskRunner
//
type TaskRunner interface {
	TaskRun(id TaskID, commandID TaskCommandID, notifierID string, notificationOfRequestResult bool) (succeeded bool)
	TaskRunWithContext(id TaskID, commandID TaskCommandID, ctx context.Context, notifierID string, notificationOfRequestResult bool) (succeeded bool)
	TaskCancel(instanceID TaskInstanceID) (succeeded bool)
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
			// @@@@@
			////////////////////////////////////
			if validTaskCommand(TaskID(taskRunData.id), TaskCommandID(taskRunData.commandID)) == false {
				// @@@@@id, commnadid에대한 유효성 체크
				m := fmt.Sprintf("'%s::%s'는 등록되지 않은 Task입니다.", taskRunData.id, taskRunData.commandID)

				log.Error(m)
				s.taskNotificationSender.Notify(taskRunData.notifierID, m, taskRunData.ctx)

				continue
			}

			var instanceId TaskInstanceID

			s.runningMu.Lock()
			for {
				instanceId = s.taskInstanceIDGenerator.New()
				if _, exists := s.taskHandlers[instanceId]; exists == false {
					break
				}
			}
			s.runningMu.Unlock()

			log.Debugf("새로운 '%s::%s' Task 실행 요청 수신(TaskInstanceID:%d)", taskRunData.id, taskRunData.commandID, instanceId)

			// @@@@@
			a := supportedTasks[TaskID(taskRunData.id)]
			var h = a.newTaskFunc(instanceId, taskRunData)
			if h == nil {
				m := fmt.Sprintf("'%s::%s'는 등록되지 않은 Task입니다.", taskRunData.id, taskRunData.commandID)

				log.Error(m)
				s.taskNotificationSender.Notify(taskRunData.notifierID, m, taskRunData.ctx)

				continue
			}

			s.runningMu.Lock()
			s.taskHandlers[instanceId] = h
			s.runningMu.Unlock()

			s.taskStopWaiter.Add(1)
			go h.Run(s.taskNotificationSender, s.taskStopWaiter, s.taskDoneC)

			if taskRunData.notificationOfRequestResult == true {
				// @@@@@
				taskRunData.ctx = context.WithValue(taskRunData.ctx, "cancelInstanceId", instanceId)

				// @@@@@ 스케쥴러 또는 텔레그램에서 요청된 메시지, 스케쥴러에서 요청된 메시지의 응답을 보내야 할까?
				// @@@@@ 취소 instanceid가 넘어가야됨, 여기서 /cancel 을 추가하면 안됨 notifier에서 추가해야됨
				s.taskNotificationSender.Notify(taskRunData.notifierID, "작업 진행중입니다. 잠시만 기다려 주세요.\n/cancel_xxx", taskRunData.ctx)
			}
			////////////////////////////////////

		case instanceId := <-s.taskDoneC:
			// @@@@@
			////////////////////////////////////
			s.runningMu.Lock()
			if taskHandler, exists := s.taskHandlers[instanceId]; exists == true {
				log.Debugf("'%s::%s' Task의 작업이 완료되었습니다.(TaskInstanceID:%d)", taskHandler.ID(), taskHandler.CommandID(), instanceId)

				delete(s.taskHandlers, instanceId)
			} else {
				log.Warnf("등록되지 않은 Task에 대한 작업완료 메시지가 수신되었습니다.(TaskInstanceID:%d)", instanceId)
			}
			s.runningMu.Unlock()
			////////////////////////////////////

		case instanceId := <-s.taskCancelC:
			// @@@@@
			////////////////////////////////////
			s.runningMu.Lock()
			// @@@@@ notify 취소요청되었다는 메시지를 보낸다.
			if taskHandler, exists := s.taskHandlers[instanceId]; exists == true {
				log.Debugf("'%s::%s' Task의 작업이 취소되었습니다.(TaskInstanceID:%d)", taskHandler.ID(), taskHandler.CommandID(), instanceId)

				taskHandler.Cancel()
			} else {
				log.Warnf("등록되지 않은 Task에 대한 작업취소요청 메시지가 수신되었습니다.(TaskInstanceID:%d)", instanceId)
			}
			s.runningMu.Unlock()
			////////////////////////////////////

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

func (s *TaskService) TaskRun(id TaskID, commandID TaskCommandID, notifierID string, notificationOfRequestResult bool) (succeeded bool) {
	return s.TaskRunWithContext(id, commandID, nil, notifierID, notificationOfRequestResult)
}

func (s *TaskService) TaskRunWithContext(id TaskID, commandID TaskCommandID, ctx context.Context, notifierID string, notificationOfRequestResult bool) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("'%s::%s' Task 실행 요청중에 panic이 발생하였습니다.(panic:%s", id, commandID, r)
		}
	}()

	s.taskRunC <- &taskRunData{
		id:        id,
		commandID: commandID,
		ctx:       ctx,

		notifierID: notifierID,

		notificationOfRequestResult: notificationOfRequestResult,
	}

	return true
}

func (s *TaskService) TaskCancel(instanceID TaskInstanceID) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("Task 취소 요청중에 panic이 발생하였습니다.(TaskInstanceID:%s, panic:%s", instanceID, r)
		}
	}()

	s.taskCancelC <- instanceID

	return true
}

func (s *TaskService) SetTaskNotificationSender(taskNotificiationSender TaskNotificationSender) {
	s.taskNotificationSender = taskNotificiationSender
}
