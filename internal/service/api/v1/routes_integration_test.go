package v1

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	apiauth "github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	"github.com/darkkaiser/notify-server/internal/service/notification"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Integration Tests: Success Scenarios
// =============================================================================

func TestV1API_Success(t *testing.T) {
	_, _, authenticator := setupIntegrationTest(t)

	// 공통 테스트 데이터
	validBody := request.NotificationRequest{
		ApplicationID: "test-app",
		Message:       "Integration Test Message",
	}

	tests := []struct {
		name           string
		path           string
		appKeyLocation string // "header" or "query"
		body           request.NotificationRequest
		verifyResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "Success: Standard Notification (Header Auth)",
			path:           "/api/v1/notifications",
			appKeyLocation: "header",
			body:           validBody,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, rec.Code)
				var resp response.SuccessResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, 0, resp.ResultCode)
			},
		},
		{
			name:           "Success: Standard Notification (Query Auth)",
			path:           "/api/v1/notifications",
			appKeyLocation: "query",
			body:           validBody,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, 0, getResultCode(t, rec.Body.Bytes()))
			},
		},
		{
			name:           "Success: ErrorOccurred Flag",
			path:           "/api/v1/notifications",
			appKeyLocation: "header",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Error Notification",
				ErrorOccurred: true,
			},
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, rec.Code)
			},
		},
		{
			name:           "Success: Legacy Endpoint (Deprecated Check)",
			path:           "/api/v1/notice/message",
			appKeyLocation: "header",
			body:           validBody,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, rec.Code)
				// Deprecated Headers Verification
				assert.Contains(t, rec.Header().Get("Warning"), "299")
				assert.Equal(t, "true", rec.Header().Get("X-API-Deprecated"))
				assert.Equal(t, "/api/v1/notifications", rec.Header().Get("X-API-Deprecated-Replacement"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Mock
			mockSender := mocks.NewMockNotificationSender()
			// Default expectation for success
			mockSender.On("NotifyWithTitle", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			h := handler.New(mockSender)

			// Register Routes (New Router per test to ensure clean state)
			// Note: In a real integration test, we might reuse the router, but here strict isolation is safer.
			router := echo.New()
			RegisterRoutes(router, h, authenticator)

			req := createJSONRequest(t, http.MethodPost, tt.path, tt.body)

			// Apply Auth
			if tt.appKeyLocation == "header" {
				req.Header.Set("X-App-Key", "test-app-key")
			} else {
				q := req.URL.Query()
				q.Add("app_key", "test-app-key")
				req.URL.RawQuery = q.Encode()
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			tt.verifyResponse(t, rec)
		})
	}
}

// =============================================================================
// Integration Tests: Failure Scenarios
// =============================================================================

