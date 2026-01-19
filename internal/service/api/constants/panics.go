package constants

// 시스템 시작/구동 시 발생할 수 있는 크리티컬한 패닉 메시지 상수입니다.
const (
	// PanicMsgAppConfigRequired 패닉 메시지: AppConfig 필수
	PanicMsgAppConfigRequired = "AppConfig는 필수입니다"

	// PanicMsgNotificationSenderRequired 패닉 메시지: NotificationSender 필수
	PanicMsgNotificationSenderRequired = "NotificationSender는 필수입니다"

	// PanicMsgHealthCheckerRequired 패닉 메시지: HealthChecker 필수
	PanicMsgHealthCheckerRequired = "HealthChecker는 필수입니다"
)
