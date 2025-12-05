package handler

import (
	"net/http"
	"runtime"
	"time"

	"github.com/darkkaiser/notify-server/pkg/common"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/service/api/model/response"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

const (
	// Health Check 상태 상수
	statusHealthy   = "healthy"
	statusUnhealthy = "unhealthy"

	// 의존성 서비스 키
	dependencyNotificationService = "notification_service"
)

var serverStartTime = time.Now()

// SystemHandler 시스템 관련 요청(헬스체크, 버전 등)을 처리하는 핸들러입니다.
type SystemHandler struct {
	notificationSender notification.NotificationSender

	buildInfo common.BuildInfo
}

// NewSystemHandler SystemHandler 인스턴스를 생성합니다.
func NewSystemHandler(notificationSender notification.NotificationSender, buildInfo common.BuildInfo) *SystemHandler {
	return &SystemHandler{
		notificationSender: notificationSender,

		buildInfo: buildInfo,
	}
}

// HealthCheckHandler godoc
// @Summary 서버 상태 확인
// @Description 서버가 정상적으로 동작하는지 확인합니다.
// @Description
// @Description 이 엔드포인트는 인증 없이 호출할 수 있으며, 모니터링 시스템에서 서버 상태를 확인하는 데 사용됩니다.
// @Description
// @Description ## 응답 필드
// @Description - status: 전체 서버 상태 (healthy, unhealthy)
// @Description - uptime: 서버 가동 시간 (초)
// @Description - dependencies: 의존성 서비스 상태 (notification_service 등)
// @Tags System
// @Produce json
// @Success 200 {object} response.HealthResponse "서버 정상"
// @Failure 500 {object} response.ErrorResponse "서버 내부 오류"
// @Router /health [get]
func (h *SystemHandler) HealthCheckHandler(c echo.Context) error {
	applog.WithComponentAndFields("api.handler", log.Fields{
		"endpoint": "/health",
	}).Debug("헬스체크 요청")

	uptime := int64(time.Since(serverStartTime).Seconds())

	// 의존성 상태 체크
	deps := make(map[string]response.DependencyStatus)

	// NotificationSender 상태 체크
	if h.notificationSender != nil {
		deps[dependencyNotificationService] = response.DependencyStatus{
			Status:  statusHealthy,
			Message: "정상 작동 중",
		}
	} else {
		deps[dependencyNotificationService] = response.DependencyStatus{
			Status:  statusUnhealthy,
			Message: "서비스가 초기화되지 않음",
		}
	}

	// 전체 상태 결정
	overallStatus := statusHealthy
	for _, dep := range deps {
		if dep.Status != statusHealthy {
			overallStatus = statusUnhealthy
			break
		}
	}

	return c.JSON(http.StatusOK, response.HealthResponse{
		Status:       overallStatus,
		Uptime:       uptime,
		Dependencies: deps,
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
// @Success 200 {object} response.VersionResponse "버전 정보"
// @Failure 500 {object} response.ErrorResponse "서버 내부 오류"
// @Router /version [get]
func (h *SystemHandler) VersionHandler(c echo.Context) error {
	applog.WithComponentAndFields("api.handler", log.Fields{
		"endpoint": "/version",
	}).Debug("버전 정보 요청")

	return c.JSON(http.StatusOK, response.VersionResponse{
		Version:     h.buildInfo.Version,
		BuildDate:   h.buildInfo.BuildDate,
		BuildNumber: h.buildInfo.BuildNumber,
		GoVersion:   runtime.Version(),
	})
}
