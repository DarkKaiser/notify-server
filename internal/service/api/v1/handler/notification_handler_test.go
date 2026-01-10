package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupTestHandler 테스트용 핸들러와 Mock을 생성합니다.
func setupTestHandler(t *testing.T) (*Handler, *mocks.MockNotificationSender) {
	t.Helper()

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
	appManager := auth.NewAuthenticator(appConfig)
	handler := NewHandler(appManager, mockService)

	return handler, mockService
}

// createTestRequest 테스트용 HTTP 요청을 생성합니다.
func createTestRequest(t *testing.T, appKey string, useHeader bool, body interface{}) (*http.Request, *httptest.ResponseRecorder, echo.Context) {
	t.Helper()

	e := echo.New()

	var bodyStr string
	if s, ok := body.(string); ok {
		bodyStr = s
	} else {
		b, _ := json.Marshal(body)
		bodyStr = string(b)
	}

	// useHeader에 따라 헤더 또는 쿼리 파라미터로 App Key 전달
	var reqURL string
	if useHeader {
		reqURL = "/"
	} else {
		reqURL = "/?app_key=" + appKey
	}

	req := httptest.NewRequest(http.MethodPost, reqURL, strings.NewReader(bodyStr))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	// 헤더 방식인 경우 X-App-Key 헤더 설정
	if useHeader && appKey != "" {
		req.Header.Set(constants.HeaderAppKey, appKey)
	}

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	return req, rec, c
}

// =============================================================================
// Helper Function Tests
// =============================================================================

// TestHandler_log는 log() 헬퍼 함수를 검증합니다.
func TestHandler_log(t *testing.T) {
	handler, _ := setupTestHandler(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/notifications")

	// log() 호출
	logEntry := handler.log(c)

	// 로그 엔트리가 생성되는지 확인
	assert.NotNil(t, logEntry, "log() should return a non-nil entry")
}

// =============================================================================
// Handler Tests
// =============================================================================

func TestHandler_PublishNotificationHandler_Table(t *testing.T) {
	tests := []struct {
		name              string
		appKey            string
		useHeader         bool // true면 헤더로, false면 쿼리 파라미터로 전달
		reqBody           interface{}
		mockFail          bool
		expectedStatus    int
		verifyErrResponse func(*testing.T, response.ErrorResponse)
		verifyMock        func(*testing.T, *mocks.MockNotificationSender)
	}{
		{
			name:      "Success with Header AppKey",
			appKey:    "valid-key",
			useHeader: true,
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Test Message",
				ErrorOccurred: false,
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
			name:      "Success with ErrorOccurred=true",
			appKey:    "valid-key",
			useHeader: true,
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Error Message",
				ErrorOccurred: true,
			},
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *mocks.MockNotificationSender) {
				assert.True(t, m.NotifyCalled)
				assert.Equal(t, "test-notifier", m.LastNotifierID)
				assert.Equal(t, "Test App", m.LastTitle)
				assert.Equal(t, "Error Message", m.LastMessage)
				assert.True(t, m.LastErrorOccurred, "ErrorOccurred should be true")
			},
		},
		{
			name:      "Success with Query Param AppKey (Legacy)",
			appKey:    "valid-key",
			useHeader: false,
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
			reqBody:        "invalid-json",
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
			},
		},
		{
			name:   "Missing AppKey (Both Header and Query)",
			appKey: "",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Test Message",
			},
			expectedStatus: http.StatusBadRequest,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "app_key")
				assert.Contains(t, errResp.Message, "필수")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockService := setupTestHandler(t)
			mockService.Reset()
			mockService.ShouldFail = tt.mockFail

			_, rec, c := createTestRequest(t, tt.appKey, tt.useHeader, tt.reqBody)

			err := handler.PublishNotificationHandler(c)

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
					assert.Error(t, err, "Expected error")
				}
			}

			if tt.verifyMock != nil {
				tt.verifyMock(t, mockService)
			}
		})
	}
}
