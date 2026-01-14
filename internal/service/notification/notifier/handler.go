package notifier

import (
	"context"

	"github.com/darkkaiser/notify-server/internal/service/notification/types"
	"github.com/darkkaiser/notify-server/internal/service/task"
)

// NotifierHandler 개별 알림 채널(예: Telegram, Slack) 구현을 위한 인터페이스입니다.
// Service는 이 인터페이스를 통해 다양한 알림 수단을 일관된 방식으로 관리하고 사용합니다.
// 주로 notification 패키지 내부에서 사용되나, 통합 테스트 및 외부 확장을 위해 공개되어 있습니다.
type NotifierHandler interface {
	ID() types.NotifierID

	// Run Notifier의 메인 루프를 실행합니다.
	// 메시지 큐를 소비하여 실제 발송 작업을 수행합니다.
	Run(notificationStopCtx context.Context)

	// Notify 알림 발송 요청을 처리합니다.
	// 실제 발송은 비동기 큐를 통해 처리될 수 있습니다.
	//
	// 반환값:
	//   - succeeded: 요청이 정상적으로 접수되었는지 여부
	Notify(taskCtx task.TaskContext, message string) (succeeded bool)

	SupportsHTML() bool
}
