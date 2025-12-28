package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// HTTP Server Tests
// =============================================================================

// TestNewHTTPServer_Table은 HTTP 서버 생성을 검증합니다.
//
// 검증 항목:
//   - Debug 모드 설정
//   - Banner 숨김 설정
//   - Logger 설정
func TestNewHTTPServer_Table(t *testing.T) {
	tests := []struct {
		name         string
		config       HTTPServerConfig
		expectDebug  bool
		expectBanner bool // HideBanner
	}{
		{
			name: "Debug Enabled",
			config: HTTPServerConfig{
				Debug:        true,
				AllowOrigins: []string{"*"},
			},
			expectDebug:  true,
			expectBanner: true, // NewHTTPServer sets HideBanner=true
		},
		{
			name: "Debug Disabled",
			config: HTTPServerConfig{
				Debug:        false,
				AllowOrigins: []string{"http://example.com"},
			},
			expectDebug:  false,
			expectBanner: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewHTTPServer(tt.config)

			require.NotNil(t, e, "Echo instance should be created")
			assert.Equal(t, tt.expectDebug, e.Debug, "Debug mode mismatch")
			assert.Equal(t, tt.expectBanner, e.HideBanner, "HideBanner mismatch")
			require.NotNil(t, e.Logger, "Logger should be set")
		})
	}
}

// TestServerMiddlewares_Table은 서버 미들웨어 동작을 검증합니다.
//
// 검증 항목:
//   - CORS 미들웨어
//   - Recover 미들웨어 (panic 처리)
//   - RequestID 미들웨어
func TestServerMiddlewares_Table(t *testing.T) {
	// Common config
	cfg := HTTPServerConfig{
		Debug:        true,
		AllowOrigins: []string{"*"},
	}

	tests := []struct {
		name           string
		setupRequest   func() (*http.Request, *httptest.ResponseRecorder)
		handler        echo.HandlerFunc
		path           string
		expectStatus   int
		verifyResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "CORS Middleware",
			setupRequest: func() (*http.Request, *httptest.ResponseRecorder) {
				req := httptest.NewRequest(http.MethodOptions, "/test", nil)
				req.Header.Set("Origin", "http://example.com")
				req.Header.Set("Access-Control-Request-Method", "GET")
				return req, httptest.NewRecorder()
			},
			handler:      func(c echo.Context) error { return c.String(http.StatusOK, "ok") },
			path:         "/test",
			expectStatus: http.StatusNoContent, // OPTIONS request usually returns 204 No Content for CORS preflight
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
			},
		},
		{
			name: "Recover Middleware",
			setupRequest: func() (*http.Request, *httptest.ResponseRecorder) {
				return httptest.NewRequest(http.MethodGet, "/panic", nil), httptest.NewRecorder()
			},
			handler:      func(c echo.Context) error { panic("test panic") },
			path:         "/panic",
			expectStatus: http.StatusInternalServerError, // 500
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				// Body might contain error info or be standard error page depending on config
			},
		},
		{
			name: "RequestID Middleware",
			setupRequest: func() (*http.Request, *httptest.ResponseRecorder) {
				return httptest.NewRequest(http.MethodGet, "/test", nil), httptest.NewRecorder()
			},
			handler:      func(c echo.Context) error { return c.String(http.StatusOK, "ok") },
			path:         "/test",
			expectStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewHTTPServer(cfg)

			// Register handler
			e.GET(tt.path, tt.handler)

			req, rec := tt.setupRequest()
			e.ServeHTTP(rec, req)

			// Special handling for CORS Preflight which might differ based on Echo version or config
			// But generally if handler is executed it returns StatusOK, if CORS handles OPTIONS it might return NO Content.
			// Let's assert status code if specified
			if tt.expectStatus != 0 {
				assert.Equal(t, tt.expectStatus, rec.Code)
			}

			if tt.verifyResponse != nil {
				tt.verifyResponse(t, rec)
			}
		})
	}
}
