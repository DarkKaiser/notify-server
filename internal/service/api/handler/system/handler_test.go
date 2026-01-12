package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/pkg/version"
	"github.com/darkkaiser/notify-server/internal/service/api/model/system"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupSystemHandlerTest 테스트에 필요한 Handler와 의존성을 설정합니다.
func setupSystemHandlerTest(t *testing.T) (*Handler, *mocks.MockNotificationSender, *echo.Echo) {
	t.Helper()

	// 로그 레벨 조정 (테스트 중 불필요한 로그 방지)
	applog.SetLevel(applog.FatalLevel)
	t.Cleanup(func() {
		applog.SetLevel(applog.InfoLevel)
	})

	mockSender := mocks.NewMockNotificationSender()
	buildInfo := version.Info{
		Version:     "1.0.0",
		BuildDate:   "2024-01-01",
		BuildNumber: "100",
	}

	h := NewHandler(mockSender, buildInfo)
	e := echo.New()

	return h, mockSender, e
}

func TestNewHandler(t *testing.T) {
	t.Run("성공: 올바른 의존성으로 핸들러 생성", func(t *testing.T) {
		mockSender := mocks.NewMockNotificationSender()
		buildInfo := version.Info{Version: "1.0.0"}

		h := NewHandler(mockSender, buildInfo)

		assert.NotNil(t, h)
		assert.Equal(t, mockSender, h.notificationSender)
		assert.Equal(t, buildInfo, h.buildInfo)
		assert.False(t, h.serverStartTime.IsZero(), "서버 시작 시간이 설정되어야 합니다")
		assert.WithinDuration(t, time.Now(), h.serverStartTime, 1*time.Second, "서버 시작 시간은 현재 시간과 비슷해야 합니다")
	})

	t.Run("실패: NotificationSender가 nil인 경우 Panic", func(t *testing.T) {
		buildInfo := version.Info{Version: "1.0.0"}

		assert.PanicsWithValue(t, "NotificationSender는 필수입니다", func() {
			NewHandler(nil, buildInfo)
		})
	})
}

func TestHandler_HealthCheckHandler(t *testing.T) {
	t.Run("성공: 모든 의존성이 정상일 때 Healthy 반환", func(t *testing.T) {
		h, _, e := setupSystemHandlerTest(t)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// 핸들러 실행
		err := h.HealthCheckHandler(c)

		// 검증
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp system.HealthResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, statusHealthy, resp.Status)
		assert.GreaterOrEqual(t, resp.Uptime, int64(0)) // Uptime은 0 이상이어야 함

		// 의존성 상태 확인
		require.Contains(t, resp.Dependencies, dependencyNotificationService)
		assert.Equal(t, statusHealthy, resp.Dependencies[dependencyNotificationService].Status)
	})

	t.Run("성공: 의존성이 비정상일 때 Unhealthy 반환", func(t *testing.T) {
		// 의도적으로 nil sender를 주입하여 unhealthy 상태 시뮬레이션
		// (NewHandler는 panic을 일으키므로 구조체 직접 생성)
		h := &Handler{
			notificationSender: nil,
			serverStartTime:    time.Now(),
		}
		e := echo.New()

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// 핸들러 실행
		err := h.HealthCheckHandler(c)

		// 검증 - 상태 코드는 200 OK (비즈니스 로직상 Unhealthy라도 응답 자체는 성공)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp system.HealthResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		// 전체 상태 Unhealthy 확인
		assert.Equal(t, statusUnhealthy, resp.Status, "의존성이 누락되었으므로 전체 상태는 Unhealthy여야 함")

		// 의존성별 상태 확인
		require.Contains(t, resp.Dependencies, dependencyNotificationService)
		assert.Equal(t, statusUnhealthy, resp.Dependencies[dependencyNotificationService].Status)
		assert.Equal(t, "서비스가 초기화되지 않음", resp.Dependencies[dependencyNotificationService].Message)
	})
}

func TestHandler_VersionHandler(t *testing.T) {
	t.Run("성공: 버전 정보 반환", func(t *testing.T) {
		h, _, e := setupSystemHandlerTest(t)

		req := httptest.NewRequest(http.MethodGet, "/version", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// 핸들러 실행
		err := h.VersionHandler(c)

		// 검증
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp system.VersionResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, "1.0.0", resp.Version)
		assert.Equal(t, "2024-01-01", resp.BuildDate)
		assert.Equal(t, "100", resp.BuildNumber)
		assert.Equal(t, runtime.Version(), resp.GoVersion)
	})
}
