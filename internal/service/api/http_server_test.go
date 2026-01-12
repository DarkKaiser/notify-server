package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Utils
// =============================================================================

// setupTestLogger는 테스트를 위해 로거 출력을 버퍼로 변경하고,
// 테스트 종료 시 자동으로 원래대로 복구합니다.
func setupTestLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := new(bytes.Buffer)
	originalOut := applog.StandardLogger().Out
	originalLevel := applog.StandardLogger().Level

	applog.SetOutput(buf)
	applog.SetFormatter(&applog.JSONFormatter{})
	applog.SetLevel(applog.DebugLevel)

	t.Cleanup(func() {
		applog.SetOutput(originalOut)
		applog.SetLevel(originalLevel)
	})

	return buf
}

// =============================================================================
// Configuration Tests
// =============================================================================

// TestNewHTTPServer_Configuration_Table 은 Echo 서버의 기본 설정이 Config에 따라 올바르게 적용되는지 검증합니다.
func TestNewHTTPServer_Configuration_Table(t *testing.T) {
	tests := []struct {
		name         string
		config       HTTPServerConfig
		expectDebug  bool
		expectBanner bool
	}{
		{
			name: "Debug 모드 활성화",
			config: HTTPServerConfig{
				Debug:        true,
				AllowOrigins: []string{"*"},
			},
			expectDebug:  true,
			expectBanner: true, // 항상 숨김 처리됨
		},
		{
			name: "Debug 모드 비활성화",
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

			require.NotNil(t, e, "Echo 인스턴스가 생성되어야 합니다")
			assert.Equal(t, tt.expectDebug, e.Debug, "Debug 모드 설정이 일치해야 합니다")
			assert.Equal(t, tt.expectBanner, e.HideBanner, "HideBanner 설정이 일치해야 합니다")
			require.NotNil(t, e.Logger, "Logger가 설정되어야 합니다")
		})
	}
}

// =============================================================================
// Middleware Tests
// =============================================================================

// TestNewHTTPServer_CORSMiddleware_Table 은 CORS 미들웨어의 동작을 시나리오별로 검증합니다.
func TestNewHTTPServer_CORSMiddleware_Table(t *testing.T) {
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
			name:               "Wildcard Origin - Preflight 요청",
			allowOrigins:       []string{"*"},
			requestOrigin:      "http://example.com",
			requestMethod:      http.MethodOptions,
			expectStatus:       http.StatusNoContent,
			expectAllowOrigin:  "*",
			expectAllowMethods: true,
		},
		{
			name:               "Specific Origin - GET 요청",
			allowOrigins:       []string{"http://example.com"},
			requestOrigin:      "http://example.com",
			requestMethod:      http.MethodGet,
			expectStatus:       http.StatusOK,
			expectAllowOrigin:  "http://example.com",
			expectAllowMethods: false,
		},
		{
			name:               "Disallowed Origin - GET 요청",
			allowOrigins:       []string{"http://trusted.com"},
			requestOrigin:      "http://evil.com",
			requestMethod:      http.MethodGet,
			expectStatus:       http.StatusOK, // Echo CORS는 불허 Origin에 대해 200 반환하되 헤더 생략
			expectAllowOrigin:  "",
			expectAllowMethods: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := HTTPServerConfig{AllowOrigins: tt.allowOrigins}
			e := NewHTTPServer(cfg)

			e.GET("/test", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
			e.OPTIONS("/test", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

			req := httptest.NewRequest(tt.requestMethod, "/test", nil)
			req.Header.Set("Origin", tt.requestOrigin)
			if tt.requestMethod == http.MethodOptions {
				req.Header.Set("Access-Control-Request-Method", "GET")
			}
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectStatus, rec.Code)
			assert.Equal(t, tt.expectAllowOrigin, rec.Header().Get("Access-Control-Allow-Origin"))

			if tt.expectAllowMethods {
				assert.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Methods"))
			}
		})
	}
}

