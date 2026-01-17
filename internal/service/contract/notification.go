package contract

// NotificationSender 알림 발송 기능을 제공하는 인터페이스입니다.
// API, Task와 같은 클라이언트는 이 인터페이스를 통해 알림 서비스를 사용합니다.
type NotificationSender interface {
	// Notify 지정된 Notifier를 통해 알림 메시지를 발송합니다.
	// 작업 실행 컨텍스트를 함께 전달하여, 알림 수신자가 작업의 메타데이터(TaskID, Title, 실행 시간 등)를
	// 확인할 수 있도록 지원합니다.
	//
	// 파라미터:
	//   - ctx: 작업 실행 컨텍스트 정보
	//   - notifierID: 알림을 발송할 대상 Notifier의 식별자
	//   - message: 전송할 메시지 내용
	//
	// 반환값:
	//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환
	Notify(ctx TaskContext, notifierID NotifierID, message string) error

	// NotifyWithTitle 지정된 Notifier를 통해 제목을 포함한 알림 메시지를 발송합니다.
	// 제목을 명시하여 알림의 맥락을 명확히 전달할 수 있습니다.
	// errorOccurred 플래그를 통해 해당 알림이 오류 상황에 대한 것인지 명시할 수 있습니다.
	//
	// 파라미터:
	//   - notifierID: 알림을 발송할 대상 Notifier의 식별자
	//   - title: 알림 메시지의 제목
	//   - message: 전송할 메시지 내용
	//   - errorOccurred: 오류 발생 여부
	//
	// 반환값:
	//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환 (ErrServiceStopped, ErrNotFoundNotifier 등)
	NotifyWithTitle(notifierID NotifierID, title string, message string, errorOccurred bool) error

	// NotifyDefault 시스템에 설정된 기본 Notifier를 통해 알림 메시지를 발송합니다.
	//
	// 파라미터:
	//   - message: 전송할 메시지 내용
	//
	// 반환값:
	//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환
	NotifyDefault(message string) error

	// NotifyDefaultWithError 시스템에 설정된 기본 Notifier를 통해 "오류" 성격의 알림 메시지를 발송합니다.
	// 시스템 내부 에러, 작업 실패 등 관리자의 주의가 필요한 긴급 상황 알림에 적합합니다.
	// 내부적으로 오류 플래그가 설정되어 발송되므로, 수신 측에서 이를 인지하여 처리할 수 있습니다.
	//
	// 파라미터:
	//   - message: 전송할 오류 메시지 내용
	//
	// 반환값:
	//   - error: 발송 요청이 정상적으로 큐에 등록(실제 전송 결과와는 무관)되면 nil, 실패 시 에러 반환
	NotifyDefaultWithError(message string) error

	// SupportsHTML 지정된 ID의 Notifier가 HTML 형식을 지원하는지 여부를 반환합니다.
	//
	// 파라미터:
	//   - notifierID: 지원 여부를 확인할 Notifier의 식별자
	//
	// 반환값:
	//   - bool: HTML 포맷 지원 여부
	SupportsHTML(notifierID NotifierID) bool
}

// NotificationHealthChecker Notification 서비스의 상태를 확인하는 인터페이스입니다.
type NotificationHealthChecker interface {
	// Health 서비스가 정상적으로 실행 중인지 확인합니다.
	//
	// 반환값:
	//   - error: 서비스가 정상 동작 중이면 nil, 그렇지 않으면 에러 반환 (예: ErrServiceStopped)
	Health() error
}
