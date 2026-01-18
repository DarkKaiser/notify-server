package notifier

import (
	"context"

	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// Notifier 다양한 알림 채널(예: 텔레그램, 슬랙 등)을 추상화한 인터페이스입니다.
type Notifier interface {
	// ID Notifier 인스턴스의 고유 식별자(ID)를 반환합니다.
	ID() contract.NotifierID

	// Run 알림 발송을 처리하는 백그라운드 워커(Consumer)를 실행합니다.
	// 이 메서드는 블로킹(Blocking)되며, 큐에 쌓인 알림 요청을 하나씩 꺼내어 실제로 전송하는 역할을 합니다.
	// Context가 취소되거나 내부에서 치명적인 에러가 발생하여 종료될 때까지 실행됩니다.
	Run(ctx context.Context)

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
	Send(taskCtx contract.TaskContext, message string) error

	// Done Notifier의 종료 상태를 감지할 수 있는 읽기 전용 채널을 반환합니다.
	//
	// 반환된 채널이 닫혔다면, 해당 Notifier가 Close() 호출에 의해 종료되었음을 의미합니다.
	// 주로 Select 구문 내에서 종료 시그널을 감지하여 고루틴을 안전하게 정리(Graceful Shutdown)하는 데 사용됩니다.
	Done() <-chan struct{}

	// SupportsHTML 알림 채널이 HTML 스타일의 메시지 포맷팅을 지원하는지 여부를 반환합니다.
	// true인 경우, 메시지 내용에 <b>, <i>, <a href="..."> 등의 태그를 사용할 수 있습니다.
	SupportsHTML() bool
}
