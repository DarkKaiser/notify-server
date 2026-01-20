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
	"github.com/darkkaiser/notify-server/internal/service/api/auth"
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
		auth.SetApplication(c, app)
	}

	return rec, c
}

// =============================================================================
// PublishNotificationHandler Tests
// =============================================================================

// TestPublishNotificationHandler는 알림 게시 핸들러를 전문가 수준으로 검증합니다.
//
// 검증 범위:
//   - 정상적인 알림 전송 (성공 응답)
//   - 다양한 입력 검증 실패 (누락, 길이 초과, 불일치)
//   - JSON 바인딩 및 파싱 오류
//   - 서비스 계층 에러 매핑 (503, 404, 500)
//   - Context 누락 식별 (Panic)
func TestPublishNotificationHandler(t *testing.T) {
	// 공통 테스트 데이터
	testApp := &domain.Application{
		ID:                "test-app",
		Title:             "Test App",
		DefaultNotifierID: contract.NotifierID("test-notifier"),
	}

	tests := []struct {
		name           string
		reqBody        interface{}
		app            *domain.Application
		mockFail       bool
		failError      error // Mock 서비스가 반환할 에러
		expectedStatus int
		expectedErr    error  // 예상되는 에러 (없으면 nil)
		expectedErrMsg string // 동적 에러 메시지 검증용 (포함 여부 확인)
		verifyMock     func(*testing.T, *mocks.MockNotificationSender)
	}{
		// ---------------------------------------------------------------------
		// 1. 성공 케이스 (Success)
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
		// 2. 입력 검증 실패 (Validation Failure)
		// ---------------------------------------------------------------------
		{
			name: "실패: Application ID 누락",
			reqBody: request.NotificationRequest{
				ApplicationID: "",
				Message:       "Test Message",
			},
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "애플리케이션 ID는 필수입니다",
		},
		{
			name: "실패: Application ID 불일치 (보안 검증)",
			reqBody: request.NotificationRequest{
				ApplicationID: "diff-app",
				Message:       "Test Mismatch",
			},
			app:            testApp, // ID: "test-app"
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "요청 본문의 application_id와 인증된 애플리케이션이 일치하지 않습니다", // NewErrAppIDMismatch
		},
		{
			name: "실패: Message 누락",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "",
			},
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "메시지는 필수입니다",
		},
		{
			name: "실패: Message 길이 초과 (4097자)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       strings.Repeat("a", 4097),
			},
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "메시지는 최대 4096자까지 입력 가능합니다",
		},

		// ---------------------------------------------------------------------
		// 3. 바인딩 실패 (Binding Failure)
		// ---------------------------------------------------------------------
		{
			name:           "실패: 잘못된 JSON 형식",
			reqBody:        "invalid-json",
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			expectedErr:    ErrInvalidBody,
		},
		{
			name:           "실패: JSON 필드 타입 불일치 (error_occurred 문자열 전달)",
			reqBody:        `{"application_id":"test-app","message":"msg","error_occurred":"not-boolean"}`, // 문자열 직접 주입
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			expectedErr:    ErrInvalidBody,
		},

		// ---------------------------------------------------------------------
		// 4. 서비스 실패 (Service Failure)
		// ---------------------------------------------------------------------
		{
			name: "실패: 알림 서비스 중지 (503 ServiceStopped)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Fail Message",
			},
			app:            testApp,
			mockFail:       true,
			failError:      notification.ErrServiceStopped,
			expectedStatus: http.StatusServiceUnavailable,
			expectedErr:    ErrServiceStopped,
		},
		{
			name: "실패: 알림 채널 미등록 (404 NotifierNotFound)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Fail Message",
			},
			app:            testApp,
			mockFail:       true,
			failError:      notification.ErrNotifierNotFound,
			expectedStatus: http.StatusNotFound,
			expectedErr:    ErrNotifierNotFound,
		},
		{
			name: "실패: 큐 가득 참 (503 ServiceOverloaded)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Queue Full",
			},
			app:            testApp,
			mockFail:       true,
			failError:      apperrors.New(apperrors.Unavailable, "Queue Full"), // Unavailable 타입 에러 시뮬레이션
			expectedStatus: http.StatusServiceUnavailable,
			expectedErr:    ErrServiceOverloaded,
		},
		{
			name: "실패: 기타 내부 오류 (500 ServiceInterrupted)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "Generic Error",
			},
			app:            testApp,
			mockFail:       true,
			failError:      errors.New("generic error"), // 일반 에러
			expectedStatus: http.StatusInternalServerError,
			expectedErr:    ErrServiceInterrupted,
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

				// 1. 에러 변수 매칭 검증 (assert.ErrorIs 사용)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr, "예상된 에러 변수와 일치하지 않습니다")
				}

				// 2. HTTP 에러 상태 코드 검증
				var httpErr *echo.HTTPError
				if assert.ErrorAs(t, err, &httpErr) {
					assert.Equal(t, tt.expectedStatus, httpErr.Code)

					// 3. 에러 메시지 검증 (동적 메시지인 경우)
					if tt.expectedErrMsg != "" {
						errResp, ok := httpErr.Message.(response.ErrorResponse)
						// Echo 핸들러에서 리턴하는 에러는 httputil로 생성되어 Message가 ErrorResponse 구조체일 가능성이 큼
						// 하지만 httputil 구조에 따라 string일 수도 있으니 유의. 현재 코드 베이스는 ErrorResponse로 추정됨.
						if ok {
							assert.Contains(t, errResp.Message, tt.expectedErrMsg)
						} else {
							// 만약 단순 string인 경우
							assert.Contains(t, fmt.Sprint(httpErr.Message), tt.expectedErrMsg)
						}
					}
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
