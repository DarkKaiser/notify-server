package notification

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"

	applog "github.com/darkkaiser/notify-server/pkg/log"
)

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
	return &Service{
		appConfig: appConfig,

		notifiers:       make(map[contract.NotifierID]notifier.Notifier),
		defaultNotifier: nil,

		creator: creator,

		executor: executor,

		notifiersStopWG: sync.WaitGroup{},

		running:   false,
		runningMu: sync.RWMutex{},
	}
}

// Start Notification 서비스를 시작하고 모든 Notifier를 초기화합니다.
//
// 이 메서드는 서비스의 전체 생명주기를 관리하는 핵심 진입점입니다.
//
// 각 Notifier는 독립적인 고루틴에서 실행되며, 하나의 Notifier에서 패닉이 발생하더라도
// 다른 Notifier의 동작에는 영향을 주지 않습니다. 서비스 종료 시에는 모든 Notifier가
// 안전하게 정리될 때까지 대기합니다.
//
// 매개변수:
//   - serviceStopCtx: 서비스 종료 신호를 전달받는 컨텍스트입니다.
//     이 컨텍스트가 취소되면 모든 Notifier의 정리 프로세스가 시작됩니다.
//   - serviceStopWG: 서비스 종료 시 모든 고루틴(Notifier 및 종료 처리 고루틴)이
//     완전히 종료될 때까지 대기하기 위한 WaitGroup입니다.
//     호출자는 이 WaitGroup을 통해 안전한 종료를 보장할 수 있습니다.
//
// 반환값:
//   - error: 초기화 실패, 중복 실행, 또는 설정 오류 시 에러를 반환합니다.
func (s *Service) Start(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent(component).Info("서비스 시작 진입: Notification 서비스 초기화 프로세스를 시작합니다")

	if s.executor == nil {
		defer serviceStopWG.Done()
		return ErrExecutorNotInitialized
	}

	if s.running {
		defer serviceStopWG.Done()
		applog.WithComponent(component).Warn("Notification 서비스가 이미 실행 중입니다 (중복 호출)")
		return nil
	}

	// ========================================
	// 1단계: Notifier 인스턴스 생성
	// ========================================
	// Factory를 통해 설정 파일에 정의된 모든 Notifier 인스턴스를 생성합니다.
	// 이 시점에서는 인스턴스만 생성되며, 아직 고루틴은 시작되지 않습니다.
	notifiers, err := s.creator.CreateAll(s.appConfig, s.executor)
	if err != nil {
		defer serviceStopWG.Done()
		return NewErrNotifierInitFailed(err)
	}

	// ========================================
	// 2단계: Notifier ID 중복 검사
	// ========================================
	// 각 Notifier는 고유한 ID를 가져야 합니다. 중복된 ID가 발견되면 설정 오류로 간주하여
	// 서비스 시작을 중단합니다.
	seenIDs := make(map[contract.NotifierID]bool)
	for _, n := range notifiers {
		if seenIDs[n.ID()] {
			defer serviceStopWG.Done()

			// [리소스 누수 방지]
			// 이 시점에서는 아직 고루틴(n.Run)이 시작되지 않았으므로
			// 호출자가 serviceStopCtx를 취소해도 Notifier의 Close()가 실행되지 않습니다.
			// 따라서 생성된 Notifier 인스턴스들을 명시적으로 닫아주어야 합니다.
			for _, notifier := range notifiers {
				notifier.Close()
			}

			return NewErrDuplicateNotifierID(string(n.ID()))
		}
		seenIDs[n.ID()] = true
	}

	// 설정 파일에서 기본 Notifier ID를 가져옵니다.
	defaultNotifierID := contract.NotifierID(s.appConfig.Notifier.DefaultNotifierID)

	// ========================================
	// 3단계: Notifier 등록 및 고루틴 실행
	// ========================================
	// 생성된 각 Notifier를 서비스에 등록하고, 독립적인 고루틴에서 실행합니다.
	for _, n := range notifiers {
		s.notifiers[n.ID()] = n

		// 기본 Notifier 설정: 알림 요청 시 NotifierID가 비어있으면 이 Notifier로 전송됩니다.
		if n.ID() == defaultNotifierID {
			s.defaultNotifier = n
		}

		// WaitGroup 카운터 증가: 서비스 종료 시 모든 Notifier 고루틴이 완전히 종료될 때까지 대기하기 위함입니다.
		s.notifiersStopWG.Add(1)

		go func(notifier notifier.Notifier) {
			defer s.notifiersStopWG.Done()

			// 하나의 Notifier에서 패닉이 발생하더라도 다른 Notifier의 동작에는
			// 영향을 주지 않도록 격리합니다. 패닉 발생 시 에러 로그를 남기고
			// 해당 Notifier만 종료됩니다.
			defer func() {
				if r := recover(); r != nil {
					applog.WithComponentAndFields(component, applog.Fields{
						"notifier_id": notifier.ID(),
						"error":       r,
					}).Error("Notifier 패닉 복구: 런타임 에러 발생 (안정성 점검 필요)")
				}
			}()

			// Notifier 메인 루프 실행: 이 메서드는 블로킹되며, serviceStopCtx가 취소될 때까지 실행됩니다.
			notifier.Run(serviceStopCtx)
		}(n)

		applog.WithComponentAndFields(component, applog.Fields{
			"notifier_id":   n.ID(),
			"is_default":    n.ID() == defaultNotifierID,
			"supports_html": n.SupportsHTML(),
		}).Debug("Notifier 등록 완료: 인스턴스 초기화 및 고루틴 실행")
	}

	// ========================================
	// 4단계: 기본 Notifier 검증
	// ========================================
	// 설정 파일에 기본 Notifier ID가 지정되어 있지만 실제로 해당 ID를 가진
	// Notifier가 존재하지 않으면 설정 오류로 간주합니다.
	//
	// 중요: 이 시점에서 일부 Notifier 고루틴은 이미 실행 중입니다.
	// 에러 반환 시 호출자는 반드시 serviceStopCtx를 취소하여 Notifier 고루틴에 종료 신호를 보내야 합니다.
	// (Context 취소만이 실행 중인 Run 루프를 멈추고 리소스를 해제할 수 있는 유일한 방법입니다)
	//
	// 리소스 정리 메커니즘:
	// 1. 호출자가 Context 취소 -> 2. Notifier.Run() 루프 종료 -> 3. defer cleanup() 실행 -> 4. Notifier.Close() 호출
	// 이 과정을 통해 실행 중인 고루틴과 리소스가 안전하게 정리됩니다.
	if s.defaultNotifier == nil {
		defer serviceStopWG.Done()

		// 여기서 별도의 리소스 정리를 하지 않는 이유:
		// 1. 에러가 발생하면 프로그램이 바로 종료되므로 별도의 정리가 불필요합니다.
		// 2. 또한 호출자가 에러 감지 후 serviceStopCtx를 취소하므로,
		//    이미 실행된 Notifier들은 Run() 루프 종료 -> defer cleanup() 흐름을 타게 되어
		//    자동으로 안전하게 종료됩니다. 따라서 여기서 중복으로 정리할 필요가 없습니다.

		return NewErrDefaultNotifierNotFound(string(defaultNotifierID))
	}

	// ========================================
	// 5단계: 종료 처리 고루틴 시작
	// ========================================
	// 서비스 종료 신호(serviceStopCtx 취소)를 감지하고, 모든 Notifier를 안전하게
	// 종료하는 별도의 고루틴을 시작합니다.
	go s.waitForShutdown(serviceStopCtx, serviceStopWG)

	s.running = true

	applog.WithComponentAndFields(component, applog.Fields{
		"notifiers_count":     len(s.notifiers),
		"default_notifier_id": defaultNotifierID,
	}).Info("서비스 시작 완료: Notification 서비스가 정상적으로 초기화되었습니다")

	return nil
}

