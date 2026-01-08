package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRequestLoggerHandler_Table(t *testing.T) {
	// Common Setup
	setupLogger := func() (*bytes.Buffer, func()) {
		var buf bytes.Buffer
		applog.SetOutput(&buf)
		applog.SetFormatter(&applog.JSONFormatter{})
		restore := func() {
			applog.SetOutput(applog.StandardLogger().Out)
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
		{
			name:          "Standard Request with Query Params",
			requestPath:   "/api/v1/notifications?app_key=secret-key&other=value",
			requestMethod: http.MethodGet,
			handler: func(c echo.Context) error {
				return c.String(http.StatusOK, "test")
			},
			expectedStatus: http.StatusOK,
			verifyLog: func(t *testing.T, entry map[string]interface{}) {
				uri, ok := entry["uri"].(string)
				assert.True(t, ok)
				assert.Contains(t, uri, "app_key=secr%2A%2A%2A") // secret-key (10) -> secr*** -> URL encoded
				assert.Contains(t, uri, "other=value")
				assert.NotContains(t, uri, "secret-key")
			},
		},
		{
			name:          "No Sensitive Data",
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
				assert.NotContains(t, uri, "*****")
			},
		},
		{
			name:          "Handler Error",
			requestPath:   "/error",
			requestMethod: http.MethodGet,
			handler: func(c echo.Context) error {
				return echo.NewHTTPError(http.StatusBadRequest, "bad request")
			},
			expectedStatus: http.StatusBadRequest,
			verifyLog: func(t *testing.T, entry map[string]interface{}) {
				status, ok := entry["status"].(float64)
				assert.True(t, ok)
				assert.Equal(t, float64(http.StatusBadRequest), status)
			},
		},
		{
			name:          "Content Length Logged",
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

			// Execute Middleware
			// httpLoggerHandler logic: calls next(c), if error, call c.Error(err).
			// We need to simulate the 'next' handler.
			// But httpLoggerHandler signature is (c echo.Context, next echo.HandlerFunc) -> echo.HandlerFunc? No.
			// It is (c echo.Context, next echo.HandlerFunc) error? Let's check signature.
			// Signature in file: func httpLoggerHandler(c echo.Context, next echo.HandlerFunc) error

			err := httpLoggerHandler(c, tt.handler)

			// If handler returns error (like in Handler Error case), httpLoggerHandler handles it via c.Error(err) and returns nil typically?
			// Let's assume standard behavior. If tt.handler returns error, httpLoggerHandler might return error depending on implementation.
			// In previous test it returned nil.

			if tt.expectedStatus >= 400 {
				// If we expect error status, assert.NoError(t, err) might fail if middleware propagates it.
				// But previously: assert.NoError(t, h) passed even for error case.
			}
			assert.NoError(t, err)

			// Log Verification
			// Since logger writes synchronously or buffered?
			// But we are unmarshaling JSON.
			// If panic recovery is involved, we might have multiple logs, but here only one request log expected.

			// We need to handle potential empty buffer if nothing logged (bug?)
			if buf.Len() == 0 {
				t.Fatal("No logs written")
			}

			var logEntry map[string]interface{}
			// Fix: sometimes there might be multiple lines if handler logs something too.
			// We assume the last line or the one containing request log fields.
			// Simple approach: unmarshal the whole buffer. if multiple JSONs, it fails.
			// Logger formatted output adds newline.

			// Split by newline and parse the line that looks like request log
			lines := bytes.Split(buf.Bytes(), []byte("\n"))
			found := false
			for _, line := range lines {
				if len(line) == 0 {
					continue
				}
				if json.Unmarshal(line, &logEntry) == nil {
					// Check if it's the request log (has "uri" or "status")
					if _, ok := logEntry["uri"]; ok {
						found = true
						break
					}
				}
			}

			if !found {
				// If not found, maybe retry parsing the first line as default
				err = json.Unmarshal(buf.Bytes(), &logEntry)
				assert.NoError(t, err, "Failed to parse log entry: %s", buf.String())
			}

			if tt.verifyLog != nil {
				tt.verifyLog(t, logEntry)
			}
		})
	}
}

func TestMaskSensitiveQueryParams_Table(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "app_key Masking",
			input:    "/api/v1/test?app_key=secret123", // 9 chars -> "secr***"
			expected: "/api/v1/test?app_key=secr%2A%2A%2A",
		},
		{
			name:     "password Masking",
			input:    "/api/v1/test?password=pass123", // 7 chars -> "pass***"
			expected: "/api/v1/test?password=pass%2A%2A%2A",
		},
		{
			name:     "Multiple Keys",
			input:    "/api/v1/test?app_key=secret&password=pass123&id=100",
			expected: "/api/v1/test?app_key=secr%2A%2A%2A&id=100&password=pass%2A%2A%2A",
		},
		{
			name:     "No Sensitive Data",
			input:    "/api/v1/test?id=123&name=test",
			expected: "/api/v1/test?id=123&name=test",
		},
		{
			name:     "Invalid URI",
			input:    "://invalid",
			expected: "://invalid",
		},
		{
			name:  "token Masking",
			input: "/api/v1/test?token=abc123", // 6 chars -> "abc1***" -> 12 chars or less logic?
			// Logic: if len <= 12 { prefix = val[:4] + "***" }
			// abc123 (6) -> abc1***
			expected: "/api/v1/test?token=abc1%2A%2A%2A",
		},
		{
			name:     "api_key Masking",
			input:    "/api/v1/test?api_key=xyz789",
			expected: "/api/v1/test?api_key=xyz7%2A%2A%2A",
		},
		{
			name:  "secret Masking (Long > 12)",
			input: "/api/v1/test?secret=1234567890123", // 13 chars -> first 4 *** last 4
			// 1234 + *** + 0123
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
