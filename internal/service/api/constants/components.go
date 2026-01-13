package constants

// 로그 발생 위치(컴포넌트) 식별을 위한 상수입니다.
const (
	// ComponentService 서비스 컴포넌트 이름
	ComponentService = "api.service"

	// ComponentHandler 핸들러 컴포넌트 이름
	ComponentHandler = "api.handler"

	// ComponentMiddlewareAuthentication 인증 미들웨어 컴포넌트 이름
	ComponentMiddlewareAuthentication = "api.middleware.auth"

	// ComponentMiddlewareRateLimit 속도 제한 미들웨어 컴포넌트 이름
	ComponentMiddlewareRateLimit = "api.middleware.rate_limit"

	// ComponentMiddlewarePanicRecovery 패닉 복구 미들웨어 컴포넌트 이름
	ComponentMiddlewarePanicRecovery = "api.middleware.panic_recovery"

	// ComponentMiddlewareDeprecated Deprecated 미들웨어 컴포넌트 이름
	ComponentMiddlewareDeprecated = "api.middleware.deprecated"

	// ComponentMiddlewareContentType Content-Type 검증 미들웨어 컴포넌트 이름
	ComponentMiddlewareContentType = "api.middleware.content_type"

	// ComponentErrorHandler 에러 핸들러 컴포넌트 이름
	ComponentErrorHandler = "api.error_handler"
)
