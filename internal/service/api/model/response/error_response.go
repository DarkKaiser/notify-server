package response

// ErrorResponse API 오류 응답
type ErrorResponse struct {
	// 오류 상세 메시지
	Message string `json:"message" example:"APP_KEY가 유효하지 않습니다.(ID:my-app)"`
}
