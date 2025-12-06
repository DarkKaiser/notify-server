package notification

import (
	"context"
	"sync"

	"github.com/darkkaiser/notify-server/service/task"
)

// NotifierID 알림 채널(Notifier)를 식별하는 고유 ID입니다.
type NotifierID string

// NotifierHandler 알림 채널(예: Telegram, Slack)의 공통 인터페이스입니다.
// 실제 알림 발송 로직은 이 인터페이스를 구현하여 정의합니다.
type NotifierHandler interface {
	ID() NotifierID

	// Notify 알림 발송 요청을 처리합니다.
	// 실제 발송은 비동기 큐를 통해 처리될 수 있습니다.
	//
	// 반환값:
	//   - succeeded: 요청이 정상적으로 접수되었는지 여부
	Notify(message string, taskCtx task.TaskContext) (succeeded bool)

	// Run Notifier의 메인 루프를 실행합니다.
	// 메시지 큐를 소비하여 실제 발송 작업을 수행합니다.
	Run(taskRunner task.TaskRunner, notificationStopCtx context.Context, notificationStopWaiter *sync.WaitGroup)

	SupportsHTMLMessage() bool
}

// NotificationSender 알림 발송 기능을 제공하는 인터페이스입니다.
// 외부 컴포넌트(API, 스케줄러 등)는 이 인터페이스를 통해 알림 서비스를 사용합니다.
type NotificationSender interface {
	Notify(notifierID string, title string, message string, errorOccurred bool) bool
	NotifyToDefault(message string) bool
	NotifyWithErrorToDefault(message string) bool
}
