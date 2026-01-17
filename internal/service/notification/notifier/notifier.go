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

	// Notify 알림 발송 요청을 내부 버퍼(Queue)에 등록하고 즉시 반환합니다(Non-blocking).
	// 실제 전송은 Run() 메서드가 실행 중인 고루틴에서 비동기로 처리됩니다.
	//
	// 반환값 (ok):
	//   - true: 요청이 성공적으로 대기열에 등록됨.
	//   - false: 대기열이 가득 찼거나, Notifier가 종료되어 더 이상 요청을 받을 수 없음.
	Notify(taskCtx contract.TaskContext, message string) (ok bool)

	// SupportsHTML 알림 채널이 HTML 스타일의 메시지 포맷팅을 지원하는지 여부를 반환합니다.
	// true인 경우, 메시지 내용에 <b>, <i>, <a href="..."> 등의 태그를 사용할 수 있습니다.
	SupportsHTML() bool

	// Done Notifier의 모든 작업이 완료되고 안전하게 종료되었는지 확인할 수 있는 신호 채널을 반환합니다.
	// 반환된 채널이 닫히면, 더 이상 처리할 작업이 없고 리소스가 정리되었음을 의미합니다.
	Done() <-chan struct{}
}
