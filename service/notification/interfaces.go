package notification

import (
	"context"
	"sync"

	"github.com/darkkaiser/notify-server/service/task"
)

// Service 알림 발송 기능을 제공하는 인터페이스입니다.
// 외부 컴포넌트(API, 스케줄러 등)는 이 인터페이스를 통해 알림 서비스를 사용합니다.
type Service interface {
	// NotifyWithTitle 지정된 NotifierID로 제목(Title)이 포함된 알림 메시지를 발송합니다.
	// 일반 메시지뿐만 아니라 제목을 명시하여 알림의 맥락을 명확히 전달할 수 있습니다.
	// errorOccurred 플래그를 통해 해당 알림이 오류 상황에 대한 것인지 명시할 수 있습니다.
	//
	// 파라미터:
	//   - notifierID: 메시지를 발송할 대상 Notifier의 고유 ID
	//   - title: 알림 메시지의 제목 (강조 표시 등에 활용)
	//   - message: 전송할 알림 메시지의 본문
	//   - errorOccurred: 오류 발생 여부 (true일 경우 오류 상황으로 처리되어 시각적 강조 등이 적용될 수 있음)
	//
	// 반환값:
	//   - bool: 발송 요청이 성공적으로 큐에 등록되었는지 여부 (실제 전송 결과와는 무관)
	NotifyWithTitle(notifierID string, title string, message string, errorOccurred bool) bool

	// NotifyDefault 시스템에 설정된 기본(Default) 알림 채널로 일반 메시지를 발송합니다.
	// 주로 시스템 전반적인 알림이나, 특정 대상을 지정하지 않은 일반적인 정보 전달에 사용됩니다.
	//
	// 파라미터:
	//   - message: 전송할 메시지 내용
	//
	// 반환값:
	//   - bool: 발송 요청이 성공적으로 큐에 등록되었는지 여부
	NotifyDefault(message string) bool

	// NotifyDefaultWithError 시스템 기본 알림 채널로 "오류" 성격의 알림 메시지를 발송합니다.
	// 시스템 내부 에러, 작업 실패 등 관리자의 주의가 필요한 긴급 상황 알림에 적합합니다.
	// 내부적으로 오류 플래그가 설정되어 발송되므로, 수신 측에서 이를 인지하여 처리할 수 있습니다.
	//
	// 파라미터:
	//   - message: 전송할 오류 메시지 내용
	//
	// 반환값:
	//   - bool: 발송 요청이 성공적으로 큐에 등록되었는지 여부
	NotifyDefaultWithError(message string) bool
}

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
	Run(executor task.Executor, notificationStopCtx context.Context, notificationStopWaiter *sync.WaitGroup)

	SupportsHTML() bool
}
