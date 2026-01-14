package api

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	appmiddleware "github.com/darkkaiser/notify-server/internal/service/api/middleware"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Configuration & Helpers
// =============================================================================

// setupTestLogger는 테스트를 위해 애플리케이션 로거의 출력을 버퍼로 리다이렉트합니다.
// 반환된 버퍼를 통해 로그 내용을 검증할 수 있습니다.
// t.Cleanup을 사용하여 테스트 종료 후 자동으로 설정을 복원합니다.
func setupTestLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := new(bytes.Buffer)
	originalOut := applog.StandardLogger().Out
	originalLevel := applog.StandardLogger().Level

	applog.SetOutput(buf)
	applog.SetFormatter(&applog.JSONFormatter{}) // 파싱하기 쉽게 JSON 포맷 사용
	applog.SetLevel(applog.DebugLevel)

	t.Cleanup(func() {
		applog.SetOutput(originalOut)
		applog.SetLevel(originalLevel)
	})

	return buf
}

// =============================================================================
// Server Initialization & Configuration Logic Tests
// =============================================================================

// TestNewHTTPServer_Configuration 은 HTTPServerConfig가 Echo 인스턴스에 올바르게 적용되는지 검증합니다.
func TestNewHTTPServer_Configuration(t *testing.T) {
	tests := []struct {
		name           string
		cfg            HTTPServerConfig
		wantDebug      bool
		wantHideBanner bool
	}{
		{
			name: "Debug Mode Enabled",
			cfg: HTTPServerConfig{
				Debug:        true,
				AllowOrigins: []string{"*"},
			},
			wantDebug:      true,
			wantHideBanner: true,
		},
		{
			name: "Debug Mode Disabled",
			cfg: HTTPServerConfig{
				Debug:        false,
				AllowOrigins: []string{"https://example.com"},
			},
			wantDebug:      false,
			wantHideBanner: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewHTTPServer(tt.cfg)

			assert.Equal(t, tt.wantDebug, e.Debug)
			assert.Equal(t, tt.wantHideBanner, e.HideBanner)
			// Logger 설정 확인 (기본 Logger가 아닌 appmiddleware.Logger로 교체되었는지)
			_, ok := e.Logger.(appmiddleware.Logger)
			assert.True(t, ok, "Logger가 appmiddleware.Logger로 교체되어야 합니다")
		})
	}
}

// TestNewHTTPServer_Defaults 는 설정 값이 누락되었을 때 기본값이 올바르게 적용되는지 검증합니다.
func TestNewHTTPServer_Defaults(t *testing.T) {
	t.Run("Default RequestTimeout", func(t *testing.T) {
		// Timeouts: 0으로 설정 -> Default 값 적용 여부 확인
		cfg := HTTPServerConfig{RequestTimeout: 0}
		e := NewHTTPServer(cfg)

		// Echo Middleware Timeout 설정은 e.Router나 Middleware 체인을 직접 조회하기 어려우므로
		// 실제 동작(Timeout 적용 여부)을 통해 간접 검증하거나,
		// 여기서는 Unit Test 관점에서 "Server 객체의 타임아웃 속성"을 확인하는 형태로 접근
		// (단, RequestTimeout은 Echo 미들웨어 레벨이라 Server 객체 필드가 아님)

		// 대신 Server 필드의 기본 타임아웃은 항상 constant 값이어야 함
		assert.Equal(t, constants.DefaultReadTimeout, e.Server.ReadTimeout)
		assert.Equal(t, constants.DefaultWriteTimeout, e.Server.WriteTimeout)
	})
}

// TestNewHTTPServer_ServerTimeouts 는 http.Server의 중요 타임아웃 설정이
// 상수에 정의된 보안 권장 값과 일치하는지 검증합니다.
func TestNewHTTPServer_ServerTimeouts(t *testing.T) {
	e := NewHTTPServer(HTTPServerConfig{})

	require.NotNil(t, e.Server, "http.Server 객체가 초기화되어야 합니다")
	assert.Equal(t, constants.DefaultReadTimeout, e.Server.ReadTimeout, "ReadTimeout 불일치")
	assert.Equal(t, constants.DefaultReadHeaderTimeout, e.Server.ReadHeaderTimeout, "ReadHeaderTimeout 불일치")
	assert.Equal(t, constants.DefaultWriteTimeout, e.Server.WriteTimeout, "WriteTimeout 불일치")
	assert.Equal(t, constants.DefaultIdleTimeout, e.Server.IdleTimeout, "IdleTimeout 불일치")
}

