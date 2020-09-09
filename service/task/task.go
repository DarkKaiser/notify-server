package task

import (
	"context"
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service"
	"github.com/darkkaiser/notify-server/service/notify"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
)

type TaskID string
type TaskCommandID string
type TaskInstanceID uint64

type taskInstanceIDGenerator struct {
	id TaskInstanceID
}

func (g *taskInstanceIDGenerator) New() TaskInstanceID {
	return TaskInstanceID(atomic.AddUint64((*uint64)(&g.id), 1))
}

// @@@@@ 맵으로 바꿔서 지원가능한 명령 목록을 생성해두는건 어떤지...
var taskList = make(map[TaskID][]TaskCommandID)
var taskList1 = make(map[TaskID]struct {
	a []TaskCommandID
	b func(instanceId TaskInstanceID, taskRunData *taskRunData)
})

// 지원가능한 Task 및 Command
// @@@@@ 외부 프로그램에 대한 처리도 할것(웹서비스를 통한 수행)
const (
//	TidAlganicMall TaskId = "ALGANICMALL" // 엘가닉몰(http://www.alganicmall.com/)

//	TcidAlganicMallWatchNewEvents TaskCommandId = "WatchNewEvents" // 엘가닉몰 신규 이벤트 감시
)

// @@@@@
func init() {
	//	taskList[TidAlganicMall] = append(taskList[TidAlganicMall], TcidAlganicMallWatchNewEvents)
}

// @@@@@
func validTaskCommand(taskID TaskID, commandID TaskCommandID) bool {
	ids, exists := taskList[taskID]
	if exists == false {
		return false
	}

	for _, id := range ids {
		if id == commandID {
			return true
		}
	}

	return false
}

