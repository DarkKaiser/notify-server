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

// API 요청 시 URL 쿼리 스트링으로 전달되는 파라미터 키 상수입니다.
const (
	// QueryParamAppKey 애플리케이션 인증에 사용되는 쿼리 파라미터 키입니다.
	QueryParamAppKey = "app_key"
)

// API 요청 및 응답에 사용되는 HTTP 헤더 키 상수입니다.
const (
	// HeaderAppKey 애플리케이션 인증에 사용되는 HTTP 헤더 키입니다.
	// 향후 쿼리 파라미터 대신 헤더 기반 인증으로 전환 시 사용됩니다.
	HeaderAppKey = "X-App-Key"
)

// 클라이언트에게 반환되는 표준 에러 메시지 상수입니다.
const (
	// ErrMsgBadRequest 400 Bad Request 에러 메시지입니다.
	ErrMsgBadRequest = "잘못된 요청입니다."

	// ErrMsgNotFound 404 Not Found 에러 메시지입니다.
	ErrMsgNotFound = "페이지를 찾을 수 없습니다."

	// ErrMsgInternalServer 500 Internal Server Error 메시지입니다.
	ErrMsgInternalServer = "내부 서버 오류가 발생했습니다."

	// ErrMsgAppKeyRequired app_key 파라미터가 누락되었을 때의 에러 메시지입니다.
	ErrMsgAppKeyRequired = "app_key는 필수입니다."
)

// 보안상 로그에 남길 때 마스킹(가림) 처리해야 할 쿼리 파라미터 목록입니다.
var SensitiveQueryParams = []string{
	QueryParamAppKey,
	"api_key",
	"password",
	"token",
	"secret",
}