// TestNewHTTPServer_ErrorHandler 는 커스텀 에러 핸들러(httputil.ErrorHandler)가
// 올바르게 등록되었는지 검증합니다.
func TestNewHTTPServer_ErrorHandler(t *testing.T) {
	e := NewHTTPServer(HTTPServerConfig{})

	// 함수 포인터 이름을 비교하여 검증
	handlerName := runtime.FuncForPC(reflect.ValueOf(e.HTTPErrorHandler).Pointer()).Name()
	expectedName := runtime.FuncForPC(reflect.ValueOf(httputil.ErrorHandler).Pointer()).Name()

	assert.Equal(t, expectedName, handlerName, "전역 에러 핸들러가 올바르게 설정되어야 합니다")
}

// =============================================================================
// Security Middleware Tests
// =============================================================================

// TestSecurityHeaders_HSTS는 HSTS 설정 활성화 여부에 따른 보안 헤더 적용을 검증합니다.
func TestSecurityHeaders_HSTS(t *testing.T) {
	tests := []struct {
		name       string
		enableHSTS bool
		wantHSTS   bool
	}{
		{
			name:       "HSTS Enabled",
			enableHSTS: true,
			wantHSTS:   true,
		},
		{
			name:       "HSTS Disabled",
			enableHSTS: false,
			wantHSTS:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewHTTPServer(HTTPServerConfig{EnableHSTS: tt.enableHSTS})
			e.GET("/secure", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

			req := httptest.NewRequest(http.MethodGet, "https://example.com/secure", nil)
			// HSTS 조건: HTTPS 스키마 + TLS State 존재
			if tt.enableHSTS {
				req.TLS = &tls.ConnectionState{}
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			headers := rec.Header()
			// 기본 보안 헤더 확인 (항상 존재해야 함)
			assert.Equal(t, "1; mode=block", headers.Get("X-XSS-Protection"), "XSS 보호 헤더 누락")
			assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"), "MIME 스니핑 방지 헤더 누락")

			// HSTS 헤더 확인
			hstsHeader := headers.Get("Strict-Transport-Security")
			if tt.wantHSTS {
				assert.NotEmpty(t, hstsHeader, "HSTS 헤더가 설정되어야 합니다")
				assert.Contains(t, hstsHeader, "max-age=31536000", "HSTS max-age 설정이 올바르지 않습니다")
			} else {
				assert.Empty(t, hstsHeader, "HSTS 비활성화 시 헤더가 없어야 합니다")
			}
		})
	}
}

// TestServerHeaderRemoval 은 보안을 위해 'Server' 헤더가 응답에서 제거되었는지 검증합니다.
func TestServerHeaderRemoval(t *testing.T) {
	e := NewHTTPServer(HTTPServerConfig{})
	e.GET("/ping", func(c echo.Context) error { return c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Server 헤더가 아예 없거나 비어 있어야 함
	assert.Empty(t, rec.Header().Get("Server"), "정보 노출 방지를 위해 Server 헤더는 제거되어야 합니다")
}

// =============================================================================
// Middleware Chain & Ordering Tests
// =============================================================================

// TestBodyLimit 은 설정된 제한(128KB)보다 큰 요청이 거부되는지 검증합니다.
func TestBodyLimit(t *testing.T) {
	e := NewHTTPServer(HTTPServerConfig{})
	e.POST("/upload", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	// 128KB + 1byte (경계값 테스트)
	limitBytes := 128 * 1024
	body := strings.Repeat("a", limitBytes+1)

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body))
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, "128KB를 초과하는 요청은 413 에러여야 합니다")

	// 정상 범위 테스트 (1KB)
	validBody := strings.Repeat("b", 1024)
	reqValid := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(validBody))
	recValid := httptest.NewRecorder()

	e.ServeHTTP(recValid, reqValid)
	assert.Equal(t, http.StatusOK, recValid.Code, "제한 범위 내의 요청은 성공해야 합니다")
}

