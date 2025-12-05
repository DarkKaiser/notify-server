package response

// ErrorResponse API 요청 처리 중 오류가 발생했을 때 반환되는 표준 응답 모델입니다.
type ErrorResponse struct {
	// 오류에 대한 상세 메시지입니다.
	// 발생 원인과 해결을 위한 가이드를 포함할 수 있습니다.
	Message string `json:"message" example:"APP_KEY가 유효하지 않습니다.(ID:my-app)"`
}
