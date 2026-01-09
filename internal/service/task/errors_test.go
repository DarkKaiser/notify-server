package task

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/errors"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Error Type Tests
// =============================================================================

// TestErrorTypes는 ErrorType 정의의 문자열 상수를 검증합니다.
//
// 검증 항목:
//   - ErrorType 문자열 값
func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name     string
		errType  errors.ErrorType
		expected string
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.errType.String())
		})
	}
}

// TestErrorCreation은 특정 에러 타입이 pkg/errors.New와 올바르게 작동하는지 검증합니다.
//
// 검증 항목:
//   - GetType이 올바른 타입 반환
//   - errors.Is 헬퍼 동작
//   - 에러 메시지 포함
func TestErrorCreation(t *testing.T) {
	tests := []struct {
		name    string
		errType errors.ErrorType
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errType, "test error message")

			// Verify GetType returns the correct type
			assert.Equal(t, tt.errType, errors.GetType(err))

			// Verify our custom errors.Is helper works
			assert.True(t, errors.Is(err, tt.errType))

			// Verify message
			assert.Contains(t, err.Error(), "test error message")
		})
	}
}

// =============================================================================
// Sentinel Error Tests
// =============================================================================

// TestSentinelErrors는 미리 정의된 에러 인스턴스를 검증합니다.
//
// 검증 항목:
//   - ErrTaskNotSupported 에러
//   - ErrCommandNotSupported 에러
//   - 에러 타입 분류
//   - 에러 메시지
func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name         string
		actualErr    error
		expectedType errors.ErrorType
		expectedMsg  string
	}{
		{
			name:         "ErrTaskNotSupported",
			actualErr:    ErrTaskNotSupported,
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "지원하지 않는 작업입니다",
		},
		{
			name:         "ErrCommandNotSupported",
			actualErr:    ErrCommandNotSupported,
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "지원하지 않는 명령입니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.actualErr, "Error should not be nil")

			// Check underlying type classification
			assert.Equal(t, tt.expectedType, errors.GetType(tt.actualErr))

			// Check error message content
			assert.Equal(t, "[InvalidInput] "+tt.expectedMsg, tt.actualErr.Error())

			// Verify standard errors.Is identity
			var appErr *errors.AppError
			require.True(t, errors.As(tt.actualErr, &appErr), "Should be AppError")
		})
	}
}
