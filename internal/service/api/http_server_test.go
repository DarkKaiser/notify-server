package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Utils
// =============================================================================

// LogEntry 로그 검증을 위한 구조체
type LogEntry struct {
	Level   string `json:"level"`
	Message string `json:"msg"`
	Status  int    `json:"status"`
	Method  string `json:"method"`
	Error   string `json:"error,omitempty"`
}

// setupTestLogger는 테스트를 위해 로거 출력을 버퍼로 변경합니다.
func setupTestLogger(buf *bytes.Buffer) {
	applog.SetOutput(buf)
	applog.SetFormatter(&applog.JSONFormatter{})
	applog.SetLevel(applog.DebugLevel)
}

// restoreLogger는 로거 출력을 표준 출력으로 복구합니다.
func restoreLogger() {
	applog.SetOutput(applog.StandardLogger().Out)
}

// =============================================================================
// Configuration Tests
// =============================================================================

// TestNewHTTPServer_Configuration_Table 은 Echo 서버의 기본 설정을 검증합니다.
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
			expectBanner: true, // NewHTTPServer 메서드 내에서 항상 true로 설정됨
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

// TestNewHTTPServer_CORSMiddleware_Table 은 CORS 미들웨어 동작을 검증합니다.
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
			expectStatus:       http.StatusOK, // 핸들러가 200 OK 반환
			expectAllowOrigin:  "http://example.com",
			expectAllowMethods: false,
		},
		{
			name:               "Disallowed Origin - GET 요청",
			allowOrigins:       []string{"http://trusted.com"},
			requestOrigin:      "http://evil.com",
			requestMethod:      http.MethodGet,
			expectStatus:       http.StatusOK, // Echo CORS는 허용되지 않은 Origin에 대해 200을 반환하지만, Access-Control-Allow-Origin 헤더를 생략함
			expectAllowOrigin:  "",            // 헤더가 없어야 함
			expectAllowMethods: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := HTTPServerConfig{AllowOrigins: tt.allowOrigins}
			e := NewHTTPServer(cfg)

			// 테스트 핸들러 등록
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

// TestNewHTTPServer_PanicRecoveryMiddleware 는 패닉 발생 시 서버가 복구되고 500 에러를 반환하는지 검증합니다.
func TestNewHTTPServer_PanicRecoveryMiddleware(t *testing.T) {
	// 로거 캡처 설정 (패닉 로그 확인용)
	buf := new(bytes.Buffer)
	setupTestLogger(buf)
	defer restoreLogger()

	cfg := HTTPServerConfig{AllowOrigins: []string{"*"}, Debug: false}
	e := NewHTTPServer(cfg)

	// 의도적으로 패닉을 발생시키는 핸들러
	e.GET("/panic", func(c echo.Context) error {
		panic("intentional panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	// 패닉이 복구되어야 하므로 함수가 정상 종료되어야 함
	assert.NotPanics(t, func() {
		e.ServeHTTP(rec, req)
	})

	// 1. 상태 코드 검증 (500 Internal Server Error)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// 2. 로그 검증 (패닉 정보가 기록되었는지 확인)
	assert.Contains(t, buf.String(), "intentional panic", "로그에 패닉 메시지가 포함되어야 합니다")
	assert.Contains(t, buf.String(), "\"level\":\"error\"", "Error 레벨로 로깅되어야 합니다")
}

// TestNewHTTPServer_HTTPLoggerMiddleware 는 요청/응답 로그가 제대로 기록되는지 검증합니다.
func TestNewHTTPServer_HTTPLoggerMiddleware(t *testing.T) {
	buf := new(bytes.Buffer)
	setupTestLogger(buf)
	defer restoreLogger()

	cfg := HTTPServerConfig{AllowOrigins: []string{"*"}}
	e := NewHTTPServer(cfg)

	e.GET("/log-test", func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest(http.MethodGet, "/log-test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// 로그 검증
	// HTTP 로거는 JSON 형태로 로그를 남기므로 파싱 가능해야 함 (단, 여러 줄일 수 있으므로 마지막 줄 확인 등 필요할 수 있음)
	// 여기서는 단순히 문자열 포함 여부로 검증 (복잡성을 줄이기 위해)
	logContent := buf.String()
	assert.Contains(t, logContent, "\"method\":\"GET\"", "HTTP 메서드가 로그에 포함되어야 합니다")
	assert.Contains(t, logContent, "\"status\":200", "상태 코드가 로그에 포함되어야 합니다")
	assert.Contains(t, logContent, "\"uri\":\"/log-test\"", "URI가 로그에 포함되어야 합니다")
}

// TestNewHTTPServer_StandardMiddleware 는 보안 헤더, RequestID 등 기본 미들웨어를 검증합니다.
func TestNewHTTPServer_StandardMiddleware_Table(t *testing.T) {
	tests := []struct {
		name          string
		checkHeader   string
		expectPattern string // 정규식 또는 값
	}{
		{
			name:          "Request ID 헤더 존재",
			checkHeader:   echo.HeaderXRequestID,
			expectPattern: ".+", // 비어있지 않음
		},
		{
			name:          "X-XSS-Protection 헤더 존재 (Secure 미들웨어)",
			checkHeader:   "X-XSS-Protection",
			expectPattern: "1; mode=block",
		},
		{
			name:          "X-Content-Type-Options 헤더 존재 (Secure 미들웨어)",
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
