package task

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

const (
	defaultChannelBufferSize = 10

	msgTaskNotFound           = "등록되지 않은 작업입니다.😱"
	msgTaskRunning            = "작업 진행중입니다. 잠시만 기다려 주세요."
	msgTaskAlreadyRunning     = "요청하신 작업은 이미 진행중입니다.\n이전 작업을 취소하시려면 아래 명령어를 클릭하여 주세요."
	msgTaskCanceledByUser     = "사용자 요청에 의해 작업이 취소되었습니다."
	msgTaskCancelInfoNotFound = "해당 작업에 대한 정보를 찾을 수 없습니다.😱\n취소 요청이 실패하였습니다.(ID:%s)"
)

// Service
type Service struct {
	appConfig *config.AppConfig

	running   bool
	runningMu sync.Mutex

	scheduler scheduler

	handlers map[InstanceID]Handler

	instanceIDGenerator instanceIDGenerator

	notificationSender NotificationSender

	taskRunC    chan *RunRequest
	taskDoneC   chan InstanceID
	taskCancelC chan InstanceID

	taskStopWaiter *sync.WaitGroup

	storage TaskResultStorage
}

func NewService(appConfig *config.AppConfig) *Service {
	return &Service{
		appConfig: appConfig,

		running:   false,
		runningMu: sync.Mutex{},

		scheduler: scheduler{},

		handlers: make(map[InstanceID]Handler),

		instanceIDGenerator: instanceIDGenerator{},

		notificationSender: nil,

		taskRunC:    make(chan *RunRequest, defaultChannelBufferSize),
		taskDoneC:   make(chan InstanceID, defaultChannelBufferSize),
		taskCancelC: make(chan InstanceID, defaultChannelBufferSize),

		taskStopWaiter: &sync.WaitGroup{},

		storage: NewFileTaskResultStorage(config.AppName),
	}
}

func (s *Service) Start(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent("task.service").Info("Task 서비스 시작중...")

	// NotificationSender 검증
	if s.notificationSender == nil {
		defer serviceStopWaiter.Done()
		return apperrors.New(apperrors.ErrInternal, "NotificationSender 객체가 초기화되지 않았습니다")
	}

	if s.running {
		defer serviceStopWaiter.Done()
		applog.WithComponent("task.service").Warn("Task 서비스가 이미 시작됨!!!")
		return nil
	}

	// Task 스케쥴러를 시작한다.
	s.scheduler.Start(s.appConfig, s, s.notificationSender)

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	applog.WithComponent("task.service").Info("Task 서비스 시작됨")

	return nil
}

func (s *Service) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	for {
		select {
		case req := <-s.taskRunC:
			s.handleRunRequest(req)

		case instanceID := <-s.taskDoneC:
			s.handleTaskDone(instanceID)

		case instanceID := <-s.taskCancelC:
			s.handleTaskCancel(instanceID)

		case <-serviceStopCtx.Done():
			s.handleStop()
			return
		}
	}
}

func (s *Service) handleRunRequest(req *RunRequest) {
	applog.WithComponentAndFields("task.service", log.Fields{
		"task_id":    req.TaskID,
		"command_id": req.CommandID,
		"run_by":     req.RunBy,
	}).Debug("새로운 Task 실행 요청 수신")

	if req.TaskContext == nil {
		req.TaskContext = NewTaskContext()
	}
	req.TaskContext = req.TaskContext.WithTask(req.TaskID, req.CommandID)

	cfg, err := findConfig(req.TaskID, req.CommandID)
	if err != nil {
		m := msgTaskNotFound

		applog.WithComponentAndFields("task.service", log.Fields{
			"task_id":    req.TaskID,
			"command_id": req.CommandID,
			"error":      err,
		}).Error(m)

		go s.notificationSender.Notify(req.TaskContext.WithError(), req.NotifierID, m)

		return
	}

	// 인스턴스 중복 실행 확인 (Concurrency Control)
	if !cfg.Command.AllowMultiple {
		if s.checkConcurrencyLimit(req) {
			return
		}
	}

	s.createAndStartTask(req, cfg)
}

func (s *Service) checkConcurrencyLimit(req *RunRequest) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	var alreadyRunHandler Handler
	for _, handler := range s.handlers {
		if handler.GetID() == req.TaskID && handler.GetCommandID() == req.CommandID && !handler.IsCanceled() {
			alreadyRunHandler = handler
			break
		}
	}

	if alreadyRunHandler != nil {
		req.TaskContext = req.TaskContext.WithInstanceID(alreadyRunHandler.GetInstanceID(), alreadyRunHandler.ElapsedTimeAfterRun())
		go s.notificationSender.Notify(req.TaskContext, req.NotifierID, msgTaskAlreadyRunning)
		return true
	}

	return false
}

