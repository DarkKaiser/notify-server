package handlers

import (
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/api/models"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/labstack/echo"
	"net/http"
)

//
// NotifyHandlers
//
type NotifyHandlers struct {
	allowedApplications []*models.AllowedApplication

	notificationSender notification.NotificationSender
}

func NewNotifyHandlers(config *g.AppConfig, notificationSender notification.NotificationSender) *NotifyHandlers {
	// 허용된 Application 목록을 구한다.
	var applications []*models.AllowedApplication
	for _, app := range config.NotifyAPI.Applications {
		applications = append(applications, &models.AllowedApplication{
			Id:                app.ID,
			Title:             app.Title,
			Description:       app.Description,
			ApiKey:            app.APIKey,
			DefaultNotifierID: app.DefaultNotifierID,
		})
	}

	return &NotifyHandlers{
		allowedApplications: applications,

		notificationSender: notificationSender,
	}
}

// @@@@@
func (h *NotifyHandlers) MessageNotifyHandler(c echo.Context) error {
	for _, a := range h.allowedApplications {
		if a.Id == "lottoPrediction" {
			h.notificationSender.Notify(a.DefaultNotifierID, "title", c.Param("message"), false)
			// http://notify-api.darkkaiser.com/api/notify/

			// 허용가능한 ID목록+인증키를 읽어서 메시지를 보내면 체크한다.
			// 등록되지 않은 id이면 거부한다.
			// 등록된 id이면 webNotificationSender.Notify(id, notifierid, message, isError)
			// commandTitle:       fmt.Sprintf("%s > %s", t.Title, c.Title), => json 파일에 저장되어 있는값을 notifier에서 알아서 읽어온다.(notifier.webSenderTitle 등의 이름으로 따로 관리)
			// => 이렇게 되면 notifier가 추가될때마다 항상 같이 관리가 되어져야 됨, title을 직접 넘김
			// => webNotificationSender.Notify(notifierid, commandTitle, message, isError)
			// web -> task -> notifier 순으로 가는건????

			break
		}
	}
	return c.String(http.StatusOK, "Hello, World!  "+c.Param("message"))
}
