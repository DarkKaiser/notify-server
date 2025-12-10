package notification

import (
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/service/task"
	log "github.com/sirupsen/logrus"
)

// notifyRequest 내부 채널을 통해 전달되는 알림 데이터입니다.
type notifyRequest struct {
	message string
	taskCtx task.TaskContext
}

// notifier NotifierHandler의 기본 구현체입니다.
// 공통적인 알림 채널 처리 로직을 제공하며, 구체적인 구현체에 임베딩되어 사용됩니다.
type notifier struct {
	id NotifierID

	supportsHTML bool

	requestC chan *notifyRequest
}

// NewNotifier Notifier를 생성하고 초기화합니다.
func NewNotifier(id NotifierID, supportsHTML bool, bufferSize int) notifier {
	return notifier{
		id: id,

		supportsHTML: supportsHTML,

		requestC: make(chan *notifyRequest, bufferSize),
	}
}

func (n *notifier) ID() NotifierID {
	return n.id
}

// Notify 메시지를 큐에 등록하여 비동기 발송을 요청합니다.
// 전송 중 패닉이 발생해도 recover하여 서비스 안정성을 유지합니다.
func (n *notifier) Notify(taskCtx task.TaskContext, message string) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			applog.WithComponentAndFields("notification.service", log.Fields{
				"notifier_id":    n.ID(),
				"message_length": len(message),
				"panic":          r,
			}).Error("알림메시지 발송중에 panic 발생")
		}
	}()

	// 채널이 닫혔거나 초기화되지 않은 경우
	if n.requestC == nil {
		return false
	}

	n.requestC <- &notifyRequest{
		message: message,
		taskCtx: taskCtx,
	}

	return true
}

func (n *notifier) SupportsHTML() bool {
	return n.supportsHTML
}

// Close 알림 채널을 닫고 리소스를 정리합니다.
func (n *notifier) Close() {
	if n.requestC != nil {
		close(n.requestC)
		n.requestC = nil
	}
}
