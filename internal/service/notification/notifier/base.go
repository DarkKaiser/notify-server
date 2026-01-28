package notifier

import (
	"context"
	"sync"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"

	applog "github.com/darkkaiser/notify-server/pkg/log"
)

// notificationRequest 알림 발송 요청 정보를 담고 있는 내부 데이터 구조체입니다.
//
// Service.Notify 메서드를 통해 접수된 알림 요청은 이 구조체로 포장되어,
// 내부 채널(notificationC)을 통해 비동기적으로 각 Notifier의 Sender 고루틴에게 전달됩니다.
//
// Go 관례상 context.Context를 구조체에 저장하는 것은 지양되지만,
// Worker Pool 패턴에서 채널을 통해 Context를 전달하기 위해 내부적으로만 사용하는 래퍼입니다.
type notificationRequest struct {
	Ctx          context.Context
	Notification contract.Notification
}

// Base 모든 Notifier 구현체가 공통적으로 상속(임베딩)받아 사용하는 기본 구조체입니다.
//
// 구체적인 Notifier 구현체(예: telegramNotifier)는 이 Base를 필드로 포함함으로써,
// "알림을 큐에 넣고 관리하는 책임"을 Base에 위임하고, "실제로 외부 API를 호출하는 책임"에만 집중할 수 있습니다.
type Base struct {
	id contract.NotifierID

	supportsHTML bool

	// enqueueTimeout 요청 큐(notificationC)가 가득 찼을 때, 요청을 바로 버리지 않고 기다려줄 최대 시간입니다.
	// 이 시간 동안에도 빈 공간이 생기지 않으면, 시스템 보호를 위해 해당 요청은 드롭(Drop)됩니다.
	enqueueTimeout time.Duration

	// notificationC 알림 발송 요청들을 순차적으로 처리하기 위해 버퍼링하는 내부 채널(Queue)입니다.
	// 비동기 처리를 통해 요청자는 즉시 리턴받고, 발송자(Sender)는 자신의 속도에 맞춰 메시지를 가져갑니다.
	notificationC chan *notificationRequest

	// mu 내부 상태(closed, done, notificationC)와 채널 접근 시 발생하는 경쟁 상태를 방지하기 위한 동기화 객체입니다.
	// 상태를 읽기만 할 때는 RLock을, 변경할 때는 Lock을 사용하여 성능과 안전성을 최적화합니다.
	mu sync.RWMutex

	// closed Close() 메서드가 호출되어 Notifier가 영구적으로 종료되었는지를 나타내는 상태 플래그입니다.
	// 이 값이 true가 되면, 더 이상 새로운 알림 요청을 수락하지 않고 거부합니다.
	closed bool

	// done Notifier의 종료 이벤트를 모든 대기중인 고루틴에게 안전하게 전파(Broadcast)하기 위한 신호 채널입니다.
	// 채널이 닫히는 것 자체가 신호로 사용되며, 별도의 데이터를 전달하지 않으므로 struct{} 타입을 사용합니다.
	done chan struct{}

	// pendingSendsWG Send() 또는 TrySend() 메서드를 통해 현재 채널 전송을 시도 중인 고루틴들을 추적하는 WaitGroup입니다.
	//
	// 이 필드는 Graceful Shutdown 시 메시지 유실을 방지하기 위한 핵심 동기화 장치입니다.
	// Close() 호출 후에도 이미 sendInternal()에 진입한 고루틴들이 채널에 메시지를 넣을 기회를 보장하기 위해,
	// 워커(Consumer) 고루틴은 종료 전 WaitForSenders()를 호출하여 이 카운터가 0이 될 때까지 대기합니다.
	pendingSendsWG sync.WaitGroup
}

// NewBase 새로운 Base Notifier 인스턴스를 생성하고 초기화합니다.
func NewBase(id contract.NotifierID, supportsHTML bool, bufferSize int, enqueueTimeout time.Duration) *Base {
	return &Base{
		id: id,

		supportsHTML: supportsHTML,

		enqueueTimeout: enqueueTimeout,

		notificationC: make(chan *notificationRequest, bufferSize),

		done: make(chan struct{}),
	}
}

// ID Notifier 인스턴스의 고유 식별자(ID)를 반환합니다.
func (b *Base) ID() contract.NotifierID {
	return b.id
}

