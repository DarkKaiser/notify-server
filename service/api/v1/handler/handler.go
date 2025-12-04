package handler

import (
	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/notification"
)

// Handler API 요청을 처리하고 비즈니스 로직을 연결하는 역할을 담당합니다.
type Handler struct {
	applications map[string]*Application

	notificationSender notification.NotificationSender

	// 빌드 정보
	version     string
	buildDate   string
	buildNumber string
}

func NewHandler(appConfig *config.AppConfig, notificationSender notification.NotificationSender, version, buildDate, buildNumber string) *Handler {
	return &Handler{
		applications: loadApplicationsFromConfig(appConfig),

		notificationSender: notificationSender,

		version:     version,
		buildDate:   buildDate,
		buildNumber: buildNumber,
	}
}
