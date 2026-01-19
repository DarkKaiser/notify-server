package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification"
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
	handler := New(mockService)

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
//   - 메시지 길이 제한 검증 (최소 1자, 최대 4096자)
//   - JSON 바인딩 오류 처리
//   - 서비스 혼잡(503) 시 에러 처리
func TestPublishNotificationHandler(t *testing.T) {
	// 공통 테스트 데이터
	testApp := &domain.Application{
		ID:                "test-app",
		Title:             "Test App",
		DefaultNotifierID: contract.NotifierID("test-notifier"),
	}

	tests := []struct {
		name              string
		reqBody           interface{}
		app               *domain.Application
		mockFail          bool
		failError         error // 실패 시 반환할 에러
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
				assert.Equal(t, contract.NotifierID("test-notifier"), m.LastNotifierID)
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
		{
			name: "성공: Message 최소 길이 (1자)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "a",
				ErrorOccurred: false,
			},
			app:            testApp,
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *mocks.MockNotificationSender) {
				assert.True(t, m.NotifyCalled)
				assert.Equal(t, 1, len(m.LastMessage))
			},
		},
		{
			name: "성공: Message 최대 길이 (4096자)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       strings.Repeat("a", 4096),
			},
			app:            testApp,
			expectedStatus: http.StatusOK,
			verifyMock: func(t *testing.T, m *mocks.MockNotificationSender) {
				assert.True(t, m.NotifyCalled)
				assert.Equal(t, 4096, len(m.LastMessage))
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
				// 검증 라이브러리 메시지는 상수가 아니므로 부분 일치 확인
				assert.Contains(t, errResp.Message, "애플리케이션 ID")
				assert.Contains(t, errResp.Message, "필수")
			},
		},
		{
			name: "실패: Application ID 불일치 (인증 정보와 다름)",
			reqBody: request.NotificationRequest{
				ApplicationID: "diff-app",
				Message:       "Test Mismatch",
			},
			app:            testApp, // ID: "test-app"
			expectedStatus: http.StatusBadRequest,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				// ErrMsgBadRequestAppIdMismatch = "요청 본문의 application_id와 인증된 애플리케이션이 일치하지 않습니다 (요청: %s, 인증: %s)"
				expectedMsg := fmt.Sprintf(constants.ErrMsgBadRequestAppIdMismatch, "diff-app", "test-app")
				assert.Equal(t, expectedMsg, errResp.Message)
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
				assert.Contains(t, errResp.Message, constants.ErrMsgBadRequestInvalidBody)
			},
		},

		// ---------------------------------------------------------------------
		// 서비스 실패
		// ---------------------------------------------------------------------
		{
			name: "실패: 알림 서비스 중지 (503)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Fail Message",
			},
			app:            testApp,
			mockFail:       true,
			failError:      notification.ErrServiceStopped,
			expectedStatus: http.StatusServiceUnavailable,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Equal(t, constants.ErrMsgServiceUnavailable, errResp.Message)
			},
		},
		{
			name: "실패: 알림 채널 미등록 (404)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Fail Message",
			},
			app:            testApp,
			mockFail:       true,
			failError:      notification.ErrNotFoundNotifier,
			expectedStatus: http.StatusNotFound,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Equal(t, constants.ErrMsgNotFoundNotifier, errResp.Message)
			},
		},
		{
			name: "실패: 기타 내부 오류 (500)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Fail Message",
			},
			app:            testApp,
			mockFail:       true,
			failError:      errors.New("generic error"), // 임의의 에러
			expectedStatus: http.StatusInternalServerError,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Equal(t, constants.ErrMsgInternalServerInterrupted, errResp.Message)
			},
		},
		{
			name: "실패: 큐 가득 참 (503)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Queue Full Message",
			},
			app:            testApp,
			mockFail:       true,
			failError:      apperrors.New(apperrors.Unavailable, "Queue Full"), // Unavailable 타입 에러
			expectedStatus: http.StatusServiceUnavailable,
			verifyErrResponse: func(t *testing.T, errResp response.ErrorResponse) {
				assert.Equal(t, constants.ErrMsgServiceUnavailableOverloaded, errResp.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			handler, mockService := setupTestHandler(t)
			mockService.Reset()
			mockService.ShouldFail = tt.mockFail
			mockService.FailError = tt.failError

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

// Note: TestHandler_log removed as it tests internal implementation details.

// TestPublishNotificationHandler_Panic_MissingContext는 Context에 Application이 없을 때 패닉이 발생하는지 검증합니다.
// 이 테스트는 미들웨어(RequireAuthentication)와 핸들러 간의 계약(Contract)을 보장합니다.
func TestPublishNotificationHandler_Panic_MissingContext(t *testing.T) {
	// Setup
	handler, _ := setupTestHandler(t)
	e := echo.New()

	// 유효한 요청 데이터지만, Context에 Application 설정 누락
	reqBody := `{"application_id":"test-app","message":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Verify Panic
	assert.Panics(t, func() {
		// Execute (Should Panic)
		_ = handler.PublishNotificationHandler(c)
	}, "Context에 Application 정보가 없으면 패닉이 발생해야 합니다")
}
