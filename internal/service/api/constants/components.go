package constants

// 로깅 시 로그의 발생 위치(컴포넌트)를 식별하기 위한 상수입니다.
const (
	// ComponentHandler 핸들러 로그의 컴포넌트 이름입니다.
	ComponentHandler = "api.handler"

	// ComponentService 서비스 로그의 컴포넌트 이름입니다.
	ComponentService = "api.service"

	// ComponentMiddleware 미들웨어 로그의 컴포넌트 이름입니다.
	ComponentMiddleware = "api.middleware"

	// ComponentErrorHandler 에러 핸들러 로그의 컴포넌트 이름입니다.
	ComponentErrorHandler = "api.error_handler"
)
