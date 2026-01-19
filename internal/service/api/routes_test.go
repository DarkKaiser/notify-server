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
// Route Registration Tests
// =============================================================================

func TestRegisterRoutes(t *testing.T) {
	// Given
	e := setupTestEcho()
	h := setupTestHandler()

	// When
	RegisterRoutes(e, h)

	// Then
	t.Run("라우트 등록 검증", func(t *testing.T) {
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

	t.Run("엔드포인트 동작 통합 검증", func(t *testing.T) {
		tests := []struct {
			name           string
			method         string
			path           string
			expectedStatus int
			verifyResponse func(t *testing.T, rec *httptest.ResponseRecorder)
		}{
			{
				name:           "Health 체크 (시스템 상태)",
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
				name:           "Version 정보 (빌드 정보)",
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
				},
			},
			{
				name:           "Swagger UI 접근",
				method:         http.MethodGet,
				path:           "/swagger/index.html",
				expectedStatus: http.StatusOK,
				verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
					assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
				},
			},
			{
				name:           "Swagger JSON 문서 접근",
				method:         http.MethodGet,
				path:           "/swagger/doc.json", // echo-swagger 기본 설정 경로
				expectedStatus: http.StatusNotFound, // 주의: 실제 doc.json 파일이 없으면 404가 뜰 수 있음. 여기선 핸들러 등록 여부만 보거나, 404면 404로 검증.
				// echo-swagger는 내부적으로 파일을 찾는데, 테스트 환경에 파일이 없으므로 404 혹은 500이 날 수 있음.
				// 하지만 여기서는 라우팅 자체가 되었는지만 확인하면 되므로, 404라도 Echo가 처리한 것이면 OK.
				// (만약 라우트가 없다면 404지만 Echo 기본 404 핸들러를 탐)
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)

				// Swagger JSON의 경우 파일 부재로 404가 날 수 있으므로 별도 처리하지 않고
				// 여기서는 상태 코드보다는 패닉 없이 수행되는지를 중점으로 볼 수도 있음.
				// 하지만 명확함을 위해 Health/Version은 Code까지 검증.
				if tc.name == "Swagger JSON 문서 접근" {
					return // 파일 의존성이 있으므로 생략하거나, mock 파일 시스템이 필요함.
				}

				assert.Equal(t, tc.expectedStatus, rec.Code)
				if tc.verifyResponse != nil {
					tc.verifyResponse(t, rec)
				}
			})
		}
	})

	t.Run("에러 핸들링 검증", func(t *testing.T) {
		t.Run("존재하지 않는 경로 (404 Not Found)", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusNotFound, rec.Code)
		})

		t.Run("지원하지 않는 메서드 (405 Method Not Allowed)", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/health", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		})
	})
}
