package notification

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/constants"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

// TODO 미완료
// Service 알림 발송 요청을 처리하는 핵심 서비스 구조체입니다.
type Service struct {
	appConfig *config.AppConfig

	// notifiersMap 현재 서비스에서 관리 중인 모든 Notifier 인스턴스 맵 (ID -> 핸들러)
	notifiersMap map[contract.NotifierID]notifier.NotifierHandler
	// defaultNotifier 알림 채널 미지정 시 사용되는 기본 Notifier 핸들러
	defaultNotifier notifier.NotifierHandler

	// notifierFactory 런타임에 동적으로 Notifier 인스턴스를 생성하고 초기화하는 팩토리
	notifierFactory notifier.NotifierFactory

	// notifiersStopWG 서비스 종료 시, 모든 하위 Notifier의 고루틴들이 안전하게 종료될 때까지 대기하는 동기화 객체
	notifiersStopWG sync.WaitGroup

	executor contract.TaskExecutor

	running   bool
	runningMu sync.RWMutex
}

// NewService Notification 서비스를 생성합니다.
func NewService(appConfig *config.AppConfig, factory notifier.NotifierFactory, executor contract.TaskExecutor) *Service {
	service := &Service{
		appConfig: appConfig,

		notifiersMap:    make(map[contract.NotifierID]notifier.NotifierHandler),
		defaultNotifier: nil,

		notifierFactory: factory,

		notifiersStopWG: sync.WaitGroup{},

		executor: executor,

		running:   false,
		runningMu: sync.RWMutex{},
	}

	return service
}

// Start 알림 서비스를 시작하여 등록된 Notifier들을 활성화합니다.
func (s *Service) Start(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent(constants.ComponentService).Info(constants.LogMsgServiceStarting)

	if s.executor == nil {
		defer serviceStopWG.Done()
		return apperrors.New(apperrors.Internal, "Executor 객체가 초기화되지 않았습니다")
	}

	if s.running {
		defer serviceStopWG.Done()
		applog.WithComponent(constants.ComponentService).Warn(constants.LogMsgServiceAlreadyStarted)
		return nil
	}

	// 1. Notifier들을 초기화 및 실행
	notifiers, err := s.notifierFactory.CreateNotifiers(s.appConfig, s.executor)
	if err != nil {
		defer serviceStopWG.Done()
		return apperrors.Wrap(err, apperrors.Internal, "Notifier 초기화 중 에러가 발생했습니다")
	}

	// 중복 ID 검사
	seenIDs := make(map[contract.NotifierID]bool)
	for _, h := range notifiers {
		if seenIDs[h.ID()] {
			defer serviceStopWG.Done()
			return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("중복된 Notifier ID('%s')가 감지되었습니다. 설정을 확인해주세요.", h.ID()))
		}
		seenIDs[h.ID()] = true
	}

	defaultNotifierID := contract.NotifierID(s.appConfig.Notifier.DefaultNotifierID)

	for _, h := range notifiers {
		s.notifiersMap[h.ID()] = h

		if h.ID() == defaultNotifierID {
			s.defaultNotifier = h
		}

		s.notifiersStopWG.Add(1)

		go func(handler notifier.NotifierHandler) {
			defer s.notifiersStopWG.Done()

			// 개별 Notifier의 Panic이 서비스 전체로 전파되지 않도록 격리
			defer func() {
				if r := recover(); r != nil {
					applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
						"notifier_id": handler.ID(),
						"panic":       r,
					}).Error(constants.LogMsgNotificationServicePanicRecovered)
				}
			}()

			handler.Run(serviceStopCtx)
		}(h)

		applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
			"notifier_id": h.ID(),
		}).Debug(constants.LogMsgNotifierRegistered)
	}

	// 2. 기본 Notifier 존재 여부 확인
	if s.defaultNotifier == nil {
		defer serviceStopWG.Done()
		return apperrors.New(apperrors.NotFound, fmt.Sprintf("기본 NotifierID('%s')를 찾을 수 없습니다", s.appConfig.Notifier.DefaultNotifierID))
	}

	// 3. 서비스 종료 감시 루틴 실행
	go s.waitForShutdown(serviceStopCtx, serviceStopWG)

	s.running = true

	applog.WithComponent(constants.ComponentService).Info(constants.LogMsgServiceStarted)

	return nil
}

