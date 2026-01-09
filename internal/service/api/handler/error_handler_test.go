package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Error Handler Tests
// =============================================================================

// TestCustomHTTPErrorHandler_Comprehensive는 커스텀 HTTP 에러 핸들러의 모든 동작을 검증합니다.
//
// 주요 검증 항목:
//   - 다양한 에러 타입 처리 (echo.HTTPError, 일반 error)
//   - 에러 메시지 타입 처리 (string, response.ErrorResponse, 기타)
//   - 404 Not Found 메시지 강제 변환
//   - 500 Internal Server Error 로깅
//   - HEAD 요청 처리 (Body 없음)
//   - 이미 응답이 커밋된 경우 처리
func TestCustomHTTPErrorHandler_Comprehensive(t *testing.T) {
	// 로거 캡처 설정
	buf := new(bytes.Buffer)
	setupTestLogger(buf)
	defer restoreLogger()

	tests := []struct {
		name           string
		method         string
		path           string
		err            error
		preCommit      bool // 핸들러 실행 전 응답 커밋 여부 시뮬레이션
		expectedStatus int
		expectedBody   interface{} // 문자열 또는 response.ErrorResponse
		expectLog      string      // 로그에 포함되어야 할 문자열 (없으면 "")
	}{
		{
			name:           "404 Not Found_기본 메시지",
			method:         http.MethodGet,
			err:            echo.NewHTTPError(http.StatusNotFound, "Not Found"),
			expectedStatus: http.StatusNotFound,
			expectedBody:   response.ErrorResponse{Message: "페이지를 찾을 수 없습니다."},
		},
		{
			name:           "404 Not Found_커스텀 메시지 무시",
			method:         http.MethodGet,
			err:            echo.NewHTTPError(http.StatusNotFound, "Custom Check"),
			expectedStatus: http.StatusNotFound,
			expectedBody:   response.ErrorResponse{Message: "페이지를 찾을 수 없습니다."},
		},
		{
			name:           "404 Not Found_ErrorResponse 타입 메시지 무시",
			method:         http.MethodGet,
			err:            echo.NewHTTPError(http.StatusNotFound, response.ErrorResponse{Message: "Detail Info"}),
			expectedStatus: http.StatusNotFound,
			expectedBody:   response.ErrorResponse{Message: "페이지를 찾을 수 없습니다."},
		},
		{
			name:           "405 Method Not Allowed",
			method:         http.MethodPost,
			err:            echo.NewHTTPError(http.StatusMethodNotAllowed, "method not allowed"),
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   response.ErrorResponse{Message: "method not allowed"},
		},
		{
			name:           "400 Bad Request_ErrorResponse 타입 메시지",
			method:         http.MethodPost,
			err:            echo.NewHTTPError(http.StatusBadRequest, response.ErrorResponse{Message: "잘못된 요청입니다"}),
			expectedStatus: http.StatusBadRequest,
			expectedBody:   response.ErrorResponse{Message: "잘못된 요청입니다"},
		},
		{
			name:           "500 Internal Server Error_일반 에러",
			method:         http.MethodGet,
			err:            errors.New("database connection failed"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   response.ErrorResponse{Message: "내부 서버 오류가 발생했습니다."},
			expectLog:      "내부 서버 오류 발생",
		},
		{
			name:           "500 Internal Server Error_Echo 에러",
			method:         http.MethodGet,
			err:            echo.NewHTTPError(http.StatusInternalServerError, "critical failure"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   response.ErrorResponse{Message: "critical failure"},
			expectLog:      "내부 서버 오류 발생",
		},
		{
			name:           "알 수 없는 메시지 타입_기본 메시지 유지",
			method:         http.MethodGet,
			err:            echo.NewHTTPError(http.StatusBadRequest, 12345), // int는 처리되지 않음
			expectedStatus: http.StatusBadRequest,
			// 코드상 message 변수의 초기값인 "내부 서버 오류가 발생했습니다."가 사용됨
			expectedBody: response.ErrorResponse{Message: "내부 서버 오류가 발생했습니다."},
		},
		{
			name:           "HEAD 요청_Body 없음",
			method:         http.MethodHead,
			err:            echo.NewHTTPError(http.StatusNotFound, "Not Found"),
			expectedStatus: http.StatusNotFound,
			expectedBody:   nil, // Body가 비어있어야 함
		},
		{
			name:           "이미 응답 커밋됨_작업 중단",
			method:         http.MethodGet,
			err:            errors.New("error after write"),
			preCommit:      true,
			expectedStatus: http.StatusOK, // 핸들러가 상태 코드를 덮어쓰지 않아야 함 (초기 상태 200)
			expectedBody:   nil,           // 아무것도 쓰지 않아야 함
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 초기화
			buf.Reset()
			e := echo.New()
			req := httptest.NewRequest(tt.method, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Pre-commit 시뮬레이션
			if tt.preCommit {
				c.Response().Committed = true
			}

			// 테스트 실행
			CustomHTTPErrorHandler(tt.err, c)

			// 응답 검증
			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.expectedBody != nil {
				var expected response.ErrorResponse
				if val, ok := tt.expectedBody.(response.ErrorResponse); ok {
					expected = val
				}

				// Body 파싱 확인
				var actual response.ErrorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &actual); err != nil {
					t.Fatalf("Failed to decode response body: %v", err)
				}
				assert.Equal(t, expected.Message, actual.Message)
			} else {
				// Body가 비어있어야 하는 경우 (Head 요청 or Committed)
				assert.Empty(t, rec.Body.String())
			}

			// 로그 검증
			if tt.expectLog != "" {
				assert.Contains(t, buf.String(), tt.expectLog)
			}
		})
	}
}

// setupTestLogger는 테스트를 위해 로거 출력을 버퍼로 변경합니다.
func setupTestLogger(buf *bytes.Buffer) {
	applog.SetOutput(buf)
	applog.SetFormatter(&applog.JSONFormatter{}) // 로그 파싱이 쉽도록 JSON 포맷 사용 (선택사항)
}

// restoreLogger는 로거 출력을 표준 출력으로 복구합니다.
func restoreLogger() {
	applog.SetOutput(applog.StandardLogger().Out)
}
