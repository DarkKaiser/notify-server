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

func TestPanicRecovery(t *testing.T) {
	// Setup
	e := echo.New()
	e.Use(PanicRecovery())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Capture logs
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	defer logrus.SetOutput(logrus.StandardLogger().Out)

	// Middleware execution with panic
	h := func(c echo.Context) error {
		panic("test panic")
	}

	// Execute handler through middleware chain
	// Echo middleware chain execution is a bit complex to simulate directly with just function calls
	// So we use the middleware returned function
	err := PanicRecovery()(h)(c)

	// Assertions
	assert.NoError(t, err) // Recover middleware should swallow the panic and return nil (or handle error internally)
	// Note: Echo's recover middleware usually handles the error and commits response.
	// Our implementation calls c.Error(err) which might set status code.

	// Log verification
	var logEntry map[string]interface{}
	jsonErr := json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, jsonErr)

	msg, ok := logEntry["msg"].(string)
	assert.True(t, ok)
	assert.Equal(t, "PANIC RECOVERED", msg)

	level, ok := logEntry["level"].(string)
	assert.True(t, ok)
	assert.Equal(t, "error", level)

	fields, ok := logEntry["component"].(string)
	assert.True(t, ok)
	assert.Equal(t, "api.middleware", fields)

	errorField, ok := logEntry["error"].(string)
	assert.True(t, ok)
	assert.Contains(t, errorField, "test panic")

	stack, ok := logEntry["stack"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, stack)
}

func TestPanicRecovery_WithError(t *testing.T) {
	// Setup
	e := echo.New()
	e.Use(PanicRecovery())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Capture logs
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	defer logrus.SetOutput(logrus.StandardLogger().Out)

	// Middleware execution with panic error
	h := func(c echo.Context) error {
		panic(assert.AnError)
	}

	err := PanicRecovery()(h)(c)

	assert.NoError(t, err)

	// Log verification
	var logEntry map[string]interface{}
	jsonErr := json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, jsonErr)

	errorField, ok := logEntry["error"].(string)
	assert.True(t, ok)
	assert.Contains(t, errorField, assert.AnError.Error())
}
