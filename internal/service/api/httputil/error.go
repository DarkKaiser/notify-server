package httputil

import (
	"net/http"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
)

// ErrorHandler Echo 프레임워크의 전역 에러 핸들러입니다.
//
// 모든 HTTP 에러를 가로채서 표준 ErrorResponse JSON 형식으로 변환하여 반환합니다.
// 에러 발생 시 적절한 로그 레벨(Error/Warn)로 상세 정보를 기록합니다.
func ErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	message := constants.ErrMsgInternalServer

	// Echo HTTPError 타입 확인
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		if msg, ok := he.Message.(string); ok {
			message = msg
		} else if resp, ok := he.Message.(response.ErrorResponse); ok {
			message = resp.Message
		}
	}

	// 404 에러는 사용자 친화적인 한국어 메시지로 통일
	if code == http.StatusNotFound {
		message = constants.ErrMsgNotFound
	}

	// 에러 로깅 (보안 및 디버깅 용도)
	fields := applog.Fields{
		"path":        c.Request().URL.Path,
		"method":      c.Request().Method,
		"status_code": code,
		"error":       err,
		"remote_ip":   c.RealIP(),
		"request_id":  c.Response().Header().Get(echo.HeaderXRequestID),
	}

	// 인증된 애플리케이션 정보 추가 (있는 경우)
	if app := c.Get(constants.ContextKeyApplication); app != nil {
		if application, ok := app.(*domain.Application); ok {
			fields["application_id"] = application.ID
		}
	}

	if code >= http.StatusInternalServerError {
		// 5xx: 서버 내부 오류 - 즉시 조치 필요
		applog.WithComponentAndFields(constants.ComponentErrorHandler, fields).Error(constants.LogMsgHTTP5xxServerError)
	} else if code >= http.StatusBadRequest {
		// 4xx: 클라이언트 요청 오류 - 정상적인 거부 응답
		applog.WithComponentAndFields(constants.ComponentErrorHandler, fields).Warn(constants.LogMsgHTTP4xxClientError)
	}

	// 이중 응답 방지: 이미 응답이 전송된 경우 추가 응답 시도하지 않음
	if c.Response().Committed {
		return
	}

	// HEAD 요청 처리: HTTP 명세에 따라 헤더만 반환하고 본문은 생략
	if c.Request().Method == http.MethodHead {
		c.NoContent(code)
		return
	}

	// 일반 요청: 표준 ErrorResponse JSON 형식으로 응답
	c.JSON(code, response.ErrorResponse{
		ResultCode: code,
		Message:    message,
	})
}