// waitForShutdown 서비스의 종료 신호를 감지하고 모든 리소스를 안전하게 정리합니다.
//
// 이 메서드는 Start() 메서드에서 별도의 고루틴으로 실행되며, 서비스의 전체 생명주기 동안
// 백그라운드에서 대기하다가 종료 신호(serviceStopCtx 취소)를 받으면 다음 순서로 정리 작업을 수행합니다:
//
//  1. 새로운 알림 요청 차단 (s.running = false)
//     → Notify() 메서드가 더 이상 요청을 받지 않도록 게이트를 닫습니다
//
//  2. 모든 Notifier에 종료 신호 전파 (n.Close())
//     → 각 Notifier의 내부 큐를 닫아 더 이상의 메시지를 받지 않도록 합니다
//
//  3. 모든 Notifier 고루틴 종료 대기 (notifiersStopWG.Wait())
//     → 각 Notifier가 큐에 남은 메시지를 처리하고 안전하게 종료될 때까지 기다립니다
//
//  4. 리소스 정리 및 상태 초기화
//     → executor, notifiers, defaultNotifier 참조를 제거하여 메모리 누수를 방지합니다
//
// 이 메서드가 완료되면 서비스는 완전히 정리된 상태가 되며, serviceStopWG.Done()을 통해
// 호출자(main.go)에게 종료 완료를 알립니다.
func (s *Service) waitForShutdown(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) {
	defer serviceStopWG.Done()

	// ========================================
	// 종료 신호 대기
	// ========================================
	// 서비스가 정상적으로 실행되는 동안 이 채널은 블로킹됩니다.
	// main.go에서 serviceStopCancel()이 호출되면 이 채널이 닫히며, 아래 정리 프로세스가 시작됩니다.
	<-serviceStopCtx.Done()

	applog.WithComponent(component).Info("종료 절차 진입: Notification 서비스 중지 시그널을 수신했습니다")

	// ========================================
	// 1단계: 새로운 알림 요청 차단
	// ========================================
	// 서비스 상태를 '중지됨(false)'으로 변경하여 Notify() 메서드가 더 이상 새로운 요청을 받지 않도록 합니다.
	// Notifier 고루틴들은 여전히 큐에 남은 메시지를 처리 중일 수 있지만,
	// 서비스 레벨에서는 추가 요청을 거부하여 종료 프로세스를 시작합니다.
	s.runningMu.Lock()
	s.running = false
	s.runningMu.Unlock()

	// ========================================
	// 2단계: 모든 Notifier 고루틴 종료 대기
	// ========================================
	s.notifiersStopWG.Wait()

	// ========================================
	// 3단계: 리소스 정리 및 상태 초기화
	// ========================================
	s.runningMu.Lock()

	// 방어적 검증: 모든 Notifier가 정상 종료되었는지 확인
	// notifiersStopWG.Wait()가 완료되었으므로 이론적으로는 모든 Notifier가 정상 종료되었어야 하지만,
	// 각 Notifier의 Done() 채널을 명시적으로 확인하여 비정상 상황이 있는지 확인합니다.
	// 만약 Done() 채널이 아직 열려있다면, Notifier 구현에 버그가 있거나
	// cleanup() 로직이 제대로 실행되지 않은 것이므로 경고 로그를 남깁니다.
	for id, notifier := range s.notifiers {
		select {
		case <-notifier.Done():
			// 정상 종료됨: Done() 채널이 닫혀있음

		default:
			// 비정상 상황: WaitGroup은 완료되었으나 Done() 채널은 아직 열려있음
			applog.WithComponentAndFields(component, applog.Fields{
				"notifier_id": id,
			}).Warn("Notifier 종료 확인 실패: 고루틴은 종료되었으나 리소스 해제가 완료되지 않음")
		}
	}

	s.executor = nil
	s.notifiers = nil
	s.defaultNotifier = nil
	s.runningMu.Unlock()

	applog.WithComponent(component).Info("Notification 서비스 종료 완료: 모든 리소스가 정리되었습니다")
}

