package handler

import (
	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/pkg/common"
	"github.com/darkkaiser/notify-server/service/api/v1/model/domain"
	"github.com/darkkaiser/notify-server/service/notification"
)

// Handler API 요청을 처리하고 비즈니스 로직을 연결하는 역할을 담당합니다.
type Handler struct {
	applications map[string]*domain.Application

	notificationSender notification.NotificationSender

	buildInfo common.BuildInfo
}

func NewHandler(appConfig *config.AppConfig, notificationSender notification.NotificationSender, buildInfo common.BuildInfo) *Handler {
	return &Handler{
		applications: loadApplicationsFromConfig(appConfig),

		notificationSender: notificationSender,

		buildInfo: buildInfo,
	}
}

func loadApplicationsFromConfig(appConfig *config.AppConfig) map[string]*domain.Application {
	applications := make(map[string]*domain.Application)
	for _, application := range appConfig.NotifyAPI.Applications {
		applications[application.ID] = &domain.Application{
			ID:                application.ID,
			Title:             application.Title,
			Description:       application.Description,
			DefaultNotifierID: application.DefaultNotifierID,
			AppKey:            application.AppKey,
		}
	}
	return applications
}
