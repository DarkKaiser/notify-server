package task

import (
	"testing"

	"github.com/darkkaiser/notify-server/pkg/errors"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// TestErrorTypes verifies the string constants of ErrorType definitions.
func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name     string
		errType  errors.ErrorType
		expected string
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.errType))
		})
	}
}

// TestErrorCreation verifies that specific error types function correctly with pkg/errors.New.
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

// TestSentinelErrors verifies the pre-defined error instances.
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
		{
			name:         "ErrCommandNotImplemented",
			actualErr:    ErrCommandNotImplemented,
			expectedType: apperrors.Internal,
			expectedMsg:  "작업 명령에 대한 구현이 없습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.actualErr)

			// Check underlying type classification
			assert.Equal(t, tt.expectedType, errors.GetType(tt.actualErr))

			// Check error message content
			assert.Equal(t, tt.expectedMsg, tt.actualErr.Error())

			// Verify standard errors.Is identity
			var appErr *errors.AppError
			assert.True(t, errors.As(tt.actualErr, &appErr))
		})
	}
}
