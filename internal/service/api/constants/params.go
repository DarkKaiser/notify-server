package constants

// URL 쿼리 파라미터 키 상수입니다.
const (
	// AppKeyQuery 애플리케이션 인증용 쿼리 파라미터 키
	AppKeyQuery = "app_key"
)

// HTTP 헤더 키 상수입니다.
const (
	// ------------------------------------------------------------------------------------------------
	// 인증
	// ------------------------------------------------------------------------------------------------

	// XAppKey 애플리케이션 인증용 HTTP 헤더 키 (권장 방식)
	XAppKey = "X-App-Key"

	// XApplicationID 애플리케이션 식별용 HTTP 헤더 키 (성능 최적화 및 GET 요청용)
	// 이 헤더가 존재하면 Body 파싱을 건너뛰고 헤더 값으로 인증합니다.
	XApplicationID = "X-Application-Id"

	// ------------------------------------------------------------------------------------------------
	// Deprecated 엔드포인트
	// ------------------------------------------------------------------------------------------------

	// Warning RFC 7234 표준 Warning 헤더 (deprecated 엔드포인트 경고용)
	Warning = "Warning"

	// XAPIDeprecated deprecated 상태 표시용 커스텀 헤더
	XAPIDeprecated = "X-API-Deprecated"

	// XAPIDeprecatedReplacement 대체 엔드포인트 표시용 커스텀 헤더
	XAPIDeprecatedReplacement = "X-API-Deprecated-Replacement"

	// ------------------------------------------------------------------------------------------------
	// Rate Limiting
	// ------------------------------------------------------------------------------------------------

	// RetryAfter RFC 7231 표준 Retry-After 헤더 (Rate Limiting 재시도 대기 시간)
	RetryAfter = "Retry-After"
)

// HTTP 헤더 값 상수입니다.
const (
	// RetryAfterSeconds Rate Limiting 초과 시 권장 재시도 대기 시간 (초)
	RetryAfterSeconds = "1"
)

// Context 키 상수입니다.
const (
	// ContextKeyApplication 인증된 Application 객체 저장용 Context 키
	ContextKeyApplication = "authenticated_application"
)
