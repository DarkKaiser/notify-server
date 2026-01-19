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
)

// Context 키 상수입니다.
const (
	// ContextKeyApplication 인증된 Application 객체 저장용 Context 키
	ContextKeyApplication = "authenticated_application"
)
