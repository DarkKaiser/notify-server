package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLogrusMiddlewareHandler(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notice/message?app_key=secret-key&other=value", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Capture logs
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	defer logrus.SetOutput(logrus.StandardLogger().Out) // Restore

	// Middleware execution
	h := logrusMiddlewareHandler(c, func(c echo.Context) error {
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
	assert.Contains(t, uri, "app_key=%2A%2A%2A%2A%2A")
	assert.Contains(t, uri, "other=value")
	assert.NotContains(t, uri, "secret-key")
}

func TestLogrusMiddlewareHandler_NoAppKey(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notice/message?other=value", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Capture logs
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	defer logrus.SetOutput(logrus.StandardLogger().Out)

	// Middleware execution
	h := logrusMiddlewareHandler(c, func(c echo.Context) error {
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

func TestLogger_Level(t *testing.T) {
	l := logrus.New()
	logger := Logger{l}

	l.SetLevel(logrus.DebugLevel)
	assert.Equal(t, log.DEBUG, logger.Level())

	l.SetLevel(logrus.InfoLevel)
	assert.Equal(t, log.INFO, logger.Level())

	l.SetLevel(logrus.WarnLevel)
	assert.Equal(t, log.WARN, logger.Level())

	l.SetLevel(logrus.ErrorLevel)
	assert.Equal(t, log.ERROR, logger.Level())
}

func TestLogger_SetLevel(t *testing.T) {
	l := logrus.New()
	logger := Logger{l}

	logger.SetLevel(log.DEBUG)
	assert.Equal(t, logrus.DebugLevel, l.Level)

	logger.SetLevel(log.INFO)
	assert.Equal(t, logrus.InfoLevel, l.Level)

	logger.SetLevel(log.WARN)
	assert.Equal(t, logrus.WarnLevel, l.Level)

	logger.SetLevel(log.ERROR)
	assert.Equal(t, logrus.ErrorLevel, l.Level)
}

func TestLogger_Output(t *testing.T) {
	l := logrus.New()
	logger := Logger{l}
	var buf bytes.Buffer
	logger.SetOutput(&buf)

	assert.Equal(t, &buf, logger.Output())
}

func TestLogger_Methods(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	logger := Logger{l}

	// Test Print
	logger.Print("test print")
	assert.Contains(t, buf.String(), "test print")
	buf.Reset()

	// Test Info
	logger.Info("test info")
	assert.Contains(t, buf.String(), "test info")
	assert.Contains(t, buf.String(), "info")
	buf.Reset()

	// Test Warn
	logger.Warn("test warn")
	assert.Contains(t, buf.String(), "test warn")
	assert.Contains(t, buf.String(), "warning")
	buf.Reset()

	// Test Error
	logger.Error("test error")
	assert.Contains(t, buf.String(), "test error")
	assert.Contains(t, buf.String(), "error")
	buf.Reset()

	// Test Debug (Level must be debug)
	logger.SetLevel(log.DEBUG)
	logger.Debug("test debug")
	assert.Contains(t, buf.String(), "test debug")
	assert.Contains(t, buf.String(), "debug")
	buf.Reset()
}

func TestLogger_JSONMethods(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	logger := Logger{l}

	j := log.JSON{"key": "value"}

	// Test Infoj
	logger.Infoj(j)
	assert.Contains(t, buf.String(), "value")
	buf.Reset()

	// Test Warnj
	logger.Warnj(j)
	assert.Contains(t, buf.String(), "value")
	buf.Reset()

	// Test Errorj
	logger.Errorj(j)
	assert.Contains(t, buf.String(), "value")
	buf.Reset()
}

func TestMaskSensitiveQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "app_key 마스킹",
			input:    "/api/v1/test?app_key=secret123",
			expected: "/api/v1/test?app_key=%2A%2A%2A%2A%2A", // URL 인코딩된 *****
		},
		{
			name:     "password 마스킹",
			input:    "/api/v1/test?password=pass123",
			expected: "/api/v1/test?password=%2A%2A%2A%2A%2A",
		},
		{
			name:     "여러 민감 정보 마스킹",
			input:    "/api/v1/test?app_key=secret&password=pass123&id=100",
			expected: "/api/v1/test?app_key=%2A%2A%2A%2A%2A&id=100&password=%2A%2A%2A%2A%2A",
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
			input:    "/api/v1/test?token=abc123",
			expected: "/api/v1/test?token=%2A%2A%2A%2A%2A",
		},
		{
			name:     "api_key 마스킹",
			input:    "/api/v1/test?api_key=xyz789",
			expected: "/api/v1/test?api_key=%2A%2A%2A%2A%2A",
		},
		{
			name:     "secret 마스킹",
			input:    "/api/v1/test?secret=confidential",
			expected: "/api/v1/test?secret=%2A%2A%2A%2A%2A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSensitiveQueryParams(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
