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
	"github.com/darkkaiser/notify-server/internal/service/api/model/system"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
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

	mockSender := mocks.NewMockNotificationSender()
	buildInfo := version.Info{
		Version:     "1.0.0",
		BuildDate:   "2024-01-01",
		BuildNumber: "100",
	}

	h := New(mockSender, buildInfo)
	e := echo.New()

	return h, mockSender, e
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("성공: 올바른 의존성으로 핸들러 생성", func(t *testing.T) {
		t.Parallel()
		mockSender := mocks.NewMockNotificationSender()
		buildInfo := version.Info{Version: "1.0.0"}

		h := New(mockSender, buildInfo)

		assert.NotNil(t, h)
		assert.Equal(t, mockSender, h.healthChecker)
		assert.Equal(t, buildInfo, h.buildInfo)
		assert.False(t, h.serverStartTime.IsZero(), "서버 시작 시간이 설정되어야 합니다")
		assert.WithinDuration(t, time.Now(), h.serverStartTime, 1*time.Second, "서버 시작 시간은 현재 시간과 비슷해야 합니다")
	})

	t.Run("실패: NotificationSender가 nil인 경우 Panic", func(t *testing.T) {
		t.Parallel()
		buildInfo := version.Info{Version: "1.0.0"}

		assert.PanicsWithValue(t, "HealthChecker는 필수입니다", func() {
			New(nil, buildInfo)
		})
	})
}

// =============================================================================
// Health Check Tests
// =============================================================================

func TestHandler_HealthCheckHandler(t *testing.T) {
	t.Parallel()

	// 공통 검증 로직 Helper
	assertHealthResponse := func(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus string, expectedDeps map[string]system.DependencyStatus) {
		t.Helper()

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, echo.MIMEApplicationJSON, rec.Header().Get(echo.HeaderContentType))

		var resp system.HealthResponse
		err := json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.Equal(t, expectedStatus, resp.Status)
		assert.GreaterOrEqual(t, resp.Uptime, int64(0)) // Uptime은 0 이상
		assert.Equal(t, expectedDeps, resp.Dependencies)
	}

	tests := []struct {
		name        string
		setupMock   func(*mocks.MockNotificationSender)
		forceNil    bool // handler 생성 시 healthChecker를 nil로 강제 설정
		expectPanic bool
		verify      func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name: "성공: 모든 시스템 정상 (Healthy)",
			setupMock: func(m *mocks.MockNotificationSender) {
				m.On("Health").Return(nil)
			},
			verify: func(t *testing.T, rec *httptest.ResponseRecorder) {
				expectedDeps := map[string]system.DependencyStatus{
					depNotificationService: {
						Status:  healthStatusHealthy,
						Message: depNotificationServiceStatusHealthy,
					},
				}
				assertHealthResponse(t, rec, healthStatusHealthy, expectedDeps)
			},
		},
		{
			name: "실패: Notification 서비스 장애 (Unhealthy - Deep Check)",
			setupMock: func(m *mocks.MockNotificationSender) {
				m.On("Health").Return(errors.New("service stopped"))
			},
			verify: func(t *testing.T, rec *httptest.ResponseRecorder) {
				expectedDeps := map[string]system.DependencyStatus{
					depNotificationService: {
						Status:  healthStatusUnhealthy,
						Message: "service stopped",
					},
				}
				assertHealthResponse(t, rec, healthStatusUnhealthy, expectedDeps)
			},
		},
		{
			name:        "실패: Notification Sender 미초기화 (Unhealthy - Safety Check)",
			forceNil:    true,
			expectPanic: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var h *Handler
			var e *echo.Echo
			var mockSender *mocks.MockNotificationSender

			if tt.forceNil {
				// New를 우회하여 강제로 nil 의존성 주입
				h = &Handler{
					healthChecker:   nil,
					serverStartTime: time.Now(),
				}
				e = echo.New()
			} else {
				h, mockSender, e = setupSystemHandlerTest(t)
				if tt.setupMock != nil {
					tt.setupMock(mockSender)
				}
			}

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if tt.expectPanic {
				assert.Panics(t, func() {
					_ = h.HealthCheckHandler(c)
				})
			} else {
				err := h.HealthCheckHandler(c)
				assert.NoError(t, err)
				tt.verify(t, rec)
			}

			if mockSender != nil {
				mockSender.AssertExpectations(t)
			}
		})
	}
}

// =============================================================================
// Version Info Tests
// =============================================================================

func TestHandler_VersionHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		buildInfo version.Info
		verify    func(t *testing.T, resp system.VersionResponse)
	}{
		{
			name: "성공: 정상 버전 정보 반환",
			buildInfo: version.Info{
				Version:     "1.0.0",
				BuildDate:   "2024-01-01",
				BuildNumber: "100",
			},
			verify: func(t *testing.T, resp system.VersionResponse) {
				assert.Equal(t, "1.0.0", resp.Version)
				assert.Equal(t, "2024-01-01", resp.BuildDate)
				assert.Equal(t, "100", resp.BuildNumber)
				assert.Equal(t, runtime.Version(), resp.GoVersion)
			},
		},
		{
			name:      "성공: 빈 버전 정보 반환 (Zero Values)",
			buildInfo: version.Info{}, // Empty
			verify: func(t *testing.T, resp system.VersionResponse) {
				assert.Equal(t, "", resp.Version)
				assert.Equal(t, "", resp.BuildDate)
				assert.Equal(t, "", resp.BuildNumber)
				assert.Equal(t, runtime.Version(), resp.GoVersion)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// VersionHandler doesn't use the sender, so we can pass a dummy mock
			// But we use the helper for consistency, although MockSender is unused here
			// To be explicit, we can mock nothing.
			mockSender := mocks.NewMockNotificationSender()

			h := New(mockSender, tt.buildInfo)
			e := echo.New()

			req := httptest.NewRequest(http.MethodGet, "/version", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := h.VersionHandler(c)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, echo.MIMEApplicationJSON, rec.Header().Get(echo.HeaderContentType))

			var resp system.VersionResponse
			err = json.Unmarshal(rec.Body.Bytes(), &resp)
			require.NoError(t, err)

			tt.verify(t, resp)

			// No expectations set on mockSender, so AssertExpectations will pass if nothing was called
			mockSender.AssertExpectations(t)
		})
	}
}
