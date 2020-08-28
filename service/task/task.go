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
	ctx        context.Context //@@@@@
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

// @@@@@
func (t *task) Context() context.Context {
	return t.ctx
}

type TaskRunRequester interface {
	TaskRun(id TaskId, commandId TaskCommandId) (succeeded bool)
}

type taskRunData struct {
	id        TaskId
	commandId TaskCommandId
	ctx       context.Context // @@@@@
}

// @@@@@
type TaskHandler interface {
	InstanceId() TaskInstanceId
	Run()
	Cancel()
	Context() context.Context
}

type taskService struct {
	config *global.AppConfig

	serviceCtx        context.Context
	serviceStopWaiter *sync.WaitGroup

	running   bool
	runningMu sync.Mutex

	scheduler scheduler

	taskInstanceIdGenerator taskInstanceIdGenerator

	taskRunRequestC chan *taskRunData

	// @@@@@
	RunningTasks   map[TaskInstanceId]TaskHandler
	cancelChan     chan *struct{}
	taskcancelChan chan *struct{}
	taskdone       chan TaskInstanceId
	twg            *sync.WaitGroup
}

func NewTaskService(config *global.AppConfig, serviceCtx context.Context, serviceStopWaiter *sync.WaitGroup) service.Service {
	return &taskService{
		config: config,

		serviceCtx:        serviceCtx,
		serviceStopWaiter: serviceStopWaiter,

		running:   false,
		runningMu: sync.Mutex{},

		scheduler: scheduler{},

		taskInstanceIdGenerator: taskInstanceIdGenerator{id: 0},

		taskRunRequestC: make(chan *taskRunData, 10),

		// @@@@@
		RunningTasks:   make(map[TaskInstanceId]TaskHandler),
		cancelChan:     make(chan *struct{}, 10),
		taskdone:       make(chan TaskInstanceId, 10),
		twg:            &sync.WaitGroup{},
		taskcancelChan: make(chan *struct{}, 10),
	}
}

func (s *taskService) Run() {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running == true {
		return
	}

	s.serviceStopWaiter.Add(1)
	go func() {
		defer s.serviceStopWaiter.Done()

		s._run_()
	}()

	s.running = true
}

func (s *taskService) _run_() {
	// Task 스케쥴러를 시작한다.
	s.scheduler.Start(s.config, s)

	for {
		select {
		case taskRunData := <-s.taskRunRequestC:
			log.Debugf("새로운 Task 실행 요청이 수신되었습니다(TaskId:%s, CommandId:%s)", taskRunData.id, taskRunData.commandId)

			switch taskRunData.id {
			case TidAlganicMall:
				// @@@@@
				s.twg.Add(1)
				addUint64 := s.taskInstanceIdGenerator.New()
				// addUint64 := TaskInstanceId(atomic.AddUint64((*uint64)(&latestInstanceId), 1))
				taskHandler, err := newAlganicMallTask(addUint64, taskRunData, taskRunData.commandId, s.twg, s.taskcancelChan, s.taskdone, taskRunData.ctx)
				println(err)
				// @@@@@ task 실행중 취소하는 방법은?
				s.RunningTasks[addUint64] = taskHandler
				go taskHandler.Run()

			default:
				log.Errorf("등록되지 않은 Task 실행 요청이 수신되었습니다(TaskId:%s, CommandId:%s)", taskRunData.id, taskRunData.commandId)
			}

			// @@@@@
		case id2 := <-s.taskdone:
			//log.Info("##### 완료 task 수신됨: " + strconv.Itoa(id2))
			// @@@@@ 메시지도 수신받아서 notifyserver로 보내기, 이때 유효한 task인지 체크도 함
			//				handler := s.RunningTasks[newId]
			//ctx2 := handler.Context()
			//notifyserverChan<- struct {
			//				message:
			//					ctx : ctx2
			//				}
			delete(s.RunningTasks, id2)

			// @@@@@
		case <-s.cancelChan:
			taskHandler := s.RunningTasks[0]
			taskHandler.Cancel()
			delete(s.RunningTasks, 0)
			// @@@@@ 해당 task만 취소되어야됨

		case <-s.serviceCtx.Done():
			log.Debug("Task 서비스를 중지합니다.")

			// Task 스케쥴러를 중지한다.
			s.scheduler.Stop()

			// @@@@@
			/////////////////
			close(s.taskRunRequestC)
			close(s.cancelChan)
			close(s.taskcancelChan)
			s.twg.Wait()
			/////////////////

			s.runningMu.Lock()
			s.running = false
			s.runningMu.Unlock()

			log.Debug("Task 서비스가 중지되었습니다.")

			return
		}
	}
}

func (s *taskService) TaskRun(id TaskId, commandId TaskCommandId) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("Task 실행 요청중에 panic이 발생하였습니다.(TaskId:%s, TaskCommandId:%s, panic:%s", id, commandId, r)
		}
	}()

	s.taskRunRequestC <- &taskRunData{
		id:        id,
		commandId: commandId,
	}

	return true
}
