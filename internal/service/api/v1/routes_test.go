package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	apiauth "github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/darkkaiser/notify-server/internal/service/api/testutil"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestAppConfig() *config.AppConfig {
	return &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			Applications: []config.ApplicationConfig{
				{
					ID:                "test-app",
					Title:             "Test Application",
					DefaultNotifierID: "test-notifier",
					AppKey:            "test-app-key",
				},
			},
		},
	}
}

func TestSetupRoutes_Table(t *testing.T) {
	// Setup
	e := echo.New()
	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewApplicationManager(appConfig)
	mockService := &testutil.MockNotificationSender{}
	h := handler.NewHandler(applicationManager, mockService)
	SetupRoutes(e, h)

	tests := []struct {
		name        string
		method      string
		path        string
		shouldExist bool
	}{
		{"Notifications POST", http.MethodPost, "/api/v1/notifications", true},
		{"Legacy Message POST", http.MethodPost, "/api/v1/notice/message", true},
		{"Random Path", http.MethodGet, "/api/v1/random", false},
	}

	routes := e.Routes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, route := range routes {
				if route.Path == tt.path && route.Method == tt.method {
					found = true
					break
				}
			}
			assert.Equal(t, tt.shouldExist, found, "Route existence mismatch for %s %s", tt.method, tt.path)
		})
	}
}

func TestNotificationsEndpoint_Integration_Table(t *testing.T) {
	// Common Setup
	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewApplicationManager(appConfig)

	tests := []struct {
		name           string
		method         string
		path           string
		appKey         string
		body           interface{}
		shouldFail     bool
		expectedStatus int
		verifyResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "Success Notification",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Test Message",
			},
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var successResp response.SuccessResponse
				err := json.Unmarshal(rec.Body.Bytes(), &successResp)
				require.NoError(t, err)
				assert.Equal(t, 0, successResp.ResultCode)
			},
		},
		{
			name:   "Missing AppKey",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Message",
			},
			expectedStatus: http.StatusBadRequest,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errorResp response.ErrorResponse
				json.Unmarshal(rec.Body.Bytes(), &errorResp)
				assert.Contains(t, errorResp.Message, "app_key")
			},
		},
		{
			name:   "Invalid AppKey",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "wrong-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Message",
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid JSON",
			method:         http.MethodPost,
			path:           "/api/v1/notifications",
			appKey:         "test-app-key",
			body:           "invalid-json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "Missing Message",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Method Not Allowed",
			method:         http.MethodGet,
			path:           "/api/v1/notifications",
			appKey:         "test-app-key",
			body:           nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:   "Unknown ApplicationID",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "any-key",
			body: request.NotificationRequest{
				ApplicationID: "unknown-app",
				Message:       "test",
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "Service Failure (Legacy 200)",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "fail test",
			},
			shouldFail:     true,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Per-test Setup
			e := echo.New()
			mockService := &testutil.MockNotificationSender{ShouldFail: tt.shouldFail}
			h := handler.NewHandler(applicationManager, mockService)
			SetupRoutes(e, h)

			// Request Creation
			var bodyBytes []byte
			if strBody, ok := tt.body.(string); ok {
				if strBody == "invalid-json" {
					bodyBytes = []byte(`{"invalid json`)
				} else {
					bodyBytes = []byte(strBody)
				}
			} else if tt.body != nil {
				jsonBytes, _ := json.Marshal(tt.body)
				bodyBytes = jsonBytes
			}

			reqPath := tt.path
			if tt.appKey != "" {
				reqPath += "?app_key=" + tt.appKey
			}

			req := httptest.NewRequest(tt.method, reqPath, bytes.NewReader(bodyBytes))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			// Execute
			e.ServeHTTP(rec, req)

			// Verify
			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.verifyResponse != nil {
				tt.verifyResponse(t, rec)
			}
		})
	}
}
