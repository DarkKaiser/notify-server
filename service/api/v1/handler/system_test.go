package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/api/v1/model"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHandler_HealthCheckHandler(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewHandler(&config.AppConfig{}, nil, "1.0.0", "2024-01-01", "100")

	// Assertions
	if assert.NoError(t, h.HealthCheckHandler(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)

		var response model.HealthResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "healthy", response.Status)
		assert.GreaterOrEqual(t, response.Uptime, int64(0))
	}
}

func TestHandler_VersionHandler(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	version := "1.0.0"
	buildDate := "2024-01-01"
	buildNumber := "100"
	h := NewHandler(&config.AppConfig{}, nil, version, buildDate, buildNumber)

	// Assertions
	if assert.NoError(t, h.VersionHandler(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)

		var response model.VersionResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, version, response.Version)
		assert.Equal(t, buildDate, response.BuildDate)
		assert.Equal(t, buildNumber, response.BuildNumber)
		assert.Equal(t, runtime.Version(), response.GoVersion)
	}
}
