package handler

import (
	"fmt"
	"github.com/darkkaiser/notify-server/service/api/model"
	"github.com/labstack/echo"
	"net/http"
)

func (h *Handler) NotifyMessageSendHandler(c echo.Context) error {
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
