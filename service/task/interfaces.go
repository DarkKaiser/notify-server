package task

// Runner 작업을 실행하는 인터페이스입니다.
type Runner interface {
	// Run 작업을 실행합니다. 실행 성공 여부(error)를 반환합니다.
	Run(req *RunRequest) error
}

// Canceler 실행 중인 작업을 취소하는 인터페이스입니다.
type Canceler interface {
	// Cancel 특정 작업 인스턴스를 취소합니다. 취소 성공 여부(error)를 반환합니다.
	Cancel(taskInstanceID InstanceID) error
}

// Executor 작업을 실행하고 취소할 수 있는 Combined 인터페이스입니다.
type Executor interface {
	Runner
	Canceler
}

// NotificationSender 작업(Task)의 실행 상태, 결과, 또는 에러 상황을 외부 알림 서비스로 전송하기 위한 추상화된 인터페이스입니다.
// 구현체는 Telegram, Slack, Email 등 구체적인 알림 수단과의 통신을 담당합니다.
type NotificationSender interface {
	// NotifyToDefault 사전에 정의된 기본 알림 채널(Default Notifier)로 메시지를 전송합니다.
	NotifyToDefault(message string) bool

	// NotifyWithTaskContext 지정된 NotifierID를 통해 메시지를 전송합니다.
	// TaskContext를 함께 전달하여 작업의 메타데이터(TaskID, Title 등)를 알림에 포함하거나 로깅에 활용할 수 있습니다.
	NotifyWithTaskContext(notifierID string, message string, taskCtx TaskContext) bool

	// SupportsHTMLMessage 지정된 Notifier가 HTML 포맷의 메시지 본문을 지원하는지 확인합니다.
	// 텍스트 스타일링(굵게, 링크 등)이 필요한 경우 이 메서드를 통해 지원 여부를 먼저 확인해야 합니다.
	SupportsHTMLMessage(notifierID string) bool
}
