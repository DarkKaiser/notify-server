package notifier

import (
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
	id NotifierID

	supportsHTML bool

	RequestC chan *NotifyRequest
}

// NewBaseNotifier BaseNotifier를 생성하고 초기화합니다.
func NewBaseNotifier(id NotifierID, supportsHTML bool, bufferSize int) BaseNotifier {
	return BaseNotifier{
		id: id,

		supportsHTML: supportsHTML,

		RequestC: make(chan *NotifyRequest, bufferSize),
	}
}

func (n *BaseNotifier) ID() NotifierID {
	return n.id
}

// Notify 메시지를 큐에 등록하여 비동기 발송을 요청합니다.
// 전송 중 패닉이 발생해도 recover하여 서비스 안정성을 유지합니다.
func (n *BaseNotifier) Notify(taskCtx task.TaskContext, message string) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			applog.WithComponentAndFields("notification.service", applog.Fields{
				"notifier_id":    n.ID(),
				"message_length": len(message),
				"panic":          r,
			}).Error("알림메시지 발송중에 panic 발생")
		}
	}()

	// 채널이 닫혔거나 초기화되지 않은 경우
	if n.RequestC == nil {
		return false
	}

	// 채널이 가득 찬 경우 대기하지 않고 즉시 false 반환
	select {
	case n.RequestC <- &NotifyRequest{
		TaskCtx: taskCtx,
		Message: message,
	}:
		return true
	default:
		applog.WithComponentAndFields("notification.service", applog.Fields{
			"notifier_id": n.ID(),
		}).Warn("알림 채널 버퍼가 가득 차서 메시지를 전송할 수 없습니다 (Drop)")

		return false
	}
}

func (n *BaseNotifier) SupportsHTML() bool {
	return n.supportsHTML
}

// Close 알림 채널을 닫고 리소스를 정리합니다.
func (n *BaseNotifier) Close() {
	if n.RequestC != nil {
		close(n.RequestC)
		n.RequestC = nil
	}
}