func TestV1API_Failures(t *testing.T) {
	_, _, authenticator := setupIntegrationTest(t)

	// Helper to extract error message from an error object (Echo HTTPError wrapper)
	getExpectedErrMsg := func(targetErr error) string {
		if httpErr, ok := targetErr.(*echo.HTTPError); ok {
			if resp, ok := httpErr.Message.(response.ErrorResponse); ok {
				return resp.Message
			}
			return fmt.Sprint(httpErr.Message)
		}
		return targetErr.Error()
	}

	// Helper to simulate Auth package errors manually since we can't import internal/service/api/auth/errors.go directly if it's internal (it is accessible here as v1 is same level? No, auth is sibling).
	// But we can check literal strings as we did for others or use helpers if available.
	// Since we can't easily reach into api/auth/errors.go's private NewErr... if they are not exported?
	// Wait, auth.NewErrInvalidAppKey IS exported. We need to import "github.com/darkkaiser/notify-server/internal/service/api/auth" which is already imported as "apiauth".
	// But `apiauth` is aliased.

	tests := []struct {
		name           string
		appKey         string
		appIDHeader    string      // For testing AppID mismatch where Auth passes but Body differs
		reqBody        interface{} // string or struct
		setupMock      func(*mocks.MockNotificationSender)
		expectedStatus int
		expectedErrMsg string
		verifyDetails  string // Substring to check for dynamic validation errors
	}{
		// 1. Authentication Failures
		{
			name:           "Failure: Missing AppKey",
			appKey:         "",
			reqBody:        request.NotificationRequest{ApplicationID: "test-app", Message: "fail"},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "app_key는 필수입니다 (X-App-Key 헤더 또는 app_key 쿼리 파라미터)",
		},
		{
			name:           "Failure: Invalid AppKey",
			appKey:         "invalid-key",
			reqBody:        request.NotificationRequest{ApplicationID: "test-app", Message: "fail"},
			expectedStatus: http.StatusUnauthorized,
			expectedErrMsg: "app_key가 유효하지 않습니다 (application_id: test-app)", // auth.NewErrInvalidAppKey("test-app")
		},
		{
			name:           "Failure: AppID Mismatch (Auth vs Body)",
			appKey:         "another-key",                                                           // Key for "another-app"
			appIDHeader:    "another-app",                                                           // Auth succeeds for "another-app"
			reqBody:        request.NotificationRequest{ApplicationID: "test-app", Message: "fail"}, // Body requests "test-app"
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: getExpectedErrMsg(handler.NewErrAppIDMismatch("test-app", "another-app")),
		},

		// 2. Validation & Binding Failures
		{
			name:           "Failure: Invalid JSON Body",
			appKey:         "test-app-key",
			reqBody:        "INVALID_JSON_{{",
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "잘못된 JSON 형식입니다",
		},
		{
			name:           "Failure: Validation (Missing Message)",
			appKey:         "test-app-key",
			reqBody:        request.NotificationRequest{ApplicationID: "test-app", Message: ""},
			expectedStatus: http.StatusBadRequest,
			verifyDetails:  "메시지는 필수입니다", // Localized validation message
		},

		// 3. Service Level Failures
		{
			name:    "Failure: Service Stopped (503)",
			appKey:  "test-app-key",
			reqBody: request.NotificationRequest{ApplicationID: "test-app", Message: "fail"},
			setupMock: func(m *mocks.MockNotificationSender) {
				m.On("NotifyWithTitle", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(notification.ErrServiceStopped)
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedErrMsg: getExpectedErrMsg(handler.ErrServiceStopped),
		},
		{
			name:    "Failure: Notifier Not Found (404)",
			appKey:  "test-app-key",
			reqBody: request.NotificationRequest{ApplicationID: "test-app", Message: "fail"},
			setupMock: func(m *mocks.MockNotificationSender) {
				m.On("NotifyWithTitle", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(notification.ErrNotifierNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedErrMsg: getExpectedErrMsg(handler.ErrNotifierNotFound),
		},
		{
			name:    "Failure: Service Overloaded (503)",
			appKey:  "test-app-key",
			reqBody: request.NotificationRequest{ApplicationID: "test-app", Message: "fail"},
			setupMock: func(m *mocks.MockNotificationSender) {
				m.On("NotifyWithTitle", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(apperrors.New(apperrors.Unavailable, "overload"))
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedErrMsg: getExpectedErrMsg(handler.ErrServiceOverloaded), // Handler maps Unavailable to Overloaded
		},
		{
			name:    "Failure: Internal Error (500)",
			appKey:  "test-app-key",
			reqBody: request.NotificationRequest{ApplicationID: "test-app", Message: "fail"},
			setupMock: func(m *mocks.MockNotificationSender) {
				m.On("NotifyWithTitle", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("unknown error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedErrMsg: getExpectedErrMsg(handler.ErrServiceInterrupted), // Handler maps unknown to Interrupted/Internal
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock Setup
			mockSender := mocks.NewMockNotificationSender()
			if tt.setupMock != nil {
				tt.setupMock(mockSender)
			}
			h := handler.New(mockSender)

			router := echo.New()
			RegisterRoutes(router, h, authenticator)

			// Create Request
			req := createJSONRequest(t, http.MethodPost, "/api/v1/notifications", tt.reqBody)
			if tt.appKey != "" {
				req.Header.Set("X-App-Key", tt.appKey)
			} else {
				req.Header.Del("X-App-Key") // Explicitly remove if empty
			}
			if tt.appIDHeader != "" {
				req.Header.Set("X-Application-Id", tt.appIDHeader)
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			// Verify Status
			assert.Equal(t, tt.expectedStatus, rec.Code)

			// Verify Error Message
			if tt.expectedErrMsg != "" {
				var resp response.ErrorResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err, "Response body should be JSON error response: "+rec.Body.String())
				assert.Equal(t, tt.expectedErrMsg, resp.Message)
			} else if tt.verifyDetails != "" {
				var resp response.ErrorResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Contains(t, resp.Message, tt.verifyDetails)
			}
		})
	}
}

// =============================================================================
// Concurrency Test
// =============================================================================

// TestV1API_ConcurrentRequests 동시 요청 처리 능력을 검증합니다.
func TestV1API_ConcurrentRequests(t *testing.T) {
	e, _, authenticator := setupIntegrationTest(t)
	mockSender := mocks.NewMockNotificationSender()
	// Allow calls
	mockSender.On("NotifyWithTitle", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	h := handler.New(mockSender)
	RegisterRoutes(e, h, authenticator)

	const numRequests = 20
	var wg sync.WaitGroup
	wg.Add(numRequests)
	var successCount int32

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			body := request.NotificationRequest{ApplicationID: "test-app", Message: "Concurrent"}
			req := createJSONRequest(t, http.MethodPost, "/api/v1/notifications", body)
			req.Header.Set("X-App-Key", "test-app-key")
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(numRequests), atomic.LoadInt32(&successCount), "모든 동시 요청이 성공해야 합니다")
}

// =============================================================================
// Helpers
// =============================================================================

func setupIntegrationTest(t *testing.T) (*echo.Echo, *config.AppConfig, *apiauth.Authenticator) {
	t.Helper()
	appConfig := createTestAppConfig()
	authenticator := apiauth.NewAuthenticator(appConfig.NotifyAPI.Applications)
	e := echo.New()
	return e, appConfig, authenticator
}

// createTestAppConfig 테스트용 애플리케이션 설정을 생성합니다.
func createTestAppConfig() *config.AppConfig {
	return &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			Applications: []config.ApplicationConfig{
				{ID: "test-app", Title: "Test App", DefaultNotifierID: "test-notifier", AppKey: "test-app-key"},
				{ID: "another-app", Title: "Other App", DefaultNotifierID: "another-notifier", AppKey: "another-key"},
			},
		},
	}
}

func createJSONRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	t.Helper()
	var bodyBytes []byte
	var err error

	if str, ok := body.(string); ok {
		bodyBytes = []byte(str)
	} else {
		bodyBytes, err = json.Marshal(body)
		require.NoError(t, err)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	// Content-Length 설정 (미들웨어 패스용)
	req.ContentLength = int64(len(bodyBytes))
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))

	return req
}

func getResultCode(t *testing.T, body []byte) int {
	t.Helper()
	var resp response.SuccessResponse
	err := json.Unmarshal(body, &resp)
	require.NoError(t, err)
	return resp.ResultCode
}
