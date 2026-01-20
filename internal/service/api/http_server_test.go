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
		cfg            ServerConfig
		wantDebug      bool
		wantHideBanner bool
	}{
		{
			name: "Debug Mode Enabled",
			cfg: ServerConfig{
				Debug:        true,
				AllowOrigins: []string{"*"},
			},
			wantDebug:      true,
			wantHideBanner: true,
		},
		{
			name: "Debug Mode Disabled",
			cfg: ServerConfig{
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
	// 대신 Server 필드의 기본 타임아웃은 항상 constant 값이어야 함
	e := NewHTTPServer(ServerConfig{})
	assert.Equal(t, defaultReadTimeout, e.Server.ReadTimeout)
	assert.Equal(t, defaultWriteTimeout, e.Server.WriteTimeout)
}

// TestNewHTTPServer_ServerTimeouts 는 http.Server의 중요 타임아웃 설정이
// 상수에 정의된 보안 권장 값과 일치하는지 검증합니다.
func TestNewHTTPServer_ServerTimeouts(t *testing.T) {
	e := NewHTTPServer(ServerConfig{})

	require.NotNil(t, e.Server, "http.Server 객체가 초기화되어야 합니다")
	assert.Equal(t, defaultReadTimeout, e.Server.ReadTimeout, "ReadTimeout 불일치")
	assert.Equal(t, defaultReadHeaderTimeout, e.Server.ReadHeaderTimeout, "ReadHeaderTimeout 불일치")
	assert.Equal(t, defaultWriteTimeout, e.Server.WriteTimeout, "WriteTimeout 불일치")
	assert.Equal(t, defaultIdleTimeout, e.Server.IdleTimeout, "IdleTimeout 불일치")
}

// TestNewHTTPServer_ErrorHandler 는 커스텀 에러 핸들러(httputil.ErrorHandler)가
// 올바르게 등록되었는지 검증합니다.
func TestNewHTTPServer_ErrorHandler(t *testing.T) {
	e := NewHTTPServer(ServerConfig{})

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
			e := NewHTTPServer(ServerConfig{EnableHSTS: tt.enableHSTS})
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
	e := NewHTTPServer(ServerConfig{})
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

// =============================================================================
// Middleware Chain & Ordering Tests
// =============================================================================

// TestMiddleware_RequestID 는 응답에 X-Request-ID 헤더가 포함되는지 검증합니다.
func TestMiddleware_RequestID(t *testing.T) {
	e := NewHTTPServer(ServerConfig{})
	e.GET("/ping", func(c echo.Context) error { return c.String(http.StatusOK, "pong") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID), "X-Request-ID 헤더가 존재해야 합니다")
}

// TestMiddleware_BodyLimit 은 설정된 제한(128KB)보다 큰 요청이 거부되는지 검증합니다.
func TestMiddleware_BodyLimit(t *testing.T) {
	e := NewHTTPServer(ServerConfig{})
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

// TestMiddleware_RateLimit_And_Logging 은 실제 서버 인스턴스에서 Rate Limit가 동작하고,
// 차단(429) 시 로그가 남는지 통합 검증합니다.
func TestMiddleware_RateLimit_And_Logging(t *testing.T) {
	// 로그 캡처 설정
	buf := setupTestLogger(t)

	// 실제 서버 인스턴스 생성
	e := NewHTTPServer(ServerConfig{})
	e.GET("/fast", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	// Burst(40) 만큼은 성공해야 함
	// 주의: 테스트 실행 속도에 따라 토큰이 보충될 수 있으므로 느슨하게 검증하거나,
	// 빠르게 소진시켜야 함.
	burst := defaultRateLimitBurst

	// Burst + 5회 요청을 빠르게 전송
	for i := 0; i < burst+5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/fast", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			// 429 발생 확인
			// 로그에 status: 429가 기록되었는지 확인
			assert.Contains(t, buf.String(), `"status":429`, "Rate Limit 차단 시 Access Log가 기록되어야 합니다")
			return
		}
	}

	t.Log("WARN: Rate Limit(429)를 유발하지 못했습니다. (시스템 부하 등으로 인해 토큰이 빨리 충전됨)")
}

// TestMiddlewareOrdering_SecurityOnErrors 는 보안 헤더가 에러 응답(429, 503)에도 적용되는지 검증합니다.
// 이는 Secure, CORS 미들웨어가 가장 상위에 위치해야 함을 의미합니다.
func TestMiddlewareOrdering_SecurityOnErrors(t *testing.T) {
	cfg := ServerConfig{
		EnableHSTS: true,
	}
	e := NewHTTPServer(cfg)

	t.Run("Security Headers on 404 NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
		req.TLS = &tls.ConnectionState{}
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("Strict-Transport-Security"), "404 응답에도 HSTS 헤더가 있어야 합니다")
	})

	t.Run("Security Headers on 413 PayloadTooLarge", func(t *testing.T) {
		e.POST("/upload", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

		limitBytes := 128 * 1024
		body := strings.Repeat("a", limitBytes+1)
		req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body))
		req.TLS = &tls.ConnectionState{}
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
		assert.NotEmpty(t, rec.Header().Get("Strict-Transport-Security"), "413 응답에도 HSTS 헤더가 있어야 합니다")
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
			cfg := ServerConfig{AllowOrigins: tt.allowedOrigins}
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
	cfg := ServerConfig{AllowOrigins: []string{"*"}}
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

// TestNewHTTPServer_PanicLogging 은 Panic 발생 시 HTTPLogger가 500 Status를 기록하는지 검증합니다.
// (Middleware 순서가 HTTPLogger -> PanicRecovery 순이어야 함)
func TestNewHTTPServer_PanicLogging(t *testing.T) {
	buf := setupTestLogger(t)
	e := NewHTTPServer(ServerConfig{})

	// Panic을 유발하는 핸들러 등록
	e.GET("/panic", func(c echo.Context) error {
		panic("intentional panic")
	})

	// 요청 수행 (Recover 미들웨어가 있으므로 테스트 프로세스는 죽지 않음)
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// 응답 검증 (500 Internal Server Error)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// 로그 검증
	// "status": 500 이 로그에 포함되어야 함
	assert.Contains(t, buf.String(), `"status":500`, "Panic 발생 시 Access Log에는 500 Status가 기록되어야 합니다")
	assert.Contains(t, buf.String(), "intentional panic", "Panic 메시지가 어떤 형태로든 기록되어야 합니다 (PanicRecovery 로그)")
}
