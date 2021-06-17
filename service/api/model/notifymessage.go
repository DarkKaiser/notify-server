package model

type NotifyMessage struct {
	ApplicationID string `json:"application_id" form:"application_id" query:"application_id"`
	Message       string `json:"message" form:"message" query:"message"`
	ErrorOccurred bool   `json:"error_occurred" form:"error_occurred" query:"error_occurred"`
}
