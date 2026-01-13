package system

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/pkg/version"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/system"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupSystemHandlerTest 테스트에 필요한 Handler와 의존성을 설정합니다.
// 테스트 격리를 위해 매번 새로운 인스턴스를 생성합니다.
func setupSystemHandlerTest(t *testing.T) (*Handler, *mocks.MockNotificationSender, *echo.Echo) {
	t.Helper()

	// 로그 레벨 조정 (테스트 중 불필요한 로그 방지)
	originalLevel := applog.StandardLogger().Level
	applog.SetLevel(applog.FatalLevel)

	t.Cleanup(func() {
		applog.SetLevel(originalLevel)
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

// =============================================================================
// Constructor Tests
// =============================================================================

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

		assert.PanicsWithValue(t, constants.PanicMsgNotificationSenderRequired, func() {
			NewHandler(nil, buildInfo)
		})
	})
}

// =============================================================================
// Health Check Tests
// =============================================================================

func TestHandler_HealthCheckHandler(t *testing.T) {
	// 공통 검증 로직 Helper
	assertHealthResponse := func(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus string, expectedDeps map[string]system.DependencyStatus) {
		t.Helper()

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp system.HealthResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, expectedStatus, resp.Status)
		assert.GreaterOrEqual(t, resp.Uptime, int64(0)) // Uptime은 0 이상
		assert.Equal(t, expectedDeps, resp.Dependencies)
	}

	t.Run("성공: 모든 시스템 정상 (Healthy)", func(t *testing.T) {
		h, mockSender, e := setupSystemHandlerTest(t)

		// Mock 설정: Health() 성공 (nil 반환)
		mockSender.ShouldFail = false

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.HealthCheckHandler(c)
		assert.NoError(t, err)

		expectedDeps := map[string]system.DependencyStatus{
			constants.DependencyNotificationService: {
				Status:  constants.HealthStatusHealthy,
				Message: constants.MsgDepStatusHealthy,
			},
		}
		assertHealthResponse(t, rec, constants.HealthStatusHealthy, expectedDeps)
	})

	t.Run("실패: Notification 서비스 장애 (Unhealthy - Deep Check)", func(t *testing.T) {
		h, mockSender, e := setupSystemHandlerTest(t)

		// Mock 설정: Health() 실패 시뮬레이션
		mockSender.ShouldFail = true
		mockSender.FailError = errors.New("service stopped")

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.HealthCheckHandler(c)
		assert.NoError(t, err)

		expectedDeps := map[string]system.DependencyStatus{
			constants.DependencyNotificationService: {
				Status:  constants.HealthStatusUnhealthy,
				Message: "service stopped",
			},
		}
		// 하나라도 Unhealthy면 전체 상태도 Unhealthy
		assertHealthResponse(t, rec, constants.HealthStatusUnhealthy, expectedDeps)
	})

	t.Run("실패: Notification Sender 미초기화 (Unhealthy - Safety Check)", func(t *testing.T) {
		// NewHandler를 우회하여 강제로 nil 의존성 주입
		h := &Handler{
			notificationSender: nil,
			serverStartTime:    time.Now(),
		}
		e := echo.New()

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.HealthCheckHandler(c)
		assert.NoError(t, err)

		expectedDeps := map[string]system.DependencyStatus{
			constants.DependencyNotificationService: {
				Status:  constants.HealthStatusUnhealthy,
				Message: constants.MsgDepStatusNotInitialized,
			},
		}
		assertHealthResponse(t, rec, constants.HealthStatusUnhealthy, expectedDeps)
	})
}

// =============================================================================
// Version Info Tests
// =============================================================================

func TestHandler_VersionHandler(t *testing.T) {
	t.Run("성공: 버전 정보 반환", func(t *testing.T) {
		h, _, e := setupSystemHandlerTest(t)

		req := httptest.NewRequest(http.MethodGet, "/version", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.VersionHandler(c)
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
