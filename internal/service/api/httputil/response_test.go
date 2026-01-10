package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Error Response Tests
// =============================================================================

// TestErrorResponses는 모든 에러 응답 헬퍼 함수를 검증합니다.
//
// 검증 항목:
//   - 올바른 HTTP 상태 코드 반환
//   - ErrorResponse 구조체로 래핑
//   - 메시지 정확성 (빈 문자열 포함)
func TestErrorResponses(t *testing.T) {
	tests := []struct {
		name           string
		createError    func(string) error
		message        string
		expectedStatus int
	}{
		{
			name:           "BadRequest with message",
			createError:    NewBadRequestError,
			message:        "잘못된 요청입니다",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "BadRequest with empty message",
			createError:    NewBadRequestError,
			message:        "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Unauthorized",
			createError:    NewUnauthorizedError,
			message:        "인증이 필요합니다",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "NotFound",
			createError:    NewNotFoundError,
			message:        "리소스를 찾을 수 없습니다",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "TooManyRequests",
			createError:    NewTooManyRequestsError,
			message:        "요청이 너무 많습니다",
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "InternalServerError",
			createError:    NewInternalServerError,
			message:        "서버 내부 오류",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "ServiceUnavailable",
			createError:    NewServiceUnavailableError,
			message:        "서비스 이용 불가",
			expectedStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 에러 생성
			err := tt.createError(tt.message)

			// 에러가 반환되는지 확인
			require.Error(t, err)

			// echo.HTTPError 타입 확인
			httpErr, ok := err.(*echo.HTTPError)
			require.True(t, ok, "Error should be *echo.HTTPError")

			// 상태 코드 확인
			assert.Equal(t, tt.expectedStatus, httpErr.Code)

			// ErrorResponse 구조체 확인
			errResp, ok := httpErr.Message.(response.ErrorResponse)
			require.True(t, ok, "Message should be response.ErrorResponse")

			// 메시지 확인
			assert.Equal(t, tt.message, errResp.Message)
		})
	}
}

// =============================================================================
// Success Response Tests
// =============================================================================

// TestNewSuccessResponse는 성공 응답 생성을 검증합니다.
//
// 검증 항목:
//   - 200 OK 상태 코드
//   - application/json Content-Type
//   - ResultCode가 0인 SuccessResponse
func TestNewSuccessResponse(t *testing.T) {
	// Echo 컨텍스트 설정
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// 성공 응답 생성
	err := NewSuccessResponse(c)

	// 에러가 없어야 함
	assert.NoError(t, err)

	// HTTP 상태 코드 확인
	assert.Equal(t, http.StatusOK, rec.Code)

	// Content-Type 확인
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	// 응답 본문 파싱
	var resp response.SuccessResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err, "Response body should be valid JSON")

	// ResultCode 확인
	assert.Equal(t, 0, resp.ResultCode)
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestErrorResponses_LongMessage는 긴 메시지 처리를 검증합니다.
func TestErrorResponses_LongMessage(t *testing.T) {
	longMessage := string(make([]byte, 10000)) // 10KB 메시지

	err := NewBadRequestError(longMessage)
	require.Error(t, err)

	httpErr := err.(*echo.HTTPError)
	errResp := httpErr.Message.(response.ErrorResponse)

	assert.Equal(t, longMessage, errResp.Message)
}

// TestErrorResponses_SpecialCharacters는 특수 문자 처리를 검증합니다.
func TestErrorResponses_SpecialCharacters(t *testing.T) {
	specialChars := "특수문자: <>&\"'\n\t\r"

	err := NewUnauthorizedError(specialChars)
	require.Error(t, err)

	httpErr := err.(*echo.HTTPError)
	errResp := httpErr.Message.(response.ErrorResponse)

	assert.Equal(t, specialChars, errResp.Message)
}
