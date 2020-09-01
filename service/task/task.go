package task

import (
	"context"
	"github.com/darkkaiser/notify-server/global"
	"github.com/darkkaiser/notify-server/service"
	"github.com/darkkaiser/notify-server/service/notify"
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
	// 엘가닉몰
	TcidAlganicMallWatchNewEvents TaskCommandId = "WatchNewEvents"
)

type task struct {
	id         TaskId
	commandId  TaskCommandId
	instanceId TaskInstanceId

	ctx context.Context

	cancel bool
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

func (t *task) Context() context.Context {
	return t.ctx
}

func (t *task) Cancel() {
	t.cancel = true
}

type taskHandler interface {
	Id() TaskId
	CommandId() TaskCommandId
	InstanceId() TaskInstanceId

	Context() context.Context

	Cancel()

	Run(r notify.NotifyRequester, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceId)
}

type taskRunData struct {
	id        TaskId
	commandId TaskCommandId
	ctx       context.Context
}

type TaskRunRequester interface {
	TaskRun(id TaskId, commandId TaskCommandId) (succeeded bool)
	TaskRunWithContext(id TaskId, commandId TaskCommandId, ctx context.Context) (succeeded bool)

	TaskCancel(id TaskInstanceId) (succeeded bool)
}

type taskService struct {
	config *global.AppConfig

	running   bool
	runningMu sync.Mutex

	scheduler scheduler

	taskHandlers map[TaskInstanceId]taskHandler

	taskInstanceIdGenerator taskInstanceIdGenerator

	taskDoneC          chan TaskInstanceId
	taskRunRequestC    chan *taskRunData
	taskCancelRequestC chan TaskInstanceId

	taskStopWaiter *sync.WaitGroup

	notifyRequester notify.NotifyRequester
}

func NewTaskService(config *global.AppConfig) service.Service {
	return &taskService{
		config: config,

		running:   false,
		runningMu: sync.Mutex{},

		scheduler: scheduler{},

		taskHandlers: make(map[TaskInstanceId]taskHandler),

		taskInstanceIdGenerator: taskInstanceIdGenerator{id: 0},

		taskDoneC:          make(chan TaskInstanceId, 10),
		taskRunRequestC:    make(chan *taskRunData, 10),
		taskCancelRequestC: make(chan TaskInstanceId, 10),

		taskStopWaiter: &sync.WaitGroup{},

		notifyRequester: nil,
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

	// NotifyRequester 객체를 구한다.
	if o := valueCtx.Value("NotifyRequester"); o != nil {
		r, ok := o.(notify.NotifyRequester)
		if ok == false {
			log.Panicf("NotifyRequester 객체를 구할 수 없습니다.")
		}
		s.notifyRequester = r
	} else {
		log.Panicf("NotifyRequester 객체를 구할 수 없습니다.")
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
				// @@@@@ notify
				log.Errorf("등록되지 않은 Task 실행 요청이 수신되었습니다(TaskId:%s, CommandId:%s)", taskRunData.id, taskRunData.commandId)

				continue
			}

			s.runningMu.Lock()
			s.taskHandlers[instanceId] = h
			s.runningMu.Unlock()

			s.taskStopWaiter.Add(1)
			go h.Run(s.notifyRequester, s.taskStopWaiter, s.taskDoneC)

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
			// @@@@@ cancel일때 여기서 삭제하지 않고 task에서 done을 날려서 거기서 삭제하는건??? 삭제하는게 여러군데 있음..
			s.runningMu.Lock()
			if taskHandler, exists := s.taskHandlers[instanceId]; exists == true {
				log.Debugf("Task 작업이 취소되었습니다.(TaskId:%s, CommandId:%s, InstanceId:%d)", taskHandler.Id(), taskHandler.CommandId(), instanceId)

				taskHandler.Cancel()
				delete(s.taskHandlers, instanceId)
			} else {
				log.Warnf("등록되지 않은 Task 취소 요청이 수신되었습니다.(InstanceId:%d)", instanceId)
			}
			s.runningMu.Unlock()

		case <-serviceStopCtx.Done():
			log.Debug("Task 서비스 중지중...")

			// Task 스케쥴러를 중지한다.
			s.scheduler.Stop()

			// @@@@@ 아래 블럭도 mutex를 감싸야하나?
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
			s.runningMu.Unlock()

			log.Debug("Task 서비스 중지됨")

			return
		}
	}
}

func (s *taskService) TaskRun(id TaskId, commandId TaskCommandId) (succeeded bool) {
	return s.TaskRunWithContext(id, commandId, nil)
}

func (s *taskService) TaskRunWithContext(id TaskId, commandId TaskCommandId, ctx context.Context) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("Task 실행 요청중에 panic이 발생하였습니다.(TaskId:%s, TaskCommandId:%s, panic:%s", id, commandId, r)
		}
	}()

	s.taskRunRequestC <- &taskRunData{
		id:        id,
		commandId: commandId,
		ctx:       ctx,
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
