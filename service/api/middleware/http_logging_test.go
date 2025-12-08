package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestRequestLoggerHandler(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?app_key=secret-key&other=value", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Capture logs
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	defer logrus.SetOutput(logrus.StandardLogger().Out) // Restore

	// Middleware execution
	h := httpLoggerHandler(c, func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	// Assertions
	assert.NoError(t, h)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Log verification
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err)

	uri, ok := logEntry["uri"].(string)
	assert.True(t, ok)
	// URL 인코딩된 별표(*) 확인 (%2A)
	// secret-key -> secr***-key -> secr%2A%2A%2A-key (길이 10자)
	// 하지만 MaskSensitiveData 로직에 따르면:
	// len("secret-key") = 10 -> "secr" + "***" + "-key" (앞4 + 마스킹 + 뒤4 X -> 12자 이하는 앞4 + ***)
	// "secret-key" (10자) -> "secr***"
	// URL Encoding: "secr%2A%2A%2A"
	assert.Contains(t, uri, "app_key=secr%2A%2A%2A")
	assert.Contains(t, uri, "other=value")
	assert.NotContains(t, uri, "secret-key")
}

func TestRequestLoggerHandler_NoAppKey(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?other=value", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Capture logs
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	defer logrus.SetOutput(logrus.StandardLogger().Out)

	// Middleware execution
	h := httpLoggerHandler(c, func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	// Assertions
	assert.NoError(t, h)

	// Log verification
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err)

	uri, ok := logEntry["uri"].(string)
	assert.True(t, ok)
	assert.Contains(t, uri, "other=value")
	assert.NotContains(t, uri, "*****")
}

func TestMaskSensitiveQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "app_key 마스킹",
			input:    "/api/v1/test?app_key=secret123", // 9자 -> "secr***"
			expected: "/api/v1/test?app_key=secr%2A%2A%2A",
		},
		{
			name:     "password 마스킹",
			input:    "/api/v1/test?password=pass123", // 7자 -> "pass***"
			expected: "/api/v1/test?password=pass%2A%2A%2A",
		},
		{
			name:     "여러 민감 정보 마스킹",
			input:    "/api/v1/test?app_key=secret&password=pass123&id=100", // "secret" (6자) -> "secr***", "pass123" (7자) -> "pass***"
			expected: "/api/v1/test?app_key=secr%2A%2A%2A&id=100&password=pass%2A%2A%2A",
		},
		{
			name:     "민감 정보 없음",
			input:    "/api/v1/test?id=123&name=test",
			expected: "/api/v1/test?id=123&name=test",
		},
		{
			name:     "잘못된 URI",
			input:    "://invalid",
			expected: "://invalid",
		},
		{
			name:     "token 마스킹",
			input:    "/api/v1/test?token=abc123", // 6자 -> "abc1***" (X) -> "abc1" (4자) + "***"
			expected: "/api/v1/test?token=abc1%2A%2A%2A",
		},
		{
			name:     "api_key 마스킹", // xyz789 (6자) -> "xyz7***"
			input:    "/api/v1/test?api_key=xyz789",
			expected: "/api/v1/test?api_key=xyz7%2A%2A%2A",
		},
		{
			name:     "secret 마스킹 (긴 토큰 13자)",
			input:    "/api/v1/test?secret=1234567890123", // 13자 -> "1234***0123"
			expected: "/api/v1/test?secret=1234%2A%2A%2A0123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSensitiveQueryParams(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
