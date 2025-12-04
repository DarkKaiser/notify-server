package model

// ErrorResponse 에러 응답 모델
type ErrorResponse struct {
	// 에러 메시지 (에러 발생 원인 및 해결 방법 포함)
	Message string `json:"message" example:"APP_KEY가 유효하지 않습니다.(ID:my-app)"`
}
