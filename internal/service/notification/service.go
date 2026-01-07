package notification

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

type Service struct {
	appConfig *config.AppConfig

	notifiers       []NotifierHandler
	defaultNotifier NotifierHandler

	notifierFactory NotifierFactory

	// notifiersStopWG 모든 하위 Notifier의 종료를 대기하는 WaitGroup
	notifiersStopWG *sync.WaitGroup

	executor task.Executor

	running   bool
	runningMu sync.Mutex
}

func NewService(appConfig *config.AppConfig, executor task.Executor) *Service {
	service := &Service{
		appConfig: appConfig,

		defaultNotifier: nil,

		notifiersStopWG: &sync.WaitGroup{},

		executor: executor,

		running:   false,
		runningMu: sync.Mutex{},
	}

	// Factory 생성 및 Processor 등록
	factory := NewNotifierFactory()
	factory.RegisterProcessor(NewTelegramConfigProcessor(newTelegramNotifier))
	service.notifierFactory = factory

	return service
}

func (s *Service) SetNotifierFactory(factory NotifierFactory) {
	s.notifierFactory = factory
}

// Start 알림 서비스를 시작하여 등록된 Notifier들을 활성화합니다.
func (s *Service) Start(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent("notification.service").Info("Notification 서비스 시작중...")

	if s.executor == nil {
		defer serviceStopWG.Done()
		return apperrors.New(apperrors.Internal, "Executor 객체가 초기화되지 않았습니다")
	}

	if s.running {
		defer serviceStopWG.Done()
		applog.WithComponent("notification.service").Warn("Notification 서비스가 이미 시작됨!!!")
		return nil
	}

	// 1. Notifier들을 초기화 및 실행
	notifiers, err := s.notifierFactory.CreateNotifiers(s.appConfig, s.executor)
	if err != nil {
		defer serviceStopWG.Done()
		return apperrors.Wrap(err, apperrors.Internal, "Notifier 초기화 중 에러가 발생했습니다")
	}

	defaultNotifierID := NotifierID(s.appConfig.Notifier.DefaultNotifierID)

	for _, h := range notifiers {
		s.notifiers = append(s.notifiers, h)

		if h.ID() == defaultNotifierID {
			s.defaultNotifier = h
		}

		s.notifiersStopWG.Add(1)

		go func(handler NotifierHandler) {
			defer s.notifiersStopWG.Done()
			handler.Run(serviceStopCtx)
		}(h)

		applog.WithComponentAndFields("notification.service", log.Fields{
			"notifier_id": h.ID(),
		}).Debug("Notifier가 Notification 서비스에 등록됨")
	}

	// 2. 기본 Notifier 존재 여부 확인
	if s.defaultNotifier == nil {
		defer serviceStopWG.Done()
		return apperrors.New(apperrors.NotFound, fmt.Sprintf("기본 NotifierID('%s')를 찾을 수 없습니다", s.appConfig.Notifier.DefaultNotifierID))
	}

	// 3. 서비스 종료 감시 루틴 실행
	go s.waitForShutdown(serviceStopCtx, serviceStopWG)

	s.running = true

	applog.WithComponent("notification.service").Info("Notification 서비스 시작됨")

	return nil
}

// waitForShutdown 서비스의 종료 신호를 감지하고 리소스를 안전하게 정리합니다.
func (s *Service) waitForShutdown(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) {
	defer serviceStopWG.Done()

	<-serviceStopCtx.Done()

	applog.WithComponent("notification.service").Info("Notification 서비스 중지중...")

	// 등록된 모든 Notifier의 고루틴 작업이 완료(종료)될 때까지 대기합니다.
	// 각 Notifier의 Run 메서드에서 defer s.notifiersStopWG.Done()이 호출되어야 합니다.
	s.notifiersStopWG.Wait()

	s.runningMu.Lock()
	s.running = false
	s.executor = nil
	s.notifiers = nil
	s.defaultNotifier = nil
	s.runningMu.Unlock()

	applog.WithComponent("notification.service").Info("Notification 서비스 중지됨")
}

