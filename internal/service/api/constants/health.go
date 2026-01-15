package constants

// 헬스체크 및 시스템 상태 관련 상수입니다.
const (
	// ------------------------------------------------------------------------------------------------
	// 헬스체크 상태
	// ------------------------------------------------------------------------------------------------

	// HealthStatusHealthy 헬스체크 상태: 정상
	HealthStatusHealthy = "healthy"

	// HealthStatusUnhealthy 헬스체크 상태: 비정상
	HealthStatusUnhealthy = "unhealthy"

	// ------------------------------------------------------------------------------------------------
	// 외부 의존성 상태
	// ------------------------------------------------------------------------------------------------

	// DependencyNotificationService 외부 의존성 ID: 알림 서비스
	DependencyNotificationService = "notification_service"

	// MsgDepStatusHealthy 외부 의존성 상태: 정상
	MsgDepStatusHealthy = "정상 작동 중"

	// MsgDepStatusNotInitialized 외부 의존성 상태: 미초기화
	MsgDepStatusNotInitialized = "서비스가 초기화되지 않음"
)
