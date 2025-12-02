package handler

import (
	"fmt"
	"net/http"

	"github.com/darkkaiser/notify-server/service/api/model"
	"github.com/labstack/echo/v4"
)

// NotifyMessageSendHandler godoc
// @Summary 알림 메시지 전송
// @Description 외부 애플리케이션에서 텔레그램 등의 메신저로 알림 메시지를 전송합니다.
// @Description
// @Description 이 API를 사용하려면 사전에 등록된 애플리케이션 ID와 App Key가 필요합니다.
// @Description 설정 파일(notify-server.json)의 notify_api.applications에 애플리케이션을 등록해야 합니다.
// @Description
// @Description ## 사용 예시 (로컬 환경)
// @Description ```bash
// @Description curl -X POST "http://localhost:2443/api/v1/notice/message?app_key=your-app-key" \
// @Description   -H "Content-Type: application/json" \
// @Description   -d '{"application_id":"my-app","message":"테스트 메시지","error_occurred":false}'
// @Description ```
// @Tags Notification
// @Accept json
// @Produce json
// @Param app_key query string true "Application Key (인증용)" example(your-app-key-here)
// @Param message body model.NotifyMessage true "알림 메시지 정보"
// @Success 200 {object} model.SuccessResponse "성공"
// @Failure 400 {object} model.ErrorResponse "잘못된 요청 (필수 필드 누락, JSON 형식 오류 등)"
// @Failure 401 {object} model.ErrorResponse "인증 실패 (잘못된 App Key 또는 미등록 애플리케이션)"
// @Failure 500 {object} model.ErrorResponse "서버 내부 오류"
// @Security ApiKeyAuth
// @Router /api/v1/notice/message [post]
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
