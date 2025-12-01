package model

// SuccessResponse 성공 응답 모델
type SuccessResponse struct {
	// 결과 코드 (0: 성공)
	ResultCode int `json:"result_code" example:"0"`
}