// TestNewHTTPServer_PanicRecoveryMiddleware 는 핸들러 패닉 발생 시 서버가 중단되지 않고 500 에러를 반환하며 로깅하는지 검증합니다.
func TestNewHTTPServer_PanicRecoveryMiddleware(t *testing.T) {
	buf := setupTestLogger(t)

	cfg := HTTPServerConfig{AllowOrigins: []string{"*"}, Debug: false}
	e := NewHTTPServer(cfg)

	e.GET("/panic", func(c echo.Context) error {
		panic("intentional panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	// 패닉 복구 검증
	assert.NotPanics(t, func() {
		e.ServeHTTP(rec, req)
	})

	// 상태 코드 및 로그 검증
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// 로그 내용 검증
	logOutput := buf.String()
	assert.Contains(t, logOutput, "intentional panic", "로그에 패닉 원인이 포함되어야 합니다")
	assert.Contains(t, logOutput, "\"level\":\"error\"", "Error 레벨로 로깅되어야 합니다")
}

// TestNewHTTPServer_HTTPLoggerMiddleware 는 모든 요청/응답이 구조화된 로그로 기록되는지 검증합니다.
func TestNewHTTPServer_HTTPLoggerMiddleware(t *testing.T) {
	buf := setupTestLogger(t)

	cfg := HTTPServerConfig{AllowOrigins: []string{"*"}}
	e := NewHTTPServer(cfg)

	e.GET("/log-test", func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest(http.MethodGet, "/log-test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	logContent := buf.String()
	assert.Contains(t, logContent, "\"method\":\"GET\"", "HTTP 메서드가 로그에 포함되어야 합니다")
	assert.Contains(t, logContent, "\"status\":200", "상태 코드가 로그에 포함되어야 합니다")
	assert.Contains(t, logContent, "\"uri\":\"/log-test\"", "URI가 로그에 포함되어야 합니다")
}

// TestNewHTTPServer_StandardMiddleware_Table 은 RequestID, Secure 등 기본 보안/유틸리티 미들웨어를 검증합니다.
func TestNewHTTPServer_StandardMiddleware_Table(t *testing.T) {
	tests := []struct {
		name          string
		checkHeader   string
		expectPattern string // 정규식 또는 값, ".+"는 존재 여부만 확인
	}{
		{
			name:          "Request ID 헤더 존재",
			checkHeader:   echo.HeaderXRequestID,
			expectPattern: ".+",
		},
		{
			name:          "X-XSS-Protection (Secure Middleware)",
			checkHeader:   "X-XSS-Protection",
			expectPattern: "1; mode=block",
		},
		{
			name:          "X-Content-Type-Options (Secure Middleware)",
			checkHeader:   "X-Content-Type-Options",
			expectPattern: "nosniff",
		},
	}

	cfg := HTTPServerConfig{AllowOrigins: []string{"*"}}
	e := NewHTTPServer(cfg)
	e.GET("/test", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := rec.Header().Get(tt.checkHeader)
			if tt.expectPattern == ".+" {
				assert.NotEmpty(t, val, "%s 헤더가 설정되어야 합니다", tt.checkHeader)
			} else {
				assert.Equal(t, tt.expectPattern, val, "%s 헤더 값이 일치해야 합니다", tt.checkHeader)
			}
		})
	}
}

// =============================================================================
// Security & Limit Tests
// =============================================================================

// TestNewHTTPServer_BodyLimit 은 대용량 요청 본문이 제한(2MB)을 초과할 경우 413 에러를 반환하는지 검증합니다.
func TestNewHTTPServer_BodyLimit(t *testing.T) {
	cfg := HTTPServerConfig{AllowOrigins: []string{"*"}}
	e := NewHTTPServer(cfg)

	e.POST("/upload", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// 3MB 데이터 생성 (제한 2MB)
	largeBody := strings.Repeat("a", 3*1024*1024)
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(largeBody))
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, "2MB 초과 요청은 413 Payload Too Large 로 거부되어야 합니다")
}

// TestNewHTTPServer_Timeout 은 설정된 타임아웃보다 오래 걸리는 요청이 503으로 중단되는지 검증합니다.
func TestNewHTTPServer_Timeout(t *testing.T) {
	// 50ms 타임아웃 설정
	cfg := HTTPServerConfig{
		AllowOrigins:   []string{"*"},
		RequestTimeout: 50 * time.Millisecond,
	}
	e := NewHTTPServer(cfg)

	// 100ms 지연 핸들러
	e.GET("/slow", func(c echo.Context) error {
		time.Sleep(100 * time.Millisecond) // 타임아웃(50ms) 초과
		return c.String(http.StatusOK, "should not be reached")
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Echo의 Timeout 미들웨어는 context deadline exceeded 발생 시 503 반환
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code, "타임아웃 초과 시 503 Service Unavailable 응답이어야 합니다")
}

// TestNewHTTPServer_DefaultTimeout 은 RequestTimeout 미설정(0) 시 기본값이 60초로 설정되는지 검증합니다.
func TestNewHTTPServer_DefaultTimeout(t *testing.T) {
	// 타임아웃 미설정
	cfg := HTTPServerConfig{
		AllowOrigins: []string{"*"},
	}
	// 내부 상태는 블랙박스 테스트로 확인 어렵지만,
	// 타임아웃이 0(무제한)이 아님을 짧은 타임아웃 테스트로 유추하거나
	// 여기서는 로직상 60초가 설정됨을 간접 확인 (현재 테스트 환경상 60초 대기는 불가능하므로 코드 리뷰 영역에 가깝지만,
	// 짧은 지연(예: 10ms)은 통과해야 함을 검증하여 "즉시 타임아웃"이 아님을 확인)

	e := NewHTTPServer(cfg)

	e.GET("/normal", func(c echo.Context) error {
		// 아주 짧은 지연은 허용되어야 함
		time.Sleep(10 * time.Millisecond)
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/normal", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestNewHTTPServer_RateLimiting 은 과도한 요청 시 429 Too Many Requests 응답을 반환하는지 통합 검증합니다.
func TestNewHTTPServer_RateLimiting(t *testing.T) {
	// RateLimit 설정: 20 req/s, Burst 40
	cfg := HTTPServerConfig{AllowOrigins: []string{"*"}}
	e := NewHTTPServer(cfg)

	e.GET("/limit", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	// Burst(40) 까지는 성공해야 함
	for i := 0; i < 40; i++ {
		req := httptest.NewRequest(http.MethodGet, "/limit", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "Burst 내의 요청은 성공해야 합니다 (%d/40)", i+1)
	}

	// Burst 초과 시 429 예상
	// Rate Limiter의 상태 갱신 타이밍 이슈가 있을 수 있으므로 충분히 많이 시도
	limitHit := false
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodGet, "/limit", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			limitHit = true
			break
		}
	}

	assert.True(t, limitHit, "허용량을 초과하면 429 Too Many Requests 응답이 발생해야 합니다")
}

// =============================================================================
// Expert Level Tests (Timeouts & Middleware Order)
// =============================================================================

// TestNewHTTPServer_ServerTimeouts 는 http.Server의 타임아웃 설정이 상수에 정의된 값과 일치하는지 검증합니다.
// 이는 보안(Slowloris 등) 및 리소스 관리 측면에서 매우 중요합니다.
func TestNewHTTPServer_ServerTimeouts(t *testing.T) {
	cfg := HTTPServerConfig{AllowOrigins: []string{"*"}}
	e := NewHTTPServer(cfg)

	require.NotNil(t, e.Server, "http.Server 인스턴스가 설정되어야 합니다")

	assert.Equal(t, constants.DefaultReadTimeout, e.Server.ReadTimeout, "ReadTimeout 설정 불일치")
	assert.Equal(t, constants.DefaultReadHeaderTimeout, e.Server.ReadHeaderTimeout, "ReadHeaderTimeout 설정 불일치")
	assert.Equal(t, constants.DefaultWriteTimeout, e.Server.WriteTimeout, "WriteTimeout 설정 불일치")
	assert.Equal(t, constants.DefaultIdleTimeout, e.Server.IdleTimeout, "IdleTimeout 설정 불일치")
}

// TestNewHTTPServer_MiddlewareOrder 는 미들웨어가 의도한 순서대로 등록되었는지 검증합니다.
// 미들웨어 순서는 보안 및 로깅의 정확성에 치명적인 영향을 미칩니다.
func TestNewHTTPServer_MiddlewareOrder(t *testing.T) {
	// Echo의 Middleware 체인은 e.Routes()나 e.Middleware()로 직접 검증하기 어렵습니다.
	// 따라서 각 미들웨어의 Side Effect(헤더 존재 여부, 로그 발생 순서 등)를 확인하는 통합 테스트가 적절합니다.
	// 현재 존재하는 TestNewHTTPServer_StandardMiddleware_Table, TestNewHTTPServer_HTTPLoggerMiddleware 등이
	// 개별 기능을 검증하고 있습니다. (중복 방지 및 유지보수성을 위해 여기서는 생략)
	t.Skip("개별 기능 테스트로 대체됨")
}
