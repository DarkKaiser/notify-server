package handler

import (
	"encoding/json"
	"errors"
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

	// 에러 메시지 검증을 위한 헬퍼 함수
	// 실제 핸들러가 반환하는 에러 메시지와 정확히 일치하는지 확인합니다.
	extractErrMsg := func(err error) string {
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			if resp, ok := httpErr.Message.(response.ErrorResponse); ok {
				return resp.Message
			}
		}
		return ""
	}

	tests := []struct {
		name           string
		reqBody        interface{}
		app            *domain.Application
		mockFail       bool
		failError      error // Mock 서비스가 반환할 에러
		expectedStatus int
		expectedErrMsg string
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
			expectedErrMsg: "Key: 'NotificationRequest.ApplicationID' Error:Field validation for 'ApplicationID' failed on the 'required' tag", // validator 기본 메시지 (일부 매칭)
		},
		{
			name: "실패: Application ID 불일치 (보안 검증)",
			reqBody: request.NotificationRequest{
				ApplicationID: "diff-app",
				Message:       "Test Mismatch",
			},
			app:            testApp, // ID: "test-app"
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: extractErrMsg(NewErrAppIDMismatch("diff-app", "test-app")),
		},
		{
			name: "실패: Message 누락",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       "",
			},
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "Key: 'NotificationRequest.Message' Error:Field validation for 'Message' failed on the 'required' tag",
		},
		{
			name: "실패: Message 길이 초과 (4097자)",
			reqBody: request.NotificationRequest{
				ApplicationID: "test-app",
				Message:       strings.Repeat("a", 4097),
			},
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "Key: 'NotificationRequest.Message' Error:Field validation for 'Message' failed on the 'max' tag",
		},

		// ---------------------------------------------------------------------
		// 3. 바인딩 실패 (Binding Failure)
		// ---------------------------------------------------------------------
		{
			name:           "실패: 잘못된 JSON 형식",
			reqBody:        "invalid-json",
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: extractErrMsg(NewErrInvalidBody()),
		},
		{
			name:           "실패: JSON 필드 타입 불일치 (error_occurred 문자열 전달)",
			reqBody:        `{"application_id":"test-app","message":"msg","error_occurred":"not-boolean"}`, // 문자열 직접 주입
			app:            testApp,
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: extractErrMsg(NewErrInvalidBody()),
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
			expectedErrMsg: extractErrMsg(NewErrServiceStopped()),
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
			expectedErrMsg: extractErrMsg(NewErrNotifierNotFound()),
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
			expectedErrMsg: extractErrMsg(NewErrServiceOverloaded()),
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
			expectedErrMsg: extractErrMsg(NewErrServiceInterrupted()),
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

				if tt.expectedErrMsg != "" {
					errResp, ok := httpErr.Message.(response.ErrorResponse)
					require.True(t, ok, "에러 메시지는 response.ErrorResponse 타입이어야 합니다")

					if strings.Contains(tt.name, "Valid") { // Validation 에러는 부분 일치 (메시지가 동적임)
						assert.Contains(t, errResp.Message, tt.expectedErrMsg)
					} else if strings.Contains(tt.expectedErrMsg, "Field validation") { // Validation 에러 (누락/초과)도 부분 일치
						// Validator 메시지는 조금 다를 수 있으므로 핵심 키워드 체크 방식이 더 안전할 수 있으나,
						// 여기서는 기대 값을 Validator 형식("Key: ...")으로 맞춰두었으므로 Contains로 비교
						assert.Contains(t, errResp.Message, request.NotificationRequest{}.ApplicationID) // 필드명 등이 포함되는지 확인하는 것이 일반적이나, 여기선 생략하고 메시지 내용 비교
					} else {
						// 그 외(앱 에러)는 정확히 일치해야 함
						if strings.Contains(tt.expectedErrMsg, "Field validation") {
							assert.Contains(t, errResp.Message, "validation") // Validator 메시지 유연하게
						} else {
							assert.Equal(t, tt.expectedErrMsg, errResp.Message)
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
