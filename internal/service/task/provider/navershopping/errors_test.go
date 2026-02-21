package navershopping

import (
	"errors"
	"fmt"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Sentinel 에러 검증
// =============================================================================

// TestSentinelErrors 패키지 수준에서 선언된 Sentinel 에러들이
// 올바른 타입과 메시지를 가지고 있는지 검증합니다.
func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		wantType    apperrors.ErrorType
		wantMessage string
	}{
		{
			name:        "ErrClientIDMissing",
			err:         ErrClientIDMissing,
			wantType:    apperrors.InvalidInput,
			wantMessage: "client_id는 필수 설정값입니다",
		},
		{
			name:        "ErrClientSecretMissing",
			err:         ErrClientSecretMissing,
			wantType:    apperrors.InvalidInput,
			wantMessage: "client_secret은 필수 설정값입니다",
		},
		{
			name:        "ErrEmptyQuery",
			err:         ErrEmptyQuery,
			wantType:    apperrors.InvalidInput,
			wantMessage: "query가 입력되지 않았거나 공백입니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Error(t, tt.err)

			// ErrorType 검증 (apperrors.Is 사용)
			assert.True(t, apperrors.Is(tt.err, tt.wantType),
				"에러 타입이 %v 이어야 합니다", tt.wantType)

			// 메시지 검증 (*AppError 타입 단언을 통해)
			var appErr *apperrors.AppError
			require.True(t, errors.As(tt.err, &appErr),
				"*apperrors.AppError 타입이어야 합니다")
			assert.Equal(t, tt.wantMessage, appErr.Message())
		})
	}
}

// TestSentinelErrors_AreDistinct 세 Sentinel 에러가 서로 다른 독립적인 인스턴스임을 보장합니다.
// 하나의 에러를 Wrap해도 다른 Sentinel 에러에 영향이 없어야 합니다.
func TestSentinelErrors_AreDistinct(t *testing.T) {
	t.Parallel()

	assert.NotEqual(t, ErrClientIDMissing, ErrClientSecretMissing)
	assert.NotEqual(t, ErrClientSecretMissing, ErrEmptyQuery)
	assert.NotEqual(t, ErrClientIDMissing, ErrEmptyQuery)
}

// =============================================================================
// newErrInvalidPrice 팩토리 함수 검증
// =============================================================================

// TestNewErrInvalidPrice newErrInvalidPrice 팩토리 함수가
// 올바른 타입과 포맷의 메시지를 담은 에러를 반환하는지 검증합니다.
func TestNewErrInvalidPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input int
	}{
		{name: "음수 입력값 (-1)", input: -1},
		{name: "영(0) 입력값", input: 0},
		{name: "큰 음수 입력값 (-999)", input: -999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newErrInvalidPrice(tt.input)

			require.Error(t, err)

			// ErrorType은 반드시 InvalidInput
			assert.True(t, apperrors.Is(err, apperrors.InvalidInput),
				"에러 타입이 InvalidInput 이어야 합니다")

			// 메시지에 입력값이 포함되어야 함
			var appErr *apperrors.AppError
			require.True(t, errors.As(err, &appErr))
			assert.Equal(t,
				fmt.Sprintf("price_less_than은 0보다 커야 합니다 (입력값: %d)", tt.input),
				appErr.Message(),
			)

			// cause가 없는 단순(non-wrapped) 에러
			assert.NoError(t, errors.Unwrap(err),
				"팩토리 에러는 다른 에러를 래핑하지 않아야 합니다")
		})
	}
}

// TestNewErrInvalidPrice_EachCallCreatesNewInstance 동일한 입력값으로 두 번 호출했을 때
// 서로 다른 에러 인스턴스가 반환되는지 확인합니다.
func TestNewErrInvalidPrice_EachCallCreatesNewInstance(t *testing.T) {
	t.Parallel()

	err1 := newErrInvalidPrice(-1)
	err2 := newErrInvalidPrice(-1)

	// 동일한 내용이지만 서로 다른 인스턴스
	assert.NotSame(t, err1, err2)
	assert.Equal(t, err1.Error(), err2.Error())
}

// =============================================================================
// newErrEndpointParseFailed 팩토리 함수 검증
// =============================================================================

// TestNewErrEndpointParseFailed newErrEndpointParseFailed 팩토리 함수가
// 원인 에러를 올바르게 래핑하고 Internal 타입을 가지는지 검증합니다.
func TestNewErrEndpointParseFailed(t *testing.T) {
	t.Parallel()

	cause := errors.New("invalid control character in URL")
	err := newErrEndpointParseFailed(cause)

	require.Error(t, err)

	// ErrorType 검증
	assert.True(t, apperrors.Is(err, apperrors.Internal),
		"에러 타입이 Internal 이어야 합니다")

	// 메시지 검증
	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "네이버 쇼핑 검색 API 엔드포인트 URL 파싱에 실패하였습니다", appErr.Message())

	// 원인 에러가 체인에 포함되어야 함 (errors.Is를 통한 표준 체이닝 검증)
	assert.True(t, errors.Is(err, cause),
		"원인 에러가 에러 체인 안에 포함되어 있어야 합니다")

	// Error() 문자열에 원인 메시지가 출력되어야 함
	assert.Contains(t, err.Error(), cause.Error())
}

// TestNewErrEndpointParseFailed_NilCause cause가 nil이면 nil을 반환하는지 검증합니다.
// apperrors.Wrap의 nil 방어 로직이 작동해야 합니다.
func TestNewErrEndpointParseFailed_NilCause(t *testing.T) {
	t.Parallel()

	err := newErrEndpointParseFailed(nil)
	assert.NoError(t, err, "cause가 nil이면 에러도 nil을 반환해야 합니다")
}
