package service

import (
	"context"
	"fmt"
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

	if t.runFunc == nil {
		log.Panicf("'%s::%s' Task 객체의 runFunc이 할당되지 않았습니다.", t.id, t.commandId)
	}

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

	// @@@@@
	response bool
}

type TaskRunner interface {
	TaskRun(id TaskId, commandId TaskCommandId, notifierId NotifierId, response bool) (succeeded bool)
	TaskRunWithContext(id TaskId, commandId TaskCommandId, notifierId NotifierId, notifierCtx context.Context, response bool) (succeeded bool)
	TaskCancel(id TaskInstanceId) (succeeded bool)
}

type taskService struct {
	config *global.AppConfig

	running   bool
	runningMu sync.Mutex

	scheduler taskScheduler

	taskHandlers map[TaskInstanceId]taskHandler

	taskInstanceIdGenerator taskInstanceIdGenerator

	taskRunC    chan *taskRunData
	taskDoneC   chan TaskInstanceId
	taskCancelC chan TaskInstanceId

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

		taskRunC:    make(chan *taskRunData, 10),
		taskDoneC:   make(chan TaskInstanceId, 10),
		taskCancelC: make(chan TaskInstanceId, 10),

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
	s.scheduler.Start(s.config, s, s.notifySender)

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	log.Debug("Task 서비스 시작됨")
}

func (s *taskService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	for {
		select {
		case taskRunData := <-s.taskRunC:
			var instanceId TaskInstanceId

			s.runningMu.Lock()
			for {
				instanceId = s.taskInstanceIdGenerator.New()
				if _, exists := s.taskHandlers[instanceId]; exists == false {
					break
				}
			}
			s.runningMu.Unlock()

			log.Debugf("새로운 '%s::%s' Task 실행 요청 수신(TaskInstanceID:%d)", taskRunData.id, taskRunData.commandId, instanceId)

			var h taskHandler
			switch taskRunData.id {
			case TidAlganicMall:
				h = newAlganicMallTask(instanceId, taskRunData)

			default:
				m := fmt.Sprintf("'%s::%s'는 등록되지 않은 Task입니다.", taskRunData.id, taskRunData.commandId)

				log.Error(m)
				s.notifySender.Notify(taskRunData.notifierId, taskRunData.notifierCtx, m)

				continue
			}

			s.runningMu.Lock()
			s.taskHandlers[instanceId] = h
			s.runningMu.Unlock()

			s.taskStopWaiter.Add(1)
			go h.Run(s.notifySender, s.taskStopWaiter, s.taskDoneC)

			if taskRunData.response == true {
				// @@@@@
				taskRunData.notifierCtx = context.WithValue(taskRunData.notifierCtx, "cancelInstanceId", instanceId)

				// @@@@@ 스케쥴러 또는 텔레그램에서 요청된 메시지, 스케쥴러에서 요청된 메시지의 응답을 보내야 할까?
				// @@@@@ 취소 instanceid가 넘어가야됨, 여기서 /cancel 을 추가하면 안됨 notifier에서 추가해야됨
				s.notifySender.Notify(taskRunData.notifierId, taskRunData.notifierCtx, "작업 진행중입니다. 잠시만 기다려 주세요.\n/cancel_xxx")
			}

		case instanceId := <-s.taskDoneC:
			s.runningMu.Lock()
			if taskHandler, exists := s.taskHandlers[instanceId]; exists == true {
				log.Debugf("'%s::%s' Task의 작업이 완료되었습니다.(TaskInstanceID:%d)", taskHandler.Id(), taskHandler.CommandId(), instanceId)

				delete(s.taskHandlers, instanceId)
			} else {
				log.Warnf("등록되지 않은 Task에 대한 작업완료 메시지가 수신되었습니다.(TaskInstanceID:%d)", instanceId)
			}
			s.runningMu.Unlock()

		case instanceId := <-s.taskCancelC:
			s.runningMu.Lock()
			// @@@@@ notify 취소요청되었다는 메시지를 보낸다.
			if taskHandler, exists := s.taskHandlers[instanceId]; exists == true {
				log.Debugf("'%s::%s' Task의 작업이 취소되었습니다.(TaskInstanceID:%d)", taskHandler.Id(), taskHandler.CommandId(), instanceId)

				taskHandler.Cancel()
			} else {
				log.Warnf("등록되지 않은 Task에 대한 작업취소요청 메시지가 수신되었습니다.(TaskInstanceID:%d)", instanceId)
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

func (s *taskService) TaskRun(id TaskId, commandId TaskCommandId, notifierId NotifierId, response bool) (succeeded bool) {
	return s.TaskRunWithContext(id, commandId, notifierId, nil, response)
}

func (s *taskService) TaskRunWithContext(id TaskId, commandId TaskCommandId, notifierId NotifierId, notifierCtx context.Context, response bool) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("'%s::%s' Task 실행 요청중에 panic이 발생하였습니다.(panic:%s", id, commandId, r)
		}
	}()

	s.taskRunC <- &taskRunData{
		id:        id,
		commandId: commandId,

		notifierId:  notifierId,
		notifierCtx: notifierCtx,

		// @@@@@
		response: response,
	}

	return true
}

func (s *taskService) TaskCancel(id TaskInstanceId) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("Task 취소 요청중에 panic이 발생하였습니다.(TaskInstanceID:%s, panic:%s", id, r)
		}
	}()

	s.taskCancelC <- id

	return true
}
