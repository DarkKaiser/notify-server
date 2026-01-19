package constants

// 로그 발생 위치(컴포넌트) 식별을 위한 상수입니다.
const (
	// Service 서비스 컴포넌트 이름
	Service = "api.service"

	// Handler 핸들러 컴포넌트 이름
	Handler = "api.handler"

	// MiddlewareAuth 인증 미들웨어 컴포넌트 이름
	MiddlewareAuth = "api.middleware.auth"

	// MiddlewareDeprecated Deprecated 미들웨어 컴포넌트 이름
	MiddlewareDeprecated = "api.middleware.deprecated"

	// MiddlewareContentType Content-Type 검증 미들웨어 컴포넌트 이름
	MiddlewareContentType = "api.middleware.content_type"

	// ErrorHandler 에러 핸들러 컴포넌트 이름
	ErrorHandler = "api.error_handler"
)
