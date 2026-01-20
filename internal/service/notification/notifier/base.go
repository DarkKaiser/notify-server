package notifier

import (
	"sync"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"

	applog "github.com/darkkaiser/notify-server/pkg/log"
)

// Notification 알림 발송 요청 정보를 담고 있는 데이터 구조체입니다.
//
// Service.Notify 메서드를 통해 접수된 알림 요청은 이 구조체로 포장되어,
// 내부 채널(notificationC)을 통해 비동기적으로 각 Notifier의 Sender 고루틴에게 전달됩니다.
//
// 이 구조체는 실제 알림 메시지뿐만 아니라, 해당 알림이 어떤 작업(Task)과 연관되었는지
// 추적할 수 있는 컨텍스트 정보(TaskContext)를 함께 포함합니다.
type Notification struct {
	TaskContext contract.TaskContext
	Message     string
}

// Base 모든 Notifier 구현체가 공통적으로 상속(임베딩)받아 사용하는 기본 구조체입니다.
//
// 구체적인 Notifier 구현체(예: telegramNotifier)는 이 Base를 필드로 포함함으로써,
// "알림을 큐에 넣고 관리하는 책임"을 Base에 위임하고, "실제로 외부 API를 호출하는 책임"에만 집중할 수 있습니다.
type Base struct {
	id contract.NotifierID

	supportsHTML bool

	// mu 내부 상태(closed, done, notificationC)와 채널 접근 시 발생하는 경쟁 상태를 방지하기 위한 동기화 객체입니다.
	// 상태를 읽기만 할 때는 RLock을, 변경할 때는 Lock을 사용하여 성능과 안전성을 최적화합니다.
	mu sync.RWMutex

	// closed Close() 메서드가 호출되어 Notifier가 영구적으로 종료되었는지를 나타내는 상태 플래그입니다.
	// 이 값이 true가 되면, 더 이상 새로운 알림 요청을 수락하지 않고 거부합니다.
	closed bool

	// done Notifier의 종료 이벤트를 모든 대기중인 고루틴에게 안전하게 전파(Broadcast)하기 위한 신호 채널입니다.
	// 채널이 닫히는 것 자체가 신호로 사용되며, 별도의 데이터를 전달하지 않으므로 struct{} 타입을 사용합니다.
	done chan struct{}

	// notificationC 알림 발송 요청들을 순차적으로 처리하기 위해 버퍼링하는 내부 채널(Queue)입니다.
	// 비동기 처리를 통해 요청자는 즉시 리턴받고, 발송자(Sender)는 자신의 속도에 맞춰 메시지를 가져갑니다.
	notificationC chan *Notification

	// enqueueTimeout 요청 큐(notificationC)가 가득 찼을 때, 요청을 바로 버리지 않고 기다려줄 최대 시간입니다.
	// 이 시간 동안에도 빈 공간이 생기지 않으면, 시스템 보호를 위해 해당 요청은 드롭(Drop)됩니다.
	enqueueTimeout time.Duration
}

// NewBase 새로운 Base Notifier 인스턴스를 생성하고 초기화합니다.
func NewBase(id contract.NotifierID, supportsHTML bool, bufferSize int, enqueueTimeout time.Duration) Base {
	return Base{
		id: id,

		supportsHTML: supportsHTML,

		done: make(chan struct{}),

		notificationC: make(chan *Notification, bufferSize),

		enqueueTimeout: enqueueTimeout,
	}
}

// ID Notifier 인스턴스의 고유 식별자(ID)를 반환합니다.
func (n *Base) ID() contract.NotifierID {
	return n.id
}

