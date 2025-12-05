package v1

import (
	"github.com/darkkaiser/notify-server/service/api/v1/handler"
	"github.com/labstack/echo/v4"
)

// SetupRoutes Echo 인스턴스에 v1 API 라우트를 설정합니다.
//
// 라우트는 다음과 같이 구성됩니다:
//   - API v1 엔드포인트: /api/v1/* (인증 필요)
func SetupRoutes(e *echo.Echo, h *handler.Handler) {
	// API v1 엔드포인트
	grp := e.Group("/api/v1")
	{
		grp.POST("/notice/message", h.SendNotifyMessageHandler)
	}
}
