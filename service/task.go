package service

import (
	"context"
	"github.com/darkkaiser/notify-server/global"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
)

type TaskId string
type TaskCommandId string
type TaskInstanceId uint32

type taskInstanceIdGenerator struct {
	id TaskInstanceId
}

func (g *taskInstanceIdGenerator) New() TaskInstanceId {
	return TaskInstanceId(atomic.AddUint32((*uint32)(&g.id), 1))
}

const (
	TidAlganicMall TaskId = "ALGANICMALL" // 엘가닉몰(http://www.alganicmall.com/)
)

const (
	TcidAlganicMallWatchNewEvents TaskCommandId = "WatchNewEvents" // 엘가닉몰 신규 이벤트 감시
)

type task struct {
	id         TaskId
	commandId  TaskCommandId
	instanceId TaskInstanceId

	notifierId  NotifierId
	notifierCtx context.Context

	cancel bool

	runFunc func(sender NotifySender)
}

func (t *task) Id() TaskId {
	return t.id
}

func (t *task) CommandId() TaskCommandId {
	return t.commandId
}

func (t *task) InstanceId() TaskInstanceId {
	return t.instanceId
}

func (t *task) NotifierId() NotifierId {
	return t.notifierId
}

func (t *task) NotifierContext() context.Context {
	return t.notifierCtx
}

func (t *task) Run(sender NotifySender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceId) {
	defer taskStopWaiter.Done()
	defer func() {
		taskDoneC <- t.instanceId
	}()

	t.runFunc(sender)
}

func (t *task) Cancel() {
	t.cancel = true
}

type taskHandler interface {
	Id() TaskId
	CommandId() TaskCommandId
	InstanceId() TaskInstanceId

	NotifierId() NotifierId
	NotifierContext() context.Context

	Run(sender NotifySender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceId)

	Cancel()
}

type taskRunData struct {
	id        TaskId
	commandId TaskCommandId

	notifierId  NotifierId
	notifierCtx context.Context
}

type TaskRunner interface {
	TaskRun(id TaskId, commandId TaskCommandId, notifierId NotifierId) (succeeded bool)
	TaskRunWithContext(id TaskId, commandId TaskCommandId, notifierId NotifierId, notifierCtx context.Context) (succeeded bool)
	TaskCancel(id TaskInstanceId) (succeeded bool)
}

type taskService struct {
	config *global.AppConfig

	running   bool
	runningMu sync.Mutex

	scheduler taskScheduler

	taskHandlers map[TaskInstanceId]taskHandler

	taskInstanceIdGenerator taskInstanceIdGenerator

	taskDoneC          chan TaskInstanceId
	taskRunRequestC    chan *taskRunData
	taskCancelRequestC chan TaskInstanceId

	taskStopWaiter *sync.WaitGroup

	notifySender NotifySender
}

func NewTaskService(config *global.AppConfig) Service {
	return &taskService{
		config: config,

		running:   false,
		runningMu: sync.Mutex{},

		scheduler: taskScheduler{},

		taskHandlers: make(map[TaskInstanceId]taskHandler),

		taskInstanceIdGenerator: taskInstanceIdGenerator{id: 0},

		taskDoneC:          make(chan TaskInstanceId, 10),
		taskRunRequestC:    make(chan *taskRunData, 10),
		taskCancelRequestC: make(chan TaskInstanceId, 10),

		taskStopWaiter: &sync.WaitGroup{},

		notifySender: nil,
	}
}

func (s *taskService) Run(valueCtx context.Context, serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Debug("Task 서비스 시작중...")

	if s.running == true {
		defer serviceStopWaiter.Done()

		log.Warn("Task 서비스가 이미 시작됨!!!")

		return
	}

	// NotifySender 객체를 구한다.
	if o := valueCtx.Value("notifysender"); o != nil {
		r, ok := o.(NotifySender)
		if ok == false {
			log.Panicf("NotifySender 객체를 구할 수 없습니다.")
		}
		s.notifySender = r
	} else {
		log.Panicf("NotifySender 객체를 구할 수 없습니다.")
	}

	// Task 스케쥴러를 시작한다.
	s.scheduler.Start(s.config, s)

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	log.Debug("Task 서비스 시작됨")
}

