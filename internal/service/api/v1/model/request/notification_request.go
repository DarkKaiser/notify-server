package request

// NotificationRequest 알림 메시지 게시 요청
type NotificationRequest struct {
	// 인증에 사용할 애플리케이션 식별자
	ApplicationID string `json:"application_id" form:"application_id" query:"application_id" validate:"required" korean:"애플리케이션 ID" example:"my-app"`
	// 알림 메시지 본문 (Markdown 지원, 최대 4096자)
	Message string `json:"message" form:"message" query:"message" validate:"required,min=1,max=4096" korean:"메시지" example:"서버에서 중요한 이벤트가 발생했습니다."`
	// 에러 발생 여부
	ErrorOccurred bool `json:"error_occurred" form:"error_occurred" query:"error_occurred" korean:"에러 발생 여부" example:"false"`
}
