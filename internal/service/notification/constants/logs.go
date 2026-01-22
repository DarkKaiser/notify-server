package constants

// TODO 미완료

// 로그 메시지 상수 정의
const (
	// LogMsgNotificationServicePanicRecovered 패닉 로그: 서비스 내 개별 Notifier 실행 중 패닉 발생
	LogMsgNotificationServicePanicRecovered = "Notification 서비스 > 실행 중 패닉 복구됨"

	// --- Service Logs ---
	LogMsgServiceStarting          = "Notification 서비스 시작중..."
	LogMsgServiceAlreadyStarted    = "Notification 서비스가 이미 시작됨!!!"
	LogMsgNotifierRegistered       = "Notifier가 Notification 서비스에 등록됨"
	LogMsgServiceStarted           = "Notification 서비스 시작됨"
	LogMsgServiceStopping          = "Notification 서비스 중지중..."
	LogMsgServiceStopped           = "Notification 서비스가 중지된 상태여서 메시지를 전송할 수 없습니다"
	LogMsgServiceStopCompleted     = "Notification 서비스 중지됨"
	LogMsgNotifierNotStopped       = "Notifier가 아직 종료되지 않았습니다 (비정상 상황)"
	LogMsgNotifierNotFoundRejected = "등록되지 않은 Notifier ID('%s')입니다. 메시지 발송이 거부되었습니다. 원본 메시지: %s"
	LogMsgDefaultNotifierFailed    = "기본 Notifier로 에러 알림 전송 실패 (큐 가득 참 또는 종료됨)"

	// --- Telegram Logs ---
	LogMsgTelegramSenderStarted   = "Sender 고루틴이 정상 종료됨" // 의미상 Stopped가 맞으나 원본 텍스트 유지/확인 필요. 원본: "Sender 고루틴이 정상 종료됨" -> LogMsgTelegramSenderStopped
	LogMsgTelegramRateLimitCancel = "RateLimiter 대기 중 컨텍스트 취소됨 (전송 중단)"
	LogMsgTelegramSendTimeout     = "알림 메시지 발송 시간 초과 (Timeout)"
	LogMsgTelegramSendSuccess     = "알림메시지 발송 성공"
	LogMsgTelegramSendFail        = "알림메시지 발송 실패"
	LogMsgTelegramHTMLFallback    = "HTML 파싱 오류 감지, 일반 텍스트로 전환하여 재시도합니다 (Fallback)"
	LogMsgTelegramCriticalError   = "치명적인 API 오류 발생, 재시도 중단"
	LogMsgTelegramRateLimitWait   = "Rate Limit 감지: 텔레그램 서버가 요청한 시간만큼 대기합니다."
	LogMsgTelegramRetryTimeout    = "알림 메시지 재시도 대기 중 시간 초과 (Timeout)"
	LogMsgTelegramSendFinalFail   = "알림메시지 발송 최종 실패"
)
