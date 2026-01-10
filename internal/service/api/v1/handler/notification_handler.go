package handler

import (
	"github.com/darkkaiser/notify-server/internal/pkg/validator"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
)

// PublishNotificationHandler godoc
// @Summary 알림 메시지 게시
// @Description 외부 애플리케이션에서 텔레그램 등의 메신저로 알림 메시지를 전송합니다.
// @Description
// @Description 이 API를 사용하려면 사전에 등록된 애플리케이션 ID와 App Key가 필요합니다.
// @Description 설정 파일(notify-server.json)의 notify_api.applications에 애플리케이션을 등록해야 합니다.
// @Description
// @Description ## 인증 방식
// @Description - **권장**: X-App-Key 헤더로 전달
// @Description - **레거시**: app_key 쿼리 파라미터로 전달 (하위 호환성 유지)
// @Description
// @Description ## 사용 예시 (로컬 환경)
// @Description ### 헤더 방식 (권장)
// @Description ```bash
// @Description curl -X POST "http://localhost:2443/api/v1/notifications" \
// @Description   -H "Content-Type: application/json" \
// @Description   -H "X-App-Key: your-app-key" \
// @Description   -d '{"application_id":"my-app","message":"테스트 메시지","error_occurred":false}'
// @Description ```
// @Description
// @Description ### 쿼리 파라미터 방식 (레거시)
// @Description ```bash
// @Description curl -X POST "http://localhost:2443/api/v1/notifications?app_key=your-app-key" \
// @Description   -H "Content-Type: application/json" \
// @Description   -d '{"application_id":"my-app","message":"테스트 메시지","error_occurred":false}'
// @Description ```
// @Tags Notification
// @Accept json
// @Produce json
// @Param X-App-Key header string false "Application Key (인증용, 권장)" example(your-app-key-here)
// @Param app_key query string false "Application Key (인증용, 레거시)" example(your-app-key-here)
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
		return httputil.NewBadRequestError("잘못된 요청 형식입니다")
	}

	// 2. 입력 검증
	if err := validator.Struct(req); err != nil {
		return httputil.NewBadRequestError(validator.FormatValidationError(err))
	}

	// 3. App Key 추출 (헤더 우선, 쿼리 파라미터 폴백)
	appKey := c.Request().Header.Get(constants.HeaderAppKey)
	if appKey == "" {
		// 헤더에 없으면 쿼리 파라미터 확인 (레거시 지원)
		appKey = c.QueryParam(constants.QueryParamAppKey)

		// 레거시 방식 사용 시 경고 로그
		if appKey != "" {
			h.log(c).WithField("application_id", req.ApplicationID).Warn("쿼리 파라미터로 app_key 전달됨 (deprecated, X-App-Key 헤더 사용 권장)")
		}
	}

	if appKey == "" {
		return httputil.NewBadRequestError(constants.ErrMsgAppKeyRequired)
	}

	// 4. 인증
	app, err := h.authenticator.Authenticate(req.ApplicationID, appKey)
	if err != nil {
		return err
	}

	// 5. 알림 전송 (비동기)
	// 큐가 가득 차거나 시스템이 혼잡한 경우 실패할 수 있으며, 이 경우 503 에러를 반환합니다.
	ok := h.notificationSender.NotifyWithTitle(app.DefaultNotifierID, app.Title, req.Message, req.ErrorOccurred)
	if !ok {
		h.log(c).WithFields(applog.Fields{
			"application_id": req.ApplicationID,
			"notifier_id":    app.DefaultNotifierID,
		}).Error("알림 메시지 큐 적재 실패 (서비스 혼잡 또는 종료 중)")

		return httputil.NewServiceUnavailableError("현재 알림 서비스가 혼잡하여 요청을 처리할 수 없습니다. 잠시 후 다시 시도해주세요.")
	}

	h.log(c).WithFields(applog.Fields{
		"application_id": req.ApplicationID,
		"notifier_id":    app.DefaultNotifierID,
		"message_length": len(req.Message),
	}).Info("알림 메시지 게시 요청 성공")

	// 6. 성공 응답
	return httputil.NewSuccessResponse(c)
}

// log는 공통 로깅 필드가 설정된 로거 엔트리를 반환합니다.
func (h *Handler) log(c echo.Context) *applog.Entry {
	return applog.WithComponentAndFields(constants.ComponentHandler, applog.Fields{
		"endpoint": c.Path(),
	})
}
