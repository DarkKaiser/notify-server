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

// TestV1API_Integration v1 API의 전체 동작 흐름을 검증하는 통합 테스트입니다.
func TestV1API_Integration(t *testing.T) {
	// 공통 설정
	appConfig := createTestAppConfig()
	authenticator := apiauth.NewAuthenticator(appConfig)

	// 테스트 케이스 정의
	tests := []struct {
		name           string
		method         string
		path           string
		contentType    string
		appKeyQuery    string
		appKeyHeader   string
		body           interface{}
		shouldFail     bool // Mock Server가 실패하도록 설정할지 여부
		expectedStatus int
		verifyResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		// ---------------------------------------------------------------------
		// 정상 케이스
		// ---------------------------------------------------------------------
		{
			name:         "성공: 유효한 요청 (Header Auth)",
			method:       http.MethodPost,
			path:         "/api/v1/notifications",
			contentType:  echo.MIMEApplicationJSON,
			appKeyHeader: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "정상 메시지입니다.",
			},
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var resp response.SuccessResponse
				err := json.Unmarshal(rec.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, 0, resp.ResultCode)
			},
		},
		{
			name:        "성공: 유효한 요청 (Query Auth)",
			method:      http.MethodPost,
			path:        "/api/v1/notifications",
			contentType: echo.MIMEApplicationJSON,
			appKeyQuery: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "쿼리 파라미터 인증 테스트",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:         "성공: 레거시 엔드포인트 요청 (Deprecated 헤더 확인)",
			method:       http.MethodPost,
			path:         "/api/v1/notice/message",
			contentType:  echo.MIMEApplicationJSON,
			appKeyHeader: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "레거시 요청",
			},
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				// Deprecated 관련 헤더 검증
				assert.Contains(t, rec.Header().Get("Warning"), "299")
				assert.Equal(t, "true", rec.Header().Get("X-API-Deprecated"))
				assert.Equal(t, "/api/v1/notifications", rec.Header().Get("X-API-Deprecated-Replacement"))
			},
		},
		{
			name:         "성공: ErrorOccurred 필드 포함",
			method:       http.MethodPost,
			path:         "/api/v1/notifications",
			contentType:  echo.MIMEApplicationJSON,
			appKeyHeader: "test-app-key",
			body: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "에러 발생 상황 알림",
				ErrorOccurred: true,
			},
			expectedStatus: http.StatusOK,
		},

		// ---------------------------------------------------------------------
		// 인증 실패 케이스
		// ---------------------------------------------------------------------
		{
			name:           "실패: AppKey 누락",
			method:         http.MethodPost,
			path:           "/api/v1/notifications",
			contentType:    echo.MIMEApplicationJSON,
			body:           request.NotificationRequest{ApplicationID: "test-app", Message: "Test"},
			expectedStatus: http.StatusBadRequest, // 클라이언트 에러 (키 누락)
		},
		{
			name:           "실패: 잘못된 AppKey",
			method:         http.MethodPost,
			path:           "/api/v1/notifications",
			contentType:    echo.MIMEApplicationJSON,
			appKeyHeader:   "invalid-key",
			body:           request.NotificationRequest{ApplicationID: "test-app", Message: "Test"},
			expectedStatus: http.StatusUnauthorized, // 인증 실패
		},
		{
			name:           "실패: 잘못된 ApplicationID (AppKey와 불일치)",
			method:         http.MethodPost,
			path:           "/api/v1/notifications",
			contentType:    echo.MIMEApplicationJSON,
			appKeyHeader:   "test-app-key",                                                             // test-app용 키
			body:           request.NotificationRequest{ApplicationID: "another-app", Message: "Test"}, // another-app 요청
			expectedStatus: http.StatusUnauthorized,
		},

		// ---------------------------------------------------------------------
		// 요청 검증 실패 케이스
		// ---------------------------------------------------------------------
		{
			name:           "실패: ApplicationID 필드 누락",
			method:         http.MethodPost,
			path:           "/api/v1/notifications",
			contentType:    echo.MIMEApplicationJSON,
			appKeyHeader:   "test-app-key",
			body:           request.NotificationRequest{Message: "Test"},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "실패: Message 필드 누락",
			method:         http.MethodPost,
			path:           "/api/v1/notifications",
			contentType:    echo.MIMEApplicationJSON,
			appKeyHeader:   "test-app-key",
			body:           request.NotificationRequest{ApplicationID: "test-app", Message: ""},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "실패: 잘못된 JSON 형식",
			method:         http.MethodPost,
			path:           "/api/v1/notifications",
			contentType:    echo.MIMEApplicationJSON,
			appKeyHeader:   "test-app-key",
			body:           "INVALID JSON...",
			expectedStatus: http.StatusBadRequest,
		},
		// ---------------------------------------------------------------------
		// 기타 실패 케이스
		// ---------------------------------------------------------------------
		{
			name:           "실패: Content-Type 불일치",
			method:         http.MethodPost,
			path:           "/api/v1/notifications",
			contentType:    echo.MIMETextPlain,
			appKeyHeader:   "test-app-key",
			body:           "plain text body",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "실패: 지원하지 않는 HTTP 메서드",
			method:         http.MethodGet,
			path:           "/api/v1/notifications",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "실패: 내부 서비스 오류 (503)",
			method:         http.MethodPost,
			path:           "/api/v1/notifications",
			contentType:    echo.MIMEApplicationJSON,
			appKeyHeader:   "test-app-key",
			body:           request.NotificationRequest{ApplicationID: "test-app", Message: "Fail"},
			shouldFail:     true, // Mock Sender 실패 설정
			expectedStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Per-test Setup
			e := echo.New()
			mockSender := &mocks.MockNotificationSender{ShouldFail: tt.shouldFail}
			h := handler.NewHandler(mockSender)
			SetupRoutes(e, h, authenticator)

			// Request Body 생성
			var bodyBytes []byte
			if str, ok := tt.body.(string); ok {
				bodyBytes = []byte(str)
			} else {
				jsonBytes, _ := json.Marshal(tt.body)
				bodyBytes = jsonBytes
			}

			// Request 생성
			reqPath := tt.path
			if tt.appKeyQuery != "" {
				reqPath += "?app_key=" + tt.appKeyQuery
			}
			req := httptest.NewRequest(tt.method, reqPath, bytes.NewReader(bodyBytes))

			// Header 설정
			if tt.contentType != "" {
				req.Header.Set(echo.HeaderContentType, tt.contentType)
			}
			if tt.appKeyHeader != "" {
				req.Header.Set("X-App-Key", tt.appKeyHeader)
			}

			rec := httptest.NewRecorder()

			// Execute
			e.ServeHTTP(rec, req)

			// Verify
			assert.Equal(t, tt.expectedStatus, rec.Code, "HTTP 상태 코드가 기대값과 다릅니다")

			if tt.verifyResponse != nil {
				tt.verifyResponse(t, rec)
			}
		})
	}
}

// TestV1API_ConcurrentRequests 동시 요청 처리 능력을 검증합니다.
func TestV1API_ConcurrentRequests(t *testing.T) {
	// Setup
	appConfig := createTestAppConfig()
	authenticator := apiauth.NewAuthenticator(appConfig)
	e := echo.New()
	mockSender := &mocks.MockNotificationSender{}
	h := handler.NewHandler(mockSender)
	SetupRoutes(e, h, authenticator)

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
