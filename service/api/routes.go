package api

import (
	"net/http"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/service/api/handler"
	"github.com/darkkaiser/notify-server/service/api/model/response"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
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
	e.HTTPErrorHandler = customHTTPErrorHandler
}

// customHTTPErrorHandler 커스텀 HTTP 에러 핸들러입니다.
// 모든 HTTP 에러를 표준 ErrorResponse 형식으로 반환합니다.
func customHTTPErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	message := "내부 서버 오류가 발생했습니다."

	// Echo HTTPError 타입 확인
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		if msg, ok := he.Message.(string); ok {
			message = msg
		}
	}

	// 404 에러 메시지 커스터마이징
	if code == http.StatusNotFound {
		message = "페이지를 찾을 수 없습니다."
	}

	// 500 에러 발생 시 로깅 (디버깅 용도)
	if code == http.StatusInternalServerError {
		applog.WithComponentAndFields("api.error_handler", log.Fields{
			"path":   c.Request().URL.Path,
			"method": c.Request().Method,
			"error":  err,
		}).Error("내부 서버 오류 발생")
	}

	// 응답이 이미 전송되었는지 확인
	if c.Response().Committed {
		return
	}

	// HEAD 요청은 본문 없이 응답
	if c.Request().Method == http.MethodHead {
		c.NoContent(code)
		return
	}

	// 표준 ErrorResponse 형식으로 JSON 응답
	c.JSON(code, response.ErrorResponse{
		Message: message,
	})
}
