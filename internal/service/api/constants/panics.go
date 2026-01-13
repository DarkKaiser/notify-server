package constants

const (
	// PanicMsgAppConfigRequired 패닉 메시지: AppConfig 필수
	PanicMsgAppConfigRequired = "AppConfig는 필수입니다"

	// PanicMsgNotificationSenderRequired 패닉 메시지: NotificationSender 필수
	PanicMsgNotificationSenderRequired = "NotificationSender는 필수입니다"

	// PanicMsgAuthenticatorRequired 패닉 메시지: Authenticator 필수
	PanicMsgAuthenticatorRequired = "Authenticator는 필수입니다"

	// PanicMsgRateLimitRequestsPerSecondInvalid 패닉 메시지: requestsPerSecond 설정 오류
	PanicMsgRateLimitRequestsPerSecondInvalid = "RateLimiting: requestsPerSecond는 양수여야 합니다 (현재값: %d)"

	// PanicMsgRateLimitBurstInvalid 패닉 메시지: burst 설정 오류
	PanicMsgRateLimitBurstInvalid = "RateLimiting: burst는 양수여야 합니다 (현재값: %d)"

	// PanicMsgDeprecatedEndpointEmpty 패닉 메시지: 대체 엔드포인트 비어있음
	PanicMsgDeprecatedEndpointEmpty = "대체 엔드포인트 경로가 비어있습니다"

	// PanicMsgDeprecatedEndpointInvalidPrefix 패닉 메시지: 대체 엔드포인트 접두사 오류
	PanicMsgDeprecatedEndpointInvalidPrefix = "대체 엔드포인트 경로는 '/'로 시작해야 합니다 (현재값: %s)"
)
