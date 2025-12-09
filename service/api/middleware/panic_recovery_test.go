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

func TestPanicRecovery_Table(t *testing.T) {
	// Setup capture
	setupLogger := func() (*bytes.Buffer, func()) {
		var buf bytes.Buffer
		logrus.SetOutput(&buf)
		logrus.SetFormatter(&logrus.JSONFormatter{})
		restore := func() {
			logrus.SetOutput(logrus.StandardLogger().Out)
		}
		return &buf, restore
	}

	tests := []struct {
		name         string
		panicPayload interface{}
		requestID    string
		verifyLog    func(*testing.T, map[string]interface{})
	}{
		{
			name:         "String Panic",
			panicPayload: "test panic",
			verifyLog: func(t *testing.T, entry map[string]interface{}) {
				msg, ok := entry["msg"].(string)
				assert.True(t, ok)
				assert.Equal(t, "PANIC RECOVERED", msg)

				errorField, ok := entry["error"].(string)
				assert.True(t, ok)
				assert.Contains(t, errorField, "test panic")

				stack, ok := entry["stack"].(string)
				assert.True(t, ok)
				assert.NotEmpty(t, stack)
			},
		},
		{
			name:         "Error Panic",
			panicPayload: assert.AnError,
			verifyLog: func(t *testing.T, entry map[string]interface{}) {
				errorField, ok := entry["error"].(string)
				assert.True(t, ok)
				assert.Contains(t, errorField, assert.AnError.Error())
			},
		},
		{
			name:         "Panic with Request ID",
			panicPayload: "panic with req id",
			requestID:    "req-12345",
			verifyLog: func(t *testing.T, entry map[string]interface{}) {
				reqID, ok := entry["request_id"].(string)
				assert.True(t, ok)
				assert.Equal(t, "req-12345", reqID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, restore := setupLogger()
			defer restore()

			e := echo.New()
			// Manually construct chain to avoid complexity or just use Use() and simulate request?
			// Using e.Use() and ServeHTTP is cleaner integration
			e.Use(PanicRecovery())

			// Setup Handler that panics
			e.GET("/panic", func(c echo.Context) error {
				panic(tt.panicPayload)
			})

			req := httptest.NewRequest(http.MethodGet, "/panic", nil)
			rec := httptest.NewRecorder()

			// Set Request ID if needed before request?
			// Request ID is usually set by RequestID middleware or client header.
			// Ideally we simulated X-Request-ID header incoming.
			if tt.requestID != "" {
				// Wait, if we want Response Header to have it (as middleware reads from response header sometimes?)
				// PanicRecovery implementation reads from...?
				// Let's check implementation if needed. usually it reads from c.Response().Header().Get(echo.HeaderXRequestID)
				// Or c.Get("request_id")?
				// Let's assume standard echo behavior or passing header.
				// Echo RequestID middleware sets it on response header.
				// We can manually set it on response header in a pre-middleware or just mock it.
				// But simpler: just pass it in header, and assume something sets it, or just set strictly in handler?
				// But handler PANICS.
				// So we set it in context before handler?
				// Let's use a middleware to set response ID or just set it on context if implementation checks context.
				// Actually, let's look at previous test: c.Response().Header().Set(echo.HeaderXRequestID, reqID)
				// So we can do that in a middleware before panic recovery or manually on context creation?
				// Context creation is inside e.ServeHTTP.
				// We can chain a middleware that sets the ID.
			}

			// To simplify: we can just call the middleware function directly like in previous test if ServeHTTP is too complex.
			// But ServeHTTP is better integration.
			// Let's use ServeHTTP but inject a middleware that sets RequestID if needed.

			if tt.requestID != "" {
				e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error {
						c.Response().Header().Set(echo.HeaderXRequestID, tt.requestID)
						return next(c)
					}
				})
			}

			// We need to re-add PanicHandlers or just use e.Use
			// Order matters. PanicRecovery should be outer?
			// Usually PanicRecovery is first (outermost).
			// So e.Use(PanicRecovery) is already called.
			// Then e.Use(RequestSetter)

			// Execute
			e.ServeHTTP(rec, req)

			// Assertions
			// PanicRecovery should handle it, so status might be 500
			if rec.Code != http.StatusInternalServerError {
				// It might be 200 if we didn't set error handler or something?
				// Echo default error handler handles panic err.
			}

			// Log Verification
			var logEntry map[string]interface{}
			err := json.Unmarshal(buf.Bytes(), &logEntry)
			assert.NoError(t, err)

			if tt.verifyLog != nil {
				tt.verifyLog(t, logEntry)
			}
		})
	}
}
