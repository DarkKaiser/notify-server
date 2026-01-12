package middleware

import (
	"strings"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
)

// DeprecatedEndpoint deprecated 엔드포인트에 경고 헤더를 추가하는 미들웨어를 반환합니다.
//
// 응답 헤더에 RFC 7234 표준 Warning 헤더와 커스텀 헤더를 추가하여
// 클라이언트가 deprecated 상태를 인지하고 새 엔드포인트로 마이그레이션할 수 있도록 합니다.
//
// 추가되는 헤더:
//   - Warning: "299 - \"Deprecated API endpoint. Use {newEndpoint} instead.\""
//   - X-API-Deprecated: "true"
//   - X-API-Deprecated-Replacement: {newEndpoint}
//
// Parameters:
//   - newEndpoint: 대체 엔드포인트 경로 (예: "/api/v1/notifications")
//     반드시 '/'로 시작하는 비어있지 않은 문자열이어야 함
//
// 사용 예시:
//
//	v1Group.POST("/notice/message", handler,
//	    middleware.DeprecatedEndpoint("/api/v1/notifications"))
//
// Panics:
//   - newEndpoint가 빈 문자열이거나 '/'로 시작하지 않는 경우
func DeprecatedEndpoint(newEndpoint string) echo.MiddlewareFunc {
	if newEndpoint == "" {
		panic("DeprecatedEndpoint: 대체 엔드포인트 경로가 비어있습니다")
	}
	if !strings.HasPrefix(newEndpoint, "/") {
		panic("DeprecatedEndpoint: 대체 엔드포인트 경로는 '/'로 시작해야 합니다 (현재값: " + newEndpoint + ")")
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 1. Deprecated 경고 헤더 설정
			warningMessage := "299 - \"Deprecated API endpoint. Use " + newEndpoint + " instead.\""
			c.Response().Header().Set(constants.HeaderWarning, warningMessage)
			c.Response().Header().Set(constants.HeaderXAPIDeprecated, "true")
			c.Response().Header().Set(constants.HeaderXAPIDeprecatedReplacement, newEndpoint)

			// 2. Deprecated 엔드포인트 사용 로그 기록
			applog.WithComponentAndFields(constants.ComponentMiddleware, applog.Fields{
				"deprecated_endpoint": c.Path(),
				"replacement":         newEndpoint,
				"method":              c.Request().Method,
				"remote_ip":           c.RealIP(),
				"user_agent":          c.Request().UserAgent(),
			}).Warn("Deprecated API 엔드포인트 사용됨")

			return next(c)
		}
	}
}
