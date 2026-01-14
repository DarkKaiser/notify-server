package notifier

import (
	"sync"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/notification/types"
	"github.com/darkkaiser/notify-server/internal/service/task"
	applog "github.com/darkkaiser/notify-server/pkg/log"
)

// NotifyRequest 내부 채널을 통해 전달되는 알림 데이터입니다.
type NotifyRequest struct {
	TaskCtx task.TaskContext
	Message string
}

// BaseNotifier NotifierHandler의 기본 구현체입니다.
// 공통적인 알림 채널 처리 로직을 제공하며, 구체적인 구현체에 임베딩되어 사용됩니다.
type BaseNotifier struct {
	id types.NotifierID

	supportsHTML bool

	mu     sync.RWMutex  // 채널 및 상태 보호를 위한 Mutex
	closed bool          // Notifier 종료 여부
	done   chan struct{} // 종료 신호 전파 채널

	notifyTimeout time.Duration
	RequestC      chan *NotifyRequest
}

// NewBaseNotifier BaseNotifier를 생성하고 초기화합니다.
func NewBaseNotifier(id types.NotifierID, supportsHTML bool, bufferSize int, notifyTimeout time.Duration) BaseNotifier {
	return BaseNotifier{
		id: id,

		supportsHTML: supportsHTML,

		notifyTimeout: notifyTimeout,
		RequestC:      make(chan *NotifyRequest, bufferSize),
		done:          make(chan struct{}),
	}
}

func (n *BaseNotifier) ID() types.NotifierID {
	return n.id
}

// Notify 메시지를 큐에 등록하여 비동기 발송을 요청합니다.
// 전송 중 패닉이 발생해도 recover하여 서비스 안정성을 유지합니다.
func (n *BaseNotifier) Notify(taskCtx task.TaskContext, message string) (succeeded bool) {
	n.mu.RLock()
	// 이미 종료되었거나 채널이 닫힌 경우
	if n.closed || n.RequestC == nil {
		n.mu.RUnlock()
		return false
	}
	// 채널 참조를 로컬 변수로 복사하여 락 해제 후에도 안전하게 접근
	requestC := n.RequestC
	done := n.done
	timeout := n.notifyTimeout
	n.mu.RUnlock()

	defer func() {
		if r := recover(); r != nil {
			applog.WithComponentAndFields("notification.service", applog.Fields{
				"notifier_id": n.ID(),
				"panic":       r,
			}).Error("알림 전송 요청 중 치명적인 오류(Panic)가 발생하여 복구되었습니다")
			succeeded = false
		}
	}()

	req := &NotifyRequest{
		TaskCtx: taskCtx,
		Message: message,
	}

	// 채널이 가득 찬 경우 설정된 Timeout만큼 대기 (Backpressure)
	// 중요: 락을 해제한 상태에서 채널 전송을 시도합니다.
	// 이를 통해 채널이 가득 차서 대기하는 동안에도 Close()가 호출되면
	// done 채널이 닫히고 select를 통해 즉시 종료될 수 있습니다.
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case requestC <- req:
		return true
	case <-done:
		// 전송 대기 중 Notifier가 종료된 경우
		return false
	case <-timer.C:
		// Timeout이 지날 때까지 대기열에 빈 공간이 생기지 않으면 Drop
		applog.WithComponentAndFields("notification.service", applog.Fields{
			"notifier_id": n.ID(),
		}).Warn("알림 채널 버퍼가 가득 차서 메시지를 전송할 수 없습니다 (Drop - Timeout)")

		return false
	}
}

func (n *BaseNotifier) SupportsHTML() bool {
	return n.supportsHTML
}

// Close 알림 채널을 닫고 리소스를 정리합니다.
func (n *BaseNotifier) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.closed {
		n.closed = true

		// 종료 신호 전파: Notify 메서드에서 대기 중인 모든 고루틴을 깨웁니다.
		if n.done != nil {
			close(n.done)
		}

		// RequestC는 닫지 않습니다.
		// 채널을 닫으면 Notify()에서 경쟁 상태로 인한 패닉이 발생할 수 있습니다.
		// GC가 참조되지 않는 채널을 자동으로 정리하므로 누수 문제는 없습니다.
		// 컨슈머는 done 채널이나 Context 종료를 감지하여 루프를 탈출해야 합니다.
	}
}

// Done Notifier가 종료되었는지 확인할 수 있는 채널을 반환합니다.
// 반환된 채널이 닫히면 Notifier가 종료(Close)된 상태임을 의미합니다.
func (n *BaseNotifier) Done() <-chan struct{} {
	return n.done
}
