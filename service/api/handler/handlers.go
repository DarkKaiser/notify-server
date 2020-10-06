package handler

import (
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/api/model"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/labstack/echo"
	"net/http"
)

//
// NotifyAPIHandlers
//
type NotifyAPIHandlers struct {
	allowedApplications []*model.AllowedApplication

	notificationSender notification.NotificationSender
}

func NewNotifyAPIHandlers(config *g.AppConfig, notificationSender notification.NotificationSender) *NotifyAPIHandlers {
	// 허용된 Application 목록을 구한다.
	var applications []*model.AllowedApplication
	for _, app := range config.NotifyAPI.Applications {
		applications = append(applications, &model.AllowedApplication{
			Id:                app.ID,
			Title:             app.Title,
			Description:       app.Description,
			DefaultNotifierID: app.DefaultNotifierID,
		})
	}

	return &NotifyAPIHandlers{
		allowedApplications: applications,

		notificationSender: notificationSender,
	}
}

func (h *NotifyAPIHandlers) NotifySendHandler(c echo.Context) error {
	// @@@@@
	m := new(model.TemplateObject)
	if err := c.Bind(m); err != nil {
		return err
	}

	var title string
	for _, app := range h.allowedApplications {
		if app.Id == m.Content.ID {
			title = app.Title
			break
		}
	}
	if len(title) > 0 {
		// error return
	}

	h.notificationSender.Notify(m.Content.NotifierID, title, m.Content.Message, m.Content.ErrorOccured)
	return c.JSON(http.StatusOK, m)
}
