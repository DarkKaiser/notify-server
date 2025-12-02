package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/api/model"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// MockNotificationSender is a mock implementation of NotificationSender
type MockNotificationSender struct {
	NotifyCalled      bool
	LastNotifierID    string
	LastTitle         string
	LastMessage       string
	LastErrorOccurred bool
}

func (m *MockNotificationSender) Notify(notifierID string, title string, message string, errorOccurred bool) bool {
	m.NotifyCalled = true
	m.LastNotifierID = notifierID
	m.LastTitle = title
	m.LastMessage = message
	m.LastErrorOccurred = errorOccurred
	return true
}

func (m *MockNotificationSender) NotifyToDefault(message string) bool {
	return true
}

func (m *MockNotificationSender) NotifyWithErrorToDefault(message string) bool {
	return true
}

func TestHandler_NotifyMessageSendHandler(t *testing.T) {
	// Setup
	e := echo.New()
	mockSender := &MockNotificationSender{}

	// Test Config
	appConfig := &config.AppConfig{}
	appConfig.NotifyAPI.Applications = []config.ApplicationConfig{
		{
			ID:                "test-app",
			Title:             "Test App",
			Description:       "Test Application",
			DefaultNotifierID: "test-notifier",
			AppKey:            "valid-key",
		},
	}
	h := NewHandler(appConfig, mockSender, "1.0.0", "2024-01-01", "100")

	t.Run("정상적인 메시지 전송", func(t *testing.T) {
		reqBody := model.NotifyMessage{
			ApplicationID: "test-app",
			Message:       "Test Message",
			ErrorOccurred: false,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/?app_key=valid-key", strings.NewReader(string(jsonBody)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute
		if assert.NoError(t, h.NotifyMessageSendHandler(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.True(t, mockSender.NotifyCalled)
			assert.Equal(t, "test-notifier", mockSender.LastNotifierID)
			assert.Equal(t, "Test App", mockSender.LastTitle)
			assert.Equal(t, "Test Message", mockSender.LastMessage)
		}
	})

	t.Run("잘못된 AppKey", func(t *testing.T) {
		reqBody := model.NotifyMessage{
			ApplicationID: "test-app",
			Message:       "Test Message",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/?app_key=invalid-key", strings.NewReader(string(jsonBody)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute
		err := h.NotifyMessageSendHandler(c)
		if assert.Error(t, err) {
			he, ok := err.(*echo.HTTPError)
			assert.True(t, ok)
			assert.Equal(t, http.StatusUnauthorized, he.Code)
			assert.Contains(t, he.Message, "APP_KEY가 유효하지 않습니다")
		}
	})

	t.Run("허용되지 않은 ApplicationID", func(t *testing.T) {
		reqBody := model.NotifyMessage{
			ApplicationID: "unknown-app",
			Message:       "Test Message",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/?app_key=valid-key", strings.NewReader(string(jsonBody)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute
		err := h.NotifyMessageSendHandler(c)
		if assert.Error(t, err) {
			he, ok := err.(*echo.HTTPError)
			assert.True(t, ok)
			assert.Equal(t, http.StatusUnauthorized, he.Code)
			assert.Contains(t, he.Message, "접근이 허용되지 않은 Application입니다")
		}
	})

	t.Run("잘못된 요청 본문", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/?app_key=valid-key", strings.NewReader("invalid-json"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute
		err := h.NotifyMessageSendHandler(c)
		assert.Error(t, err)
	})
}
