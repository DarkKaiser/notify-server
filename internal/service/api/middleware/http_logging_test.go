package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// HTTPLogger 미들웨어 테스트
// =============================================================================

// TestHTTPLogger는 HTTP 로깅 미들웨어의 동작을 검증합니다.
//
// 검증 항목:
//   - 정상 요청/응답 로깅 (상태 코드, URI, 메서드 등)
//   - 민감 정보(쿼리 파라미터) 마스킹 동작 확인
//   - 에러 발생 시 상태 코드 로깅
//   - Content-Length 헤더 로깅 확인
func TestHTTPLogger(t *testing.T) {
	// Setup: 로거 출력을 캡처하기 위한 설정
	setupLogger := func() (*bytes.Buffer, func()) {
		var buf bytes.Buffer
		applog.SetOutput(&buf)
		applog.SetFormatter(&applog.JSONFormatter{}) // JSON 파싱을 위해 포맷터 설정
		originalOut := applog.StandardLogger().Out
		restore := func() {
			applog.SetOutput(originalOut)
		}
		return &buf, restore
	}

	tests := []struct {
		name           string
		requestPath    string
		requestMethod  string
		requestHeaders map[string]string
		handler        echo.HandlerFunc
		expectedStatus int
		verifyLog      func(*testing.T, map[string]interface{})
	}{
		// ---------------------------------------------------------------------
		// 민감 정보 마스킹 테스트
		// ---------------------------------------------------------------------
		{
			name:          "성공: 민감한 쿼리 파라미터 마스킹",
			requestPath:   "/api/v1/notifications?app_key=secret-key&other=value",
			requestMethod: http.MethodGet,
			handler: func(c echo.Context) error {
				return c.String(http.StatusOK, "test")
			},
			expectedStatus: http.StatusOK,
			verifyLog: func(t *testing.T, entry map[string]interface{}) {
				uri, ok := entry["uri"].(string)
				assert.True(t, ok)
				// app_key=secret-key -> secr*** -> URL encoded (secr%2A%2A%2A)
				assert.Contains(t, uri, "app_key=secr%2A%2A%2A")
				assert.Contains(t, uri, "other=value")
				// 원본 키가 노출되지 않아야 함
				assert.NotContains(t, uri, "secret-key")
			},
		},
		{
			name:          "성공: 민감 정보가 없는 경우 원본 유지",
			requestPath:   "/api/v1/notifications?other=value",
			requestMethod: http.MethodGet,
			handler: func(c echo.Context) error {
				return c.String(http.StatusOK, "test")
			},
			expectedStatus: http.StatusOK,
			verifyLog: func(t *testing.T, entry map[string]interface{}) {
				uri, ok := entry["uri"].(string)
				assert.True(t, ok)
				assert.Contains(t, uri, "other=value")
				assert.NotContains(t, uri, "%2A%2A%2A") // Masking char ***
			},
		},

		// ---------------------------------------------------------------------
		// 에러 처리 테스트
		// ---------------------------------------------------------------------
		{
			name:          "성공: 핸들러 에러 발생 시 상태 코드 로깅",
			requestPath:   "/error",
			requestMethod: http.MethodGet,
			handler: func(c echo.Context) error {
				return echo.NewHTTPError(http.StatusBadRequest, "bad request")
			},
			expectedStatus: http.StatusBadRequest,
			verifyLog: func(t *testing.T, entry map[string]interface{}) {
				status, ok := entry["status"].(float64) // JSON 숫자 -> float64
				assert.True(t, ok)
				assert.Equal(t, float64(http.StatusBadRequest), status)
			},
		},

		// ---------------------------------------------------------------------
		// 헤더 처리 테스트
		// ---------------------------------------------------------------------
		{
			name:          "성공: Content-Length 로깅 확인",
			requestPath:   "/upload",
			requestMethod: http.MethodPost,
			requestHeaders: map[string]string{
				echo.HeaderContentLength: "12345",
			},
			handler: func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			},
			expectedStatus: http.StatusOK,
			verifyLog: func(t *testing.T, entry map[string]interface{}) {
				bytesIn, ok := entry["bytes_in"].(string)
				assert.True(t, ok)
				assert.Equal(t, "12345", bytesIn)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, restore := setupLogger()
			defer restore()

			e := echo.New()
			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			if tt.requestHeaders != nil {
				for k, v := range tt.requestHeaders {
					req.Header.Set(k, v)
				}
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// 미들웨어 실행
			// httpLogger는 내부적으로 httpLoggerHandler를 호출합니다.
			// 여기서는 httpLoggerHandler를 직접 호출하여 테스트합니다.
			err := httpLoggerHandler(c, tt.handler)

			// 에러 검증
			// Echo 미들웨어에서 핸들러가 에러를 반환하면 일반적으로 c.Error()를 호출하고 nil을 반환하거나,
			// 에러를 그대로 반환하여 상위 미들웨어가 처리하게 합니다.
			// httpLoggerHandler 구현상 에러를 c.Error(err)로 처리하고 로깅 후 nil을 반환하므로,
			// 외부적으로는 nil err를 받습니다 (단, c.Response().Status는 업데이트됨).
			assert.NoError(t, err)

			// 로그 검증
			require.Greater(t, buf.Len(), 0, "로그가 기록되어야 합니다")

			// 로그 파싱 (여러 줄 로그가 있을 수 있으므로 Loop로 처리)
			var logEntry map[string]interface{}
			lines := strings.Split(buf.String(), "\n")
			found := false

			for _, line := range lines {
				if strings.TrimSpace(line) == "" {
					continue
				}
				if err := json.Unmarshal([]byte(line), &logEntry); err == nil {
					// HTTP request 로그인지 확인 (uri 필드 존재 여부)
					if _, ok := logEntry["uri"]; ok {
						found = true
						break
					}
				}
			}

			require.True(t, found, "HTTP 요청 로그를 찾을 수 없습니다")

			// 공통 필드 검증
			assert.Equal(t, "HTTP 요청", logEntry["msg"])
			assert.NotEmpty(t, logEntry["time_rfc3339"])
			assert.NotEmpty(t, logEntry["latency"])

			// 케이스별 상세 검증
			if tt.verifyLog != nil {
				tt.verifyLog(t, logEntry)
			}
		})
	}
}

