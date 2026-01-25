package api

import (
	"github.com/darkkaiser/notify-server/internal/service/api/handler/system"
	"github.com/labstack/echo/v4"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// RegisterRoutes API 서비스의 전역 라우트를 등록합니다.
//
// 이 함수는 다음과 같은 공통 엔드포인트들을 설정합니다:
//   - 시스템 엔드포인트: 서비스 상태 확인(/health) 및 버전 정보(/version) (인증 불필요)
//   - API 문서: Swagger UI (/swagger/*) 제공
func RegisterRoutes(e *echo.Echo, h *system.Handler) {
	registerSystemRoutes(e, h)
	registerSwaggerRoutes(e)
}

func registerSystemRoutes(e *echo.Echo, h *system.Handler) {
	e.GET("/health", h.HealthCheckHandler)
	e.GET("/version", h.VersionHandler)
}

func registerSwaggerRoutes(e *echo.Echo) {
	// Swagger UI 엔드포인트 설정
	e.GET("/swagger/*", echoSwagger.EchoWrapHandler(
		// Swagger 문서 JSON 파일 위치 지정
		echoSwagger.URL("/swagger/doc.json"),
		// 딥 링크 활성화 (특정 API로 바로 이동 가능한 URL 지원)
		echoSwagger.DeepLinking(true),
		// 문서 로드 시 태그(Tag) 목록만 펼침 상태로 표시 ("list", "full", "none")
		echoSwagger.DocExpansion("list"),
	))
}
