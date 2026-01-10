package middleware

import (
	"strings"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
)

// DeprecatedEndpoint deprecated된 엔드포인트에 경고 헤더를 추가하는 미들웨어입니다.
//
// 이 미들웨어는 다음 작업을 수행합니다:
//   - 응답 헤더에 "Warning" 헤더 추가 (RFC 7234)
//   - 응답 헤더에 "X-API-Deprecated" 커스텀 헤더 추가
//   - 응답 헤더에 "X-API-Deprecated-Replacement" 커스텀 헤더 추가
//   - deprecated 엔드포인트 사용 로그 기록
//   - 클라이언트가 deprecated 상태를 인지하고 새 엔드포인트로 마이그레이션할 수 있도록 함
//
// Warning 헤더 형식: "299 - \"Deprecated API endpoint. Use {newEndpoint} instead.\""
//
// Parameters:
//   - newEndpoint: 대체할 새로운 엔드포인트 경로 (예: "/api/v1/notifications")
//     빈 문자열이거나 '/'로 시작하지 않으면 패닉 발생
//
// Returns:
//   - echo.MiddlewareFunc: Echo 미들웨어 함수
//
// Panics:
//   - newEndpoint가 빈 문자열인 경우
//   - newEndpoint가 '/'로 시작하지 않는 경우
//
// Example:
//
//	v1Group.POST("/notice/message", handler,
//	    middleware.DeprecatedEndpoint("/api/v1/notifications"))
func DeprecatedEndpoint(newEndpoint string) echo.MiddlewareFunc {
	// 입력 검증
	if newEndpoint == "" {
		panic("[DeprecatedEndpoint] 대체 엔드포인트 경로(newEndpoint)가 비어있습니다. 유효한 경로를 지정해야 합니다")
	}
	if !strings.HasPrefix(newEndpoint, "/") {
		panic("[DeprecatedEndpoint] 대체 엔드포인트 경로(newEndpoint)는 반드시 '/'로 시작해야 합니다. 현재 값: " + newEndpoint)
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 응답 헤더에 deprecated 경고 추가
			warningMessage := "299 - \"더 이상 사용되지 않는 API입니다. " + newEndpoint + "를 사용하세요\""
			c.Response().Header().Set(constants.HeaderWarning, warningMessage)
			c.Response().Header().Set(constants.HeaderXAPIDeprecated, "true")
			c.Response().Header().Set(constants.HeaderXAPIDeprecatedReplacement, newEndpoint)

			// deprecated 엔드포인트 사용 로그 기록
			applog.WithComponentAndFields(constants.ComponentMiddleware, applog.Fields{
				"deprecated_endpoint": c.Path(),
				"replacement":         newEndpoint,
				"method":              c.Request().Method,
				"remote_ip":           c.RealIP(),
			}).Warn("더 이상 사용되지 않는(deprecated) API 엔드포인트가 호출되었습니다")

			return next(c)
		}
	}
}
