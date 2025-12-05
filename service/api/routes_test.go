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

// TestSetupRoutes는 SetupRoutes 함수가 올바르게 라우트를 설정하는지 테스트합니다.
func TestSetupRoutes(t *testing.T) {
	e := echo.New()

	// Mock SystemHandler 생성
	buildInfo := common.BuildInfo{
		Version:     "test-version",
		BuildDate:   "2025-12-05",
		BuildNumber: "1",
	}
	systemHandler := handler.NewSystemHandler(nil, buildInfo)

	// 라우트 설정
	SetupRoutes(e, systemHandler)

	// 등록된 라우트 확인
	routes := e.Routes()

	// 최소한 다음 라우트들이 등록되어야 함
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

// TestHealthCheckRoute는 /health 엔드포인트가 정상적으로 동작하는지 테스트합니다.
func TestHealthCheckRoute(t *testing.T) {
	e := echo.New()

	buildInfo := common.BuildInfo{
		Version:     "test-version",
		BuildDate:   "2025-12-05",
		BuildNumber: "1",
	}
	systemHandler := handler.NewSystemHandler(nil, buildInfo)

	SetupRoutes(e, systemHandler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var healthResp response.HealthResponse
	err := json.Unmarshal(rec.Body.Bytes(), &healthResp)
	require.NoError(t, err)

	assert.NotEmpty(t, healthResp.Status)
	assert.GreaterOrEqual(t, healthResp.Uptime, int64(0))
}

// TestVersionRoute는 /version 엔드포인트가 정상적으로 동작하는지 테스트합니다.
func TestVersionRoute(t *testing.T) {
	e := echo.New()

	buildInfo := common.BuildInfo{
		Version:     "test-version",
		BuildDate:   "2025-12-05",
		BuildNumber: "100",
	}
	systemHandler := handler.NewSystemHandler(nil, buildInfo)

	SetupRoutes(e, systemHandler)

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var versionResp response.VersionResponse
	err := json.Unmarshal(rec.Body.Bytes(), &versionResp)
	require.NoError(t, err)

	assert.Equal(t, "test-version", versionResp.Version)
	assert.Equal(t, "2025-12-05", versionResp.BuildDate)
	assert.Equal(t, "100", versionResp.BuildNumber)
	assert.NotEmpty(t, versionResp.GoVersion)
}

// TestCustomHTTPErrorHandler_404는 404 에러 핸들러가 올바른 응답을 반환하는지 테스트합니다.
func TestCustomHTTPErrorHandler_404(t *testing.T) {
	e := echo.New()

	buildInfo := common.BuildInfo{
		Version:     "test-version",
		BuildDate:   "2025-12-05",
		BuildNumber: "1",
	}
	systemHandler := handler.NewSystemHandler(nil, buildInfo)

	SetupRoutes(e, systemHandler)

	// 존재하지 않는 경로 요청
	req := httptest.NewRequest(http.MethodGet, "/non-existent-path", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errorResp response.ErrorResponse
	err := json.Unmarshal(rec.Body.Bytes(), &errorResp)
	require.NoError(t, err)

	assert.Equal(t, "페이지를 찾을 수 없습니다.", errorResp.Message)
}

// TestCustomHTTPErrorHandler_MethodNotAllowed는 405 에러 핸들러가 올바른 응답을 반환하는지 테스트합니다.
func TestCustomHTTPErrorHandler_MethodNotAllowed(t *testing.T) {
	e := echo.New()

	buildInfo := common.BuildInfo{
		Version:     "test-version",
		BuildDate:   "2025-12-05",
		BuildNumber: "1",
	}
	systemHandler := handler.NewSystemHandler(nil, buildInfo)

	SetupRoutes(e, systemHandler)

	// GET만 허용되는 엔드포인트에 POST 요청
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	var errorResp response.ErrorResponse
	err := json.Unmarshal(rec.Body.Bytes(), &errorResp)
	require.NoError(t, err)

	assert.NotEmpty(t, errorResp.Message)
}

// TestCustomHTTPErrorHandler_HEAD는 HEAD 요청이 본문 없이 응답하는지 테스트합니다.
func TestCustomHTTPErrorHandler_HEAD(t *testing.T) {
	e := echo.New()

	buildInfo := common.BuildInfo{
		Version:     "test-version",
		BuildDate:   "2025-12-05",
		BuildNumber: "1",
	}
	systemHandler := handler.NewSystemHandler(nil, buildInfo)

	SetupRoutes(e, systemHandler)

	// 존재하지 않는 경로에 HEAD 요청
	req := httptest.NewRequest(http.MethodHead, "/non-existent-path", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Empty(t, rec.Body.String(), "HEAD 요청은 본문이 없어야 합니다")
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
