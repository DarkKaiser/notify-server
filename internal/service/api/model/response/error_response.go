package response

// ErrorResponse API 오류 응답
type ErrorResponse struct {
	// ResultCode HTTP 상태 코드 (예: 400, 401, 500)
	ResultCode int `json:"result_code" example:"400"`

	// Message 에러 메시지
	Message string `json:"message" example:"APP_KEY가 유효하지 않습니다.(ID:my-app)"`
}
