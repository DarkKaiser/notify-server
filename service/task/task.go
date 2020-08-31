package task

import (
	"context"
	"github.com/darkkaiser/notify-server/global"
	"github.com/darkkaiser/notify-server/service"
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

	cancel bool

	ctx context.Context
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

// @@@@@ setcancel???
func (t *task) Cancel() {
	t.cancel = true
}

func (t *task) Context() context.Context {
	return t.ctx
}

type TaskRunRequester interface {
	TaskRun(id TaskId, commandId TaskCommandId) (succeeded bool)
	TaskRunWithContext(id TaskId, commandId TaskCommandId, ctx context.Context) (succeeded bool)
}

type taskRunData struct {
	id        TaskId
	commandId TaskCommandId
	ctx       context.Context
}

// @@@@@
type taskHandler interface {
	Id() TaskId
	CommandId() TaskCommandId
	InstanceId() TaskInstanceId
	Context() context.Context

	Run(taskStopWaiter *sync.WaitGroup, taskDone chan<- TaskInstanceId)
	Cancel()
}

type taskService struct {
	config *global.AppConfig

	running   bool
	runningMu sync.Mutex

	scheduler scheduler

	taskInstanceIdGenerator taskInstanceIdGenerator

	taskRunRequestC chan *taskRunData

	// @@@@@
	taskHandlers   map[TaskInstanceId]taskHandler
	cancelChan     chan *struct{}
	taskDone       chan TaskInstanceId
	taskStopWaiter *sync.WaitGroup
}

func NewTaskService(config *global.AppConfig) service.Service {
	return &taskService{
		config: config,

		running:   false,
		runningMu: sync.Mutex{},

		scheduler: scheduler{},

		taskInstanceIdGenerator: taskInstanceIdGenerator{id: 0},

		taskRunRequestC: make(chan *taskRunData, 10),

		// @@@@@
		taskHandlers:   make(map[TaskInstanceId]taskHandler),
		cancelChan:     make(chan *struct{}, 10),
		taskDone:       make(chan TaskInstanceId, 10),
		taskStopWaiter: &sync.WaitGroup{},
	}
}

func (s *taskService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Debug("Task 서비스 시작중...")

	if s.running == true {
		defer serviceStopWaiter.Done()

		log.Warn("Task 서비스가 이미 시작됨!!!")

		return
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
			log.Debugf("새 Task 실행 요청 수신(TaskId:%s, CommandId:%s)", taskRunData.id, taskRunData.commandId)

			switch taskRunData.id {
			case TidAlganicMall:
				instanceId := s.taskInstanceIdGenerator.New()
				h := newAlganicMallTask(instanceId, taskRunData)

				// @@@@@
				s.runningMu.Lock()
				// 이미 동일한 id가 있다면...??
				if _, exists := s.taskHandlers[instanceId]; exists == true {
					s.runningMu.Unlock()
					// error
					continue
				}
				s.taskHandlers[instanceId] = h
				s.runningMu.Unlock()

				s.taskStopWaiter.Add(1)
				go h.Run(s.taskStopWaiter, s.taskDone)

			default:
				// @@@@@ notify
				log.Errorf("등록되지 않은 Task 실행 요청이 수신되었습니다(TaskId:%s, CommandId:%s)", taskRunData.id, taskRunData.commandId)
			}

			// @@@@@ task 작업 완료
		case id2 := <-s.taskDone:
			// @@@@@ mutex lock???
			//log.Info("##### 완료 task 수신됨: " + strconv.Itoa(id2))
			// @@@@@ 메시지도 수신받아서 notifyserver로 보내기, 이때 유효한 task인지 체크도 함
			//				handler := s.taskHandlers[newId]
			//ctx2 := handler.Context()
			//notifyserverChan<- struct {
			//				message:
			//					ctx : ctx2
			//				}
			delete(s.taskHandlers, id2)

			// @@@@@ notifier로부터 취소 명령이 들어온경우
		case <-s.cancelChan:
			// @@@@@ mutex lock???
			taskHandler := s.taskHandlers[0]
			taskHandler.Cancel()
			delete(s.taskHandlers, 0)
			// @@@@@ 해당 task만 취소되어야됨

		case <-serviceStopCtx.Done():
			log.Debug("Task 서비스 중지중...")

			// Task 스케쥴러를 중지한다.
			s.scheduler.Stop()

			// @@@@@ 아래 블럭도 mutex를 감싸야하나?
			/////////////////
			// @@@@@ mutex lock???
			for _, handler := range s.taskHandlers {
				handler.Cancel()
			}
			close(s.taskRunRequestC)
			close(s.cancelChan)
			close(s.taskDone)
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
