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
	"github.com/darkkaiser/notify-server/service/api/testutil"
	"github.com/darkkaiser/notify-server/service/api/v1/model/request"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHandler_PublishNotificationHandler(t *testing.T) {
	// Setup
	mockService := &testutil.MockNotificationService{}

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
	h := NewHandler(appManager, mockService)

	tests := []struct {
		name              string
		appKey            string
		reqBody           interface{} // string or struct
		mockFail          bool
		expectedStatus    int
		verifyErrResponse func(t *testing.T, errResp response.ErrorResponse)
		verifyMock        func(t *testing.T, m *testutil.MockNotificationService)
	}{
		{
			name:   "정상적인 메시지 전송",
			appKey: "valid-key",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Test Message",
				ErrorOccurred: false,
			},
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *testutil.MockNotificationService) {
				assert.True(t, m.NotifyCalled)
				assert.Equal(t, "test-notifier", m.LastNotifierID)
				assert.Equal(t, "Test App", m.LastTitle)
				assert.Equal(t, "Test Message", m.LastMessage)
				assert.False(t, m.LastErrorOccurred)
			},
		},
		{
			name:   "잘못된 AppKey",
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
			name:   "허용되지 않은 ApplicationID",
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
			name:   "잘못된 요청 본문 (JSON 파싱 에러)",
			appKey: "valid-key",
			reqBody: func() string {
				return "invalid-json"
			}(),
			expectedStatus: http.StatusBadRequest, // Echo 바인딩 에러는 400 반환
		},
		{
			name:   "ApplicationID 누락",
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
			name:   "Message 누락",
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
			name:   "Message 길이 초과",
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
			name:   "Message 최대 길이 허용 (4096자)",
			appKey: "valid-key",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       strings.Repeat("a", 4096),
			},
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *testutil.MockNotificationService) {
				assert.True(t, m.NotifyCalled)
				assert.Equal(t, 4096, len(m.LastMessage))
			},
		},
		{
			name:   "알림 서비스 전송 실패 (여전히 200 OK)",
			appKey: "valid-key",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Fail Message",
			},
			mockFail:       true,
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *testutil.MockNotificationService) {
				assert.True(t, m.NotifyCalled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock 초기화 및 설정
			mockService.Reset()
			mockService.ShouldFail = tt.mockFail

			e := echo.New()
			var bodyStr string
			if s, ok := tt.reqBody.(string); ok {
				bodyStr = s
			} else {
				jsonBytes, _ := json.Marshal(tt.reqBody)
				bodyStr = string(jsonBytes)
			}

			req := httptest.NewRequest(http.MethodPost, "/?app_key="+tt.appKey, strings.NewReader(bodyStr))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Execute
			err := h.PublishNotificationHandler(c)

			// 검증
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
					// 에러가 발생해야 하는데 안 한 경우 (혹은 바인딩 에러가 내부적으로 처리되어 응답에 쓰여진 경우 확인)
					// Echo Handler는 에러를 리턴하는 것이 관례
					// 다만, Binding 에러 같은 경우엔 echo가 자동으로 에러를 리턴함. 우리가 handler 내에서 bind를 호출하고 에러를 리턴하므로 err != nil이어야 함.
					assert.Error(t, err, "에러가 발생해야 합니다")
				}
			}

			if tt.verifyMock != nil {
				tt.verifyMock(t, mockService)
			}
		})
	}
}
