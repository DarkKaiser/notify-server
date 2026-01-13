package notification

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task"
	applog "github.com/darkkaiser/notify-server/pkg/log"
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

		applog.WithComponentAndFields("notification.service", applog.Fields{
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

// NotifyWithTitle 지정된 Notifier를 통해 제목이 포함된 알림 메시지를 발송합니다.
// 일반 메시지뿐만 아니라 제목을 명시하여 알림의 맥락을 명확히 전달할 수 있습니다.
// errorOccurred 플래그를 통해 해당 알림이 오류 상황에 대한 것인지 명시할 수 있습니다.
//
// 파라미터:
//   - notifierID: 메시지를 발송할 대상 Notifier의 고유 ID
//   - title: 알림 메시지의 제목 (강조 표시 등에 활용)
//   - message: 전송할 메시지 내용
//   - errorOccurred: 오류 발생 여부 (true일 경우 오류 상황으로 처리되어 시각적 강조 등이 적용될 수 있음)
//
// 반환값:
//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환 (ErrServiceStopped, ErrNotFoundNotifier 등)
func (s *Service) NotifyWithTitle(notifierID string, title string, message string, errorOccurred bool) error {
	taskCtx := task.NewTaskContext().WithTitle(title)
	if errorOccurred {
		taskCtx = taskCtx.WithError()
	}

	return s.Notify(taskCtx, notifierID, message)
}

// NotifyDefault 시스템에 설정된 기본 알림 채널로 일반 메시지를 발송합니다.
// 주로 시스템 전반적인 알림이나, 특정 대상을 지정하지 않은 일반적인 정보 전달에 사용됩니다.
//
// 파라미터:
//   - message: 전송할 메시지 내용
//
// 반환값:
//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환
func (s *Service) NotifyDefault(message string) error {
	s.runningMu.Lock()
	if s.defaultNotifier == nil {
		s.runningMu.Unlock()

		applog.WithComponent("notification.service").Warn("Notification 서비스가 중지된 상태여서 메시지를 전송할 수 없습니다")

		return ErrServiceStopped
	}

	notifier := s.defaultNotifier

	s.runningMu.Unlock()

	if ok := notifier.Notify(nil, message); !ok {
		return apperrors.New(apperrors.Internal, "알림 전송 대기열이 가득 차서 요청을 처리할 수 없습니다.")
	}
	return nil
}

// NotifyDefaultWithError 시스템에 설정된 기본 알림 채널로 "오류" 성격의 알림 메시지를 발송합니다.
// 시스템 내부 에러, 작업 실패 등 관리자의 주의가 필요한 긴급 상황 알림에 적합합니다.
// 내부적으로 오류 플래그가 설정되어 발송되므로, 수신 측에서 이를 인지하여 처리할 수 있습니다.
//
// 파라미터:
//   - message: 전송할 오류 메시지 내용
//
// 반환값:
//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환
func (s *Service) NotifyDefaultWithError(message string) error {
	s.runningMu.Lock()
	if s.defaultNotifier == nil {
		s.runningMu.Unlock()

		applog.WithComponent("notification.service").Warn("Notification 서비스가 중지된 상태여서 메시지를 전송할 수 없습니다")

		return ErrServiceStopped
	}

	notifier := s.defaultNotifier

	s.runningMu.Unlock()

	if ok := notifier.Notify(task.NewTaskContext().WithError(), message); !ok {
		return apperrors.New(apperrors.Internal, "알림 전송 대기열이 가득 차서 요청을 처리할 수 없습니다.")
	}
	return nil
}

// Notify 지정된 Notifier를 통해 알림 메시지를 발송합니다.
// 작업 실행 컨텍스트를 함께 전달하여, 알림 수신자가 작업의 메타데이터(TaskID, Title, 실행 시간 등)를
// 확인할 수 있도록 지원합니다.
//
// 파라미터:
//   - taskCtx: 작업 실행 컨텍스트 정보
//   - notifierID: 메시지를 발송할 대상 Notifier의 고유 ID
//   - message: 전송할 메시지 내용
//
// 반환값:
//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환
func (s *Service) Notify(taskCtx task.TaskContext, notifierID string, message string) error {
	s.runningMu.Lock()
	if !s.running {
		s.runningMu.Unlock()

		applog.WithComponentAndFields("notification.service", applog.Fields{
			"notifier_id": notifierID,
		}).Warn("Notification 서비스가 중지된 상태여서 메시지를 전송할 수 없습니다")

		return ErrServiceStopped
	}

	var targetNotifier NotifierHandler
	var defaultNotifier = s.defaultNotifier

	id := NotifierID(notifierID)
	for _, h := range s.notifiers {
		if h.ID() == id {
			targetNotifier = h
			break
		}
	}

	s.runningMu.Unlock()

	if targetNotifier != nil {
		if ok := targetNotifier.Notify(taskCtx, message); !ok {
			return apperrors.New(apperrors.Internal, "알림 전송 대기열이 가득 차서 요청을 처리할 수 없습니다.")
		}
		return nil
	}

	m := fmt.Sprintf("등록되지 않은 Notifier ID('%s')입니다. 메시지 발송이 거부되었습니다. 원본 메시지: %s", notifierID, message)

	applog.WithComponentAndFields("notification.service", applog.Fields{
		"notifier_id": notifierID,
	}).Error(m)

	if defaultNotifier != nil {
		defaultNotifier.Notify(task.NewTaskContext().WithError(), m)
	}

	return ErrNotFoundNotifier
}

// Health 서비스의 건강 상태를 확인합니다.
func (s *Service) Health() error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		return ErrServiceStopped
	}

	return nil
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
