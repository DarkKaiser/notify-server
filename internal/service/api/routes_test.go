package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/version"
	systemhandler "github.com/darkkaiser/notify-server/internal/service/api/handler/system"
	"github.com/darkkaiser/notify-server/internal/service/api/model/system"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Functions
// =============================================================================

func setupTestEcho() *echo.Echo {
	return echo.New()
}

func setupTestHandler() *systemhandler.Handler {
	mockSender := mocks.NewMockNotificationSender()
	buildInfo := version.Info{
		Version:     "test-version",
		BuildDate:   "2025-12-05",
		BuildNumber: "1",
	}
	return systemhandler.New(mockSender, buildInfo)
}

// =============================================================================
// Unit Tests: Individual Route Registration Functions
// =============================================================================

func TestRegisterSystemRoutes(t *testing.T) {
	t.Parallel()

	t.Run("시스템 라우트 등록 확인", func(t *testing.T) {
		t.Parallel()
		e := setupTestEcho()
		h := setupTestHandler()

		registerSystemRoutes(e, h)

		routes := e.Routes()
		expectedRoutes := map[string]string{
			"/health":  http.MethodGet,
			"/version": http.MethodGet,
		}

		for path, method := range expectedRoutes {
			found := false
			for _, r := range routes {
				if r.Path == path && r.Method == method {
					found = true
					break
				}
			}
			assert.True(t, found, "라우트 %s %s가 등록되어야 합니다", method, path)
		}
	})

	t.Run("Health 엔드포인트 동작 확인", func(t *testing.T) {
		t.Parallel()
		e := setupTestEcho()
		h := setupTestHandler()
		registerSystemRoutes(e, h)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var healthResp system.HealthResponse
		err := json.Unmarshal(rec.Body.Bytes(), &healthResp)
		require.NoError(t, err)
		assert.NotEmpty(t, healthResp.Status)
	})

	t.Run("Version 엔드포인트 동작 확인", func(t *testing.T) {
		t.Parallel()
		e := setupTestEcho()
		h := setupTestHandler()
		registerSystemRoutes(e, h)

		req := httptest.NewRequest(http.MethodGet, "/version", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var versionResp system.VersionResponse
		err := json.Unmarshal(rec.Body.Bytes(), &versionResp)
		require.NoError(t, err)
		assert.Equal(t, "test-version", versionResp.Version)
	})
}

func TestRegisterSwaggerRoutes(t *testing.T) {
	t.Parallel()

	t.Run("Swagger 라우트 등록 확인", func(t *testing.T) {
		t.Parallel()
		e := setupTestEcho()

		registerSwaggerRoutes(e)

		routes := e.Routes()
		found := false
		for _, r := range routes {
			if r.Path == "/swagger/*" && r.Method == http.MethodGet {
				found = true
				break
			}
		}
		assert.True(t, found, "Swagger 라우트가 등록되어야 합니다")
	})

	t.Run("Swagger UI 접근 가능 확인", func(t *testing.T) {
		t.Parallel()
		e := setupTestEcho()
		registerSwaggerRoutes(e)

		req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	})
}

// =============================================================================
// Integration Tests: Complete Route Setup
// =============================================================================

func TestRegisterRoutes(t *testing.T) {
	t.Parallel()

	t.Run("모든 라우트 등록 확인", func(t *testing.T) {
		t.Parallel()
		e := setupTestEcho()
		h := setupTestHandler()

		RegisterRoutes(e, h)

		expectedRoutes := map[string]string{
			"/health":    http.MethodGet,
			"/version":   http.MethodGet,
			"/swagger/*": http.MethodGet,
		}

		routes := e.Routes()
		for path, method := range expectedRoutes {
			found := false
			for _, r := range routes {
				if r.Path == path && r.Method == method {
					found = true
					break
				}
			}
			assert.True(t, found, "라우트 %s %s가 등록되어야 합니다", method, path)
		}
	})

	t.Run("통합 엔드포인트 동작 검증", func(t *testing.T) {
		t.Parallel()
		e := setupTestEcho()
		h := setupTestHandler()
		RegisterRoutes(e, h)

		tests := []struct {
			name           string
			method         string
			path           string
			expectedStatus int
			verifyResponse func(t *testing.T, rec *httptest.ResponseRecorder)
		}{
			{
				name:           "Health 체크",
				method:         http.MethodGet,
				path:           "/health",
				expectedStatus: http.StatusOK,
				verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
					var healthResp system.HealthResponse
					err := json.Unmarshal(rec.Body.Bytes(), &healthResp)
					require.NoError(t, err)
					assert.NotEmpty(t, healthResp.Status)
					assert.GreaterOrEqual(t, healthResp.Uptime, int64(0))
				},
			},
			{
				name:           "Version 정보",
				method:         http.MethodGet,
				path:           "/version",
				expectedStatus: http.StatusOK,
				verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
					var versionResp system.VersionResponse
					err := json.Unmarshal(rec.Body.Bytes(), &versionResp)
					require.NoError(t, err)
					assert.Equal(t, "test-version", versionResp.Version)
					assert.Equal(t, "2025-12-05", versionResp.BuildDate)
					assert.Equal(t, "1", versionResp.BuildNumber)
					assert.NotEmpty(t, versionResp.GoVersion)
				},
			},
			{
				name:           "Swagger UI",
				method:         http.MethodGet,
				path:           "/swagger/index.html",
				expectedStatus: http.StatusOK,
				verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
					assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
				},
			},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				req := httptest.NewRequest(tc.method, tc.path, nil)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)

				assert.Equal(t, tc.expectedStatus, rec.Code)

				if tc.verifyResponse != nil {
					tc.verifyResponse(t, rec)
				}
			})
		}
	})

	t.Run("잘못된 HTTP 메서드 (405)", func(t *testing.T) {
		t.Parallel()
		e := setupTestEcho()
		h := setupTestHandler()
		RegisterRoutes(e, h)

		req := httptest.NewRequest(http.MethodPost, "/health", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})

	t.Run("존재하지 않는 경로 (404)", func(t *testing.T) {
		t.Parallel()
		e := setupTestEcho()
		h := setupTestHandler()
		RegisterRoutes(e, h)

		req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
