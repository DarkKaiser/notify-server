package handler

import (
	applog "github.com/darkkaiser/notify-server/pkg/log"
	commonhandler "github.com/darkkaiser/notify-server/service/api/handler"
	"github.com/darkkaiser/notify-server/service/api/v1/model/request"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

// PublishNotificationHandler godoc
// @Summary 알림 메시지 게시
// @Description 외부 애플리케이션에서 텔레그램 등의 메신저로 알림 메시지를 전송합니다.
// @Description
// @Description 이 API를 사용하려면 사전에 등록된 애플리케이션 ID와 App Key가 필요합니다.
// @Description 설정 파일(notify-server.json)의 notify_api.applications에 애플리케이션을 등록해야 합니다.
// @Description
// @Description ## 사용 예시 (로컬 환경)
// @Description ```bash
// @Description curl -X POST "http://localhost:2443/api/v1/notifications?app_key=your-app-key" \
// @Description   -H "Content-Type: application/json" \
// @Description   -d '{"application_id":"my-app","message":"테스트 메시지","error_occurred":false}'
// @Description ```
// @Tags Notification
// @Accept json
// @Produce json
// @Param app_key query string true "Application Key (인증용)" example(your-app-key-here)
// @Param message body request.NotificationRequest true "알림 메시지 정보"
// @Success 200 {object} response.SuccessResponse "성공"
// @Failure 400 {object} response.ErrorResponse "잘못된 요청 (필수 필드 누락, JSON 형식 오류 등)"
// @Failure 401 {object} response.ErrorResponse "인증 실패 (잘못된 App Key 또는 미등록 애플리케이션)"
// @Failure 500 {object} response.ErrorResponse "서버 내부 오류"
// @Security ApiKeyAuth
// @Router /api/v1/notifications [post]
func (h *Handler) PublishNotificationHandler(c echo.Context) error {
	// 1. 요청 바인딩
	req := new(request.NotificationRequest)
	if err := c.Bind(req); err != nil {
		h.log(c).WithField("error", err).Warn("요청 바인딩 실패")
		return commonhandler.NewBadRequestError("잘못된 요청 형식입니다")
	}

	// 2. 입력 검증
	if err := commonhandler.ValidateRequest(req); err != nil {
		h.log(c).WithField("error", err).Warn("입력 검증 실패")
		return commonhandler.NewBadRequestError(commonhandler.FormatValidationError(err))
	}

	appKey := c.QueryParam("app_key")
	if appKey == "" {
		h.log(c).WithField("application_id", req.ApplicationID).Warn("app_key가 비어있음")
		return commonhandler.NewBadRequestError("app_key는 필수입니다")
	}

	// 3. 인증
	app, err := h.applicationManager.Authenticate(req.ApplicationID, appKey)
	if err != nil {
		h.log(c).WithField("application_id", req.ApplicationID).Warn("인증 실패")
		return err
	}

	// 4. 비즈니스 로직
	h.log(c).WithFields(log.Fields{
		"application_id": req.ApplicationID,
		"notifier_id":    app.DefaultNotifierID,
		"message_length": len(req.Message),
	}).Info("알림 메시지 게시 요청 성공")

	h.notificationService.Notify(app.DefaultNotifierID, app.Title, req.Message, req.ErrorOccurred)

	// 5. 성공 응답
	return commonhandler.NewSuccessResponse(c)
}

// log는 공통 로깅 필드가 설정된 로거 엔트리를 반환합니다.
func (h *Handler) log(c echo.Context) *log.Entry {
	return applog.WithComponentAndFields("api.handler", log.Fields{
		"endpoint": c.Path(),
	})
}