type task struct {
	id         TaskID
	commandID  TaskCommandID
	instanceID TaskInstanceID

	notifierID  notify.NotifierID
	notifierCtx context.Context

	cancel bool

	runFunc func(notify.NotificationSender)
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

func (t *task) NotifierID() notify.NotifierID {
	return t.notifierID
}

func (t *task) NotifierContext() context.Context {
	return t.notifierCtx
}

func (t *task) Run(notificationSender notify.NotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceID) {
	defer taskStopWaiter.Done()
	defer func() {
		taskDoneC <- t.instanceID
	}()

	if t.runFunc == nil {
		log.Panicf("'%s::%s' Task 객체의 runFunc이 할당되지 않았습니다.", t.ID(), t.CommandID())
	}

	t.runFunc(notificationSender)
}

func (t *task) Cancel() {
	t.cancel = true
}

type taskHandler interface {
	ID() TaskID
	CommandID() TaskCommandID
	InstanceID() TaskInstanceID

	NotifierID() notify.NotifierID
	NotifierContext() context.Context

	Run(notificationSender notify.NotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceID)

	Cancel()
}

type taskRunData struct {
	id        string
	commandID string

	notifierID  notify.NotifierID
	notifierCtx context.Context

	notifyResultOfTaskRunRequest bool
}

type TaskRunner interface {
	TaskRun(id string, commandID string, notifierID notify.NotifierID, notifyResultOfTaskRunRequest bool) (succeeded bool)
	TaskRunWithContext(id string, commandID string, notifierID notify.NotifierID, notifierCtx context.Context, notifyResultOfTaskRunRequest bool) (succeeded bool)
}

type taskService struct {
	config *g.AppConfig

	running   bool
	runningMu sync.Mutex

	scheduler scheduler

	taskHandlers map[TaskInstanceID]taskHandler

	taskInstanceIDGenerator taskInstanceIDGenerator

	taskRunC    chan *taskRunData
	taskDoneC   chan TaskInstanceID
	taskCancelC chan TaskInstanceID

	taskStopWaiter *sync.WaitGroup

	notificationSender notify.NotificationSender
}

func NewService(config *g.AppConfig) service.Service {
	return &taskService{
		config: config,

		running:   false,
		runningMu: sync.Mutex{},

		scheduler: scheduler{},

		taskHandlers: make(map[TaskInstanceID]taskHandler),

		taskInstanceIDGenerator: taskInstanceIDGenerator{id: 0},

		taskRunC:    make(chan *taskRunData, 10),
		taskDoneC:   make(chan TaskInstanceID, 10),
		taskCancelC: make(chan TaskInstanceID, 10),

		taskStopWaiter: &sync.WaitGroup{},

		notificationSender: nil,
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

	// NotificationSender 객체를 구한다.
	if o := valueCtx.Value("notify.notification_sender"); o != nil {
		r, ok := o.(notify.NotificationSender)
		if ok == false {
			log.Panicf("NotificationSender 객체를 구할 수 없습니다.")
		}
		s.notificationSender = r
	} else {
		log.Panicf("NotificationSender 객체를 구할 수 없습니다.")
	}

	// Task 스케쥴러를 시작한다.
	s.scheduler.Start(s.config, s, s.notificationSender)

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	log.Debug("Task 서비스 시작됨")
}

func (s *taskService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	for {
		select {
		case taskRunData := <-s.taskRunC:
			// @@@@@id, commnadid에대한 유효성 체크, mapm으로 만들어져 있어야됨
			////////////////////////////////////
			if validTaskCommand(TaskID(taskRunData.id), TaskCommandID(taskRunData.commandID)) == false {

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

			var h taskHandler
			switch TaskID(taskRunData.id) {
			case TidAlganicMall:
				h = newAlganicMallTask(instanceId, taskRunData)

			default:
				m := fmt.Sprintf("'%s::%s'는 등록되지 않은 Task입니다.", taskRunData.id, taskRunData.commandID)

				log.Error(m)
				s.notificationSender.Notify(taskRunData.notifierID, taskRunData.notifierCtx, m)

				continue
			}

			s.runningMu.Lock()
			s.taskHandlers[instanceId] = h
			s.runningMu.Unlock()

			s.taskStopWaiter.Add(1)
			go h.Run(s.notificationSender, s.taskStopWaiter, s.taskDoneC)

			if taskRunData.notifyResultOfTaskRunRequest == true {
				// @@@@@
				taskRunData.notifierCtx = context.WithValue(taskRunData.notifierCtx, "cancelInstanceId", instanceId)

				// @@@@@ 스케쥴러 또는 텔레그램에서 요청된 메시지, 스케쥴러에서 요청된 메시지의 응답을 보내야 할까?
				// @@@@@ 취소 instanceid가 넘어가야됨, 여기서 /cancel 을 추가하면 안됨 notifier에서 추가해야됨
				s.notificationSender.Notify(taskRunData.notifierID, taskRunData.notifierCtx, "작업 진행중입니다. 잠시만 기다려 주세요.\n/cancel_xxx")
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
			s.notificationSender = nil //@@@@@
			s.runningMu.Unlock()
			/////////////////

			log.Debug("Task 서비스 중지됨")

			return
		}
	}
}

func (s *taskService) TaskRun(id string, commandID string, notifierID notify.NotifierID, notifyResultOfTaskRunRequest bool) (succeeded bool) {
	return s.TaskRunWithContext(id, commandID, notifierID, nil, notifyResultOfTaskRunRequest)
}

func (s *taskService) TaskRunWithContext(id string, commandID string, notifierID notify.NotifierID, notifierCtx context.Context, notifyResultOfTaskRunRequest bool) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("'%s::%s' Task 실행 요청중에 panic이 발생하였습니다.(panic:%s", id, commandID, r)
		}
	}()

	s.taskRunC <- &taskRunData{
		id:        id,
		commandID: commandID,

		notifierID:  notifierID,
		notifierCtx: notifierCtx,

		notifyResultOfTaskRunRequest: notifyResultOfTaskRunRequest,
	}

	return true
}

func (s *taskService) TaskCancel(instanceID uint64) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("Task 취소 요청중에 panic이 발생하였습니다.(TaskInstanceID:%s, panic:%s", instanceID, r)
		}
	}()

	s.taskCancelC <- TaskInstanceID(instanceID)

	return true
}
