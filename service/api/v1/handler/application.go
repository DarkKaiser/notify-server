package handler

// Application API 사용이 허용된 외부 애플리케이션 정보
type Application struct {
	// 애플리케이션 식별자 (예: "my-app")
	ID string
	// 애플리케이션 이름 (예: "My Application")
	Title string
	// 애플리케이션 설명
	Description string
	// 기본 알림 전송 대상 ID (예: "telegram-bot-1")
	DefaultNotifierID string
	// API 인증 키
	AppKey string
}
