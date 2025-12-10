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

// NotificationSender Task 실행 중 발생하는 다양한 이벤트(시작, 성공, 실패 등)를 외부로 알리기 위한 인터페이스입니다.
// Task 로직은 이 인터페이스를 통해 구체적인 알림 수단(Telegram, Email, Slack 등)의 구현 상세에 의존하지 않고
// 추상화된 방식으로 메시지를 전달합니다. 이를 통해 알림 채널의 유연한 교체와 확장이 가능해집니다.
type NotificationSender interface {
	// Notify 지정된 NotifierID를 통해 알림 메시지를 전송합니다.
	// Task의 실행 컨텍스트(TaskContext)를 함께 전달하여, 알림 수신자가 작업의 메타데이터(TaskID, Title, 실행 시간 등)를
	// 확인할 수 있도록 지원합니다. 메시지 형식은 Notifier 구현체에 따라 달라질 수 있습니다.
	//
	// 파라미터:
	//   - notifierID: 메시지를 발송할 대상 Notifier의 고유 ID
	//   - message: 전송할 알림 메시지 본문
	//   - taskCtx: 작업 실행 컨텍스트 정보 (필수)
	//
	// 반환값:
	//   - bool: 발송 요청이 성공적으로 처리되었는지 여부
	Notify(notifierID string, message string, taskCtx TaskContext) bool

	// NotifyDefault 시스템 기본 알림 채널로 일반 메시지를 발송합니다.
	// 특정 Notifier를 지정하지 않고, 시스템 설정에 정의된 기본 채널(예: 운영자 공통 채널)로
	// 알림을 보내야 할 때 사용합니다.
	//
	// 파라미터:
	//   - message: 전송할 메시지 내용
	//
	// 반환값:
	//   - bool: 발송 요청이 성공적으로 처리되었는지 여부
	NotifyDefault(message string) bool

	// SupportsHTML 지정된 Notifier가 HTML 포맷의 메시지 본문을 지원하는지 확인합니다.
	// 마크다운이나 텍스트 스타일링(굵게, 기울임, 링크 등)이 포함된 메시지를 전송하기 전에,
	// 해당 Notifier가 이를 올바르게 렌더링할 수 있는지 검사하는 용도로 사용됩니다.
	//
	// 파라미터:
	//   - notifierID: 지원 여부를 확인할 Notifier의 ID
	//
	// 반환값:
	//   - bool: HTML 포맷 지원 여부 (true: 지원함, false: 텍스트로만 처리됨)
	SupportsHTML(notifierID string) bool
}
