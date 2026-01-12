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
	"github.com/darkkaiser/notify-server/internal/service/notification"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
)

const (
	// 헬스체크 상태값
	statusHealthy   = "healthy"
	statusUnhealthy = "unhealthy"

	// 의존성 서비스 식별자
	dependencyNotificationService = "notification_service"
)

// Handler 시스템 엔드포인트 핸들러 (헬스체크, 버전 정보)
type Handler struct {
	notificationSender notification.Sender

	buildInfo version.Info

	serverStartTime time.Time
}

// NewHandler Handler 인스턴스를 생성합니다.
func NewHandler(notificationSender notification.Sender, buildInfo version.Info) *Handler {
	if notificationSender == nil {
		panic("NotificationSender는 필수입니다")
	}

	return &Handler{
		notificationSender: notificationSender,

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
	applog.WithComponentAndFields(constants.ComponentHandler, applog.Fields{
		"endpoint":  "/health",
		"method":    c.Request().Method,
		"remote_ip": c.RealIP(),
	}).Debug("헬스체크 조회")

	uptime := int64(time.Since(h.serverStartTime).Seconds())

	// 외부 의존성 상태 수집
	deps := make(map[string]system.DependencyStatus)

	// Notification 서비스 상태 확인
	if h.notificationSender != nil {
		deps[dependencyNotificationService] = system.DependencyStatus{
			Status:  statusHealthy,
			Message: "정상 작동 중",
		}
	} else {
		deps[dependencyNotificationService] = system.DependencyStatus{
			Status:  statusUnhealthy,
			Message: "서비스가 초기화되지 않음",
		}
	}

	// 하나라도 unhealthy면 전체 상태를 unhealthy로 설정
	serverStatus := statusHealthy
	for _, dep := range deps {
		if dep.Status != statusHealthy {
			serverStatus = statusUnhealthy
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
	applog.WithComponentAndFields(constants.ComponentHandler, applog.Fields{
		"endpoint":  "/version",
		"method":    c.Request().Method,
		"remote_ip": c.RealIP(),
	}).Debug("버전 정보 조회")

	return c.JSON(http.StatusOK, system.VersionResponse{
		Version:     h.buildInfo.Version,
		BuildDate:   h.buildInfo.BuildDate,
		BuildNumber: h.buildInfo.BuildNumber,
		GoVersion:   runtime.Version(),
	})
}
