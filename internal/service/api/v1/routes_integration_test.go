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
	"github.com/darkkaiser/notify-server/internal/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestAppConfig 테스트용 애플리케이션 설정을 생성합니다.
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

// TestV1API_Integration v1 API의 전체 플로우를 검증하는 통합 테스트입니다.
//
// 이 테스트는 다음을 검증합니다:
//   - 라우팅 설정
//   - 핸들러 실행
//   - 미들웨어 적용
//   - 인증 처리
//   - 요청 검증
//   - 응답 생성
//
// 주의: 이 테스트는 통합 테스트이므로 여러 컴포넌트를 함께 테스트합니다.
func TestV1API_Integration(t *testing.T) {
	// Common Setup
	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewAuthenticator(appConfig)

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
			name:   "Service Failure (503 Service Unavailable)",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "fail test",
			},
			shouldFail:     true,
			expectedStatus: http.StatusServiceUnavailable,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var errorResp response.ErrorResponse
				json.Unmarshal(rec.Body.Bytes(), &errorResp)
				assert.NotEmpty(t, errorResp.Message)
			},
		},
		{
			name:   "Legacy Endpoint with Deprecated Headers",
			method: http.MethodPost,
			path:   "/api/v1/notice/message",
			appKey: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Legacy Message",
			},
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				// Deprecated 헤더 검증
				assert.Contains(t, rec.Header().Get("Warning"), "더 이상 사용되지 않는 API")
				assert.Contains(t, rec.Header().Get("Warning"), "/api/v1/notifications")
				assert.Equal(t, "true", rec.Header().Get("X-API-Deprecated"))
				assert.Equal(t, "/api/v1/notifications", rec.Header().Get("X-API-Deprecated-Replacement"))
			},
		},
		{
			name:   "Error Occurred Flag True",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Error Message",
				ErrorOccurred: true,
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
			name:   "Missing ApplicationID",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "",
				Message:       "Message",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "Long Message",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			appKey: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       string(make([]byte, 10000)), // 10KB 메시지 - max 검증 실패
			},
			expectedStatus: http.StatusBadRequest, // 검증 실패로 400 반환
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Per-test Setup
			e := echo.New()
			mockService := &mocks.MockNotificationSender{ShouldFail: tt.shouldFail}
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

// TestV1API_Integration_HeaderAuth 헤더 방식 인증을 검증하는 통합 테스트입니다.
func TestV1API_Integration_HeaderAuth(t *testing.T) {
	// Setup
	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewAuthenticator(appConfig)
	e := echo.New()
	mockService := &mocks.MockNotificationSender{}
	h := handler.NewHandler(applicationManager, mockService)
	SetupRoutes(e, h)

	tests := []struct {
		name           string
		headerAppKey   string
		expectedStatus int
	}{
		{
			name:           "Valid Header AppKey",
			headerAppKey:   "test-app-key",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Header AppKey",
			headerAppKey:   "wrong-key",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Empty Header AppKey",
			headerAppKey:   "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Request Creation
			body := request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Test Message",
			}
			bodyBytes, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", bytes.NewReader(bodyBytes))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			if tt.headerAppKey != "" {
				req.Header.Set("X-App-Key", tt.headerAppKey)
			}
			rec := httptest.NewRecorder()

			// Execute
			e.ServeHTTP(rec, req)

			// Verify
			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

// TestV1API_Integration_ConcurrentRequests 동시 요청을 처리할 수 있는지 검증합니다.
func TestV1API_Integration_ConcurrentRequests(t *testing.T) {
	// Setup
	appConfig := createTestAppConfig()
	applicationManager := apiauth.NewAuthenticator(appConfig)
	e := echo.New()
	mockService := &mocks.MockNotificationSender{}
	h := handler.NewHandler(applicationManager, mockService)
	SetupRoutes(e, h)

	// Execute - 동시에 10개의 요청 전송
	const numRequests = 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			body := request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Concurrent Test",
			}
			bodyBytes, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications?app_key=test-app-key", bytes.NewReader(bodyBytes))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			done <- true
		}()
	}

	// Verify - 모든 요청이 완료될 때까지 대기
	for i := 0; i < numRequests; i++ {
		<-done
	}
}
