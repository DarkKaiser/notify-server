package handler

import (
	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/notification"
)

// Handler
type Handler struct {
	applications map[string]*Application

	notificationSender notification.NotificationSender

	// 빌드 정보
	version     string
	buildDate   string
	buildNumber string
}

func NewHandler(appConfig *config.AppConfig, notificationSender notification.NotificationSender, version, buildDate, buildNumber string) *Handler {
	// 허용된 Application 목록을 구한다.
	return &Handler{
		applications: loadApplicationsFromConfig(appConfig),

		notificationSender: notificationSender,

		version:     version,
		buildDate:   buildDate,
		buildNumber: buildNumber,
	}
}

func loadApplicationsFromConfig(appConfig *config.AppConfig) map[string]*Application {
	applications := make(map[string]*Application)
	for _, application := range appConfig.NotifyAPI.Applications {
		applications[application.ID] = &Application{
			ID:                application.ID,
			Title:             application.Title,
			Description:       application.Description,
			DefaultNotifierID: application.DefaultNotifierID,
			AppKey:            application.AppKey,
		}
	}
	return applications
}