// Send 알림 발송 요청을 내부 큐(채널)에 안전하게 등록합니다.
//
// 이 메서드는 실제 발송을 수행하지 않고, 요청을 메모리 큐에 넣는 역할만 수행하므로 매우 빠르게 리턴됩니다.
// 큐가 가득 찬 경우, 설정된 타임아웃(enqueueTimeout)만큼 대기합니다.
//
// 파라미터:
//   - ctx: 요청의 생명주기를 관리하는 컨텍스트
//   - notification: 전송할 알림 데이터
//
// 반환값:
//   - error: 성공 시 nil, 실패 시 에러 반환 (ErrQueueFull, ErrClosed 등)
func (b *Base) Send(ctx context.Context, notification contract.Notification) (err error) {
	req, notificationC, done, enqueueTimeout, cleanup, prepareErr := b.prepareSend(ctx, notification)
	if prepareErr != nil {
		return prepareErr
	}
	defer cleanup(&err)

	// 타이머 생성
	// 채널이 가득 찼을 때 무한정 대기하지 않고, 설정된 타임아웃(enqueueTimeout)만큼만 기다립니다.
	// 이는 시스템 과부하 시 요청을 "실패" 처리함으로써 전체 시스템의 응답성을 보호하는 Backpressure 역할을 합니다.
	timer := time.NewTimer(enqueueTimeout)
	defer func() {
		// 타이머 리소스 정리 가이드 (Go 공식 문서 권장사항):
		// timer.Stop()이 false를 반환하면 이미 타이머가 만료되어 시간 값(C)이 채널에 전송되었을 수 있습니다.
		// 이 경우 채널을 비워주지(Drain) 않으면 타이머 고루틴이 메모리에 남을 수 있으므로 비동기로 비워줍니다.
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	// 큐에 적재
	// 큐가 가득 찼을 때, 설정된 타임아웃(enqueueTimeout)까지 대기하며 빈 공간이 생기기를 기다립니다.
	// 타임아웃 내에 처리되지 않으면 포기(Drop)하여 시스템 전체의 지연을 방지합니다.
	select {
	case notificationC <- req:
		// 성공: 큐에 정상적으로 등록됨
		return nil

	case <-done:
		// 실패: 대기 중에 Notifier가 종료됨 (Graceful Shutdown)
		// 락 없이 채널 대기를 하므로, Close()가 호출되면 즉시 감지하여 루프를 탈출할 수 있습니다.
		return ErrClosed

	case <-ctx.Done():
		// 실패: 요청자(Caller)의 작업이 취소됨
		// 작업이 취소되었으므로 더 이상 알림을 큐에 넣을 필요가 없습니다.
		return ctx.Err()

	case <-timer.C:
		// 실패: 타임아웃 발생 (큐가 계속 가득 차 있음)
		// 시스템 보호를 위해 해당 요청을 드롭(Drop)하고 로그를 남깁니다.
		b.logQueueFull(notification)
		return ErrQueueFull
	}
}

// TrySend 알림 발송 요청을 내부 큐(채널)에 등록 시도합니다.
//
// Send와 달리, 큐가 가득 찼을 때 대기(Block)하지 않고 즉시 에러(ErrQueueFull)를 반환합니다.
// 빠른 응답이 중요하거나, 알림 유실이 허용되는 경우(예: 시스템 점유 상태 알림)에 사용합니다.
//
// 파라미터:
//   - ctx: 요청의 생명주기를 관리하는 컨텍스트
//   - notification: 전송할 알림 데이터
//
// 반환값:
//   - error: 성공 시 nil, 큐가 가득 찬 경우 즉시 ErrQueueFull 에러를 반환합니다.
func (b *Base) TrySend(ctx context.Context, notification contract.Notification) (err error) {
	req, notificationC, done, _, cleanup, prepareErr := b.prepareSend(ctx, notification)
	if prepareErr != nil {
		return prepareErr
	}
	defer cleanup(&err)

	// 큐에 적재
	// 채널이 가득 차 있어도 대기(Block)하지 않고, 즉시 ErrQueueFull을 반환합니다.
	select {
	case notificationC <- req:
		// 성공: 큐에 정상적으로 등록됨
		return nil

	case <-done:
		// 실패: 대기 중에 Notifier가 종료됨 (Graceful Shutdown)
		// 락 없이 채널 대기를 하므로, Close()가 호출되면 즉시 감지하여 루프를 탈출할 수 있습니다.
		return ErrClosed

	case <-ctx.Done():
		// 실패: 요청자(Caller)의 작업이 취소됨
		// 작업이 취소되었으므로 더 이상 알림을 큐에 넣을 필요가 없습니다.
		return ctx.Err()

	default:
		// 실패: 큐가 가득 차 있음 (즉시 리턴)
		b.logQueueFull(notification)
		return ErrQueueFull
	}
}

// prepareSend 알림 발송을 위한 사전 준비 작업을 수행하는 내부 헬퍼 메서드입니다.
//
// 이 메서드는 Send와 TrySend에서 공통으로 사용되며,
// 실제 큐 전송 전에 필요한 모든 검증과 리소스 확보를 담당합니다.
//
// 반환값:
//   - req: 큐에 전송할 알림 요청 객체
//   - notificationC: 알림 전송용 채널 (로컬 복사본)
//   - done: 종료 신호 채널 (로컬 복사본)
//   - enqueueTimeout: 블로킹 전송 시 사용할 타임아웃 값
//   - cleanup: 반드시 defer로 호출해야 하는 정리 함수 (WG.Done + Panic Recovery)
//   - err: 준비 과정에서 발생한 에러 (ErrClosed, context.Canceled 등)
func (b *Base) prepareSend(ctx context.Context, notification contract.Notification) (
	req *notificationRequest,
	notificationC chan *notificationRequest,
	done chan struct{},
	enqueueTimeout time.Duration,
	cleanup func(*error),
	err error,
) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 0. 알림 유효성 검증
	// Notifier 내부에서 직접 호출되는 경우(예: 텔레그램 봇 핸들러)에도 데이터 정합성을 보장하기 위해
	// 전송 전 반드시 유효성 검사를 수행합니다.
	if err := notification.Validate(); err != nil {
		return nil, nil, nil, 0, nil, err
	}

	// 1. 컨텍스트 취소 확인
	// 이미 취소된 컨텍스트인 경우 락 획득 등의 비용을 아끼고 즉시 종료합니다.
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, 0, nil, err
	}

	b.mu.RLock()

	// 2. 종료 상태 확인
	// 이미 Close()가 호출되었거나 채널이 초기화되지 않았다면 요청을 거부합니다.
	if b.closed || b.notificationC == nil {
		b.mu.RUnlock()
		return nil, nil, nil, 0, nil, ErrClosed
	}

	// 3. Pending Sends 카운터 증가
	b.pendingSendsWG.Add(1)

	// 4. 로컬 변수 복사
	// 채널 전송은 블로킹될 수 있는 작업이므로, 락을 잡은 상태에서 수행하면 성능 병목이 됩니다.
	// 따라서 필요한 멤버 변수들(notificationC, done, timeout)만 로컬 변수로 복사해두고,
	// 락은 즉시 해제하여 다른 고루틴들이 상태를 조회하거나 변경할 수 있게 합니다.
	done = b.done
	notificationC = b.notificationC
	enqueueTimeout = b.enqueueTimeout

	b.mu.RUnlock()

	// 5. 리소스 정리 및 패닉 복구용 함수
	cleanup = func(errPtr *error) {
		b.pendingSendsWG.Done()

		// 패닉 복구: 혹시 모를 내부 로직 오류나 채널 이슈로 패닉이 발생해도, 서비스 전체가 죽지 않도록 방어합니다.
		if r := recover(); r != nil {
			fields := applog.Fields{
				"notifier_id": b.ID(),
				"panic":       r,
			}
			if notification.TaskID != "" {
				fields["task_id"] = notification.TaskID
			}
			if notification.Title != "" {
				fields["task_title"] = notification.Title
			}
			applog.WithComponentAndFields(component, fields).Error("Notifier 패닉 복구: 알림 전송 중 예기치 않은 오류가 발생했습니다 (서비스 유지됨)")

			// Panic 발생 시 에러 리턴
			if errPtr != nil {
				*errPtr = ErrPanicRecovered
			}
		}
	}

	// 6. 요청 객체 생성
	req = &notificationRequest{
		Ctx:          ctx,
		Notification: notification,
	}

	return req, notificationC, done, enqueueTimeout, cleanup, nil
}