// TestMiddlewareLoggingOrder_ChainVerification 은 미들웨어 체인의 실행 순서를 검증합니다.
// HTTPLogger가 에러 유발 미들웨어보다 상위(Outer)에 있어야 에러 상황도 로그에 남습니다.
func TestMiddlewareLoggingOrder_ChainVerification(t *testing.T) {
	// RateLimit 발생 시(429) 로그가 남는지 검증
	t.Run("Logs 429 Too Many Requests", func(t *testing.T) {
		buf := setupTestLogger(t)

		// 실제 NewHTTPServer 대신 수동 체인 구성 (제어 용이성)
		e := echo.New()
		e.Use(appmiddleware.HTTPLogger())       // 1. Logger (Outer)
		e.Use(appmiddleware.RateLimiting(1, 1)) // 2. RateLimit (Inner)

		e.GET("/test", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

		// 1회차 성공
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))

		// 2회차 실패 (429)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test", nil))

		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.Contains(t, buf.String(), `"status":429`, "Logger가 429 에러를 기록해야 합니다")
	})

	// 참고: Timeout(503) 로깅 테스트는 비동기 로깅 특성상 Flaky할 수 있어
	// TestMiddlewareOrdering_SecurityOnErrors 에서 상태 코드 및 헤더 검증으로 대체함.
}

// TestMiddlewareOrdering_SecurityOnErrors 는 보안 헤더가 에러 응답(429, 503)에도 적용되는지 검증합니다.
// 이는 Secure, CORS 미들웨어가 가장 상위에 위치해야 함을 의미합니다.
func TestMiddlewareOrdering_SecurityOnErrors(t *testing.T) {
	// HSTS 활성화 & 짧은 타임아웃
	cfg := HTTPServerConfig{
		EnableHSTS:     true,
		RequestTimeout: 20 * time.Millisecond,
	}

	t.Run("Security Headers on 429 RateLimit", func(t *testing.T) {
		// 독립된 서버 인스턴스 (RateLimiter 상태 격리)
		e := NewHTTPServer(cfg)

		burst := constants.DefaultRateLimitBurst
		// Burst + 여유분 만큼 요청
		for i := 0; i < burst+10; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.TLS = &tls.ConnectionState{} // HSTS 트리거용
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			if rec.Code == http.StatusTooManyRequests {
				// 429 응답에도 보안 헤더 존재해야 함
				assert.NotEmpty(t, rec.Header().Get("Strict-Transport-Security"), "429 응답 HSTS 누락")
				assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
				return
			}
		}
		t.Log("RateLimit을 유발하지 못했습니다 (테스트 환경 빠름 등)")
	})

	t.Run("Security Headers on 503 Timeout", func(t *testing.T) {
		// 독립된 서버 인스턴스
		e := NewHTTPServer(cfg)
		e.GET("/timeout", func(c echo.Context) error {
			time.Sleep(100 * time.Millisecond) // 타임아웃(20ms) 초과
			return c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/timeout", nil)
		req.TLS = &tls.ConnectionState{}
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		// 503 응답에도 보안 헤더 존재해야 함
		assert.NotEmpty(t, rec.Header().Get("Strict-Transport-Security"), "503 응답 HSTS 누락")
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	})
}

// =============================================================================
// CORS Tests
// =============================================================================

func TestCORSConfig(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		wantStatus     int
		wantHeader     string
	}{
		{
			name:           "Allowed Origin",
			allowedOrigins: []string{"https://trusted.com"},
			requestOrigin:  "https://trusted.com",
			wantStatus:     http.StatusOK,
			wantHeader:     "https://trusted.com",
		},
		{
			name:           "Disallowed Origin",
			allowedOrigins: []string{"https://trusted.com"},
			requestOrigin:  "https://evil.com",
			wantStatus:     http.StatusOK,
			wantHeader:     "", // Echo CORS는 불허 시 헤더 미포함
		},
		{
			name:           "Wildcard Origin",
			allowedOrigins: []string{"*"},
			requestOrigin:  "https://any.com",
			wantStatus:     http.StatusOK,
			wantHeader:     "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := HTTPServerConfig{AllowOrigins: tt.allowedOrigins}
			e := NewHTTPServer(cfg)
			e.GET("/cors", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

			req := httptest.NewRequest(http.MethodGet, "/cors", nil)
			req.Header.Set("Origin", tt.requestOrigin)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantHeader, rec.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

// TestCORSConfig_Methods 는 허용된 메서드가 올바르게 설정되는지 OPTIONS 요청으로 검증합니다.
func TestCORSConfig_Methods(t *testing.T) {
	cfg := HTTPServerConfig{AllowOrigins: []string{"*"}}
	e := NewHTTPServer(cfg)

	// Preflight 요청 (OPTIONS)
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	allowMethods := rec.Header().Get("Access-Control-Allow-Methods")
	assert.Contains(t, allowMethods, "POST")
	assert.Contains(t, allowMethods, "GET")
	assert.Contains(t, allowMethods, "PUT")
	assert.Contains(t, allowMethods, "DELETE")
}
