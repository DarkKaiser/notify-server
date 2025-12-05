package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/pkg/common"
	"github.com/darkkaiser/notify-server/service/api/model/response"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// MockNotificationSender는 테스트용 NotificationSender입니다.
type MockNotificationSender struct{}

func (m *MockNotificationSender) Notify(notifierID string, title string, message string, errorOccurred bool) bool {
	return true
}

func (m *MockNotificationSender) NotifyToDefault(message string) bool {
	return true
}

func (m *MockNotificationSender) NotifyWithErrorToDefault(message string) bool {
	return true
}

func TestHealthCheckHandler(t *testing.T) {
	t.Run("정상 상태 반환", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockSender := &MockNotificationSender{}
		h := NewSystemHandler(mockSender, common.BuildInfo{})

		if assert.NoError(t, h.HealthCheckHandler(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)

			var healthResp response.HealthResponse
			err := json.Unmarshal(rec.Body.Bytes(), &healthResp)
			assert.NoError(t, err)
			assert.Equal(t, "healthy", healthResp.Status)
			assert.Equal(t, "healthy", healthResp.Dependencies["notification_service"].Status)
		}
	})

	t.Run("의존성 서비스 비정상 시 unhealthy 반환", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// nil NotificationSender 전달
		h := NewSystemHandler(nil, common.BuildInfo{})

		if assert.NoError(t, h.HealthCheckHandler(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)

			var healthResp response.HealthResponse
			err := json.Unmarshal(rec.Body.Bytes(), &healthResp)
			assert.NoError(t, err)
			assert.Equal(t, "unhealthy", healthResp.Status)
			assert.Equal(t, "unhealthy", healthResp.Dependencies["notification_service"].Status)
		}
	})
}

func TestVersionHandler(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	buildInfo := common.BuildInfo{
		Version:     "1.0.0",
		BuildDate:   "2024-01-01",
		BuildNumber: "100",
	}
	h := NewSystemHandler(&MockNotificationSender{}, buildInfo)

	if assert.NoError(t, h.VersionHandler(c)) {
		assert.Equal(t, http.StatusOK, rec.Code)

		var versionResp response.VersionResponse
		err := json.Unmarshal(rec.Body.Bytes(), &versionResp)
		assert.NoError(t, err)
		assert.Equal(t, "1.0.0", versionResp.Version)
		assert.Equal(t, "2024-01-01", versionResp.BuildDate)
		assert.Equal(t, "100", versionResp.BuildNumber)
		assert.NotEmpty(t, versionResp.GoVersion)
	}
}
