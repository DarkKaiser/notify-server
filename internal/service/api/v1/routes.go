// Package v1 Notify API의 v1 버전 라우트를 정의하고 설정합니다.
//
// 이 패키지는 /api/v1 경로 하위의 모든 엔드포인트를 관리하며,
// 알림 메시지 전송을 위한 RESTful API를 제공합니다.
//
// 주요 엔드포인트:
//   - POST /api/v1/notifications       - 알림 메시지 전송 (권장)
//   - POST /api/v1/notice/message      - 알림 메시지 전송 (레거시, deprecated)
//
// 모든 엔드포인트는 애플리케이션 인증(app_key)을 요구하며,
// 인증 미들웨어를 통해 요청을 검증합니다.
package v1

import (
	"github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/middleware"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/handler"
	"github.com/labstack/echo/v4"
)

// SetupRoutes Echo 인스턴스에 v1 API 라우트를 설정합니다.
//
// 이 함수는 /api/v1 그룹을 생성하고, 인증 미들웨어를 적용한 후
// 알림 전송 엔드포인트를 등록합니다.
//
// Parameters:
//   - e: Echo 서버 인스턴스
//   - h: 알림 요청을 처리하는 핸들러
//   - authenticator: 애플리케이션 인증을 담당하는 인증자
//
// 등록되는 엔드포인트:
//   - POST /api/v1/notifications  - 알림 메시지 전송 (권장)
//   - POST /api/v1/notice/message - 알림 메시지 전송 (레거시, deprecated)
//
// 미들웨어 적용:
//   - 모든 엔드포인트: RequireAuthentication (인증), ValidateContentType (JSON 검증)
//   - 레거시 엔드포인트: DeprecatedEndpoint (경고 헤더 추가)
//
// 레거시 엔드포인트 응답 헤더:
//   - Warning: 299 - "더 이상 사용되지 않는 API..."
//   - X-API-Deprecated: true
//   - X-API-Deprecated-Replacement: /api/v1/notifications
func RegisterRoutes(e *echo.Echo, h *handler.Handler, authenticator *auth.Authenticator) {
	// 1. API v1 그룹 생성 (/api/v1 prefix)
	v1Group := e.Group("/api/v1")

	// 2. 인증 미들웨어 생성 (app_key 검증)
	authMiddleware := middleware.RequireAuthentication(authenticator)

	// 3. 권장 엔드포인트 등록 (인증 미들웨어 적용 + Content-Type 검증)
	v1Group.POST("/notifications", h.PublishNotificationHandler,
		authMiddleware,
		middleware.ValidateContentType(echo.MIMEApplicationJSON),
	)

	// 4. 레거시 엔드포인트 등록 (인증 + deprecated 경고 미들웨어 적용 + Content-Type 검증)
	v1Group.POST("/notice/message", h.PublishNotificationHandler,
		authMiddleware,
		middleware.DeprecatedEndpoint("/api/v1/notifications"),
		middleware.ValidateContentType(echo.MIMEApplicationJSON),
	)
}
