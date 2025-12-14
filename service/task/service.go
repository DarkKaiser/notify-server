package task

import (
	"context"
	"fmt"
	"sync"
	"time"

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

	taskSubmitC chan *SubmitRequest
	taskDoneC   chan InstanceID
	taskCancelC chan InstanceID

	taskStopWG *sync.WaitGroup

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

		taskSubmitC: make(chan *SubmitRequest, defaultChannelBufferSize),
		taskDoneC:   make(chan InstanceID, defaultChannelBufferSize),
		taskCancelC: make(chan InstanceID, defaultChannelBufferSize),

		taskStopWG: &sync.WaitGroup{},

		storage: NewFileTaskResultStorage(config.AppName),
	}
}

func (s *Service) SetNotificationSender(notificationSender NotificationSender) {
	s.notificationSender = notificationSender
}

func (s *Service) Start(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent("task.service").Info("Task 서비스 시작중...")

	// NotificationSender 검증
	if s.notificationSender == nil {
		serviceStopWG.Done()
		return apperrors.New(apperrors.ErrInternal, "NotificationSender 객체가 초기화되지 않았습니다")
	}

	if s.running {
		serviceStopWG.Done()
		applog.WithComponent("task.service").Warn("Task 서비스가 이미 시작됨!!!")
		return nil
	}

	go s.run0(serviceStopCtx, serviceStopWG)

	s.running = true

	// Task 스케쥴러를 시작한다.
	s.scheduler.Start(s.appConfig, s, s.notificationSender)

	applog.WithComponent("task.service").Info("Task 서비스 시작됨")

	return nil
}

func (s *Service) run0(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) {
	defer serviceStopWG.Done()

	defer func() {
		if r := recover(); r != nil {
			applog.WithComponentAndFields("task.service", log.Fields{
				"panic": r,
			}).Error("Critical: Task Service 메인 루프 Panic 발생")
		}
	}()

	for {
		select {
		case req, ok := <-s.taskSubmitC:
			if !ok {
				return
			}
			s.handleSubmitRequest(req)

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

func (s *Service) handleSubmitRequest(req *SubmitRequest) {
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

func (s *Service) checkConcurrencyLimit(req *SubmitRequest) bool {
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

func (s *Service) createAndStartTask(req *SubmitRequest, cfg *ConfigLookup) {
	// ID 생성을 락 밖에서 수행하여 Lock Holding Time을 최소화한다.
	var instanceID = s.instanceIDGenerator.New()

	s.runningMu.Lock()
	// ID 충돌(매우 희박) 발생 시에만 락 내부에서 재생성한다.
	if _, exists := s.handlers[instanceID]; exists {
		for {
			instanceID = s.instanceIDGenerator.New()
			if _, exists := s.handlers[instanceID]; !exists {
				break
			}
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

	s.taskStopWG.Add(1)
	req.TaskContext = req.TaskContext.WithInstanceID(instanceID, 0)
	go h.Run(req.TaskContext, s.notificationSender, s.taskStopWG, s.taskDoneC)

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

	// [Race Condition 방지]
	// SubmitTask가 running 상태를 확인하고 채널에 전송하기(send) 전에,
	// 여기서 먼저 running을 false로 설정하여 "닫힌 채널에 전송(Panic)"을 원천 차단합니다.
	// (SubmitTask는 runningMu를 획득해야만 진행 가능하므로, 여기서 running=false 설정 시 안전이 보장됨)
	s.running = false

	// 현재 작업중인 Task의 작업을 모두 취소한다.
	for _, handler := range s.handlers {
		handler.Cancel()
	}
	s.runningMu.Unlock()

	close(s.taskSubmitC)
	close(s.taskCancelC)

	// Task의 작업이 모두 취소될 때까지 대기한다. (최대 30초)
	done := make(chan struct{})
	go func() {
		s.taskStopWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 정상적으로 모든 태스크가 종료됨
	case <-time.After(30 * time.Second):
		applog.WithComponent("task.service").Warn("Service 종료 대기 시간 초과! (30s) 강제 종료합니다.")
	}

	close(s.taskDoneC)

	s.runningMu.Lock()
	s.handlers = nil
	s.notificationSender = nil
	s.runningMu.Unlock()

	applog.WithComponent("task.service").Info("Task 서비스 중지됨")
}

func (s *Service) SubmitTask(req *SubmitRequest) (err error) {
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

	// 요청된 TaskID와 CommandID가 유효한지 먼저 검증합니다.
	// 유효하지 않은 요청을 큐에 넣지 않고 즉시 거부함으로써, 리소스 낭비를 막고 호출자에게 빠른 피드백을 제공합니다.
	if _, err := findConfig(req.TaskID, req.CommandID); err != nil {
		return err
	}

	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		return apperrors.New(apperrors.ErrInternal, "Task 서비스가 실행중이 아닙니다.")
	}

	select {
	case s.taskSubmitC <- req:
		return nil
	default:
		return apperrors.New(apperrors.ErrInternal, "Task 실행 요청 큐가 가득 찼습니다.")
	}
}

func (s *Service) CancelTask(instanceID InstanceID) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = apperrors.New(apperrors.ErrInternal, fmt.Sprintf("Task 취소 요청중에 panic 발생: %v", r))

			applog.WithComponentAndFields("task.service", log.Fields{
				"instance_id": instanceID,
				"panic":       r,
			}).Error("Task 취소 요청중에 panic 발생")
		}
	}()

	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		return apperrors.New(apperrors.ErrInternal, "Task 서비스가 실행중이 아닙니다.")
	}

	select {
	case s.taskCancelC <- instanceID:
		return nil
	default:
		return apperrors.New(apperrors.ErrInternal, "Task 취소 요청 큐가 가득 찼습니다.")
	}
}