// Notify 알림 메시지 발송을 요청합니다.
//
// 이 메서드는 일반적으로 비동기적으로 동작할 수 있으며(구현체에 따라 다름),
// 전송 요청이 성공적으로 큐에 적재되거나 시스템에 수락되었을 때 nil을 반환합니다.
// 즉, nil 반환이 반드시 "최종 사용자 도달"을 보장하는 것은 아닙니다.
//
// 매개변수:
//   - ctx: 요청의 컨텍스트 (Timeout, Cancellation 전파 용도)
//   - notification: 전송할 알림의 상세 내용 (메시지, 수신처, 메타데이터 등)
//
// 반환값:
//   - error: 요청 검증 실패, 큐 포화 상태, 또는 일시적 시스템 장애 시 에러를 반환합니다.
func (s *Service) Notify(ctx context.Context, notification contract.Notification) error {
	// ========================================
	// 1단계: 알림 데이터 유효성 검증
	// ========================================
	if err := notification.Validate(); err != nil {
		return err
	}

	// ========================================
	// 2단계: 서비스 실행 상태 확인
	// ========================================
	s.runningMu.RLock()

	if !s.running {
		s.runningMu.RUnlock()

		fields := applog.Fields{
			"notifier_id": notification.NotifierID,
		}
		if notification.TaskID != "" {
			fields["task_id"] = notification.TaskID
		}
		if notification.CommandID != "" {
			fields["command_id"] = notification.CommandID
		}
		if notification.Title != "" {
			fields["title"] = notification.Title
		}
		applog.WithComponentAndFields(component, fields).Warn("알림 전송 거부: 서비스 종료 또는 미실행 상태")

		return ErrServiceNotRunning
	}

	// ========================================
	// 3단계: 대상 Notifier 선택
	// ========================================
	// 알림 요청에 명시된 NotifierID를 기반으로 적절한 Notifier를 선택합니다.
	// NotifierID가 비어있으면 기본 Notifier를 사용하고, 지정되어 있으면 해당 Notifier를 찾습니다.
	var targetNotifier notifier.Notifier

	if notification.NotifierID == "" {
		// ─────────────────────────────────────────────────────────────────
		// Case A: 기본 Notifier 사용
		// ─────────────────────────────────────────────────────────────────
		// NotifierID가 명시되지 않은 경우, 설정 파일에 정의된 기본 Notifier로 전송합니다.
		targetNotifier = s.defaultNotifier
		if targetNotifier == nil {
			s.runningMu.RUnlock()

			// 기본 Notifier가 없는 경우는 서비스 종료 또는 미실행 상태를 의미합니다.
			// (정상 시작 시에는 반드시 기본 Notifier가 설정되어야 합니다)
			applog.WithComponent(component).Warn("알림 전송 거부: 서비스 종료 또는 미실행 상태")

			return ErrServiceNotRunning
		}
	} else {
		// ─────────────────────────────────────────────────────────────────
		// Case B: 특정 Notifier 지정
		// ─────────────────────────────────────────────────────────────────
		// 요청자가 명시적으로 특정 Notifier를 지정한 경우입니다.
		var exists bool
		targetNotifier, exists = s.notifiers[notification.NotifierID]
		if !exists {
			defaultNotifier := s.defaultNotifier
			s.runningMu.RUnlock()

			// ─────────────────────────────────────────────────────────────
			// 에러 처리: 존재하지 않는 Notifier ID
			// ─────────────────────────────────────────────────────────────
			// 지정된 Notifier를 찾을 수 없는 경우, 이는 설정 오류이거나, 잘못된 NotifierID를 전달받은 것입니다.
			m := fmt.Sprintf("알림 전송 실패: 등록되지 않은 Notifier ID('%s')", notification.NotifierID)

			fields := applog.Fields{
				"notifier_id": notification.NotifierID,
			}
			if notification.TaskID != "" {
				fields["task_id"] = notification.TaskID
			}
			if notification.CommandID != "" {
				fields["command_id"] = notification.CommandID
			}
			if notification.Title != "" {
				fields["title"] = notification.Title
			}
			applog.WithComponentAndFields(component, fields).Error(m)

			// 기본 Notifier로 에러 상황을 알립니다.
			if defaultNotifier != nil {
				errorNotification := contract.NewErrorNotification(m)
				if err := defaultNotifier.Send(ctx, errorNotification); err != nil {
					// 에러 알림 전송마저 실패한 경우 (대기열 포화 또는 서비스 종료 중)
					fields := applog.Fields{
						"notifier_id":         notification.NotifierID,
						"default_notifier_id": defaultNotifier.ID(),
						"error":               err,
					}
					if notification.TaskID != "" {
						fields["task_id"] = notification.TaskID
					}
					if notification.CommandID != "" {
						fields["command_id"] = notification.CommandID
					}
					if notification.Title != "" {
						fields["title"] = notification.Title
					}
					applog.WithComponentAndFields(component, fields).Warn("에러 알림 전송 실패: 기본 Notifier 대기열 포화 또는 서비스 종료")
				}
			}

			return ErrNotifierNotFound
		}
	}

	s.runningMu.RUnlock()

	// ========================================
	// 4단계: 알림 전송
	// ========================================
	if err := targetNotifier.Send(ctx, notification); err != nil {
		// Notifier가 이미 종료된(Closed) 경우 세부 상태를 확인하여 에러를 매핑합니다.
		if err == notifier.ErrClosed {
			// 1. 서비스 자체가 종료 중인 경우 -> ErrServiceNotRunning 반환
			s.runningMu.RLock()
			isServiceRunning := s.running
			s.runningMu.RUnlock()

			if !isServiceRunning {
				return ErrServiceNotRunning
			}

			// 2. 서비스는 실행 중인데 특정 Notifier만 종료된 경우 -> ErrNotifierUnavailable 반환
			// (패닉 복구 또는 내부 오류로 인해 해당 Notifier만 중단된 상황)
			return ErrNotifierUnavailable
		}

		return err
	}

	return nil
}

// Health 시스템이 정상적으로 동작 중인지 검사합니다.
func (s *Service) Health() error {
	s.runningMu.RLock()
	defer s.runningMu.RUnlock()

	if !s.running {
		return ErrServiceNotRunning
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
