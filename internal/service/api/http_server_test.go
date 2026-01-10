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
// HTTP Server Configuration Tests
// =============================================================================

// TestNewHTTPServer_Configuration Echo 서버의 기본 설정을 검증합니다.
//
// 검증 항목:
//   - Debug 모드 설정
//   - HideBanner 설정
//   - Logger 설정
//   - Echo 인스턴스 생성
func TestNewHTTPServer_Configuration(t *testing.T) {
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
		{
			name: "Empty Config",
			config: HTTPServerConfig{
				Debug:        false,
				AllowOrigins: []string{},
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

// =============================================================================
// Middleware Tests
// =============================================================================

// TestNewHTTPServer_CORSMiddleware CORS 미들웨어 동작을 검증합니다.
//
// 검증 항목:
//   - AllowOrigins 설정 반영
//   - Preflight 요청 (OPTIONS) 처리
//   - 실제 CORS 요청 처리
func TestNewHTTPServer_CORSMiddleware(t *testing.T) {
	tests := []struct {
		name               string
		allowOrigins       []string
		requestOrigin      string
		requestMethod      string
		expectStatus       int
		expectAllowOrigin  string
		expectAllowMethods bool
	}{
		{
			name:               "Wildcard Origin - Preflight",
			allowOrigins:       []string{"*"},
			requestOrigin:      "http://example.com",
			requestMethod:      http.MethodOptions,
			expectStatus:       http.StatusNoContent,
			expectAllowOrigin:  "*",
			expectAllowMethods: true,
		},
		{
			name:               "Wildcard Origin - GET Request",
			allowOrigins:       []string{"*"},
			requestOrigin:      "http://example.com",
			requestMethod:      http.MethodGet,
			expectStatus:       http.StatusOK,
			expectAllowOrigin:  "*",
			expectAllowMethods: false,
		},
		{
			name:               "Specific Origin - Preflight",
			allowOrigins:       []string{"http://example.com"},
			requestOrigin:      "http://example.com",
			requestMethod:      http.MethodOptions,
			expectStatus:       http.StatusNoContent,
			expectAllowOrigin:  "http://example.com",
			expectAllowMethods: true,
		},
		{
			name:               "Specific Origin - POST Request",
			allowOrigins:       []string{"http://example.com"},
			requestOrigin:      "http://example.com",
			requestMethod:      http.MethodPost,
			expectStatus:       http.StatusOK,
			expectAllowOrigin:  "http://example.com",
			expectAllowMethods: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := HTTPServerConfig{
				Debug:        false,
				AllowOrigins: tt.allowOrigins,
			}
			e := NewHTTPServer(cfg)

			// Register test handler
			e.GET("/test", func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			})
			e.POST("/test", func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			})

			// Setup request
			req := httptest.NewRequest(tt.requestMethod, "/test", nil)
			req.Header.Set("Origin", tt.requestOrigin)
			if tt.requestMethod == http.MethodOptions {
				req.Header.Set("Access-Control-Request-Method", "GET")
			}
			rec := httptest.NewRecorder()

			// Execute
			e.ServeHTTP(rec, req)

			// Verify
			assert.Equal(t, tt.expectStatus, rec.Code, "Status code mismatch")
			assert.Equal(t, tt.expectAllowOrigin, rec.Header().Get("Access-Control-Allow-Origin"), "Allow-Origin mismatch")

			if tt.expectAllowMethods {
				assert.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Methods"), "Allow-Methods should be set for preflight")
			}
		})
	}
}

// TestNewHTTPServer_SecureMiddleware Secure 미들웨어가 보안 헤더를 설정하는지 검증합니다.
//
// 검증 항목:
//   - X-XSS-Protection 헤더
//   - X-Content-Type-Options 헤더
//   - X-Frame-Options 헤더
func TestNewHTTPServer_SecureMiddleware(t *testing.T) {
	cfg := HTTPServerConfig{
		Debug:        false,
		AllowOrigins: []string{"*"},
	}
	e := NewHTTPServer(cfg)

	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Verify security headers
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-XSS-Protection"), "X-XSS-Protection header should be set")
	assert.NotEmpty(t, rec.Header().Get("X-Content-Type-Options"), "X-Content-Type-Options header should be set")
	assert.NotEmpty(t, rec.Header().Get("X-Frame-Options"), "X-Frame-Options header should be set")
}

// TestNewHTTPServer_RequestIDMiddleware RequestID 미들웨어가 요청 ID를 생성하는지 검증합니다.
//
// 검증 항목:
//   - X-Request-ID 헤더 존재
func TestNewHTTPServer_RequestIDMiddleware(t *testing.T) {
	cfg := HTTPServerConfig{
		Debug:        false,
		AllowOrigins: []string{"*"},
	}
	e := NewHTTPServer(cfg)

	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Verify Request ID header
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID), "X-Request-ID header should be set")
}

// TestNewHTTPServer_RateLimitingMiddleware Rate Limiting 미들웨어가 요청을 제한하는지 검증합니다.
//
// 검증 항목:
//   - 버스트 초과 시 429 응답
//   - Retry-After 헤더 설정
//   - IP별 독립 제한
func TestNewHTTPServer_RateLimitingMiddleware(t *testing.T) {
	t.Run("Burst Exceeded", func(t *testing.T) {
		cfg := HTTPServerConfig{
			Debug:        false,
			AllowOrigins: []string{"*"},
		}
		e := NewHTTPServer(cfg)

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		// 버스트(40) 초과 요청
		allowedCount := 0
		blockedCount := 0

		for i := 0; i < 50; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Real-IP", "192.168.1.100")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				allowedCount++
			} else if rec.Code == http.StatusTooManyRequests {
				blockedCount++
				// Verify Retry-After header
				assert.Equal(t, "1", rec.Header().Get("Retry-After"), "Retry-After header should be set")
			}
		}

		// 버스트는 40이므로 최소 40개는 허용되어야 함
		assert.GreaterOrEqual(t, allowedCount, 40, "At least burst amount should be allowed")
		assert.Greater(t, blockedCount, 0, "Some requests should be blocked after burst")
	})

	t.Run("IP Isolation", func(t *testing.T) {
		cfg := HTTPServerConfig{
			Debug:        false,
			AllowOrigins: []string{"*"},
		}
		e := NewHTTPServer(cfg)

		e.GET("/test", func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		// IP1에서 버스트 소진
		for i := 0; i < 45; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Real-IP", "192.168.1.101")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
		}

		// IP2에서 요청 (허용되어야 함)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Real-IP", "192.168.1.102")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "Different IP should have independent rate limit")
	})
}

// TestNewHTTPServer_MiddlewareIntegration 모든 미들웨어가 함께 동작하는지 검증합니다.
//
// 검증 항목:
//   - 여러 미들웨어가 동시에 적용됨
//   - 미들웨어 간 충돌 없음
func TestNewHTTPServer_MiddlewareIntegration(t *testing.T) {
	cfg := HTTPServerConfig{
		Debug:        false,
		AllowOrigins: []string{"http://example.com"},
	}
	e := NewHTTPServer(cfg)

	e.GET("/test", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Verify all middleware effects
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID), "RequestID should be set")
	assert.Equal(t, "http://example.com", rec.Header().Get("Access-Control-Allow-Origin"), "CORS should be set")
	assert.NotEmpty(t, rec.Header().Get("X-XSS-Protection"), "Secure headers should be set")
	assert.NotEmpty(t, rec.Header().Get("X-Content-Type-Options"), "Secure headers should be set")
}
