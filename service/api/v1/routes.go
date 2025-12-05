// Package v1 API v1 버전의 라우트를 정의합니다.
//
// 이 패키지는 /api/v1 경로 하위의 모든 엔드포인트를 설정합니다.
package v1

import (
	"github.com/darkkaiser/notify-server/service/api/v1/handler"
	"github.com/labstack/echo/v4"
)

// SetupRoutes Echo 인스턴스에 v1 API 라우트를 설정합니다.
//
// 이 함수는 다음 엔드포인트를 등록합니다:
//
// Notification API:
//   - POST /api/v1/notifications - 알림 메시지 전송 (인증 필요)
//
// 모든 v1 API는 애플리케이션 인증(app_key)이 필요합니다.
func SetupRoutes(e *echo.Echo, h *handler.Handler) {
	// API v1 그룹 생성
	v1Group := e.Group("/api/v1")

	// Notification 엔드포인트
	v1Group.POST("/notifications", h.PublishNotificationHandler)  // 권장
	v1Group.POST("/notice/message", h.PublishNotificationHandler) // 레거시, 하위 호환성 유지
}
