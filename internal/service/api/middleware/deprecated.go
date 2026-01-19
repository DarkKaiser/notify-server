package middleware

import (
	"fmt"
	"strings"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
)

// componentDeprecated Deprecated 엔드포인트 미들웨어의 로깅용 컴포넌트 이름
const componentDeprecated = "api.middleware.deprecated"

// Deprecated 엔드포인트 응답에 사용되는 HTTP 헤더 키 정의입니다.
const (
	// headerWarning RFC 7234 표준 헤더이며, API가 더 이상 권장되지 않음을 클라이언트에게 알립니다.
	headerWarning = "Warning"

	// headerXAPIDeprecated 클라이언트 애플리케이션이 Deprecated 상태를 쉽고 명확하게 감지할 수 있도록 추가하는 커스텀 헤더입니다.
	headerXAPIDeprecated = "X-API-Deprecated"

	// headerXAPIDeprecatedReplacement 개발자가 빠르게 마이그레이션할 수 있도록, 새로운 대체 엔드포인트 정보를 제공합니다.
	headerXAPIDeprecatedReplacement = "X-API-Deprecated-Replacement"
)

// DeprecatedEndpoint Deprecated 엔드포인트에 경고 헤더를 추가하는 미들웨어를 반환합니다.
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
		panic("Deprecated: 대체 엔드포인트 경로가 비어있습니다")
	}
	if !strings.HasPrefix(newEndpoint, "/") {
		panic(fmt.Sprintf("Deprecated: 대체 엔드포인트 경로는 '/'로 시작해야 합니다 (현재값: %s)", newEndpoint))
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 1. Deprecated 경고 헤더 설정
			warningMessage := "299 - \"Deprecated API endpoint. Use " + newEndpoint + " instead.\""
			c.Response().Header().Set(headerWarning, warningMessage)
			c.Response().Header().Set(headerXAPIDeprecated, "true")
			c.Response().Header().Set(headerXAPIDeprecatedReplacement, newEndpoint)

			// 2. Deprecated 엔드포인트 사용 로그 기록
			applog.WithComponentAndFields(componentDeprecated, applog.Fields{
				"deprecated_endpoint": c.Path(),
				"replacement":         newEndpoint,
				"method":              c.Request().Method,
				"remote_ip":           c.RealIP(),
				"user_agent":          c.Request().UserAgent(),
			}).Warn("경고: Deprecated 엔드포인트가 호출되었습니다")

			return next(c)
		}
	}
}