// waitForShutdown 서비스의 종료 신호를 감지하고 리소스를 안전하게 정리합니다.
func (s *Service) waitForShutdown(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) {
	defer serviceStopWG.Done()

	<-serviceStopCtx.Done()

	applog.WithComponent(constants.ComponentService).Info(constants.LogMsgServiceStopping)

	// 1. 새로운 요청 수락 중단
	// Notifier가 여전히 동작 중이더라도, 서비스 차원에서 더 이상의 요청은 받지 않아야 합니다.
	s.runningMu.Lock()
	s.running = false
	s.runningMu.Unlock()

	// 2. 등록된 모든 Notifier의 고루틴 작업이 완료(종료)될 때까지 대기합니다.
	// 각 Notifier의 Run 메서드에서 defer s.notifiersStopWG.Done()이 호출되어야 합니다.
	s.notifiersStopWG.Wait()

	// 3. 리소스 정리
	s.runningMu.Lock()

	// 방어적 검증: 모든 Notifier가 정상 종료되었는지 확인
	// notifiersStopWG.Wait()가 완료되었으므로 이론적으로는 모두 종료되었어야 하지만,
	// 명시적으로 확인하여 비정상 상황을 조기에 감지합니다.
	for id, handler := range s.notifiersMap {
		select {
		case <-handler.Done():
			// 정상 종료됨
		default:
			// 아직 종료되지 않음 (비정상 상황)
			applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
				"notifier_id": id,
			}).Warn(constants.LogMsgNotifierNotStopped)
		}
	}

	s.executor = nil
	s.notifiersMap = nil
	s.defaultNotifier = nil
	s.runningMu.Unlock()

	applog.WithComponent(constants.ComponentService).Info(constants.LogMsgServiceStopCompleted)
}

// NotifyWithTitle 지정된 Notifier를 통해 제목을 포함한 알림 메시지를 발송합니다.
// 제목을 명시하여 알림의 맥락을 명확히 전달할 수 있습니다.
// errorOccurred 플래그를 통해 해당 알림이 오류 상황에 대한 것인지 명시할 수 있습니다.
//
// 파라미터:
//   - notifierID: 알림을 발송할 대상 Notifier의 식별자
//   - title: 알림 메시지의 제목
//   - message: 전송할 메시지 내용
//   - errorOccurred: 오류 발생 여부
//
// 반환값:
//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환 (ErrServiceStopped, ErrNotFoundNotifier 등)
func (s *Service) NotifyWithTitle(notifierID contract.NotifierID, title string, message string, errorOccurred bool) error {
	ctx := contract.NewTaskContext().WithTitle(title)
	if errorOccurred {
		ctx = ctx.WithError()
	}

	return s.Notify(ctx, notifierID, message)
}

// NotifyDefault 시스템에 설정된 기본 Notifier를 통해 알림 메시지를 발송합니다.
//
// 파라미터:
//   - message: 전송할 메시지 내용
//
// 반환값:
//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환
func (s *Service) NotifyDefault(message string) error {
	s.runningMu.RLock()
	if s.defaultNotifier == nil {
		s.runningMu.RUnlock()

		applog.WithComponent(constants.ComponentService).Warn(constants.LogMsgServiceStopped)

		return notifier.ErrServiceStopped
	}

	notifier := s.defaultNotifier

	s.runningMu.RUnlock()

	if ok := notifier.Notify(nil, message); !ok {
		return apperrors.New(apperrors.Unavailable, "알림 전송 대기열이 가득 차서 요청을 처리할 수 없습니다.")
	}
	return nil
}