func (s *Service) createAndStartTask(req *RunRequest, cfg *ConfigLookup) {
	var instanceID InstanceID

	s.runningMu.Lock()
	for {
		instanceID = s.instanceIDGenerator.New()
		if _, exists := s.handlers[instanceID]; !exists {
			break
		}
	}
	s.runningMu.Unlock()

	h, err := cfg.Task.NewTask(instanceID, req, s.appConfig)
	if h == nil {
		applog.WithComponentAndFields("task.service", log.Fields{
			"task_id":    req.TaskID,
			"command_id": req.CommandID,
			"error":      err,
		}).Error(err)

		go s.notificationSender.Notify(req.TaskContext.WithError(), req.NotifierID, err.Error())

		return
	}

	// 생성된 Task에 Storage 주입
	// Handler 인터페이스를 통해 주입하므로 구체적인 타입을 알 필요가 없음
	h.SetStorage(s.storage)

	s.runningMu.Lock()
	s.handlers[instanceID] = h
	s.runningMu.Unlock()

	s.taskStopWaiter.Add(1)
	req.TaskContext = req.TaskContext.WithInstanceID(instanceID, 0)
	go h.Run(req.TaskContext, s.notificationSender, s.taskStopWaiter, s.taskDoneC)

	if req.NotifyOnStart {
		go s.notificationSender.Notify(req.TaskContext.WithInstanceID(instanceID, 0), req.NotifierID, msgTaskRunning)
	}
}

func (s *Service) handleTaskDone(instanceID InstanceID) {
	s.runningMu.Lock()
	if handler, exists := s.handlers[instanceID]; exists {
		applog.WithComponentAndFields("task.service", log.Fields{
			"task_id":     handler.GetID(),
			"command_id":  handler.GetCommandID(),
			"instance_id": instanceID,
		}).Debug("Task 작업 완료")

		delete(s.handlers, instanceID)
	} else {
		applog.WithComponentAndFields("task.service", log.Fields{
			"instance_id": instanceID,
		}).Warn("등록되지 않은 Task에 대한 작업완료 메시지 수신")
	}
	s.runningMu.Unlock()
}

func (s *Service) handleTaskCancel(instanceID InstanceID) {
	s.runningMu.Lock()
	if handler, exists := s.handlers[instanceID]; exists {
		handler.Cancel()

		applog.WithComponentAndFields("task.service", log.Fields{
			"task_id":     handler.GetID(),
			"command_id":  handler.GetCommandID(),
			"instance_id": instanceID,
		}).Debug("Task 작업 취소")

		go s.notificationSender.Notify(NewTaskContext().WithTask(handler.GetID(), handler.GetCommandID()), handler.GetNotifierID(), msgTaskCanceledByUser)
	} else {
		applog.WithComponentAndFields("task.service", log.Fields{
			"instance_id": instanceID,
		}).Warn("등록되지 않은 Task에 대한 작업취소 요청 메시지 수신")

		go s.notificationSender.NotifyDefault(fmt.Sprintf(msgTaskCancelInfoNotFound, instanceID))
	}
	s.runningMu.Unlock()
}

func (s *Service) handleStop() {
	applog.WithComponent("task.service").Info("Task 서비스 중지중...")

	// Task 스케쥴러를 중지한다.
	s.scheduler.Stop()

	s.runningMu.Lock()
	// 현재 작업중인 Task의 작업을 모두 취소한다.
	for _, handler := range s.handlers {
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
	s.handlers = nil
	s.notificationSender = nil
	s.runningMu.Unlock()

	applog.WithComponent("task.service").Info("Task 서비스 중지됨")
}

func (s *Service) Run(req *RunRequest) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = apperrors.New(apperrors.ErrInternal, fmt.Sprintf("Task 실행 요청중에 panic 발생: %v", r))

			applog.WithComponentAndFields("task.service", log.Fields{
				"task_id":    req.TaskID,
				"command_id": req.CommandID,
				"panic":      r,
			}).Error("Task 실행 요청중에 panic 발생")
		}
	}()

	s.taskRunC <- req

	return nil
}

func (s *Service) Cancel(instanceID InstanceID) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = apperrors.New(apperrors.ErrInternal, fmt.Sprintf("Task 취소 요청중에 panic 발생: %v", r))

			applog.WithComponentAndFields("task.service", log.Fields{
				"instance_id": instanceID,
				"panic":       r,
			}).Error("Task 취소 요청중에 panic 발생")
		}
	}()

	s.taskCancelC <- instanceID

	return nil
}

func (s *Service) SetNotificationSender(notificationSender NotificationSender) {
	s.notificationSender = notificationSender
}
