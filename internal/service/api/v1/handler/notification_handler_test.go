package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
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
	handler := NewHandler(mockService)

	return handler, mockService
}

// createTestRequest 테스트용 HTTP 요청을 생성합니다.
// 인증은 미들웨어에서 처리되므로, Context에 Application을 미리 설정합니다.
func createTestRequest(t *testing.T, method, url string, body interface{}, app *domain.Application) (*httptest.ResponseRecorder, echo.Context) {
	t.Helper()

	e := echo.New()

	var bodyBytes []byte
	if s, ok := body.(string); ok {
		bodyBytes = []byte(s)
	} else if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err, "Body marshaling failed")
		bodyBytes = b
	}

	req := httptest.NewRequest(method, url, strings.NewReader(string(bodyBytes)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Context에 인증된 Application 설정 (미들웨어가 이미 처리했다고 가정)
	if app != nil {
		c.Set(constants.ContextKeyApplication, app)
	}

	return rec, c
}

// =============================================================================
// PublishNotificationHandler Tests
// =============================================================================

// TestPublishNotificationHandler는 알림 게시 핸들러를 검증합니다.
//
// 검증 범위:
//   - 정상적인 알림 전송 (성공 응답)
//   - ErrorOccurred 필드 처리
//   - 필수 필드 누락 검증 (ApplicationID, Message)
//   - 메시지 길이 제한 검증
//   - JSON 바인딩 오류 처리
//   - 서비스 혼잡(503) 시 에러 처리
func TestPublishNotificationHandler(t *testing.T) {
	// 공통 테스트 데이터
	testApp := &domain.Application{
		ID:                "test-app",
		Title:             "Test App",
		DefaultNotifierID: "test-notifier",
	}

	tests := []struct {
		name              string
		reqBody           interface{}
		app               *domain.Application
		mockFail          bool
		expectedStatus    int
		verifyErrResponse func(*testing.T, response.ErrorResponse)
		verifyMock        func(*testing.T, *mocks.MockNotificationSender)
	}{
		// ---------------------------------------------------------------------
		// 성공 케이스
		// ---------------------------------------------------------------------
		{
			name: "성공: 정상적인 알림 전송",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Test Message",
				ErrorOccurred: false,
			},
			app:            testApp,
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *mocks.MockNotificationSender) {
				assert.True(t, m.NotifyCalled, "NotifyWithTitle이 호출되어야 합니다")
				assert.Equal(t, "test-notifier", m.LastNotifierID)
				assert.Equal(t, "Test App", m.LastTitle)
				assert.Equal(t, "Test Message", m.LastMessage)
				assert.False(t, m.LastErrorOccurred)
			},
		},
		{
			name: "성공: ErrorOccurred=true 설정",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Error Message",
				ErrorOccurred: true,
			},
			app:            testApp,
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *mocks.MockNotificationSender) {
				assert.True(t, m.LastErrorOccurred, "에러 발생 플래그가 전달되어야 합니다")
			},
		},

		// ---------------------------------------------------------------------
		// 입력 검증 실패
		// ---------------------------------------------------------------------
		{
			name: "실패: Application ID 누락",
			reqBody: request.NotificationRequest{
				ApplicationID: "",
				Message:       "Test Message",
			},
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "애플리케이션 ID")
				assert.Contains(t, errResp.Message, "필수")
			},
		},
		{
			name: "실패: Message 누락",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "",
			},
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "메시지")
				assert.Contains(t, errResp.Message, "필수")
			},
		},
		{
			name: "실패: Message 길이 초과 (4097자)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       strings.Repeat("a", 4097),
			},
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "메시지")
				assert.Contains(t, errResp.Message, "최대")
				assert.Contains(t, errResp.Message, "4096")
			},
		},

		// ---------------------------------------------------------------------
		// 바인딩 실패
		// ---------------------------------------------------------------------
		{
			name:           "실패: 잘못된 JSON 형식",
			reqBody:        "invalid-json",
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "요청 본문을 파싱할 수 없습니다")
			},
		},

		// ---------------------------------------------------------------------
		// 서비스 실패
		// ---------------------------------------------------------------------
		{
			name: "실패: 알림 서비스 혼잡 (503)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Fail Message",
			},
			app:            testApp,
			mockFail:       true,
			expectedStatus: http.StatusServiceUnavailable,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Contains(t, errResp.Message, "알림 서비스를 일시적으로 사용할 수 없습니다")
				assert.Contains(t, errResp.Message, "잠시 후 다시 시도해주세요")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			handler, mockService := setupTestHandler(t)
			mockService.Reset()
			mockService.ShouldFail = tt.mockFail

			rec, c := createTestRequest(t, http.MethodPost, "/", tt.reqBody, tt.app)

			// Execute
			err := handler.PublishNotificationHandler(c)

			// Verify
			if tt.expectedStatus == http.StatusOK {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				require.Error(t, err)
				httpErr, ok := err.(*echo.HTTPError)
				require.True(t, ok, "에러는 *echo.HTTPError 타입이어야 합니다")
				assert.Equal(t, tt.expectedStatus, httpErr.Code)

				if tt.verifyErrResponse != nil {
					errResp, ok := httpErr.Message.(response.ErrorResponse)
					require.True(t, ok, "에러 메시지는 response.ErrorResponse 타입이어야 합니다")
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

// TestHandler_log는 log() 헬퍼 함수가 올바른 엔트리를 반환하는지 검증합니다.
func TestHandler_log(t *testing.T) {
	// Setup
	handler, _ := setupTestHandler(t)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/notifications")

	// Execute
	logEntry := handler.log(c)

	// Verify
	assert.NotNil(t, logEntry, "log() 결과는 nil이 아니어야 합니다")
	// 참고: logrus.Entry 내부 필드를 직접 검증하기는 어렵지만, nil이 아님을 확인하는 것으로 충분합니다.
	// 실제 로깅 출력 검증은 통합 테스트나 별도의 로거 Mocking이 필요할 수 있습니다.
}
