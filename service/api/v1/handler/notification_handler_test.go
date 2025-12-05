package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/api/auth"
	"github.com/darkkaiser/notify-server/service/api/model/response"
	"github.com/darkkaiser/notify-server/service/api/v1/model/request"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHandler_SendNotifyMessageHandler(t *testing.T) {
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
	appManager := auth.NewApplicationManager(appConfig)
	h := NewHandler(appManager, mockSender)

	t.Run("정상적인 메시지 전송", func(t *testing.T) {
		reqBody := request.NotifyMessageRequest{
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
		if assert.NoError(t, h.SendNotificationHandler(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.True(t, mockSender.NotifyCalled)
			assert.Equal(t, "test-notifier", mockSender.LastNotifierID)
			assert.Equal(t, "Test App", mockSender.LastTitle)
			assert.Equal(t, "Test Message", mockSender.LastMessage)
		}
	})

	t.Run("잘못된 AppKey", func(t *testing.T) {
		reqBody := request.NotifyMessageRequest{
			ApplicationID: "test-app",
			Message:       "Test Message",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/?app_key=invalid-key", strings.NewReader(string(jsonBody)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute
		err := h.SendNotificationHandler(c)
		if assert.Error(t, err) {
			he, ok := err.(*echo.HTTPError)
			assert.True(t, ok)
			assert.Equal(t, http.StatusUnauthorized, he.Code)

			errResp, ok := he.Message.(response.ErrorResponse)
			assert.True(t, ok)
			assert.Contains(t, errResp.Message, "app_key가 유효하지 않습니다")
		}
	})

	t.Run("허용되지 않은 ApplicationID", func(t *testing.T) {
		reqBody := request.NotifyMessageRequest{
			ApplicationID: "unknown-app",
			Message:       "Test Message",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/?app_key=valid-key", strings.NewReader(string(jsonBody)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute
		err := h.SendNotificationHandler(c)
		if assert.Error(t, err) {
			he, ok := err.(*echo.HTTPError)
			assert.True(t, ok)
			assert.Equal(t, http.StatusUnauthorized, he.Code)

			errResp, ok := he.Message.(response.ErrorResponse)
			assert.True(t, ok)
			assert.Contains(t, errResp.Message, "접근이 허용되지 않은 application_id")
		}
	})

	t.Run("잘못된 요청 본문", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/?app_key=valid-key", strings.NewReader("invalid-json"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute
		err := h.SendNotificationHandler(c)
		assert.Error(t, err)
	})

	t.Run("ApplicationID 누락", func(t *testing.T) {
		reqBody := request.NotifyMessageRequest{
			ApplicationID: "",
			Message:       "Test Message",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/?app_key=valid-key", strings.NewReader(string(jsonBody)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute
		err := h.SendNotificationHandler(c)
		if assert.Error(t, err) {
			he, ok := err.(*echo.HTTPError)
			assert.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, he.Code)

			errResp, ok := he.Message.(response.ErrorResponse)
			assert.True(t, ok)
			assert.Contains(t, errResp.Message, "애플리케이션 ID")
			assert.Contains(t, errResp.Message, "필수")
		}
	})

	t.Run("Message 누락", func(t *testing.T) {
		reqBody := request.NotifyMessageRequest{
			ApplicationID: "test-app",
			Message:       "",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/?app_key=valid-key", strings.NewReader(string(jsonBody)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute
		err := h.SendNotificationHandler(c)
		if assert.Error(t, err) {
			he, ok := err.(*echo.HTTPError)
			assert.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, he.Code)

			errResp, ok := he.Message.(response.ErrorResponse)
			assert.True(t, ok)
			assert.Contains(t, errResp.Message, "메시지")
			assert.Contains(t, errResp.Message, "필수")
		}
	})

	t.Run("Message 길이 초과", func(t *testing.T) {
		// 4096자를 초과하는 메시지 생성
		longMessage := make([]byte, 4097)
		for i := range longMessage {
			longMessage[i] = 'a'
		}

		reqBody := request.NotifyMessageRequest{
			ApplicationID: "test-app",
			Message:       string(longMessage),
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/?app_key=valid-key", strings.NewReader(string(jsonBody)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute
		err := h.SendNotificationHandler(c)
		if assert.Error(t, err) {
			he, ok := err.(*echo.HTTPError)
			assert.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, he.Code)

			errResp, ok := he.Message.(response.ErrorResponse)
			assert.True(t, ok)
			assert.Contains(t, errResp.Message, "메시지")
			assert.Contains(t, errResp.Message, "최대")
			assert.Contains(t, errResp.Message, "4096")
		}
	})

	t.Run("Message 최대 길이 허용 (4096자)", func(t *testing.T) {
		// 정확히 4096자인 메시지
		maxMessage := make([]byte, 4096)
		for i := range maxMessage {
			maxMessage[i] = 'a'
		}

		reqBody := request.NotifyMessageRequest{
			ApplicationID: "test-app",
			Message:       string(maxMessage),
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/?app_key=valid-key", strings.NewReader(string(jsonBody)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Execute
		if assert.NoError(t, h.SendNotificationHandler(c)) {
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.True(t, mockSender.NotifyCalled)
		}
	})
}