// NotifyWithTitle 지정된 Notifier를 통해 알림 메시지를 발송합니다.
// API 핸들러 등 외부에서 특정 채널을 통하여 알림을 보내고 싶을 때 사용합니다.
//
// 파라미터:
//   - notifierID: 알림 채널 ID
//   - title: 알림 메시지의 제목 (TaskContext에 저장됨)
//   - message: 전송할 메시지 내용
//   - errorOccurred: 에러 발생 여부
//
// 반환값:
//   - bool: 발송 요청이 성공적으로 큐에 등록되었는지 여부 (실제 전송 성공 여부는 아님)
func (s *Service) NotifyWithTitle(notifierID string, title string, message string, errorOccurred bool) bool {
	taskCtx := task.NewTaskContext().WithTitle(title)
	if errorOccurred {
		taskCtx = taskCtx.WithError()
	}

	return s.Notify(taskCtx, notifierID, message)
}

// NotifyDefault 시스템 기본 알림 채널로 알림 메시지를 발송합니다.
//
// 파라미터:
//   - message: 전송할 메시지 내용
//
// 반환값:
//   - bool: 발송 요청이 성공적으로 큐에 등록되었는지 여부 (실제 전송 성공 여부는 아님)
func (s *Service) NotifyDefault(message string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.defaultNotifier == nil {
		applog.WithComponent("notification.service").Warn("Notification 서비스가 중지된 상태여서 메시지를 전송할 수 없습니다")
		return false
	}

	return s.defaultNotifier.Notify(nil, message)
}

// NotifyDefaultWithError 시스템 기본 알림 채널로 "에러" 알림 메시지를 발송합니다.
// 시스템 오류, 작업 실패 등 관리자의 주의가 필요한 상황에서 사용합니다.
// 내부적으로 TaskContext에 Error 속성을 추가하여 Notifier가 이를 인지할 수 있게 합니다.
//
// 파라미터:
//   - message: 전송할 메시지 내용
//
// 반환값:
//   - bool: 발송 요청이 성공적으로 큐에 등록되었는지 여부 (실제 전송 성공 여부는 아님)
func (s *Service) NotifyDefaultWithError(message string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.defaultNotifier == nil {
		applog.WithComponent("notification.service").Warn("Notification 서비스가 중지된 상태여서 에러 메시지를 전송할 수 없습니다")
		return false
	}

	return s.defaultNotifier.Notify(task.NewTaskContext().WithError(), message)
}

// Notify 지정된 Notifier를 통해 알림 메시지를 발송합니다.
// TaskContext를 직접 생성하여 알림을 보낼 때 사용합니다.
//
// 파라미터:
//   - taskCtx: 알림 발송 시 함께 전달할 TaskContext
//   - notifierID: 알림 채널 ID
//   - message: 전송할 메시지 내용
//
// 반환값:
//   - bool: 발송 요청이 성공적으로 큐에 등록되었는지 여부 (실제 전송 성공 여부는 아님)
func (s *Service) Notify(taskCtx task.TaskContext, notifierID string, message string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		applog.WithComponentAndFields("notification.service", log.Fields{
			"notifier_id": notifierID,
		}).Warn("Notification 서비스가 실행 중이 아니어서 메시지를 전송할 수 없습니다")
		return false
	}

	id := NotifierID(notifierID)
	for _, h := range s.notifiers {
		if h.ID() == id {
			return h.Notify(taskCtx, message)
		}
	}

	m := fmt.Sprintf("알 수 없는 Notifier('%s')입니다. 알림메시지 발송이 실패하였습니다.(Message:%s)", notifierID, message)

	applog.WithComponentAndFields("notification.service", log.Fields{
		"notifier_id": notifierID,
	}).Error(m)

	s.defaultNotifier.Notify(task.NewTaskContext().WithError(), m)

	return false
}

// SupportsHTML 해당 Notifier가 HTML 포맷을 지원하는지 확인합니다.
func (s *Service) SupportsHTML(notifierID string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	id := NotifierID(notifierID)
	for _, h := range s.notifiers {
		if h.ID() == id {
			return h.SupportsHTML()
		}
	}

	return false
}
