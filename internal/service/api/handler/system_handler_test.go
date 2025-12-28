package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/version"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// MockNotificationSender는 테스트용 NotificationService입니다.
type MockNotificationSender struct{}

func (m *MockNotificationSender) NotifyWithTitle(notifierID string, title string, message string, errorOccurred bool) bool {
	return true
}

func (m *MockNotificationSender) NotifyDefault(message string) bool {
	return true
}

func (m *MockNotificationSender) NotifyDefaultWithError(message string) bool {
	return true
}

func TestHealthCheckHandler_Table(t *testing.T) {
	tests := []struct {
		name              string
		mockService       *MockNotificationSender
		useNilService     bool
		expectedStatus    string
		expectedDepStatus string
	}{
		{
			name:              "Healthy",
			mockService:       &MockNotificationSender{},
			useNilService:     false,
			expectedStatus:    "healthy",
			expectedDepStatus: "healthy",
		},
		{
			name:              "Unhealthy (Service Nil)",
			mockService:       nil,
			useNilService:     true,
			expectedStatus:    "unhealthy",
			expectedDepStatus: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			var h *SystemHandler
			if tt.useNilService {
				h = NewSystemHandler(nil, version.Info{})
			} else {
				h = NewSystemHandler(tt.mockService, version.Info{})
			}

			if assert.NoError(t, h.HealthCheckHandler(c)) {
				assert.Equal(t, http.StatusOK, rec.Code)

				var healthResp response.HealthResponse
				err := json.Unmarshal(rec.Body.Bytes(), &healthResp)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedStatus, healthResp.Status)
				assert.Equal(t, tt.expectedDepStatus, healthResp.Dependencies["notification_service"].Status)

				if tt.expectedStatus == "healthy" {
					assert.GreaterOrEqual(t, healthResp.Uptime, int64(0), "Uptime should be >= 0")
				}
			}
		})
	}
}

func TestVersionHandler_Table(t *testing.T) {
	buildInfo := version.Info{
		Version:     "1.0.0",
		BuildDate:   "2024-01-01",
		BuildNumber: "100",
	}

	tests := []struct {
		name      string
		buildInfo version.Info
	}{
		{
			name:      "Version Info Present",
			buildInfo: buildInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/version", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			h := NewSystemHandler(&MockNotificationSender{}, tt.buildInfo)

			if assert.NoError(t, h.VersionHandler(c)) {
				assert.Equal(t, http.StatusOK, rec.Code)

				var versionResp response.VersionResponse
				err := json.Unmarshal(rec.Body.Bytes(), &versionResp)
				assert.NoError(t, err)
				assert.Equal(t, tt.buildInfo.Version, versionResp.Version)
				assert.Equal(t, tt.buildInfo.BuildDate, versionResp.BuildDate)
				assert.Equal(t, tt.buildInfo.BuildNumber, versionResp.BuildNumber)
				assert.NotEmpty(t, versionResp.GoVersion)
			}
		})
	}
}
