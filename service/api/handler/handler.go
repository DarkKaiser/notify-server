package handler

import (
	"fmt"
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
	for _, application := range config.NotifyAPI.Applications {
		applications = append(applications, &model.AllowedApplication{
			ID:                application.ID,
			Title:             application.Title,
			Description:       application.Description,
			DefaultNotifierID: application.DefaultNotifierID,
		})
	}

	return &NotifyAPIHandlers{
		allowedApplications: applications,

		notificationSender: notificationSender,
	}
}

func (h *NotifyAPIHandlers) NotifyMessageSendHandler(c echo.Context) error {
	m := new(model.NotifyMessage)
	if err := c.Bind(m); err != nil {
		return err
	}

	for _, application := range h.allowedApplications {
		if application.ID == m.ApplicationID {
			h.notificationSender.Notify(application.DefaultNotifierID, application.Title, m.Message, m.ErrorOccurred)

			return c.JSON(http.StatusOK, map[string]int{
				"result_code": 0,
			})
		}
	}

	return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("접근이 허용되지 않은 Application입니다(ID:%s)", m.ApplicationID))
}
