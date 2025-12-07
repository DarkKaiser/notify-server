package api

import (
	"github.com/darkkaiser/notify-server/service/api/handler"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// SetupRoutes 시스템 전반에 적용되는 공통 라우트를 설정합니다.
//
// 포함되는 라우트:
//   - System 엔드포인트: /health, /version (인증 불필요)
//   - Swagger UI: /swagger/*
//   - 커스텀 HTTP 에러 핸들러 (404, 500 등)
func SetupRoutes(e *echo.Echo, h *handler.SystemHandler) {
	setupSystemRoutes(e, h)
	setupSwaggerRoutes(e)
	setupErrorHandler(e)
}

func setupSystemRoutes(e *echo.Echo, h *handler.SystemHandler) {
	// System 엔드포인트 (인증 불필요)
	e.GET("/health", h.HealthCheckHandler)
	e.GET("/version", h.VersionHandler)
}

func setupSwaggerRoutes(e *echo.Echo) {
	// Swagger UI 설정
	e.GET("/swagger/*", echoSwagger.EchoWrapHandler(
		// Swagger 문서 JSON 파일 위치 지정
		echoSwagger.URL("/swagger/doc.json"),
		// 딥 링크 활성화 (특정 API로 바로 이동 가능한 URL 지원)
		echoSwagger.DeepLinking(true),
		// 문서 로드 시 태그(Tag) 목록만 펼침 상태로 표시 ("list", "full", "none")
		echoSwagger.DocExpansion("list"),
	))
}

// setupErrorHandler 커스텀 HTTP 에러 핸들러를 설정합니다.
func setupErrorHandler(e *echo.Echo) {
	e.HTTPErrorHandler = handler.CustomHTTPErrorHandler
}
