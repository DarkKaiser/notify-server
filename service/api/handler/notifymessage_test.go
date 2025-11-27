package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/api/model"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestNotifyMessageSendHandler(t *testing.T) {
	t.Run("정상적인 메시지 전송", func(t *testing.T) {
		// Echo 인스턴스 생성
		e := echo.New()

		// 테스트용 설정
		config := &g.AppConfig{}
		config.NotifyAPI.Applications = []struct {
			ID                string `json:"id"`
			Title             string `json:"title"`
			Description       string `json:"description"`
			DefaultNotifierID string `json:"default_notifier_id"`
			AppKey            string `json:"app_key"`
		}{
			{
				ID:                "test-app",
				Title:             "Test Application",
				Description:       "Test Description",
				DefaultNotifierID: "test-notifier",
				AppKey:            "secret-key-123",
			},
		}

		mockSender := &mockNotificationSender{}
		handler := NewHandler(config, mockSender)

		// 요청 데이터
		reqBody := model.NotifyMessage{
			ApplicationID: "test-app",
			Message:       "Test notification",
			ErrorOccurred: false,
		}
		jsonData, _ := json.Marshal(reqBody)

		// HTTP 요청 생성
		req := httptest.NewRequest(http.MethodPost, "/notify?app_key=secret-key-123", strings.NewReader(string(jsonData)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// 핸들러 실행
		err := handler.NotifyMessageSendHandler(c)

		assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
		assert.Equal(t, http.StatusOK, rec.Code, "상태 코드가 200이어야 합니다")

		// 응답 확인
		var response map[string]int
		json.Unmarshal(rec.Body.Bytes(), &response)
		assert.Equal(t, 0, response["result_code"], "result_code가 0이어야 합니다")

		// Mock 호출 확인
		assert.Equal(t, 1, len(mockSender.notifyCalls), "Notify가 1번 호출되어야 합니다")
		assert.Equal(t, "test-notifier", mockSender.notifyCalls[0].notifierID, "NotifierID가 일치해야 합니다")
		assert.Equal(t, "Test notification", mockSender.notifyCalls[0].message, "메시지가 일치해야 합니다")
	})

	t.Run("잘못된 app_key", func(t *testing.T) {
		e := echo.New()

		config := &g.AppConfig{}
		config.NotifyAPI.Applications = []struct {
			ID                string `json:"id"`
			Title             string `json:"title"`
			Description       string `json:"description"`
			DefaultNotifierID string `json:"default_notifier_id"`
			AppKey            string `json:"app_key"`
		}{
			{
				ID:                "test-app",
				Title:             "Test Application",
				Description:       "Test Description",
				DefaultNotifierID: "test-notifier",
				AppKey:            "correct-key",
			},
		}

		mockSender := &mockNotificationSender{}
		handler := NewHandler(config, mockSender)

		reqBody := model.NotifyMessage{
			ApplicationID: "test-app",
			Message:       "Test notification",
			ErrorOccurred: false,
		}
		jsonData, _ := json.Marshal(reqBody)

		// 잘못된 app_key 사용
		req := httptest.NewRequest(http.MethodPost, "/notify?app_key=wrong-key", strings.NewReader(string(jsonData)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.NotifyMessageSendHandler(c)

		assert.Error(t, err, "에러가 발생해야 합니다")
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok, "HTTPError여야 합니다")
		assert.Equal(t, http.StatusUnauthorized, httpErr.Code, "상태 코드가 401이어야 합니다")
	})

	t.Run("존재하지 않는 application_id", func(t *testing.T) {
		e := echo.New()

		config := &g.AppConfig{}
		config.NotifyAPI.Applications = []struct {
			ID                string `json:"id"`
			Title             string `json:"title"`
			Description       string `json:"description"`
			DefaultNotifierID string `json:"default_notifier_id"`
			AppKey            string `json:"app_key"`
		}{
			{
				ID:                "existing-app",
				Title:             "Existing Application",
				Description:       "Description",
				DefaultNotifierID: "notifier",
				AppKey:            "key-123",
			},
		}

		mockSender := &mockNotificationSender{}
		handler := NewHandler(config, mockSender)

		reqBody := model.NotifyMessage{
			ApplicationID: "non-existent-app",
			Message:       "Test notification",
			ErrorOccurred: false,
		}
		jsonData, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/notify?app_key=key-123", strings.NewReader(string(jsonData)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.NotifyMessageSendHandler(c)

		assert.Error(t, err, "에러가 발생해야 합니다")
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok, "HTTPError여야 합니다")
		assert.Equal(t, http.StatusUnauthorized, httpErr.Code, "상태 코드가 401이어야 합니다")
		assert.Contains(t, httpErr.Message, "접근이 허용되지 않은", "적절한 에러 메시지를 반환해야 합니다")
	})

	t.Run("에러 메시지 전송", func(t *testing.T) {
		e := echo.New()

		config := &g.AppConfig{}
		config.NotifyAPI.Applications = []struct {
			ID                string `json:"id"`
			Title             string `json:"title"`
			Description       string `json:"description"`
			DefaultNotifierID string `json:"default_notifier_id"`
			AppKey            string `json:"app_key"`
		}{
			{
				ID:                "error-app",
				Title:             "Error Application",
				Description:       "Description",
				DefaultNotifierID: "error-notifier",
				AppKey:            "error-key",
			},
		}

		mockSender := &mockNotificationSender{}
		handler := NewHandler(config, mockSender)

		reqBody := model.NotifyMessage{
			ApplicationID: "error-app",
			Message:       "Error occurred!",
			ErrorOccurred: true,
		}
		jsonData, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/notify?app_key=error-key", strings.NewReader(string(jsonData)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.NotifyMessageSendHandler(c)

		assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
		assert.Equal(t, 1, len(mockSender.notifyCalls), "Notify가 1번 호출되어야 합니다")
		assert.True(t, mockSender.notifyCalls[0].errorOccurred, "ErrorOccurred가 true여야 합니다")
	})
}
