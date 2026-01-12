package constants

// 클라이언트에게 반환되는 에러 메시지 상수입니다.
const (
	// ------------------------------------------------------------------------------------------------
	// 일반 HTTP 에러 (상태 코드 순)
	// ------------------------------------------------------------------------------------------------

	// ErrMsgBadRequest 400 Bad Request
	ErrMsgBadRequest = "잘못된 요청입니다"

	// ErrMsgNotFound 404 Not Found
	ErrMsgNotFound = "요청한 리소스를 찾을 수 없습니다"

	// ErrMsgRequestEntityTooLarge 413 Request Entity Too Large
	ErrMsgRequestEntityTooLarge = "요청 본문이 너무 큽니다"

	// ErrMsgUnsupportedMediaType 415 Unsupported Media Type
	ErrMsgUnsupportedMediaType = "지원하지 않는 미디어 타입입니다"

	// ErrMsgTooManyRequests 429 Too Many Requests
	ErrMsgTooManyRequests = "요청이 너무 많습니다. 잠시 후 다시 시도해주세요"

	// ErrMsgInternalServer 500 Internal Server Error
	ErrMsgInternalServer = "내부 서버 오류가 발생했습니다"

	// ------------------------------------------------------------------------------------------------
	// 요청 검증 에러
	// ------------------------------------------------------------------------------------------------

	// ErrMsgEmptyBody 빈 요청 본문
	ErrMsgEmptyBody = "요청 본문이 비어있습니다"

	// ErrMsgBodyReadFailed 요청 본문 읽기 실패
	ErrMsgBodyReadFailed = "요청 본문을 읽을 수 없습니다"

	// ErrMsgInvalidJSON 잘못된 JSON 형식
	ErrMsgInvalidJSON = "잘못된 JSON 형식입니다"

	// ------------------------------------------------------------------------------------------------
	// 인증 에러
	// ------------------------------------------------------------------------------------------------

	// ErrMsgAppKeyRequired app_key 누락 (X-App-Key 헤더 또는 app_key 쿼리 파라미터)
	ErrMsgAppKeyRequired = "app_key는 필수입니다 (X-App-Key 헤더 또는 app_key 쿼리 파라미터)"

	// ErrMsgApplicationIDRequired application_id 누락
	ErrMsgApplicationIDRequired = "application_id는 필수입니다"
)
