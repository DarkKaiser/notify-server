package middleware

import (
	"net/http"
	"strings"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
)

// ValidateContentType 요청의 Content-Type을 검증하는 미들웨어를 반환합니다.
//
// 지정된 Content-Type이 요청 헤더에 포함되어 있는지 검사합니다.
// GET, DELETE 등 본문이 없는 요청이나 본문 길이가 0인 경우에는 검증을 건너뛸 수 있습니다.
//
// Parameters:
//   - expectedContentType: 허용할 Content-Type (예: "application/json")
//
// Returns:
//   - 415 Unsupported Media Type: Content-Type이 일치하지 않는 경우
func ValidateContentType(expectedContentType string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 본문이 없는 경우(GET, HEAD 등)는 검증 건너뛰기
			if c.Request().Body == nil || c.Request().ContentLength == 0 {
				return next(c)
			}

			contentType := c.Request().Header.Get(echo.HeaderContentType)

			// Content-Type 헤더가 없거나, 기대하는 타입과 다르면 에러
			// MIME 타입 파라미터(예: charset=utf-8)를 고려하여 Contains로 검사 (대소문자 무시)
			if contentType == "" || !strings.Contains(strings.ToLower(contentType), strings.ToLower(expectedContentType)) {
				applog.WithComponentAndFields(constants.MiddlewareContentType, applog.Fields{
					"request_id": c.Response().Header().Get(echo.HeaderXRequestID),
					"method":     c.Request().Method,
					"path":       c.Request().URL.Path,
					"expected":   expectedContentType,
					"actual":     contentType,
					"remote_ip":  c.RealIP(),
				}).Warn(constants.LogMsgUnsupportedContentType)

				return echo.NewHTTPError(http.StatusUnsupportedMediaType, constants.ErrMsgUnsupportedMediaType)
			}

			return next(c)
		}
	}
}
