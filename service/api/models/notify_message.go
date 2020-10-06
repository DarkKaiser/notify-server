package models

// @@@@@
type TemplateObject struct {
	ObjectType string `json:"object_type" form:"object_type" query:"object_type"`
	Content    struct {
		ID           string `json:"id" form:"id" query:"id"`
		Message      string `json:"message" form:"message" query:"message"`
		ErrorOccured bool   `json:"error_occured" form:"error_occured" query:"error_occured"`
		NotifierID   string `json:"notifier_id" form:"notifier_id" query:"notifier_id"`
	} `json:"content" form:"content" query:"content"`
}
