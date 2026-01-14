package constants

const (
	// LogMsgNotificationServicePanicRecovered 패닉 로그: 서비스 내 개별 Notifier 실행 중 패닉 발생
	LogMsgNotificationServicePanicRecovered = "Notification 서비스 > 실행 중 패닉 복구됨"

	// LogMsgNotifierPanicRecovered 패닉 로그: BaseNotifier 알림 전송 중 패닉 발생
	LogMsgNotifierPanicRecovered = "Notifier > 알림 전송 중 패닉 복구됨"

	// LogMsgTelegramSenderPanicRecovered 패닉 로그: 텔레그램 Sender 루프 중 패닉 발생
	LogMsgTelegramSenderPanicRecovered = "Telegram Notifier > Sender 루프 패닉 복구됨"

	// LogMsgTelegramNotifyPanicRecovered 패닉 로그: 텔레그램 알림 발송 중 패닉 발생
	LogMsgTelegramNotifyPanicRecovered = "Telegram Notifier > 알림 발송 중 패닉 복구됨"

	// LogMsgTelegramDrainPanicRecovered 패닉 로그: 텔레그램 종료 시 Drain 중 패닉 발생
	LogMsgTelegramDrainPanicRecovered = "Telegram Notifier > 종료(Drain) 처리 중 패닉 복구됨"

	// LogMsgTelegramCommandPanicRecovered 패닉 로그: 텔레그램 명령어 처리 중 패닉 발생
	LogMsgTelegramCommandPanicRecovered = "Telegram Notifier > 명령어 처리 중 패닉 복구됨"
)
