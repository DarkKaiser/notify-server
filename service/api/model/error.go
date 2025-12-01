package model

// ErrorResponse 에러 응답 모델
type ErrorResponse struct {
	// 에러 메시지
	Message string `json:"message" example:"에러가 발생했습니다."`
}
