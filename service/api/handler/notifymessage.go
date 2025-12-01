package handler

import (
	"fmt"
	"net/http"

	"github.com/darkkaiser/notify-server/service/api/model"
	"github.com/labstack/echo/v4"
)

// NotifyMessageSendHandler godoc
// @Summary 알림 메시지 전송
// @Description 외부 애플리케이션에서 알림 메시지를 전송합니다.
// @Tags Notification
// @Accept json
// @Produce json
// @Param app_key query string true "Application Key"
// @Param message body model.NotifyMessage true "알림 메시지 정보"
// @Success 200 {object} model.SuccessResponse
// @Failure 400 {object} model.ErrorResponse "Bad Request"
// @Failure 401 {object} model.ErrorResponse "Unauthorized"
// @Router /notice/message [post]
func (h *Handler) NotifyMessageSendHandler(c echo.Context) error {
	m := new(model.NotifyMessage)
	if err := c.Bind(m); err != nil {
		return err
	}

	appKey := c.QueryParam("app_key")

	for _, application := range h.allowedApplications {
		if application.ID == m.ApplicationID {
			if application.AppKey != appKey {
				return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("APP_KEY가 유효하지 않습니다.(ID:%s)", m.ApplicationID))
			}

			h.notificationSender.Notify(application.DefaultNotifierID, application.Title, m.Message, m.ErrorOccurred)

			return c.JSON(http.StatusOK, map[string]int{
				"result_code": 0,
			})
		}
	}

	return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("접근이 허용되지 않은 Application입니다.(ID:%s)", m.ApplicationID))
}
