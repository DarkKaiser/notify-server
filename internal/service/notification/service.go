package notification

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

// TODO 미완료

// component Notification 서비스의 로깅용 컴포넌트 이름
const component = "notification.service"

// Service 알림 발송 요청을 처리하고 Notifier들의 생명주기를 관리하는 구조체입니다.
type Service struct {
	appConfig *config.AppConfig

	// notifiers 서비스에서 관리 중인 모든 Notifier 인스턴스 맵 (ID -> Notifier)
	notifiers map[contract.NotifierID]notifier.Notifier

	// defaultNotifier 알림 채널을 지정하지 않았을 때 사용하는 기본 Notifier
	defaultNotifier notifier.Notifier

	// creator Notifier 인스턴스를 생성하는 팩토리
	creator notifier.Creator

	// executor 작업(Task) 실행 및 스케줄링을 담당하는 추상화된 인터페이스
	executor contract.TaskExecutor

	// notifiersStopWG 서비스 종료 시, 모든 하위 Notifier들의 고루틴들이 안전하게 종료될 때까지 대기하는 동기화 객체
	notifiersStopWG sync.WaitGroup

	running   bool
	runningMu sync.RWMutex
}

// NewService Notification 서비스를 생성합니다.
func NewService(appConfig *config.AppConfig, creator notifier.Creator, executor contract.TaskExecutor) *Service {
	service := &Service{
		appConfig: appConfig,

		notifiers:       make(map[contract.NotifierID]notifier.Notifier),
		defaultNotifier: nil,

		creator: creator,

		executor: executor,

		notifiersStopWG: sync.WaitGroup{},

		running:   false,
		runningMu: sync.RWMutex{},
	}

	return service
}

