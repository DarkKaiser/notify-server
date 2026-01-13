package handler

import (
	"github.com/darkkaiser/notify-server/internal/pkg/validator"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	"github.com/darkkaiser/notify-server/internal/service/notification"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
)

// PublishNotificationHandler godoc
// @Summary 알림 메시지 발송
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
// @Description
// @Description ## 응답 예시
// @Description ### 성공 (200 OK)
// @Description ```json
// @Description {"result_code":0,"message":"성공"}
// @Description ```
// @Description
// @Description ### 실패 (400 Bad Request)
// @Description ```json
// @Description {"result_code":400,"message":"애플리케이션 ID는 필수 항목입니다"}
// @Description ```
// @Tags Notification
// @Accept json
// @Produce json
// @Param X-App-Key header string false "Application Key (인증용, 권장)" example(your-app-key-here)
// @Param app_key query string false "Application Key (인증용, 레거시)" example(your-app-key-here)
// @Param message body request.NotificationRequest true "알림 메시지 정보"
// @Success 200 {object} response.SuccessResponse "성공"
// @Failure 400 {object} response.ErrorResponse "잘못된 요청 - 필수 필드 누락, JSON 형식 오류, 메시지 길이 초과(최대 4096자)"
// @Failure 401 {object} response.ErrorResponse "인증 실패 - App Key 누락, 잘못된 App Key, 미등록 애플리케이션"
// @Failure 500 {object} response.ErrorResponse "서버 내부 오류"
// @Failure 503 {object} response.ErrorResponse "서비스 일시 불가 - 알림 큐 포화 또는 시스템 종료 중"
// @Security ApiKeyAuth
// @Router /api/v1/notifications [post]
func (h *Handler) PublishNotificationHandler(c echo.Context) error {
	// 1. HTTP 요청 바인딩
	req := new(request.NotificationRequest)
	if err := c.Bind(req); err != nil {
		return httputil.NewBadRequestError(constants.ErrMsgBadRequestInvalidBody)
	}

	// 2. 요청 데이터 유효성 검증
	if err := validator.Struct(req); err != nil {
		return httputil.NewBadRequestError(validator.FormatValidationError(err))
	}

	// 3. 인증된 Application 정보 추출
	// 미들웨어에서 이미 검증된 Application 객체를 Context에서 가져옴
	app, ok := c.Get(constants.ContextKeyApplication).(*domain.Application)
	if !ok {
		// 심각한 서버 에러: 미들웨어 설정 오류 또는 컨텍스트 손상
		h.log(c).Error(constants.LogMsgInternalAuthContextError)

		return httputil.NewInternalServerError(constants.ErrMsgInternalServer)
	}

	// 4. 알림 메시지 전송 (비동기 큐 방식)
	// 큐 포화 또는 시스템 종료 시 error 반환 → 503/500 에러 응답
	err := h.notificationSender.NotifyWithTitle(app.DefaultNotifierID, app.Title, req.Message, req.ErrorOccurred)
	if err != nil {
		if err == notification.ErrServiceStopped {
			return httputil.NewServiceUnavailableError(constants.ErrMsgServiceUnavailable)
		}
		if err == notification.ErrNotFoundNotifier {
			return httputil.NewNotFoundError(constants.ErrMsgNotFoundNotifier)
		}

		h.log(c).Error(err)

		// 그 외 에러는 500 처리
		return httputil.NewInternalServerError(constants.ErrMsgInternalServerInterrupted)
	}

	// 5. 성공 로그 기록
	h.log(c).WithFields(applog.Fields{
		"application_id": req.ApplicationID,
		"notifier_id":    app.DefaultNotifierID,
		"message_length": len(req.Message),
	}).Info(constants.LogMsgNotificationQueued)

	// 6. 성공 응답 반환
	return httputil.NewSuccessResponse(c)
}

// log 핸들러 컴포넌트 로거를 생성합니다.
//
// 공통 필드(component, endpoint)가 자동으로 포함된 로거 엔트리를 반환하여 일관된 로그 형식을 유지합니다.
func (h *Handler) log(c echo.Context) *applog.Entry {
	return applog.WithComponentAndFields(constants.ComponentHandler, applog.Fields{
		"endpoint":   c.Path(),
		"request_id": c.Response().Header().Get(echo.HeaderXRequestID),
	})
}
