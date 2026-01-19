package constants

// 클라이언트에게 반환되는 메시지 상수입니다.
const (
	// ------------------------------------------------------------------------------------------------
	// 성공 응답 메시지
	// ------------------------------------------------------------------------------------------------

	MsgSuccess = "성공"

	// ------------------------------------------------------------------------------------------------
	// 일반 HTTP 에러 (상태 코드 순)
	// ------------------------------------------------------------------------------------------------

	// 400 Bad Request
	ErrMsgBadRequest            = "잘못된 요청입니다"
	ErrMsgBadRequestInvalidJSON = "잘못된 JSON 형식입니다"

	ErrMsgBadRequestEmptyBody      = "요청 본문이 비어있습니다"
	ErrMsgBadRequestBodyReadFailed = "요청 본문을 읽을 수 없습니다"

	// 401 Unauthorized
	ErrMsgUnauthorizedInvalidAppKey         = "app_key가 유효하지 않습니다 (application_id: %s)"
	ErrMsgUnauthorizedNotFoundApplicationID = "등록되지 않은 application_id입니다 (ID: %s)"

	// 404 Not Found
	ErrMsgNotFound         = "요청한 리소스를 찾을 수 없습니다"

	// 413 Request Entity Too Large
	ErrMsgRequestEntityTooLarge = "요청 본문이 너무 큽니다"

	// 415 Unsupported Media Type
	ErrMsgUnsupportedMediaType = "지원하지 않는 미디어 타입입니다"

	// 429 Too Many Requests
	ErrMsgTooManyRequests = "요청이 너무 많습니다. 잠시 후 다시 시도해주세요"

	// 500 Internal Server Error
	ErrMsgInternalServer            = "내부 서버 오류가 발생했습니다"

	// ------------------------------------------------------------------------------------------------
	// 인증 에러
	// ------------------------------------------------------------------------------------------------

	ErrMsgAuthAppKeyRequired        = "app_key는 필수입니다 (X-App-Key 헤더 또는 app_key 쿼리 파라미터)"
	ErrMsgAuthApplicationIDRequired = "application_id는 필수입니다"

	// 내부 인증 로직 에러
	ErrMsgAuthApplicationMissingInContext = "Context에서 애플리케이션 정보를 찾을 수 없습니다"
	ErrMsgAuthApplicationTypeMismatch     = "Context에 저장된 애플리케이션 정보의 타입이 올바르지 않습니다"
)
