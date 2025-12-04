package handler

import (
	"fmt"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/service/api/v1/model"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

const (
	// API 엔드포인트 경로
	endpointNotifyMessage = "/api/v1/notice/message"
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
// @Param message body model.NotifyMessageRequest true "알림 메시지 정보"
// @Success 200 {object} model.SuccessResponse "성공"
// @Failure 400 {object} model.ErrorResponse "잘못된 요청 (필수 필드 누락, JSON 형식 오류 등)"
// @Failure 401 {object} model.ErrorResponse "인증 실패 (잘못된 App Key 또는 미등록 애플리케이션)"
// @Failure 500 {object} model.ErrorResponse "서버 내부 오류"
// @Security ApiKeyAuth
// @Router /api/v1/notice/message [post]
func (h *Handler) SendNotifyMessageHandler(c echo.Context) error {
	// 1. 요청 바인딩
	req := new(model.NotifyMessageRequest)
	if err := c.Bind(req); err != nil {
		applog.WithComponentAndFields("api.handler", log.Fields{
			"endpoint": endpointNotifyMessage,
			"error":    err,
		}).Warn("요청 바인딩 실패")

		return newBadRequestError("잘못된 요청 형식입니다")
	}

	// 2. 입력 검증
	if req.ApplicationID == "" {
		applog.WithComponentAndFields("api.handler", log.Fields{
			"endpoint": endpointNotifyMessage,
		}).Warn("ApplicationID가 비어있음")

		return newBadRequestError("application_id는 필수입니다")
	}

	appKey := c.QueryParam("app_key")
	if appKey == "" {
		applog.WithComponentAndFields("api.handler", log.Fields{
			"endpoint":       endpointNotifyMessage,
			"application_id": req.ApplicationID,
		}).Warn("app_key가 비어있음")

		return newBadRequestError("app_key는 필수입니다")
	}

	// 3. 인증
	app, err := h.findAndAuthenticateApplication(req.ApplicationID, appKey)
	if err != nil {
		applog.WithComponentAndFields("api.handler", log.Fields{
			"endpoint":       endpointNotifyMessage,
			"application_id": req.ApplicationID,
		}).Warn("인증 실패")

		return err
	}

	// 4. 비즈니스 로직
	applog.WithComponentAndFields("api.handler", log.Fields{
		"endpoint":       endpointNotifyMessage,
		"application_id": req.ApplicationID,
		"notifier_id":    app.DefaultNotifierID,
	}).Info("알림 메시지 전송 요청 성공")

	h.notificationSender.Notify(app.DefaultNotifierID, app.Title, req.Message, req.ErrorOccurred)

	// 5. 성공 응답
	return newSuccessResponse(c)
}

// findAndAuthenticateApplication 애플리케이션을 찾고 인증을 수행합니다
func (h *Handler) findAndAuthenticateApplication(applicationID, appKey string) (*Application, error) {
	app, ok := h.applications[applicationID]
	if !ok {
		return nil, newUnauthorizedError(fmt.Sprintf("접근이 허용되지 않은 application_id(%s)입니다", applicationID))
	}

	if app.AppKey != appKey {
		applog.WithComponentAndFields("api.handler", log.Fields{
			"application_id":   applicationID,
			"received_app_key": applog.MaskSensitiveData(appKey),
		}).Warn("APP_KEY 불일치")

		return nil, newUnauthorizedError(fmt.Sprintf("app_key가 유효하지 않습니다.(application_id:%s)", applicationID))
	}

	return app, nil
}