func (s *taskService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	for {
		select {
		case taskRunData := <-s.taskRunRequestC:
			var instanceId TaskInstanceId

			s.runningMu.Lock()
			for {
				instanceId = s.taskInstanceIdGenerator.New()
				if _, exists := s.taskHandlers[instanceId]; exists == false {
					break
				}
			}
			s.runningMu.Unlock()

			log.Debugf("새 Task 실행 요청 수신(TaskId:%s, CommandId:%s, InstanceId:%d)", taskRunData.id, taskRunData.commandId, instanceId)

			var h taskHandler
			switch taskRunData.id {
			case TidAlganicMall:
				h = newAlganicMallTask(instanceId, taskRunData)

			default:
				log.Errorf("등록되지 않은 Task 실행 요청이 수신되었습니다(TaskId:%s, CommandId:%s)", taskRunData.id, taskRunData.commandId)
				// @@@@@ notify

				continue
			}

			s.runningMu.Lock()
			s.taskHandlers[instanceId] = h
			s.runningMu.Unlock()

			s.taskStopWaiter.Add(1)
			go h.Run(s.notifySender, s.taskStopWaiter, s.taskDoneC)

		case instanceId := <-s.taskDoneC:
			s.runningMu.Lock()
			if taskHandler, exists := s.taskHandlers[instanceId]; exists == true {
				log.Debugf("Task 작업이 완료되었습니다.(TaskId:%s, CommandId:%s, InstanceId:%d)", taskHandler.Id(), taskHandler.CommandId(), instanceId)

				delete(s.taskHandlers, instanceId)
			} else {
				log.Warnf("등록되지 않은 Task 작업 완료가 수신되었습니다.(InstanceId:%d)", instanceId)
			}
			s.runningMu.Unlock()

		case instanceId := <-s.taskCancelRequestC:
			// @@@@@ 테스트
			s.runningMu.Lock()
			if taskHandler, exists := s.taskHandlers[instanceId]; exists == true {
				log.Debugf("Task 작업이 취소되었습니다.(TaskId:%s, CommandId:%s, InstanceId:%d)", taskHandler.Id(), taskHandler.CommandId(), instanceId)

				taskHandler.Cancel()
			} else {
				log.Warnf("등록되지 않은 Task 취소 요청이 수신되었습니다.(InstanceId:%d)", instanceId)
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
			close(s.taskRunRequestC)
			close(s.taskCancelRequestC)
			close(s.taskDoneC)
			s.taskStopWaiter.Wait()
			/////////////////

			s.runningMu.Lock()
			s.running = false
			s.taskHandlers = nil
			s.notifySender = nil //@@@@@
			s.runningMu.Unlock()

			log.Debug("Task 서비스 중지됨")

			return
		}
	}
}

func (s *taskService) TaskRun(id TaskId, commandId TaskCommandId, notifierId NotifierId) (succeeded bool) {
	return s.TaskRunWithContext(id, commandId, notifierId, nil)
}

func (s *taskService) TaskRunWithContext(id TaskId, commandId TaskCommandId, notifierId NotifierId, notifierCtx context.Context) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("Task 실행 요청중에 panic이 발생하였습니다.(TaskId:%s, TaskCommandId:%s, panic:%s", id, commandId, r)
		}
	}()

	s.taskRunRequestC <- &taskRunData{
		id:        id,
		commandId: commandId,

		notifierId:  notifierId,
		notifierCtx: notifierCtx,
	}

	return true
}

func (s *taskService) TaskCancel(id TaskInstanceId) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("Task 취소 요청중에 panic이 발생하였습니다.(TaskInstanceId:%s, panic:%s", id, r)
		}
	}()

	s.taskCancelRequestC <- id

	return true
}