// logQueueFull 발송 대기열(Queue)이 가득 차서 알림 요청이 거부되었을 때 경고 로그를 남깁니다.
func (b *Base) logQueueFull(notification contract.Notification) {
	fields := applog.Fields{
		"notifier_id": b.ID(),
	}
	if notification.TaskID != "" {
		fields["task_id"] = notification.TaskID
	}
	if notification.Title != "" {
		fields["task_title"] = notification.Title
	}
	applog.WithComponentAndFields(component, fields).Warn("알림 요청 거부: 발송 대기열 용량 초과 (Queue Full)")
}

// Close Notifier의 운영을 중단하고 관련 리소스를 정리합니다.
//
// 이 메서드가 호출되면:
//  1. Notifier의 상태가 '종료됨(Closed)'으로 변경되어 더 이상의 새로운 Notify 요청을 받지 않습니다.
//  2. Done 채널이 닫혀서, 이를 구독하고 있는 모든 고루틴(Sender, Receiver 등)에게 종료 신호를 전파합니다.
//
// 참고: 내부 메시지 채널(notificationC)은 명시적으로 닫지 않습니다. 이는 다중 프로듀서(Multi-Producer) 환경에서
// 채널 닫기에 의한 패닉을 방지하기 위함이며, 남은 메시지는 GC에 의해 수거되거나 Drain 로직에 의해 처리됩니다.
func (b *Base) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.closed {
		b.closed = true

		// 1. 종료 신호 전파
		// done 채널을 닫음으로써, 이 채널을 구독하고 있는 모든 고루틴에 "시스템이 종료되었음"을 알립니다.
		if b.done != nil {
			close(b.done)
		}

		// 2. 데이터 채널(notificationC) 처리 전략
		// Go 채널의 특성상, "닫힌 채널에 데이터를 보내면 패닉(Panic)"이 발생합니다.
		// 여러 고루틴이 동시에 Notify()를 호출할 수 있는 환경(Multi-Producer)에서는,
		// 여기서 채널을 닫아버리면 타이밍 이슈로 인해 채널에 값을 보내는 순간 패닉이 터질 위험이 큽니다.
		//
		// 따라서 채널을 명시적으로 닫지 않고 그대로 둡니다.
		// - 큐에 남은 데이터: GC가 채널에 대한 참조가 사라지면 자동으로 메모리를 회수하므로 누수 걱정은 없습니다.
		// - 컨슈머 종료: 데이터 채널의 닫힘 여부(close)가 아니라, 위의 'done 채널'이나 'Context'를 통해 종료를 감지해야 합니다.
	}
}