// TestMaskSensitiveQueryParams는 쿼리 파라미터 마스킹 로직을 검증합니다.
func TestMaskSensitiveQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "app_key 마스킹 (중간 문자)",
			input:    "/api/v1/test?app_key=secret123", // secret123 (9자) -> secr***
			expected: "/api/v1/test?app_key=secr%2A%2A%2A",
		},
		{
			name:     "password 마스킹",
			input:    "/api/v1/test?password=pass123", // pass123 (7자) -> pass***
			expected: "/api/v1/test?password=pass%2A%2A%2A",
		},
		{
			name:     "다중 쿼리 파라미터 마스킹",
			input:    "/api/v1/test?app_key=secret&password=pass123&id=100",
			expected: "/api/v1/test?app_key=secr%2A%2A%2A&id=100&password=pass%2A%2A%2A",
		},
		{
			name:     "민감 정보 없음",
			input:    "/api/v1/test?id=123&name=test",
			expected: "/api/v1/test?id=123&name=test",
		},
		{
			name:     "잘못된 URI 형식",
			input:    "://invalid",
			expected: "://invalid",
		},
		{
			name:     "api_key 마스킹",
			input:    "/api/v1/test?api_key=xyz789",
			expected: "/api/v1/test?api_key=xyz7%2A%2A%2A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSensitiveQueryParams(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHTTPLogger_MiddlewareReturn은 미들웨어 생성 함수를 검증합니다.
func TestHTTPLogger_MiddlewareReturn(t *testing.T) {
	middleware := HTTPLogger()
	assert.NotNil(t, middleware, "미들웨어 함수는 nil이 아니어야 합니다")
}
