package model

// NotifyMessage 알림 메시지 요청 모델
type NotifyMessage struct {
	// 애플리케이션 ID
	ApplicationID string `json:"application_id" form:"application_id" query:"application_id" example:"my-app"`
	// 알림 메시지 내용
	Message       string `json:"message" form:"message" query:"message" example:"테스트 메시지입니다."`
	// 에러 발생 여부
	ErrorOccurred bool   `json:"error_occurred" form:"error_occurred" query:"error_occurred" example:"false"`
}
