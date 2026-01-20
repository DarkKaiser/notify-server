package v1

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	apiauth "github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// =============================================================================
// Unit Tests: Router Wiring & Configuration
// =============================================================================

// TestRegisterRoutes_Wiring은 라우터가 올바르게 설정되었는지 검증합니다.
//
// 검증 범위:
//   - 엔드포인트 등록 여부 (POST /api/v1/notifications 등)
//   - 미들웨어 체인 동작 (인증, Content-Type, Deprecated)
//   - 핸들러 연결 여부 (Mock 호출 확인)
//   - 미지원 메서드 및 경로 처리 (405, 404)
func TestRegisterRoutes_Wiring(t *testing.T) {
	// Setup Dependencies
	e := echo.New()

	// Test Config & Authenticator (Self-contained)
	appConfig := &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			Applications: []config.ApplicationConfig{
				{ID: "test-app", Title: "Test App", DefaultNotifierID: "test-notifier", AppKey: "valid-key"},
			},
		},
	}
	auth := apiauth.NewAuthenticator(appConfig)
	mockSender := mocks.NewMockNotificationSender()
	h := handler.New(mockSender)

	// Register
	RegisterRoutes(e, h, auth)

	tests := []struct {
		name           string
		method         string
		path           string
		headers        map[string]string
		body           string
		expectedStatus int
		verify         func(*testing.T, *httptest.ResponseRecorder, *mocks.MockNotificationSender)
	}{
		// ---------------------------------------------------------------------
		// 1. 성공 케이스 (Wiring Success)
		// ---------------------------------------------------------------------
		{
			name:   "Success: Main Endpoint",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			headers: map[string]string{
				echo.HeaderContentType: echo.MIMEApplicationJSON,
				"X-App-Key":            "valid-key",
			},
			body:           `{"application_id":"test-app", "message":"hello"}`,
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, rec *httptest.ResponseRecorder, m *mocks.MockNotificationSender) {
				m.AssertCalled(t, "NotifyWithTitle", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:   "Success: Legacy Endpoint (Deprecated Headers)",
			method: http.MethodPost,
			path:   "/api/v1/notice/message",
			headers: map[string]string{
				echo.HeaderContentType: echo.MIMEApplicationJSON,
				"X-App-Key":            "valid-key",
			},
			body:           `{"application_id":"test-app", "message":"legacy"}`,
			expectedStatus: http.StatusOK,
			verify: func(t *testing.T, rec *httptest.ResponseRecorder, m *mocks.MockNotificationSender) {
				m.AssertCalled(t, "NotifyWithTitle", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				// Check Headers
				assert.Contains(t, rec.Header().Get("Warning"), "299", "Warning 헤더 299 코드 포함")
				assert.Equal(t, "true", rec.Header().Get("X-API-Deprecated"))
				assert.Equal(t, "/api/v1/notifications", rec.Header().Get("X-API-Deprecated-Replacement"))
			},
		},

		// ---------------------------------------------------------------------
		// 2. 미들웨어 동작 검증 (Middleware Wiring)
		// ---------------------------------------------------------------------
		{
			name:   "Failure: Missing Auth (Authentication Middleware)",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			headers: map[string]string{
				echo.HeaderContentType: echo.MIMEApplicationJSON,
			},
			body:           `{"application_id":"test-app", "message":"no-auth"}`,
			expectedStatus: http.StatusBadRequest, // AppKey 누락 -> 400
			verify: func(t *testing.T, rec *httptest.ResponseRecorder, m *mocks.MockNotificationSender) {
				m.AssertNotCalled(t, "NotifyWithTitle", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name:   "Failure: Invalid Content-Type (Binding/Middleware)",
			method: http.MethodPost,
			path:   "/api/v1/notifications",
			headers: map[string]string{
				echo.HeaderContentType: echo.MIMETextPlain, // Invalid
				"X-App-Key":            "valid-key",
			},
			body:           `raw-text`,
			expectedStatus: http.StatusBadRequest, // 400 (Middleware might skip if CL=0, then Handler Bind fails)
			verify: func(t *testing.T, rec *httptest.ResponseRecorder, m *mocks.MockNotificationSender) {
				m.AssertNotCalled(t, "NotifyWithTitle", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				// 415(Middleware) or 400(Bind) are both acceptable rejections
				if rec.Code == http.StatusUnsupportedMediaType {
					assert.Contains(t, rec.Body.String(), "Content-Type")
				} else {
					assert.Contains(t, rec.Body.String(), "JSON") // Bind Failure
				}
			},
		},

		// ---------------------------------------------------------------------
		// 3. 라우팅 검증 (Routing)
		// ---------------------------------------------------------------------
		{
			name:           "Failure: Method Not Allowed",
			method:         http.MethodGet,
			path:           "/api/v1/notifications",
			headers:        nil,
			body:           "",
			expectedStatus: http.StatusMethodNotAllowed,
			verify:         nil,
		},
		{
			name:           "Failure: Not Found",
			method:         http.MethodPost,
			path:           "/api/v1/unknown",
			headers:        nil,
			body:           "",
			expectedStatus: http.StatusNotFound,
			verify:         nil,
		},
	}

	for _, tt := range tests {
		tt := tt // Capture Loop Variable
		t.Run(tt.name, func(t *testing.T) {
			// Reset Mock State
			mockSender.Calls = nil
			mockSender.ExpectedCalls = nil
			// Setup default expectation for Success case (allowed to be called)
			mockSender.On("NotifyWithTitle", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

			bodyBytes := []byte(tt.body)
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(bodyBytes))
			req.ContentLength = int64(len(bodyBytes))
			req.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			// Execute
			e.ServeHTTP(rec, req)

			// Verify Status
			assert.Equal(t, tt.expectedStatus, rec.Code, "Expected Status: %d, Got: %d, Body: %s", tt.expectedStatus, rec.Code, rec.Body.String())

			// Custom Verify
			if tt.verify != nil {
				tt.verify(t, rec, mockSender)
			}

			// Mock Assertion
			mockSender.AssertExpectations(t)
		})
	}
}

// TestRegisterRoutes_Validation은 필수 의존성이 주입되지 않았을 때의 패닉 상황을 검증합니다.
func TestRegisterRoutes_Validation(t *testing.T) {
	e := echo.New()
	mockSender := mocks.NewMockNotificationSender()
	h := handler.New(mockSender)
	auth := apiauth.NewAuthenticator(&config.AppConfig{})

	tests := []struct {
		name          string
		echo          *echo.Echo
		handler       *handler.Handler
		authenticator *apiauth.Authenticator
		expectPanic   string
	}{
		{
			name:          "Echo 누락",
			echo:          nil,
			handler:       h,
			authenticator: auth,
			expectPanic:   "Echo 인스턴스는 필수입니다",
		},
		{
			name:          "Handler 누락",
			echo:          e,
			handler:       nil,
			authenticator: auth,
			expectPanic:   "Handler는 필수입니다",
		},
		{
			name:          "Authenticator 누락",
			echo:          e,
			handler:       h,
			authenticator: nil,
			expectPanic:   "Authenticator는 필수입니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			assert.PanicsWithValue(t, tt.expectPanic, func() {
				RegisterRoutes(tt.echo, tt.handler, tt.authenticator)
			})
		})
	}
}
