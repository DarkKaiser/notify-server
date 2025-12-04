package handler

import (
	"github.com/darkkaiser/notify-server/config"
)

// Application API 접근이 허용된 애플리케이션 정보를 담고 있습니다.
type Application struct {
	ID                string
	Title             string
	Description       string
	DefaultNotifierID string
	AppKey            string
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
