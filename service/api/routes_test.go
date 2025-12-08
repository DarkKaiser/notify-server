package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/pkg/common"
	"github.com/darkkaiser/notify-server/service/api/handler"
	"github.com/darkkaiser/notify-server/service/api/model/response"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGlobalRoutes_TableDriven는 전역 라우트 엔드포인트 테스트를 수행합니다.
func TestGlobalRoutes_TableDriven(t *testing.T) {
	e := echo.New()
	buildInfo := common.BuildInfo{
		Version:     "test-version",
		BuildDate:   "2025-12-05",
		BuildNumber: "1",
	}
	systemHandler := handler.NewSystemHandler(nil, buildInfo)
	SetupRoutes(e, systemHandler)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		verifyResponse func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "Health Check",
			method:         http.MethodGet,
			path:           "/health",
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var healthResp response.HealthResponse
				err := json.Unmarshal(rec.Body.Bytes(), &healthResp)
				require.NoError(t, err)
				assert.NotEmpty(t, healthResp.Status)
				assert.GreaterOrEqual(t, healthResp.Uptime, int64(0))
			},
		},
		{
			name:           "Version Check",
			method:         http.MethodGet,
			path:           "/version",
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var versionResp response.VersionResponse
				err := json.Unmarshal(rec.Body.Bytes(), &versionResp)
				require.NoError(t, err)
				assert.Equal(t, "test-version", versionResp.Version)
				assert.Equal(t, "2025-12-05", versionResp.BuildDate)
				assert.Equal(t, "1", versionResp.BuildNumber)
				assert.NotEmpty(t, versionResp.GoVersion)
			},
		},
		{
			name:           "404 Not Found 확인",
			method:         http.MethodGet,
			path:           "/undefined-route",
			expectedStatus: http.StatusNotFound,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errorResp response.ErrorResponse
				err := json.Unmarshal(rec.Body.Bytes(), &errorResp)
				assert.NoError(t, err)
				assert.Equal(t, http.StatusNotFound, rec.Code)
				// 커스텀 에러 핸들러가 동작하는지 확인 (상태 코드로 충분하지만 메시지도 확인 가능)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectedStatus, rec.Code)
			if tc.verifyResponse != nil {
				tc.verifyResponse(t, rec)
			}
		})
	}
}

// TestSetupRoutes는 SetupRoutes 함수가 올바르게 라우트를 설정하는지 테스트합니다.
func TestSetupRoutes(t *testing.T) {
	e := echo.New()
	buildInfo := common.BuildInfo{
		Version:     "test-version",
		BuildDate:   "2025-12-05",
		BuildNumber: "1",
	}
	systemHandler := handler.NewSystemHandler(nil, buildInfo)
	SetupRoutes(e, systemHandler)

	routes := e.Routes()
	expectedRoutes := map[string]string{
		"/health":    "GET",
		"/version":   "GET",
		"/swagger/*": "GET",
	}

	for path, method := range expectedRoutes {
		found := false
		for _, route := range routes {
			if route.Path == path && route.Method == method {
				found = true
				break
			}
		}
		assert.True(t, found, "라우트 %s %s가 등록되지 않았습니다", method, path)
	}
}

// TestSwaggerRoute는 /swagger/* 엔드포인트가 등록되어 있는지 테스트합니다.
func TestSwaggerRoute(t *testing.T) {
	e := echo.New()
	buildInfo := common.BuildInfo{
		Version:     "test-version",
		BuildDate:   "2025-12-05",
		BuildNumber: "1",
	}
	systemHandler := handler.NewSystemHandler(nil, buildInfo)
	SetupRoutes(e, systemHandler)

	// Swagger 라우트가 등록되어 있는지 확인
	routes := e.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/swagger/*" && route.Method == "GET" {
			found = true
			break
		}
	}
	assert.True(t, found, "Swagger 라우트가 등록되지 않았습니다")
}