// Done Notifier의 종료 상태를 감지할 수 있는 읽기 전용 채널을 반환합니다.
//
// 반환된 채널이 닫혔다면, 해당 Notifier가 Close() 호출에 의해 종료되었음을 의미합니다.
// 주로 Select 구문 내에서 종료 시그널을 감지하여 고루틴을 안전하게 정리(Graceful Shutdown)하는 데 사용됩니다.
func (b *Base) Done() <-chan struct{} {
	return b.done
}

// WaitForPendingSends 현재 진행 중인 모든 Send() 및 TrySend() 요청이 완료될 때까지 블로킹 대기합니다.
//
// 이 메서드는 Graceful Shutdown 시 메시지 유실을 방지하기 위해 워커(Consumer) 고루틴이 종료 직전에 호출합니다.
// Close()가 호출된 시점에 이미 sendInternal()에 진입한 고루틴들이 채널에 메시지를 전송할 기회를 보장하여,
// "채널 확인(Empty) → 종료 → Send(Push)" 순서로 발생하는 Race Condition을 방지합니다.
func (b *Base) WaitForPendingSends() {
	b.pendingSendsWG.Wait()
}

// NotificationC Notifier 내부에서 관리하는 '알림 요청 채널(읽기 전용)'을 반환합니다.
//
// 이 채널은 '발송자(Sender)'가 메시지를 하나씩 꺼내어 처리하기 위한 용도입니다.
// 외부에서는 오직 '읽기(<-chan)'만 가능하므로, 임의로 채널을 닫거나 데이터를 보낼 수 없습니다.
func (b *Base) NotificationC() <-chan *notificationRequest {
	return b.notificationC
}

// SupportsHTML 알림 채널이 HTML 스타일의 메시지 포맷팅을 지원하는지 여부를 반환합니다.
// true인 경우, 메시지 내용에 <b>, <i>, <a href="..."> 등의 태그를 사용할 수 있습니다.
func (b *Base) SupportsHTML() bool {
	return b.supportsHTML
}
