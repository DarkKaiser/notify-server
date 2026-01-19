package constants

// 내부 로깅을 위한 메시지 상수입니다.
const (
	// ------------------------------------------------------------------------------------------------
	// 서비스 생명주기
	// ------------------------------------------------------------------------------------------------

	LogMsgServiceStarting       = "API 서비스 시작중..."
	LogMsgServiceStarted        = "API 서비스 시작됨"
	LogMsgServiceAlreadyStarted = "API 서비스가 이미 시작됨!!!"
	LogMsgServiceStopping       = "API 서비스 중지중..."
	LogMsgServiceStopped        = "API 서비스 중지됨"
	LogMsgServiceUnexpectedExit = "API 서비스가 예기치 않게 종료되었습니다"

	LogMsgServiceHTTPServerStarting      = "API 서비스 > http 서버 시작"
	LogMsgServiceHTTPServerStopped       = "API 서비스 > http 서버 중지됨"
	LogMsgServiceHTTPServerShutdownError = "API 서비스 > http 서버 종료 중 오류 발생"
	LogMsgServiceHTTPServerFatalError    = "API 서비스 > http 서버를 구성하는 중에 치명적인 오류가 발생하였습니다."

	// ------------------------------------------------------------------------------------------------
	// 시스템 핸들러
	// ------------------------------------------------------------------------------------------------

	LogMsgHealthCheck = "헬스체크 조회"
	LogMsgVersionInfo = "버전 정보 조회"

	// ------------------------------------------------------------------------------------------------
	// 미들웨어 및 보안
	// ------------------------------------------------------------------------------------------------

	LogMsgHTTPRequest    = "HTTP 요청"

	LogMsgAuthAppKeyInQuery        = "보안 경고: 쿼리 파라미터로 App Key 전달됨 (헤더 사용 권장)"
	LogMsgAuthFailedAppKeyMismatch = "인증 실패: 제공된 App Key가 올바르지 않습니다"

	LogMsgRateLimitExceeded      = "API 요청 속도 제한 초과 (차단됨)"
	LogMsgUnsupportedContentType = "지원하지 않는 Content-Type 형식 요청"
	LogMsgDeprecatedEndpointUsed = "경고: Deprecated 엔드포인트가 호출되었습니다"

	// ------------------------------------------------------------------------------------------------
	// HTTP 에러
	// ------------------------------------------------------------------------------------------------

	LogMsgHTTP4xxClientError = "HTTP 4xx: 클라이언트 요청 오류"
	LogMsgHTTP5xxServerError = "HTTP 5xx: 서버 내부 오류"
)
