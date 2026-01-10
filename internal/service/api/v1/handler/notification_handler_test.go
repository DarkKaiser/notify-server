package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHandler_PublishNotificationHandler_Table(t *testing.T) {
	// Common Setup
	mockService := &mocks.MockNotificationSender{}
	appConfig := &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			Applications: []config.ApplicationConfig{
				{
					ID:                "test-app",
					Title:             "Test App",
					DefaultNotifierID: "test-notifier",
					AppKey:            "valid-key",
				},
			},
		},
	}
	appManager := auth.NewApplicationManager(appConfig)
	h := NewHandler(appManager, mockService)

	tests := []struct {
		name              string
		appKey            string
		reqBody           interface{}
		mockFail          bool
		expectedStatus    int
		verifyErrResponse func(*testing.T, response.ErrorResponse)
		verifyMock        func(*testing.T, *mocks.MockNotificationSender)
	}{
		{
			name:   "Success Notification",
			appKey: "valid-key",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Test Message",
			},
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *mocks.MockNotificationSender) {
				assert.True(t, m.NotifyCalled)
				assert.Equal(t, "test-notifier", m.LastNotifierID)
				assert.Equal(t, "Test App", m.LastTitle)
				assert.Equal(t, "Test Message", m.LastMessage)
				assert.False(t, m.LastErrorOccurred)
			},
		},
		{
			name:   "Invalid AppKey",
			appKey: "invalid-key",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Test Message",
			},
			expectedStatus: http.StatusUnauthorized,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "app_key가 유효하지 않습니다")
			},
		},
		{
			name:   "Unauthorized AppID",
			appKey: "valid-key",
			reqBody: request.NotificationRequest{
				ApplicationID: "unknown-app",
				Message:       "Test Message",
			},
			expectedStatus: http.StatusUnauthorized,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "접근이 허용되지 않은 application_id")
			},
		},
		{
			name:           "Invalid JSON Body",
			appKey:         "valid-key",
			reqBody:        "invalid-json", // Helper handles string as raw body logic
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "Missing ApplicationID",
			appKey: "valid-key",
			reqBody: request.NotificationRequest{
				ApplicationID: "",
				Message:       "Test Message",
			},
			expectedStatus: http.StatusBadRequest,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "애플리케이션 ID")
				assert.Contains(t, errResp.Message, "필수")
			},
		},
		{
			name:   "Missing Message",
			appKey: "valid-key",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "",
			},
			expectedStatus: http.StatusBadRequest,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "메시지")
				assert.Contains(t, errResp.Message, "필수")
			},
		},
		{
			name:   "Message Too Long",
			appKey: "valid-key",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       strings.Repeat("a", 4097),
			},
			expectedStatus: http.StatusBadRequest,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "메시지")
				assert.Contains(t, errResp.Message, "최대")
				assert.Contains(t, errResp.Message, "4096")
			},
		},
		{
			name:   "Service Failure (Still 200)",
			appKey: "valid-key",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Fail Message",
			},
			mockFail:       true,
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *mocks.MockNotificationSender) {
				assert.True(t, m.NotifyCalled)
				// Service fail logic implementation detail: does it return specific error or just bool false?
				// Handler ignores false return currently (legacy behavior).
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.Reset()
			mockService.ShouldFail = tt.mockFail

			e := echo.New()

			var bodyStr string
			if s, ok := tt.reqBody.(string); ok {
				bodyStr = s
			} else {
				b, _ := json.Marshal(tt.reqBody)
				bodyStr = string(b)
			}

			req := httptest.NewRequest(http.MethodPost, "/?app_key="+tt.appKey, strings.NewReader(bodyStr))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := h.PublishNotificationHandler(c)

			if tt.expectedStatus == http.StatusOK {
				assert.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				if err != nil {
					he, ok := err.(*echo.HTTPError)
					assert.True(t, ok)
					assert.Equal(t, tt.expectedStatus, he.Code)
					if tt.verifyErrResponse != nil {
						errResp, ok := he.Message.(response.ErrorResponse)
						assert.True(t, ok)
						tt.verifyErrResponse(t, errResp)
					}
				} else {
					// Expected error but got none
					// Check if Echo wrote error directly to body (Binding error does this)
					// But we assert assert.Error(t, err) usually
					// Standard echo binding might return error
					assert.Error(t, err, "Expected error")
				}
			}

			if tt.verifyMock != nil {
				tt.verifyMock(t, mockService)
			}
		})
	}
}
