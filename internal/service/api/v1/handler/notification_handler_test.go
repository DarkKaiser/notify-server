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
	"github.com/stretchr/testify/require"
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
	authenticator := auth.NewAuthenticator(appConfig)
	handler := NewHandler(authenticator, mockService)

	return handler, mockService
}

// createTestRequest 테스트용 HTTP 요청을 생성합니다.
func createTestRequest(t *testing.T, method, url string, appKey string, useHeader bool, body interface{}) (*httptest.ResponseRecorder, echo.Context) {
	t.Helper()

	e := echo.New()

	var bodyStr string
	if s, ok := body.(string); ok {
		bodyStr = s
	} else if body != nil {
		b, _ := json.Marshal(body)
		bodyStr = string(b)
	}

	// URL에 쿼리 파라미터 추가
	if !useHeader && appKey != "" {
		if strings.Contains(url, "?") {
			url += "&app_key=" + appKey
		} else {
			url += "?app_key=" + appKey
		}
	}

	req := httptest.NewRequest(method, url, strings.NewReader(bodyStr))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	// 헤더 방식인 경우 X-App-Key 헤더 설정
	if useHeader && appKey != "" {
		req.Header.Set(constants.HeaderAppKey, appKey)
	}

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	return rec, c
}

// =============================================================================
// PublishNotificationHandler Tests
// =============================================================================

// TestPublishNotificationHandler는 알림 게시 핸들러를 검증합니다.
//
// 검증 항목:
//   - 성공 케이스 (헤더/쿼리 파라미터 방식)
//   - 인증 실패 (잘못된 App Key, 미등록 App ID)
//   - 입력 검증 실패 (필수 필드 누락, 길이 초과)
//   - 바인딩 실패 (잘못된 JSON)
//   - 서비스 실패 (503 에러)
func TestPublishNotificationHandler(t *testing.T) {
	tests := []struct {
		name              string
		appKey            string
		useHeader         bool
		reqBody           interface{}
		mockFail          bool
		expectedStatus    int
		verifyErrResponse func(*testing.T, response.ErrorResponse)
		verifyMock        func(*testing.T, *mocks.MockNotificationSender)
	}{
		// ===== 성공 케이스 =====
		{
			name:      "성공: 헤더 방식 인증",
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
			name:      "성공: 쿼리 파라미터 방식 인증 (레거시)",
			appKey:    "valid-key",
			useHeader: false,
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Test Message",
			},
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *mocks.MockNotificationSender) {
				assert.True(t, m.NotifyCalled)
			},
		},
		{
			name:      "성공: ErrorOccurred=true",
			appKey:    "valid-key",
			useHeader: true,
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Error Message",
				ErrorOccurred: true,
			},
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *mocks.MockNotificationSender) {
				assert.True(t, m.LastErrorOccurred)
			},
		},

		// ===== 인증 실패 =====
		{
			name:   "실패: 잘못된 App Key",
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
			name:   "실패: 미등록 Application ID",
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
			name:   "실패: App Key 누락",
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

		// ===== 입력 검증 실패 =====
		{
			name:   "실패: Application ID 누락",
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
			name:   "실패: Message 누락",
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
			name:   "실패: Message 길이 초과 (4097자)",
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

		// ===== 바인딩 실패 =====
		{
			name:           "실패: 잘못된 JSON 형식",
			appKey:         "valid-key",
			reqBody:        "invalid-json",
			expectedStatus: http.StatusBadRequest,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "잘못된 요청 형식")
			},
		},

		// ===== 서비스 실패 =====
		{
			name:   "실패: 알림 서비스 혼잡 (503)",
			appKey: "valid-key",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Fail Message",
			},
			mockFail:       true,
			expectedStatus: http.StatusServiceUnavailable,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "현재 알림 서비스가 혼잡")
				assert.Contains(t, errResp.Message, "다시 시도")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mockService := setupTestHandler(t)
			mockService.Reset()
			mockService.ShouldFail = tt.mockFail

			rec, c := createTestRequest(t, http.MethodPost, "/", tt.appKey, tt.useHeader, tt.reqBody)

			err := handler.PublishNotificationHandler(c)

			if tt.expectedStatus == http.StatusOK {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				require.Error(t, err)
				httpErr, ok := err.(*echo.HTTPError)
				require.True(t, ok, "Error should be *echo.HTTPError")
				assert.Equal(t, tt.expectedStatus, httpErr.Code)

				if tt.verifyErrResponse != nil {
					errResp, ok := httpErr.Message.(response.ErrorResponse)
					require.True(t, ok, "Message should be response.ErrorResponse")
					tt.verifyErrResponse(t, errResp)
				}
			}

			if tt.verifyMock != nil {
				tt.verifyMock(t, mockService)
			}
		})
	}
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

	logEntry := handler.log(c)

	assert.NotNil(t, logEntry, "log() should return a non-nil entry")
}
