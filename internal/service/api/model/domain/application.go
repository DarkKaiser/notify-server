// Package domain API 서비스의 핵심 도메인 모델을 정의합니다.
//
// 이 패키지는 config 패키지의 설정 구조체와는 별도로,
// 런타임에서 사용되는 보안 필터링된 도메인 엔티티를 제공합니다.
package domain

// Application 알림 API를 사용하는 클라이언트 애플리케이션을 나타내는 도메인 엔티티입니다.
//
// 이 구조체는 config.ApplicationConfig에서 보안 정보(AppKey)를 제거한
// 런타임 표현으로, 인증 후 핸들러에서 안전하게 사용됩니다.
//
// 보안 고려사항:
//   - AppKey는 보안을 위해 이 구조체에 저장되지 않습니다.
//   - Authenticator에서 SHA-256 해시 형태로만 관리됩니다.
//
// 사용 예시:
//
//	// 인증 미들웨어에서 Context에 저장
//	c.Set(constants.ContextKeyApplication, app)
//
//	// 핸들러에서 사용
//	app := c.Get(constants.ContextKeyApplication).(*domain.Application)
//	notifierID := app.DefaultNotifierID
type Application struct {
	ID                string // 애플리케이션 식별자 (인증 키)
	Title             string // 애플리케이션 이름
	Description       string // 애플리케이션 설명
	DefaultNotifierID string // 애플리케이션의 기본 알림 전송자 ID
}
