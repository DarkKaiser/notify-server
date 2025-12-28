package handler

import (
	"net/http"

	applog "github.com/darkkaiser/notify-server/internal/pkg/log"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

// CustomHTTPErrorHandler 커스텀 HTTP 에러 핸들러입니다.
// 모든 HTTP 에러를 표준 ErrorResponse 형식으로 반환합니다.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	message := "내부 서버 오류가 발생했습니다."

	// Echo HTTPError 타입 확인
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		if msg, ok := he.Message.(string); ok {
			message = msg
		}
	}

	// HTTP 상태 코드가 404 (찾을 수 없음)이고, 기본 메시지가 "Not Found"인 경우
	// 사용자에게 더 친숙한 한국어 메시지로 변경하여 반환합니다.
	if code == http.StatusNotFound && message == "Not Found" {
		message = "페이지를 찾을 수 없습니다."
	}

	// 500 에러 발생 시 로깅 (디버깅 용도)
	if code == http.StatusInternalServerError {
		applog.WithComponentAndFields("api.error_handler", log.Fields{
			"path":   c.Request().URL.Path,
			"method": c.Request().Method,
			"error":  err,
		}).Error("내부 서버 오류 발생")
	}

	// 응답이 이미 전송되었는지 확인
	if c.Response().Committed {
		return
	}

	// HEAD 요청은 본문 없이 응답
	if c.Request().Method == http.MethodHead {
		c.NoContent(code)
		return
	}

	// 표준 ErrorResponse 형식으로 JSON 응답
	c.JSON(code, response.ErrorResponse{
		Message: message,
	})
}
