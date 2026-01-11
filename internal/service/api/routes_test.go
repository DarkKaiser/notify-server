package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/version"
	systemhandler "github.com/darkkaiser/notify-server/internal/service/api/handler/system"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	"github.com/darkkaiser/notify-server/internal/service/api/model/system"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Routes Setup Tests
// =============================================================================

// TestSetupRoutes_Integration_Table 은 전체 라우트 설정 및 핸들러 동작을 통합 테스트합니다.
func TestSetupRoutes_Integration_Table(t *testing.T) {
	e := echo.New()
	buildInfo := version.Info{
		Version:     "test-version",
		BuildDate:   "2025-12-05",
		BuildNumber: "1",
		GoVersion:   "go1.21",
	}
	systemHandler := systemhandler.NewHandler(nil, buildInfo)
	SetupRoutes(e, systemHandler)

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
				require.NoError(t, err, "Health 응답은 JSON이어야 함")
				assert.NotEmpty(t, healthResp.Status, "상태값이 있어야 함")
				assert.GreaterOrEqual(t, healthResp.Uptime, int64(0), "Uptime은 0 이상이어야 함")
			},
		},
		{
			name:           "Version 정보 확인",
			method:         http.MethodGet,
			path:           "/version",
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var versionResp system.VersionResponse
				err := json.Unmarshal(rec.Body.Bytes(), &versionResp)
				require.NoError(t, err, "Version 응답은 JSON이어야 함")
				assert.Equal(t, "test-version", versionResp.Version)
				assert.Equal(t, "2025-12-05", versionResp.BuildDate)
				assert.Equal(t, "1", versionResp.BuildNumber)
				assert.NotEmpty(t, versionResp.GoVersion)
			},
		},
		{
			// echo-swagger 미들웨어는 정적 파일이 없더라도 /swagger/index.html 경로에 대해
			// 기본적으로 200 OK와 함께 자체 UI 템플릿을 렌더링하려고 시도할 수 있음.
			// 따라서 상태 코드를 200으로 변경하고, Content-Type이 HTML인지 확인하여
			// 라우트가 정상적으로 연결되었음을 검증함.
			name:           "Swagger UI 접근",
			method:         http.MethodGet,
			path:           "/swagger/index.html",
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
			},
		},
		{
			name:           "존재하지 않는 라우트 (404)",
			method:         http.MethodGet,
			path:           "/undefined-route",
			expectedStatus: http.StatusNotFound,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				// SetupRoutes에서 커스텀 핸들러가 설정되었으므로 JSON 응답을 기대할 수 있음
				// 단, echo.New()는 기본적으로 텍스트 응답을 줄 수 있으나, SetupRoutes 내부에서
				// ErrorHandler를 설정하므로 JSON이어야 함.
				assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
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

// TestSetupRoutes_Registration 은 각 엔드포인트가 정확한 경로와 메서드로 등록되었는지 검증합니다.
func TestSetupRoutes_Registration(t *testing.T) {
	e := echo.New()
	handler := systemhandler.NewHandler(nil, version.Info{})
	SetupRoutes(e, handler)

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
		assert.True(t, found, "라우트 %s %s 가 등록되어야 합니다", method, path)
	}
}

// TestSetupRoutes_ErrorHandler 은 커스텀 에러 핸들러가 올바르게 설정되었는지 검증합니다.
func TestSetupRoutes_ErrorHandler(t *testing.T) {
	e := echo.New()
	handler := systemhandler.NewHandler(nil, version.Info{})

	// 설정 전 기본 핸들러 확인 (Echo 기본 핸들러는 exported 되지 않으므로 비교는 어려우나, 설정 후 변경됨을 확인)
	originalHandler := e.HTTPErrorHandler

	SetupRoutes(e, handler)

	// 설정 후 핸들러 확인
	currentHandler := e.HTTPErrorHandler

	// 1. 핸들러가 변경되었는지 확인
	// 함수 포인터 값을 비교 (Testify assert.NotEqual 은 함수 비교 불가할 수 있으므로 reflect 사용)
	funcName1 := runtime.FuncForPC(reflect.ValueOf(originalHandler).Pointer()).Name()
	funcName2 := runtime.FuncForPC(reflect.ValueOf(currentHandler).Pointer()).Name()
	funcNameTarget := runtime.FuncForPC(reflect.ValueOf(httputil.ErrorHandler).Pointer()).Name()

	assert.NotEqual(t, funcName1, funcName2, "에러 핸들러가 변경되어야 합니다")
	assert.Equal(t, funcNameTarget, funcName2, "httputil.ErrorHandler로 설정되어야 합니다")

	// 2. 실제 동작 검증 (500 에러 발생 시)
	// 임의의 경로에서 핸들러가 에러를 리턴하도록 설정할 수는 없으나,
	// e.HTTPErrorHandler를 직접 호출하여 검증 가능
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	err := echo.NewHTTPError(http.StatusInternalServerError, "test error")

	currentHandler(err, c)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json", "커스텀 핸들러는 JSON 응답을 반환해야 합니다")
}