// Send 알림 발송 요청을 내부 큐(채널)에 안전하게 등록합니다.
//
// 이 메서드는 실제 발송을 수행하지 않고, 요청을 메모리 큐에 넣는 역할만 수행하므로 매우 빠르게 리턴됩니다.
//
// 파라미터:
//   - taskCtx: 알림과 연관된 작업 컨텍스트 (TaskID, Title 등)
//   - message: 전송할 알림 메시지 본문
//
// 반환값:
//   - error: 성공 시 nil, 실패 시 에러 반환 (ErrQueueFull, ErrClosed 등)
func (n *Base) Send(taskCtx contract.TaskContext, message string) (err error) {
	n.mu.RLock()

	// 1. 종료 상태 확인
	// 이미 Close()가 호출되었거나 채널이 초기화되지 않았다면 요청을 거부합니다.
	if n.closed || n.notificationC == nil {
		n.mu.RUnlock()
		return ErrClosed
	}

	// 2. 로컬 변수 복사
	// 채널 전송은 블로킹될 수 있는 작업이므로, 락을 잡은 상태에서 수행하면 성능 병목이 됩니다.
	// 따라서 필요한 멤버 변수들(notificationC, done, timeout)만 로컬 변수로 복사해두고,
	// 락은 즉시 해제하여 다른 고루틴들이 상태를 조회하거나 변경할 수 있게 합니다.
	done := n.done
	notificationC := n.notificationC
	enqueueTimeout := n.enqueueTimeout

	n.mu.RUnlock()

	// 3. 패닉 복구
	// 혹시 모를 내부 로직 오류나 채널 이슈로 패닉이 발생해도, 서비스 전체가 죽지 않도록 방어합니다.
	defer func() {
		if r := recover(); r != nil {
			fields := applog.Fields{
				"notifier_id": n.ID(),
				"panic":       r,
			}
			if taskCtx != nil {
				if tid := taskCtx.GetTaskID(); tid != "" {
					fields["task_id"] = tid
				}
				if title := taskCtx.GetTitle(); title != "" {
					fields["task_title"] = title
				}
			}
			applog.WithComponentAndFields(component, fields).Error("Notifier > 알림 전송 처리 중 예기치 않은 패닉이 발생했으나, 서비스 유지를 위해 안전하게 복구되었습니다")

			// Panic 발생 시 에러 리턴
			err = ErrPanicRecovered
		}
	}()

	// 4. 요청 객체 생성
	req := &Notification{
		TaskContext: taskCtx,
		Message:     message,
	}

	// 5. 타이머 생성
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

	// 6. 작업 취소 감지 설정
	// 전달받은 작업 컨텍스트(taskCtx)가 있다면, 해당 작업이 취소되었는지 확인할 수 있도록 채널을 가져옵니다.
	// 없다면(nil), 취소 감지 기능은 동작하지 않습니다.
	var taskCtxDone <-chan struct{}
	if taskCtx != nil {
		taskCtxDone = taskCtx.Done()
	}

	// 7. 큐에 적재
	select {
	case notificationC <- req:
		// 성공: 큐에 정상적으로 등록됨
		return nil

	case <-done:
		// 실패: 대기 중에 Notifier가 종료됨 (Graceful Shutdown)
		// 락 없이 채널 대기를 하므로, Close()가 호출되면 즉시 감지하여 루프를 탈출할 수 있습니다.
		return ErrClosed

	case <-taskCtxDone:
		// 실패: 요청자(Caller)의 작업이 취소됨
		// 작업이 취소되었으므로 더 이상 알림을 큐에 넣을 필요가 없습니다.
		return ErrContextCanceled

	case <-timer.C:
		// 실패: 타임아웃 발생 (큐가 계속 가득 차 있음)
		// 시스템 보호를 위해 해당 요청을 드롭(Drop)하고 로그를 남깁니다.
		fields := applog.Fields{
			"notifier_id": n.ID(),
		}
		if taskCtx != nil {
			if tid := taskCtx.GetTaskID(); tid != "" {
				fields["task_id"] = tid
			}
			if title := taskCtx.GetTitle(); title != "" {
				fields["task_title"] = title
			}
		}
		applog.WithComponentAndFields(component, fields).Warn("알림 발송 대기열이 포화 상태에 도달하여, 시스템 보호를 위해 요청 처리를 건너뛰었습니다 (Queue Full)")

		return ErrQueueFull
	}
}

// Close Notifier의 운영을 중단하고 관련 리소스를 정리합니다.
//
// 이 메서드가 호출되면:
//  1. Notifier의 상태가 '종료됨(Closed)'으로 변경되어 더 이상의 새로운 Notify 요청을 받지 않습니다.
//  2. Done 채널이 닫혀서, 이를 구독하고 있는 모든 고루틴(Sender, Receiver 등)에게 종료 신호를 전파합니다.
//
// 참고: 내부 메시지 채널(notificationC)은 명시적으로 닫지 않습니다. 이는 다중 프로듀서(Multi-Producer) 환경에서
// 채널 닫기에 의한 패닉을 방지하기 위함이며, 남은 메시지는 GC에 의해 수거되거나 Drain 로직에 의해 처리됩니다.
func (n *Base) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.closed {
		n.closed = true

		// 1. 종료 신호 전파
		// done 채널을 닫음으로써, 이 채널을 구독하고 있는 모든 고루틴에 "시스템이 종료되었음"을 알립니다.
		if n.done != nil {
			close(n.done)
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
func (n *Base) Done() <-chan struct{} {
	return n.done
}

// NotificationC Notifier 내부에서 관리하는 '알림 요청 채널(읽기 전용)'을 반환합니다.
//
// 이 채널은 '발송자(Sender)'가 메시지를 하나씩 꺼내어 처리하기 위한 용도입니다.
// 외부에서는 오직 '읽기(<-chan)'만 가능하므로, 임의로 채널을 닫거나 데이터를 보낼 수 없습니다.
func (n *Base) NotificationC() <-chan *Notification {
	return n.notificationC
}

// SupportsHTML 알림 채널이 HTML 스타일의 메시지 포맷팅을 지원하는지 여부를 반환합니다.
// true인 경우, 메시지 내용에 <b>, <i>, <a href="..."> 등의 태그를 사용할 수 있습니다.
func (n *Base) SupportsHTML() bool {
	return n.supportsHTML
}
