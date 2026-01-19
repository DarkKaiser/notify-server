package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/model/response"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Error Response Tests
// =============================================================================

// TestErrorResponses는 모든 에러 응답 헬퍼 함수를 검증합니다.
func TestErrorResponses(t *testing.T) {
	t.Parallel()

	longMessage := strings.Repeat("a", 10000) // 10KB 메시지
	specialChars := "특수문자: <>&\"'\n\t\r"

	tests := []struct {
		name           string
		createError    func(string) error
		message        string
		expectedStatus int
	}{
		{
			name:           "BadRequest_일반 메시지",
			createError:    NewBadRequestError,
			message:        constants.ErrMsgBadRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "BadRequest_빈 메시지",
			createError:    NewBadRequestError,
			message:        "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Unauthorized_인증 실패",
			createError:    NewUnauthorizedError,
			message:        "인증이 필요합니다",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "NotFound_리소스 없음",
			createError:    NewNotFoundError,
			message:        "요청한 리소스를 찾을 수 없습니다",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "TooManyRequests_요청 제한",
			createError:    NewTooManyRequestsError,
			message:        "요청이 너무 많습니다",
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "InternalServerError_서버 오류",
			createError:    NewInternalServerError,
			message:        "내부 서버 오류가 발생했습니다",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "ServiceUnavailable_서비스 불가",
			createError:    NewServiceUnavailableError,
			message:        "서비스 이용 불가",
			expectedStatus: http.StatusServiceUnavailable,
		},
		// 엣지 케이스 통합
		{
			name:           "EdgeCase_매우 긴 메시지",
			createError:    NewBadRequestError,
			message:        longMessage,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "EdgeCase_특수 문자 포함",
			createError:    NewUnauthorizedError,
			message:        specialChars,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		tt := tt // 캡처
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 에러 생성
			err := tt.createError(tt.message)

			// 에러가 반환되는지 확인
			require.Error(t, err)

			// echo.HTTPError 타입 확인
			httpErr, ok := err.(*echo.HTTPError)
			require.True(t, ok, "반환된 에러는 *echo.HTTPError 타입이어야 합니다")

			// 상태 코드 확인
			assert.Equal(t, tt.expectedStatus, httpErr.Code)

			// ErrorResponse 구조체 확인
			errResp, ok := httpErr.Message.(response.ErrorResponse)
			require.True(t, ok, "에러 메시지는 response.ErrorResponse 타입이어야 합니다")

			// 메시지 확인
			assert.Equal(t, tt.message, errResp.Message)

			// ResultCode 확인
			assert.Equal(t, tt.expectedStatus, errResp.ResultCode)
		})
	}
}

// =============================================================================
// Success Response Tests
// =============================================================================

// TestSuccess는 성공 응답 생성을 검증합니다.
func TestSuccess(t *testing.T) {
	t.Parallel()

	// Echo 컨텍스트 설정
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// 성공 응답 생성
	err := Success(c)

	// 에러가 없어야 함
	require.NoError(t, err)

	// HTTP 상태 코드 확인
	assert.Equal(t, http.StatusOK, rec.Code)

	// Content-Type 확인
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	// 응답 본문 파싱
	var resp response.SuccessResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	// 값 검증
	assert.Equal(t, 0, resp.ResultCode)
	assert.Equal(t, constants.MsgSuccess, resp.Message)
}
