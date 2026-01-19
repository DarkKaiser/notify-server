package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	apiauth "github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	"github.com/darkkaiser/notify-server/internal/service/notification"
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
					Title:             "테스트 애플리케이션",
					DefaultNotifierID: "test-notifier",
					AppKey:            "test-app-key",
				},
				{
					ID:                "another-app",
					Title:             "다른 애플리케이션",
					DefaultNotifierID: "another-notifier",
					AppKey:            "another-key",
				},
			},
		},
	}
}

// =============================================================================
// Integration Tests - Success Scenarios
// =============================================================================

// TestV1API_Success_Notification 유효한 알림 전송 요청이 성공하는지 검증합니다.
func TestV1API_Success_Notification(t *testing.T) {
	e, _, authenticator := setupIntegrationTest(t)

	tests := []struct {
		name           string
		appKeyLocation string // "header" or "query"
		body           request.NotificationRequest
	}{
		{
			name:           "Header 인증",
			appKeyLocation: "header",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "정상 메시지 (Header Auth)",
			},
		},
		{
			name:           "Query 인증",
			appKeyLocation: "query",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "정상 메시지 (Query Auth)",
			},
		},
		{
			name:           "ErrorOccurred 필드 포함",
			appKeyLocation: "header",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "에러 발생 알림",
				ErrorOccurred: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock 설정
			mockSender := &mocks.MockNotificationSender{ShouldFail: false}
			h := handler.New(mockSender)
			RegisterRoutes(e, h, authenticator)

			req := createJSONRequest(t, http.MethodPost, "/api/v1/notifications", tt.body)

			// 인증 키 설정
			testAppKey := "test-app-key"
			if tt.appKeyLocation == "header" {
				req.Header.Set("X-App-Key", testAppKey)
			} else {
				q := req.URL.Query()
				q.Add("app_key", testAppKey)
				req.URL.RawQuery = q.Encode()
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)

			var resp response.SuccessResponse
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, 0, resp.ResultCode)
		})
	}
}

// TestV1API_Success_LegacyEndpoint 레거시 엔드포인트(/api/v1/notice/message)의 동작과 Deprecated 헤더를 검증합니다.
func TestV1API_Success_LegacyEndpoint(t *testing.T) {
	e, _, authenticator := setupIntegrationTest(t)
	mockSender := &mocks.MockNotificationSender{}
	h := handler.New(mockSender)
	RegisterRoutes(e, h, authenticator)

	body := request.NotificationRequest{
		ApplicationID: "test-app",
		Message:       "레거시 요청 테스트",
	}
	req := createJSONRequest(t, http.MethodPost, "/api/v1/notice/message", body)
	req.Header.Set("X-App-Key", "test-app-key")

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Status OK 확인
	require.Equal(t, http.StatusOK, rec.Code)

	// Deprecated 헤더 검증
	assert.Contains(t, rec.Header().Get("Warning"), "299", "Warning 헤더에 299 코드가 포함되어야 함")
	assert.Equal(t, "true", rec.Header().Get("X-API-Deprecated"), "X-API-Deprecated 헤더가 true여야 함")
	assert.Equal(t, "/api/v1/notifications", rec.Header().Get("X-API-Deprecated-Replacement"), "대체 API 경로가 올바르지 않음")
}

// =============================================================================
// Integration Tests - Failure Scenarios
// =============================================================================

// TestV1API_Failure_Authentication 인증 실패 시나리오를 검증합니다.
func TestV1API_Failure_Authentication(t *testing.T) {
	e, _, authenticator := setupIntegrationTest(t)
	h := handler.New(&mocks.MockNotificationSender{})
	RegisterRoutes(e, h, authenticator)

	tests := []struct {
		name         string
		appKeyHeader string
		appID        string
		expectStatus int
	}{
		{"AppKey 누락", "", "test-app", http.StatusBadRequest},
		{"잘못된 AppKey", "invalid-key", "test-app", http.StatusUnauthorized},
		{"ApplicationID 불일치", "test-app-key", "another-app", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := request.NotificationRequest{
				ApplicationID: tt.appID,
				Message:       "Auth Test",
			}
			req := createJSONRequest(t, http.MethodPost, "/api/v1/notifications", body)
			if tt.appKeyHeader != "" {
				req.Header.Set("X-App-Key", tt.appKeyHeader)
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectStatus, rec.Code)
		})
	}
}

