package handler

import (
	"net/http"
	"runtime"
	"time"

	applog "github.com/darkkaiser/notify-server/log"
	"github.com/darkkaiser/notify-server/service/api/model"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

var serverStartTime = time.Now()

// HealthCheckHandler godoc
// @Summary 서버 상태 확인
// @Description 서버가 정상적으로 동작하는지 확인합니다.
// @Description
// @Description 이 엔드포인트는 인증 없이 호출할 수 있으며, 모니터링 시스템에서 서버 상태를 확인하는 데 사용됩니다.
// @Tags System
// @Produce json
// @Success 200 {object} model.HealthResponse "서버 정상"
// @Failure 500 {object} model.ErrorResponse "서버 내부 오류"
// @Router /health [get]
func (h *Handler) HealthCheckHandler(c echo.Context) error {
	applog.WithComponentAndFields("api.handler", log.Fields{
		"endpoint": "/health",
	}).Debug("헬스체크 요청")

	uptime := int64(time.Since(serverStartTime).Seconds())

	return c.JSON(http.StatusOK, model.HealthResponse{
		Status: "healthy",
		Uptime: uptime,
	})
}

// VersionHandler godoc
// @Summary 서버 버전 정보
// @Description 서버의 빌드 정보를 반환합니다.
// @Description
// @Description Git 커밋 해시, 빌드 날짜, 빌드 번호, Go 버전 등의 정보를 제공합니다.
// @Description 이 정보는 디버깅 및 버전 확인에 유용합니다.
// @Tags System
// @Produce json
// @Success 200 {object} model.VersionResponse "버전 정보"
// @Failure 500 {object} model.ErrorResponse "서버 내부 오류"
// @Router /version [get]
func (h *Handler) VersionHandler(c echo.Context) error {
	applog.WithComponentAndFields("api.handler", log.Fields{
		"endpoint": "/version",
	}).Debug("버전 정보 요청")

	return c.JSON(http.StatusOK, model.VersionResponse{
		Version:     h.version,
		BuildDate:   h.buildDate,
		BuildNumber: h.buildNumber,
		GoVersion:   runtime.Version(),
	})
}
