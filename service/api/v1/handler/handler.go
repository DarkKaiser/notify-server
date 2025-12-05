package handler

import (
	"github.com/darkkaiser/notify-server/service/api/auth"
	"github.com/darkkaiser/notify-server/service/notification"
)

// Handler API 요청을 처리하고 비즈니스 로직을 연결하는 역할을 담당합니다.
type Handler struct {
	applicationManager *auth.ApplicationManager

	notificationSender notification.NotificationSender
}

func NewHandler(applicationManager *auth.ApplicationManager, notificationSender notification.NotificationSender) *Handler {
	return &Handler{
		applicationManager: applicationManager,

		notificationSender: notificationSender,
	}
}