// TestV1API_Failure_Validation 요청 데이터 검증 및 Content-Type 검증 실패를 테스트합니다.
func TestV1API_Failure_Validation(t *testing.T) {
	e, _, authenticator := setupIntegrationTest(t)
	h := handler.New(&mocks.MockNotificationSender{})
	RegisterRoutes(e, h, authenticator)

	tests := []struct {
		name        string
		contentType string
		body        interface{}
		validAuth   bool
	}{
		{
			name:        "ApplicationID 누락",
			contentType: echo.MIMEApplicationJSON,
			body:        request.NotificationRequest{Message: "Msg Only"},
			validAuth:   true,
		},
		{
			name:        "Message 누락",
			contentType: echo.MIMEApplicationJSON,
			body:        request.NotificationRequest{ApplicationID: "test-app"},
			validAuth:   true,
		},
		{
			name:        "잘못된 JSON 형식",
			contentType: echo.MIMEApplicationJSON,
			body:        "INVALID_JSON_{{",
			validAuth:   true,
		},
		{
			name:        "Content-Type 불일치 (Text)",
			contentType: echo.MIMETextPlain,
			body:        "Plain Text",
			validAuth:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if str, ok := tt.body.(string); ok {
				req = httptest.NewRequest(http.MethodPost, "/api/v1/notifications", bytes.NewReader([]byte(str)))
			} else {
				jsonBytes, _ := json.Marshal(tt.body)
				req = httptest.NewRequest(http.MethodPost, "/api/v1/notifications", bytes.NewReader(jsonBytes))
			}

			if tt.contentType != "" {
				req.Header.Set(echo.HeaderContentType, tt.contentType)
			}
			if tt.validAuth {
				req.Header.Set("X-App-Key", "test-app-key")
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			// 검증 실패는 400 Bad Request
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

// TestV1API_Failure_MethodNotAllowed 지원하지 않는 메서드 요청 시 처리를 검증합니다.
func TestV1API_Failure_MethodNotAllowed(t *testing.T) {
	e, _, authenticator := setupIntegrationTest(t)
	h := handler.New(&mocks.MockNotificationSender{})
	RegisterRoutes(e, h, authenticator)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// TestV1API_Failure_InternalError 내부 로직(Sender) 실패 시 503 처리를 검증합니다.
func TestV1API_Failure_InternalError(t *testing.T) {
	e, _, authenticator := setupIntegrationTest(t)

	// Mock Sender 강제 실패 설정
	mockSender := &mocks.MockNotificationSender{
		ShouldFail: true,
		FailError:  notification.ErrServiceStopped,
	}
	h := handler.New(mockSender)
	RegisterRoutes(e, h, authenticator)

	body := request.NotificationRequest{
		ApplicationID: "test-app",
		Message:       "This should fail",
	}
	req := createJSONRequest(t, http.MethodPost, "/api/v1/notifications", body)
	req.Header.Set("X-App-Key", "test-app-key")

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// =============================================================================
// Helpers
// =============================================================================

func setupIntegrationTest(t *testing.T) (*echo.Echo, *config.AppConfig, *apiauth.Authenticator) {
	t.Helper()
	appConfig := createTestAppConfig()
	authenticator := apiauth.NewAuthenticator(appConfig)
	e := echo.New()
	return e, appConfig, authenticator
}

func createJSONRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	t.Helper()
	jsonBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(method, path, bytes.NewReader(jsonBytes))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	return req
}

// TestV1API_ConcurrentRequests 동시 요청 처리 능력을 검증합니다.
func TestV1API_ConcurrentRequests(t *testing.T) {
	// Setup
	appConfig := createTestAppConfig()
	authenticator := apiauth.NewAuthenticator(appConfig)
	e := echo.New()
	mockSender := &mocks.MockNotificationSender{}
	h := handler.New(mockSender)
	RegisterRoutes(e, h, authenticator)

	const numRequests = 20
	var wg sync.WaitGroup
	wg.Add(numRequests)

	var successCount int32

	// Execute
	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()

			body := request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Concurrent Test Message",
			}
			bodyBytes, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", bytes.NewReader(bodyBytes))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			req.Header.Set("X-App-Key", "test-app-key")
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else {
				t.Logf("Request failed with status: %d, body: %s", rec.Code, rec.Body.String())
			}
		}()
	}

	wg.Wait()

	// Verify
	assert.Equal(t, int32(numRequests), atomic.LoadInt32(&successCount), "모든 동시 요청이 성공해야 합니다")
}
