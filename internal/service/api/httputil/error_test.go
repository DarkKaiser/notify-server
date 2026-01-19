package httputil

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/domain"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Error Handler Tests
// =============================================================================

// LogEntry 로그 검증을 위한 구조체
type LogEntry struct {
	Level         string `json:"level"`
	Message       string `json:"msg"`
	Path          string `json:"path"`
	Method        string `json:"method"`
	StatusCode    int    `json:"status_code"`
	RemoteIP      string `json:"remote_ip"`
	RequestID     string `json:"request_id"`
	ApplicationID string `json:"application_id,omitempty"`
}

// TestErrorHandler_Comprehensive는 커스텀 HTTP 에러 핸들러의 모든 동작을 검증합니다.
//
// 주의: 이 테스트는 pkg/log의 글로벌 상태를 변경하므로 t.Parallel()을 사용할 수 없습니다.
// 반드시 직렬로 실행되어야 합니다.
func TestErrorHandler_Comprehensive(t *testing.T) {
	// 로거 캡처 설정
	buf := new(bytes.Buffer)
	setupTestLogger(buf)
	defer restoreLogger()

	tests := []struct {
		name            string
		method          string
		path            string
		err             error
		setupContext    func(c echo.Context, req *http.Request, rec *httptest.ResponseRecorder)
		expectedStatus  int
		expectedBody    interface{} // 문자열 또는 response.ErrorResponse
		expectedLog     *LogEntry   // 검증할 로그 필드 (nil이면 로그 검증 건너뜀)
		expectedLogPart string      // 로그에 포함되어야 할 문자열 (메시지 등 단순 확인용)
		expectNoLog     bool        // 로그가 생성되지 않아야 함을 명시
	}{
		{
			name:           "404 Not Found_기본 메시지",
			method:         http.MethodGet,
			err:            echo.NewHTTPError(http.StatusNotFound, "Not Found"),
			expectedStatus: http.StatusNotFound,
			expectedBody:   response.ErrorResponse{Message: "요청한 리소스를 찾을 수 없습니다"},
			expectedLog: &LogEntry{
				Level:      "warning",
				Message:    "HTTP 4xx: 클라이언트 요청 오류",
				StatusCode: http.StatusNotFound,
			},
		},
		{
			name:            "404 Not Found_커스텀 메시지 유지",
			method:          http.MethodGet,
			err:             echo.NewHTTPError(http.StatusNotFound, "Custom Check"),
			expectedStatus:  http.StatusNotFound,
			expectedBody:    response.ErrorResponse{Message: "Custom Check"},
			expectedLogPart: "클라이언트 요청 오류",
		},
		{
			name:           "405 Method Not Allowed",
			method:         http.MethodPost,
			err:            echo.NewHTTPError(http.StatusMethodNotAllowed, "method not allowed"),
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   response.ErrorResponse{Message: "method not allowed"},
			expectedLog: &LogEntry{
				Level:      "warning",
				StatusCode: http.StatusMethodNotAllowed,
			},
		},
		{
			name:           "400 Bad Request_ErrorResponse 타입 메시지",
			method:         http.MethodPost,
			err:            echo.NewHTTPError(http.StatusBadRequest, response.ErrorResponse{Message: "잘못된 요청입니다"}),
			expectedStatus: http.StatusBadRequest,
			expectedBody:   response.ErrorResponse{Message: "잘못된 요청입니다"},
			expectedLog: &LogEntry{
				Level:      "warning",
				StatusCode: http.StatusBadRequest,
			},
		},
		{
			name:           "401 Unauthorized_인증 실패",
			method:         http.MethodPost,
			err:            echo.NewHTTPError(http.StatusUnauthorized, "인증이 필요합니다"),
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   response.ErrorResponse{Message: "인증이 필요합니다"},
			expectedLog: &LogEntry{
				Level:      "warning",
				StatusCode: http.StatusUnauthorized,
			},
		},
		{
			name:           "500 Internal Server Error_일반 에러",
			method:         http.MethodGet,
			err:            errors.New("database connection failed"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   response.ErrorResponse{Message: "내부 서버 오류가 발생했습니다"},
			expectedLog: &LogEntry{
				Level:      "error",
				Message:    "HTTP 5xx: 서버 내부 오류",
				StatusCode: http.StatusInternalServerError,
			},
		},
		{
			name:   "로깅 필드 검증_IP 및 RequestID",
			method: http.MethodGet,
			err:    echo.NewHTTPError(http.StatusBadRequest, "Bad Request"),
			setupContext: func(c echo.Context, req *http.Request, rec *httptest.ResponseRecorder) {
				req.RemoteAddr = "192.168.1.100:12345"
				rec.Header().Set(echo.HeaderXRequestID, "test-req-id-123")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   response.ErrorResponse{Message: "Bad Request"},
			expectedLog: &LogEntry{
				RemoteIP:  "192.168.1.100",
				RequestID: "test-req-id-123",
			},
		},
		{
			name:   "로깅 필드 검증_Application ID",
			method: http.MethodPost,
			err:    echo.NewHTTPError(http.StatusForbidden, "Forbidden"),
			setupContext: func(c echo.Context, req *http.Request, rec *httptest.ResponseRecorder) {
				// 인증된 애플리케이션 주입
				app := &domain.Application{ID: "my-test-app"}
				c.Set(constants.ContextKeyApplication, app)
			},
			expectedStatus: http.StatusForbidden,
			expectedBody:   response.ErrorResponse{Message: "Forbidden"},
			expectedLog: &LogEntry{
				ApplicationID: "my-test-app",
			},
		},
		{
			name:           "HEAD 요청_Body 없음",
			method:         http.MethodHead,
			err:            echo.NewHTTPError(http.StatusNotFound, "Not Found"),
			expectedStatus: http.StatusNotFound,
			expectedBody:   nil, // Body가 비어있어야 함
		},
		{
			name:   "이미 응답 커밋됨_작업 중단",
			method: http.MethodGet,
			err:    errors.New("error after write"),
			setupContext: func(c echo.Context, req *http.Request, rec *httptest.ResponseRecorder) {
				c.Response().Committed = true
			},
			expectedStatus: http.StatusOK, // 핸들러가 상태 코드를 덮어쓰지 않아야 함
			expectedBody:   nil,
		},
		// --- 엣지 케이스 추가 ---
		{
			name:           "EdgeCase_HTTPError 메시지 타입 불일치(int)",
			method:         http.MethodGet,
			err:            echo.NewHTTPError(http.StatusBadRequest, 12345), // int 메시지
			expectedStatus: http.StatusBadRequest,
			// 메시지가 string이나 ErrorResponse가 아니면 기본값("내부 서버 오류...")이 유지됨.
			// 400 에러지만 메시지는 내부 오류로 나가는 현재 동작을 검증.
			expectedBody: response.ErrorResponse{Message: "내부 서버 오류가 발생했습니다"},
			expectedLog: &LogEntry{
				Level: "warning",
			},
		},
		{
			name:   "EdgeCase_Context Application 타입 불일치",
			method: http.MethodGet,
			err:    errors.New("error"),
			setupContext: func(c echo.Context, req *http.Request, rec *httptest.ResponseRecorder) {
				c.Set(constants.ContextKeyApplication, "invalid-string-type") // *domain.Application이 아님
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   response.ErrorResponse{Message: "내부 서버 오류가 발생했습니다"},
			expectedLog: &LogEntry{
				ApplicationID: "", // string 타입이므로 로깅되지 않아야 함
			},
		},
		{
			name:           "EdgeCase_Status 3xx (로그 제외)",
			method:         http.MethodGet,
			err:            echo.NewHTTPError(http.StatusFound, "Redirecting"),
			expectedStatus: http.StatusFound,
			expectNoLog:    true, // 400 미만 상태 코드는 로그를 남기지 않음
			expectedBody:   response.ErrorResponse{Message: "Redirecting"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 초기화 (직렬 실행이므로 루프 내 Reset 필수)
			buf.Reset()
			e := echo.New()
			req := httptest.NewRequest(tt.method, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// 추가 컨텍스트 설정
			if tt.setupContext != nil {
				tt.setupContext(c, req, rec)
			}

			// 테스트 실행
			ErrorHandler(tt.err, c)

			// 응답 상태 코드 검증
			assert.Equal(t, tt.expectedStatus, rec.Code, "HTTP 상태 코드가 일치해야 합니다")

			// 응답 본문 검증
			if tt.expectedBody != nil {
				var expected response.ErrorResponse
				if val, ok := tt.expectedBody.(response.ErrorResponse); ok {
					expected = val
				}

				// Body 파싱 확인
				var actual response.ErrorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &actual); err != nil {
					t.Logf("Response Body: %s", rec.Body.String())
					t.Fatalf("Failed to decode response body: %v", err)
				}
				assert.Equal(t, expected.Message, actual.Message, "응답 메시지가 일치해야 합니다")

				// ResultCode 검증
				assert.Equal(t, tt.expectedStatus, actual.ResultCode, "ResultCode는 HTTP 상태 코드와 일치해야 합니다")
			} else {
				// Body가 비어있어야 하는 경우
				assert.Empty(t, rec.Body.String(), "응답 본문이 비어있어야 합니다")
			}

			// 로그 검증
			if tt.expectNoLog {
				assert.Empty(t, buf.String(), "로그가 생성되지 않아야 합니다")
			} else {
				if tt.expectedLog != nil {
					var logEntry LogEntry
					err := json.Unmarshal(buf.Bytes(), &logEntry)
					require.NoError(t, err, "로그 파싱에 실패했습니다: %s", buf.String())

					if tt.expectedLog.Level != "" {
						assert.Equal(t, tt.expectedLog.Level, logEntry.Level)
					}
					if tt.expectedLog.Message != "" {
						assert.Equal(t, tt.expectedLog.Message, logEntry.Message)
					}
					if tt.expectedLog.StatusCode != 0 {
						assert.Equal(t, tt.expectedLog.StatusCode, logEntry.StatusCode)
					}
					if tt.expectedLog.RemoteIP != "" {
						assert.Equal(t, tt.expectedLog.RemoteIP, logEntry.RemoteIP)
					}
					if tt.expectedLog.RequestID != "" {
						assert.Equal(t, tt.expectedLog.RequestID, logEntry.RequestID)
					}
					// ApplicationID가 명시적으로 기대되는(빈 값 포함) 경우에만 검증
					// 구조체 초기화 값("")과 구분을 위해 로직상 주의가 필요하지만,
					// 여기서는 단순 비교.
					if tt.expectedLog.ApplicationID != "" {
						assert.Equal(t, tt.expectedLog.ApplicationID, logEntry.ApplicationID)
					} else {
						// ApplicationID가 기대치에 없으면 빈 문자열이어야 함
						assert.Empty(t, logEntry.ApplicationID)
					}
				}

				if tt.expectedLogPart != "" {
					assert.Contains(t, buf.String(), tt.expectedLogPart)
				}
			}
		})
	}
}

// setupTestLogger는 테스트를 위해 로거 출력을 버퍼로 변경합니다.
func setupTestLogger(buf *bytes.Buffer) {
	applog.SetOutput(buf)
	applog.SetFormatter(&applog.JSONFormatter{}) // 로그 파싱이 쉽도록 JSON 포맷 사용
}

// restoreLogger는 로거 출력을 표준 출력으로 복구합니다.
func restoreLogger() {
	applog.SetOutput(applog.StandardLogger().Out)
}
