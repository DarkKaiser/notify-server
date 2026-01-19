package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
// 주요 개선 사항:
//   - 모든 엣지 케이스(긴 메시지, 특수 문자, 빈 메시지 등) 통합 관리
//   - 테이블 기반 테스트로 일관된 검증 로직 적용
func TestErrorResponses(t *testing.T) {
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
			message:        "잘못된 요청입니다",
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
			message:        "리소스를 찾을 수 없습니다",
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
			message:        "서버 내부 오류",
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
		t.Run(tt.name, func(t *testing.T) {
			// 에러 생성
			err := tt.createError(tt.message)

			// 에러가 반환되는지 확인
			require.Error(t, err, "에러가 반환되어야 합니다")

			// echo.HTTPError 타입 확인
			httpErr, ok := err.(*echo.HTTPError)
			require.True(t, ok, "반환된 에러는 *echo.HTTPError 타입이어야 합니다")

			// 상태 코드 확인
			assert.Equal(t, tt.expectedStatus, httpErr.Code, "HTTP 상태 코드가 일치해야 합니다")

			// ErrorResponse 구조체 확인
			errResp, ok := httpErr.Message.(response.ErrorResponse)
			require.True(t, ok, "에러 메시지는 response.ErrorResponse 타입이어야 합니다")

			// 메시지 확인
			assert.Equal(t, tt.message, errResp.Message, "에러 메시지가 일치해야 합니다")

			// ResultCode 확인
			// API 응답의 ResultCode가 HTTP 상태 코드와 동일한지 검증합니다.
			assert.Equal(t, tt.expectedStatus, errResp.ResultCode, "ResultCode는 HTTP 상태 코드와 일치해야 합니다")
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
//   - Message 필드가 "성공"인지 확인
func TestSuccess(t *testing.T) {
	// Echo 컨텍스트 설정
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// 성공 응답 생성
	err := Success(c)

	// 에러가 없어야 함
	assert.NoError(t, err, "성공 응답 생성 시 에러가 없어야 합니다")

	// HTTP 상태 코드 확인
	assert.Equal(t, http.StatusOK, rec.Code, "HTTP 상태 코드는 200이어야 합니다")

	// Content-Type 확인
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json", "Content-Type은 application/json이어야 합니다")

	// 응답 본문 파싱
	var resp response.SuccessResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err, "응답 본문은 유효한 JSON이어야 합니다")

	// ResultCode 확인
	assert.Equal(t, 0, resp.ResultCode, "ResultCode는 0이어야 합니다")

	// Message 확인
	assert.Equal(t, "성공", resp.Message, "Message는 '성공'이어야 합니다")
}