// Start 알림 서비스를 시작합니다.
//
// 이 메서드는 설정된 Notifier들을 초기화하고 각각의 고루틴을 실행하여 알림 발송을 준비합니다.
// 서비스가 이미 실행 중이거나 필수 의존성(executor)이 없는 경우 에러를 반환합니다.
//
// 파라미터:
//   - serviceStopCtx: 서비스 종료 신호를 전달받는 컨텍스트입니다. 이 컨텍스트가 취소되면 모든 Notifier가 정리됩니다.
//   - serviceStopWG: 서비스 종료 시 모든 고루틴이 완전히 종료될 때까지 대기하기 위한 WaitGroup입니다.
//
// 반환값:
//   - error: 초기화 실패, 중복 실행, 또는 설정 오류 시 에러를 반환합니다.
func (s *Service) Start(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent(component).Info("Notification 서비스 시작중...")

	if s.executor == nil {
		defer serviceStopWG.Done()
		return NewErrExecutorNotInitialized()
	}

	if s.running {
		defer serviceStopWG.Done()
		applog.WithComponent(component).Warn("Notification 서비스가 이미 시작됨!!!")
		return nil
	}

	// 1단계: Notifier 인스턴스 생성
	notifiers, err := s.creator.CreateAll(s.appConfig, s.executor)
	if err != nil {
		defer serviceStopWG.Done()
		return NewErrNotifierInitFailed(err)
	}

	// 2단계: Notifier ID 중복 검사
	// 각 Notifier는 고유한 ID를 가져야 하므로, 중복이 있으면 설정 오류로 간주합니다
	seenIDs := make(map[contract.NotifierID]bool)
	for _, n := range notifiers {
		if seenIDs[n.ID()] {
			defer serviceStopWG.Done()
			return NewErrDuplicateNotifierID(string(n.ID()))
		}
		seenIDs[n.ID()] = true
	}

	defaultNotifierID := contract.NotifierID(s.appConfig.Notifier.DefaultNotifierID)

	// 3단계: Notifier 등록 및 고루틴 실행
	// 각 Notifier를 서비스에 등록하고, 별도의 고루틴에서 실행합니다
	for _, n := range notifiers {
		s.notifiers[n.ID()] = n

		// 기본 Notifier 설정: 알림 채널이 지정되지 않은 요청에 사용됩니다
		if n.ID() == defaultNotifierID {
			s.defaultNotifier = n
		}

		s.notifiersStopWG.Add(1)

		// 각 Notifier를 독립적인 고루틴에서 실행합니다
		// 패닉이 발생해도 다른 Notifier에 영향을 주지 않도록 복구 로직을 포함합니다
		go func(notifier notifier.Notifier) {
			defer s.notifiersStopWG.Done()

			// 패닉 복구: Notifier 실행 중 예상치 못한 오류가 발생해도 서비스는 계속 동작합니다
			defer func() {
				if r := recover(); r != nil {
					applog.WithComponentAndFields(component, applog.Fields{
						"notifier_id": notifier.ID(),
						"error":       r,
					}).Error("Notifier 고루틴에서 패닉이 발생하여 복구되었습니다. 해당 인스턴스의 안정성을 점검하십시오")
				}
			}()

			notifier.Run(serviceStopCtx)
		}(n)

		applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
			"notifier_id": n.ID(),
		}).Debug(constants.LogMsgNotifierRegistered)
	}

	// 2. 기본 Notifier 존재 여부 확인
	if s.defaultNotifier == nil {
		defer serviceStopWG.Done()
		return NewErrDefaultNotifierNotFound(s.appConfig.Notifier.DefaultNotifierID)
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
	for id, notifier := range s.notifiersMap {
		select {
		case <-notifier.Done():
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
//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환 (ErrServiceStopped, ErrNotifierNotFound 등)
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

		return ErrServiceStopped
	}

	notifier := s.defaultNotifier

	s.runningMu.RUnlock()

	if err := notifier.Send(nil, message); err != nil {
		return err
	}
	return nil
}

// Notify 알림 메시지 발송을 요청합니다.
//
// 이 메서드는 일반적으로 비동기적으로 동작할 수 있으며(구현체에 따라 다름),
// 전송 요청이 성공적으로 큐에 적재되거나 시스템에 수락되었을 때 nil을 반환합니다.
// 즉, nil 반환이 반드시 "최종 사용자 도달"을 보장하는 것은 아닙니다.
//
// 파라미터:
//   - ctx: 요청의 컨텍스트 (Timeout, Cancellation 전파 용도)
//   - notification: 전송할 알림의 상세 내용 (메시지, 수신처, 메타데이터 등)
//
// 반환값:
//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환
func (s *Service) NotifyDefaultWithError(message string) error {
	s.runningMu.RLock()
	if s.defaultNotifier == nil {
		s.runningMu.RUnlock()

		applog.WithComponent(constants.ComponentService).Warn(constants.LogMsgServiceStopped)

		return ErrServiceStopped
	}

	notifier := s.defaultNotifier

	s.runningMu.RUnlock()

	if err := notifier.Send(contract.NewTaskContext().WithError(), message); err != nil {
//   - error: 요청 검증 실패, 큐 포화 상태, 또는 일시적 시스템 장애 시 에러를 반환합니다.
		return err
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
func (s *Service) Notify(taskCtx contract.TaskContext, notifierID contract.NotifierID, message string) error {
	s.runningMu.RLock()
	if !s.running {
		s.runningMu.RUnlock()

		applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
			"notifier_id": notifierID,
		}).Warn(constants.LogMsgServiceStopped)

		return ErrServiceStopped
	}

	targetNotifier := s.notifiersMap[notifierID]
	defaultNotifier := s.defaultNotifier

	s.runningMu.RUnlock()

	if targetNotifier != nil {
		if err := targetNotifier.Send(taskCtx, message); err != nil {
			// Notifier가 종료된 경우 서비스 중단 에러로 매핑
			if err == notifierpkg.ErrClosed {
				return ErrServiceStopped
			}
			return err
		}
		return nil
	}

	m := fmt.Sprintf(constants.LogMsgNotifierNotFoundRejected, notifierID, strutil.Truncate(message, 100))

	applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
		"notifier_id": notifierID,
	}).Error(m)

	if defaultNotifier != nil {
		if err := defaultNotifier.Send(contract.NewTaskContext().WithError(), m); err != nil {
			applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
				"notifier_id":         notifierID,
				"default_notifier_id": defaultNotifier.ID(),
				"error":               err,
			}).Warn(constants.LogMsgDefaultNotifierFailed)
		}
	}

	return ErrNotifierNotFound
}

// Health 시스템이 정상적으로 동작 중인지 검사합니다.
func (s *Service) Health() error {
	s.runningMu.RLock()
	defer s.runningMu.RUnlock()

	if !s.running {
		return ErrServiceStopped
	}

	return nil
}

// SupportsHTML 지정된 Notifier가 HTML 형식의 메시지 본문을 지원하는지의 여부를 반환합니다.
func (s *Service) SupportsHTML(notifierID contract.NotifierID) bool {
	s.runningMu.RLock()
	defer s.runningMu.RUnlock()

	if n, exists := s.notifiers[notifierID]; exists {
		return n.SupportsHTML()
	}

	return false
}
