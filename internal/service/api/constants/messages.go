package constants

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
