package handler

import (
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/api/model"
	"github.com/darkkaiser/notify-server/service/notification"
)

// Handler
type Handler struct {
	allowedApplications []*model.AllowedApplication

	notificationSender notification.NotificationSender

	// 빌드 정보
	version     string
	buildDate   string
	buildNumber string
}

func NewHandler(appConfig *g.AppConfig, notificationSender notification.NotificationSender, version, buildDate, buildNumber string) *Handler {
	// 허용된 Application 목록을 구한다.
	var applications []*model.AllowedApplication
	for _, application := range appConfig.NotifyAPI.Applications {
		applications = append(applications, &model.AllowedApplication{
			ID:                application.ID,
			Title:             application.Title,
			Description:       application.Description,
			DefaultNotifierID: application.DefaultNotifierID,
			AppKey:            application.AppKey,
		})
	}

	return &Handler{
		allowedApplications: applications,

		notificationSender: notificationSender,

		version:     version,
		buildDate:   buildDate,
		buildNumber: buildNumber,
	}
}
