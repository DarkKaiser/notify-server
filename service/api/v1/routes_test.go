package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/config"
	apiauth "github.com/darkkaiser/notify-server/service/api/auth"
	"github.com/darkkaiser/notify-server/service/api/model/response"
	"github.com/darkkaiser/notify-server/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/service/api/v1/model/request"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockNotificationService는 테스트용 NotificationService 구현체입니다.
type mockNotificationService struct {
	notifyCalled bool
	lastMessage  string
	shouldFail   bool
}

func (m *mockNotificationService) Notify(notifierID string, title string, message string, errorOccurred bool) bool {
	m.notifyCalled = true
	m.lastMessage = message
	return !m.shouldFail
}

func (m *mockNotificationService) NotifyToDefault(message string) bool {
	m.notifyCalled = true
	m.lastMessage = message
	return !m.shouldFail
}

func (m *mockNotificationService) NotifyWithErrorToDefault(message string) bool {
	m.notifyCalled = true
	m.lastMessage = message
	return !m.shouldFail
}

// createTestAppConfig는 테스트용 AppConfig를 생성합니다.
func createTestAppConfig() *config.AppConfig {
	return &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			Applications: []config.ApplicationConfig{
				{
					ID:                "test-app",
					Title:             "Test Application",
					Description:       "Test Description",
					DefaultNotifierID: "test-notifier",
					AppKey:            "test-app-key",
				},
			},
		},
	}
}

// TestSetupRoutes는 SetupRoutes 함수가 올바르게 v1 라우트를 설정하는지 테스트합니다.
func TestSetupRoutes(t *testing.T) {
	e := echo.New()

	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewApplicationManager(appConfig)
	mockService := &mockNotificationService{}

	h := handler.NewHandler(applicationManager, mockService)

	// v1 라우트 설정
	SetupRoutes(e, h)

	// 등록된 라우트 확인
	routes := e.Routes()

	// v1 API 라우트들이 등록되어야 함
	expectedRoutes := map[string]string{
		"/api/v1/notifications":  "POST",
		"/api/v1/notice/message": "POST", // 레거시 엔드포인트
	}

	for path, method := range expectedRoutes {
		found := false
		for _, route := range routes {
			if route.Path == path && route.Method == method {
				found = true
				break
			}
		}
		assert.True(t, found, "라우트 %s %s가 등록되지 않았습니다", method, path)
	}
}

// TestNotificationsEndpoint_TableDriven는 다양한 시나리오에 대한 알림 게시 엔드포인트 테스트를 수행합니다.
func TestNotificationsEndpoint_TableDriven(t *testing.T) {
	e := echo.New()
	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewApplicationManager(appConfig)

	type testCase struct {
		name           string
		method         string
		path           string
		appKey         string
		body           interface{}
		shouldFail     bool // 모의 서비스 실패 여부
		expectedStatus int
		verifyResponse func(t *testing.T, rec *httptest.ResponseRecorder)
	}

	tests := []testCase{
		{
			name:   "정상적인 알림 전송",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "테스트 메시지",
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
			name:   "AppKey 누락",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "메시지",
			},
			expectedStatus: http.StatusBadRequest,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errorResp response.ErrorResponse
				json.Unmarshal(rec.Body.Bytes(), &errorResp)
				assert.Contains(t, errorResp.Message, "app_key")
			},
		},
		{
			name:   "잘못된 AppKey",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "wrong-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "메시지",
			},
			expectedStatus: http.StatusUnauthorized,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errorResp response.ErrorResponse
				json.Unmarshal(rec.Body.Bytes(), &errorResp)
				assert.NotEmpty(t, errorResp.Message)
			},
		},
		{
			name:           "잘못된 JSON 형식",
			method:         http.MethodPost,
			path:           "/api/v1/notifications",
			appKey:         "test-app-key",
			body:           "invalid-json", // 문자열로 처리되어 마샬링 시 에러 유발 혹은 그냥 문자열 바이트로 전송
			expectedStatus: http.StatusBadRequest,
			verifyResponse: nil,
		},
		{
			name:   "필수 필드(Message) 누락",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "",
			},
			expectedStatus: http.StatusBadRequest,
			verifyResponse: nil,
		},
		{
			name:           "지원하지 않는 메서드(GET)",
			method:         http.MethodGet,
			path:           "/api/v1/notifications",
			appKey:         "test-app-key",
			body:           nil,
			expectedStatus: http.StatusMethodNotAllowed,
			verifyResponse: nil,
		},
		{
			name:   "등록되지 않은 ApplicationID",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "any-key",
			body: request.NotificationRequest{
				ApplicationID: "unknown-app",
				Message:       "test",
			},
			expectedStatus: http.StatusUnauthorized,
			verifyResponse: nil,
		},
		{
			name:   "알림 서비스 전송 실패 (Legacy: 200 OK)",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "fail test",
			},
			shouldFail:     true,
			expectedStatus: http.StatusOK,
			verifyResponse: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockService := &mockNotificationService{
				shouldFail: tc.shouldFail,
			}
			h := handler.NewHandler(applicationManager, mockService)
			SetupRoutes(e, h)

			var bodyBytes []byte
			if strBody, ok := tc.body.(string); ok {
				if strBody == "invalid-json" {
					bodyBytes = []byte(`{"invalid json`)
				} else {
					bodyBytes = []byte(strBody)
				}
			} else if tc.body != nil {
				jsonBytes, _ := json.Marshal(tc.body)
				bodyBytes = jsonBytes
			}

			reqPath := tc.path
			if tc.appKey != "" {
				reqPath += "?app_key=" + tc.appKey
			}

			req := httptest.NewRequest(tc.method, reqPath, bytes.NewReader(bodyBytes))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectedStatus, rec.Code)

			if tc.verifyResponse != nil {
				tc.verifyResponse(t, rec)
			}
		})
	}
}
