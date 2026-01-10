package domain

// Application 애플리케이션 정보를 담는 도메인 모델입니다.
//
// 보안 고려사항:
//   - AppKey는 보안을 위해 이 구조체에 저장되지 않습니다.
//   - Authenticator에서 SHA-256 해시 형태로만 관리됩니다.
type Application struct {
	ID                string // 애플리케이션 고유 ID
	Title             string // 애플리케이션 제목
	Description       string // 애플리케이션 설명
	DefaultNotifierID string // 기본 알림 전송자 ID
}
