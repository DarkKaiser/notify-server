package httpserver

import (
	"net/http"

	"github.com/darkkaiser/notify-server/service/api/v1/handler"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// SetupRoutes Echo 인스턴스에 모든 라우트를 설정합니다.
//
// 라우트는 다음과 같이 구성됩니다:
//   - System 엔드포인트: /health, /version (인증 불필요)
//   - API v1 엔드포인트: /api/v1/* (인증 필요)
//   - Swagger UI: /swagger/*
//   - 404 Not Found 핸들러
func SetupRoutes(e *echo.Echo, h *handler.Handler) {
	// System 엔드포인트 (인증 불필요)
	e.GET("/health", h.HealthCheckHandler)
	e.GET("/version", h.VersionHandler)

	// API v1 엔드포인트
	grp := e.Group("/api/v1")
	{
		grp.POST("/notice/message", h.SendNotifyMessageHandler)
	}

	// Swagger UI 설정
	e.GET("/swagger/*", echoSwagger.EchoWrapHandler(
		// Swagger 문서 JSON 파일 위치 지정
		echoSwagger.URL("/swagger/doc.json"),
		// 딥 링크 활성화 (특정 API로 바로 이동 가능한 URL 지원)
		echoSwagger.DeepLinking(true),
		// 문서 로드 시 태그(Tag) 목록만 펼침 상태로 표시 ("list", "full", "none")
		echoSwagger.DocExpansion("list"),
	))

	// 404 Not Found 핸들러
	echo.NotFoundHandler = func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "페이지를 찾을 수 없습니다.")
	}
}
