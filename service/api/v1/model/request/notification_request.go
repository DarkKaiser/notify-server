package request

// NotifyMessageRequest 알림 메시지 요청 모델
type NotifyMessageRequest struct {
	// 애플리케이션 ID
	ApplicationID string `json:"application_id" form:"application_id" query:"application_id" validate:"required" korean:"애플리케이션 ID" example:"my-app"`
	// 알림 메시지 내용 (최대 4096자, 마크다운 형식 지원)
	Message string `json:"message" form:"message" query:"message" validate:"required,min=1,max=4096" korean:"메시지" example:"서버에서 중요한 이벤트가 발생했습니다."`
	// 에러 발생 여부 (true인 경우 에러 알림으로 표시됨)
	ErrorOccurred bool `json:"error_occurred" form:"error_occurred" query:"error_occurred" korean:"에러 발생 여부" example:"false"`
}