// NotifyDefaultWithError 시스템에 설정된 기본 Notifier를 통해 "오류" 성격의 알림 메시지를 발송합니다.
// 시스템 내부 에러, 작업 실패 등 관리자의 주의가 필요한 긴급 상황 알림에 적합합니다.
// 내부적으로 오류 플래그가 설정되어 발송되므로, 수신 측에서 이를 인지하여 처리할 수 있습니다.
//
// 파라미터:
//   - message: 전송할 오류 메시지 내용
//
// 반환값:
//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환
func (s *Service) NotifyDefaultWithError(message string) error {
	s.runningMu.RLock()
	if s.defaultNotifier == nil {
		s.runningMu.RUnlock()

		applog.WithComponent(constants.ComponentService).Warn(constants.LogMsgServiceStopped)

		return notifier.ErrServiceStopped
	}

	notifier := s.defaultNotifier

	s.runningMu.RUnlock()

	if ok := notifier.Notify(contract.NewTaskContext().WithError(), message); !ok {
		return apperrors.New(apperrors.Unavailable, "알림 전송 대기열이 가득 차서 요청을 처리할 수 없습니다.")
	}
	return nil
}

// Notify 지정된 Notifier를 통해 알림 메시지를 발송합니다.
// 작업 실행 컨텍스트를 함께 전달하여, 알림 수신자가 작업의 메타데이터(TaskID, Title, 실행 시간 등)를
// 확인할 수 있도록 지원합니다.
//
// 파라미터:
//   - ctx: 작업 실행 컨텍스트 정보
//   - notifierID: 알림을 발송할 대상 Notifier의 식별자
//   - message: 전송할 메시지 내용
//
// 반환값:
//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환
func (s *Service) Notify(ctx contract.TaskContext, notifierID contract.NotifierID, message string) error {
	s.runningMu.RLock()
	if !s.running {
		s.runningMu.RUnlock()

		applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
			"notifier_id": notifierID,
		}).Warn(constants.LogMsgServiceStopped)

		return notifier.ErrServiceStopped
	}

	targetNotifier := s.notifiersMap[notifierID]
	defaultNotifier := s.defaultNotifier

	s.runningMu.RUnlock()

	if targetNotifier != nil {
		if ok := targetNotifier.Notify(ctx, message); !ok {
			// Notifier가 이미 종료되었는지 확인
			select {
			case <-targetNotifier.Done():
				return notifier.ErrServiceStopped
			default:
				// 종료되지 않았다면 큐가 가득 찬 것으로 간주
				return apperrors.New(apperrors.Unavailable, "알림 전송 대기열이 가득 차서 요청을 처리할 수 없습니다.")
			}
		}
		return nil
	}

	m := fmt.Sprintf(constants.LogMsgNotifierNotFoundRejected, notifierID, strutil.Truncate(message, 100))

	applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
		"notifier_id": notifierID,
	}).Error(m)

	if defaultNotifier != nil {
		if !defaultNotifier.Notify(contract.NewTaskContext().WithError(), m) {
			applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
				"notifier_id":         notifierID,
				"default_notifier_id": defaultNotifier.ID(),
			}).Warn(constants.LogMsgDefaultNotifierFailed)
		}
	}

	return notifier.ErrNotFoundNotifier
}

// Health 서비스가 정상적으로 실행 중인지 확인합니다.
//
// 반환값:
//   - error: 서비스가 정상 동작 중이면 nil, 그렇지 않으면 에러 반환 (예: ErrServiceStopped)
func (s *Service) Health() error {
	s.runningMu.RLock()
	defer s.runningMu.RUnlock()

	if !s.running {
		return notifier.ErrServiceStopped
	}

	return nil
}

// SupportsHTML 지정된 ID의 Notifier가 HTML 형식을 지원하는지 여부를 반환합니다.
//
// 파라미터:
//   - notifierID: 지원 여부를 확인할 Notifier의 식별자
//
// 반환값:
//   - bool: HTML 포맷 지원 여부
func (s *Service) SupportsHTML(notifierID contract.NotifierID) bool {
	s.runningMu.RLock()
	defer s.runningMu.RUnlock()

	if h, exists := s.notifiersMap[notifierID]; exists {
		return h.SupportsHTML()
	}
	return false
}
