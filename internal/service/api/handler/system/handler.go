// Package system 시스템 엔드포인트 핸들러를 제공합니다.
//
// 헬스체크, 버전 정보 등 인증이 필요 없는 시스템 수준의 API를 처리합니다.
package system

import (
	"net/http"
	"runtime"
	"time"

	"github.com/darkkaiser/notify-server/internal/pkg/version"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/system"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
)

// 헬스체크 상태
const (
	// healthStatusHealthy 정상 상태
	healthStatusHealthy = "healthy"

	// healthStatusUnhealthy 비정상 상태
	healthStatusUnhealthy = "unhealthy"
)

// 외부 의존성 식별자 및 상태 메시지
const (
	// depNotificationService 외부 의존성 식별자: 알림 서비스
	depNotificationService = "notification_service"

	// depNotificationServiceStatusHealthy 외부 의존성 상태 메시지: 정상 상태
	depNotificationServiceStatusHealthy = "정상 작동 중"
)

// Handler 시스템 엔드포인트 핸들러 (헬스체크, 버전 정보)
type Handler struct {
	healthChecker contract.NotificationHealthChecker

	buildInfo version.Info

	serverStartTime time.Time
}

// New Handler 인스턴스를 생성합니다.
func New(healthChecker contract.NotificationHealthChecker, buildInfo version.Info) *Handler {
	if healthChecker == nil {
		panic(constants.PanicMsgHealthCheckerRequired)
	}

	return &Handler{
		healthChecker: healthChecker,

		buildInfo: buildInfo,

		serverStartTime: time.Now(),
	}
}

// HealthCheckHandler godoc
// @Summary 서버 헬스체크
// @Description 서버와 외부 의존성의 상태를 확인합니다.
// @Description 인증 없이 호출 가능하며, 모니터링 시스템에서 사용됩니다.
// @Description
// @Description 응답 필드:
// @Description - status: 전체 서버 상태 (healthy, unhealthy)
// @Description - uptime: 서버 가동 시간(초)
// @Description - dependencies: 외부 의존성별 상태 (notification_service 등)
// @Tags System
// @Produce json
// @Success 200 {object} system.HealthResponse "헬스체크 결과"
// @Router /health [get]
func (h *Handler) HealthCheckHandler(c echo.Context) error {
	applog.WithComponentAndFields(constants.Handler, applog.Fields{
		"endpoint":  "/health",
		"method":    c.Request().Method,
		"remote_ip": c.RealIP(),
	}).Debug(constants.LogMsgHealthCheck)

	uptime := int64(time.Since(h.serverStartTime).Seconds())

	// 외부 의존성 상태 수집
	deps := make(map[string]system.DependencyStatus)

	// Notification 서비스 상태 확인
	if err := h.healthChecker.Health(); err != nil {
		deps[depNotificationService] = system.DependencyStatus{
			Status:  healthStatusUnhealthy,
			Message: err.Error(),
		}
	} else {
		deps[depNotificationService] = system.DependencyStatus{
			Status:  healthStatusHealthy,
			Message: depNotificationServiceStatusHealthy,
		}
	}

	// 하나라도 unhealthy면 전체 상태를 unhealthy로 설정
	serverStatus := healthStatusHealthy
	for _, dep := range deps {
		if dep.Status != healthStatusHealthy {
			serverStatus = healthStatusUnhealthy
			break
		}
	}

	return c.JSON(http.StatusOK, system.HealthResponse{
		Status:       serverStatus,
		Uptime:       uptime,
		Dependencies: deps,
	})
}

// VersionHandler godoc
// @Summary 서버 버전 정보
// @Description 서버의 Git 커밋 해시, 빌드 날짜, 빌드 번호, Go 버전을 반환합니다.
// @Description 디버깅 및 배포 버전 확인에 사용됩니다.
// @Tags System
// @Produce json
// @Success 200 {object} system.VersionResponse "버전 정보"
// @Router /version [get]
func (h *Handler) VersionHandler(c echo.Context) error {
	applog.WithComponentAndFields(constants.Handler, applog.Fields{
		"endpoint":  "/version",
		"method":    c.Request().Method,
		"remote_ip": c.RealIP(),
	}).Debug(constants.LogMsgVersionInfo)

	return c.JSON(http.StatusOK, system.VersionResponse{
		Version:     h.buildInfo.Version,
		BuildDate:   h.buildInfo.BuildDate,
		BuildNumber: h.buildInfo.BuildNumber,
		GoVersion:   runtime.Version(),
	})
}
