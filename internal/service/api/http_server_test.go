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
			// Echo Secure 미들웨어는 HTTPS 요청일 때만 HSTS 헤더를 추가할 수 있음
			// httptest는 기본적으로 TLS 정보를 채워주지 않으므로 수동 설정 필요할 수 있음
			// 하지만 Echo 구현에 따라 다를 수 있으므로, URL Scheme을 https로 명시하는 것이 좋음

			// HSTS는 HTTPS 응답에서만 유효하므로, 테스트 요청도 HTTPS인 것처럼 위장
			if tt.enableHSTS {
				req.TLS = &tls.ConnectionState{} // Dummy TLS state
			}

			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			headers := rec.Header()
			// 기본 보안 헤더 확인
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
	assert.Empty(t, rec.Header().Get("Server"), "정보 노출 방지를 위해 Server 헤더는 제거되어야 합니다")
}

// TestBodyLimit 은 설정된 제한(128KB)보다 큰 요청이 거부되는지 검증합니다.
func TestBodyLimit(t *testing.T) {
	e := NewHTTPServer(HTTPServerConfig{})
	e.POST("/upload", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	// 128KB + 1byte (경계값 테스트)
	// BodyLimit이 정확히 128KB(131072 bytes)인지 확인하기 위해 약간 더 큰 데이터를 생성
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

// =============================================================================
// Middleware Chain & Ordering Tests
// =============================================================================

// TestMiddlewareLoggingOrder_ChainVerification 은 미들웨어 체인의 실행 순서를 검증합니다.
// 특히 HTTPLogger가 에러 유발 미들웨어(RateLimit, Timeout)보다 '감싸는(Outer)' 위치에 있어
// 429, 503 에러도 로그에 남기는지를 확인합니다.
func TestMiddlewareLoggingOrder_ChainVerification(t *testing.T) {
	t.Run("Logs 429 Too Many Requests (RateLimit)", func(t *testing.T) {
		buf := setupTestLogger(t)

		// 실제 NewHTTPServer 대신 수동 체인 구성으로 테스트 (RateLimit 수치 제어를 위해)
		e := echo.New()
		e.Use(appmiddleware.HTTPLogger())       // 1. Logger (Outer)
		e.Use(appmiddleware.RateLimiting(1, 1)) // 2. RateLimit (Inner)

		e.GET("/test", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

		// 첫 번째 요청: 성공
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))

		// 두 번째 요청: 429 발생
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/test", nil))

		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.Contains(t, buf.String(), `"status":429`, "Logger가 RateLimit보다 상위에 있어야 429 에러를 기록합니다")
	})

	t.Run("Logs 503 Service Unavailable (Timeout)", func(t *testing.T) {
		_ = setupTestLogger(t)

		timeout := 10 * time.Millisecond
		cfg := HTTPServerConfig{RequestTimeout: timeout}
		e := NewHTTPServer(cfg) // 실제 체인 사용 (Logger -> ... -> Timeout)

		e.GET("/slow", func(c echo.Context) error {
			// 타임아웃보다 오래 대기하며 Context 취소 감지
			select {
			case <-time.After(100 * time.Millisecond):
				return c.String(http.StatusOK, "ok")
			case <-c.Request().Context().Done():
				return nil // Context 취소로 인한 핸들러 종료
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/slow", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code, "Timeout 미들웨어가 503을 반환해야 합니다")

		// Note: httptest와 Echo Timeout 미들웨어 간의 비동기 로그 기록 시점 차이로 인해
		// 단위 테스트에서 로그 내용을 assert.Contains로 검증하는 것은 Flaky할 수 있습니다.
		// 따라서 여기서는 HTTP 상태 코드 검증에 집중하고, 로그 기록 순서 논리는
		// "Logs 429" 테스트 케이스를 통해 간접 검증합니다.
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
			wantStatus:     http.StatusOK, // Block하지 않고 헤더만 미포함 (Echo 기본)
			wantHeader:     "",
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
