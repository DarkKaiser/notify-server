package handlers

import (
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/api/models"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/labstack/echo"
	"net/http"
)

//
// NotifyAPIHandlers
//
type NotifyAPIHandlers struct {
	allowedApplications []*models.AllowedApplication

	notificationSender notification.NotificationSender
}

func NewNotifyAPIHandlers(config *g.AppConfig, notificationSender notification.NotificationSender) *NotifyAPIHandlers {
	// 허용된 Application 목록을 구한다.
	var applications []*models.AllowedApplication
	for _, app := range config.NotifyAPI.Applications {
		applications = append(applications, &models.AllowedApplication{
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

func (h *NotifyAPIHandlers) SendNotifyHandler(c echo.Context) error {
	// @@@@@
	m := new(models.TemplateObject)
	if err := c.Bind(m); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, m)

	//for _, a := range h.allowedApplications {
	//	if a.Id == "lottoPrediction" {
	//		h.notificationSender.Notify(a.DefaultNotifierID, "title", c.Param("message"), false)
	//		// http://notify-api.darkkaiser.com/api/notify/
	//		break
	//	}
	//}
	//return c.String(http.StatusOK, "Hello, World!  "+c.Param("message"))
}
