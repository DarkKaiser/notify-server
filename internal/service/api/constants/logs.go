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
)
