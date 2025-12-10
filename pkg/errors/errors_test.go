package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "Single error message",
			err:      New(ErrInvalidInput, "invalid input"),
			expected: "invalid input",
		},
		{
			name:     "Wrapped error message",
			err:      Wrap(errors.New("root cause"), ErrInternal, "internal error"),
			expected: "internal error: root cause",
		},
		{
			name:     "With formatting",
			err:      New(ErrInternal, fmt.Sprintf("op %s not supported", "foo")),
			expected: "op foo not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestUnwrap(t *testing.T) {
	rootErr := errors.New("root error")
	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{
			name:     "Unwrap wrapped error",
			err:      Wrap(rootErr, ErrInternal, "wrapped"),
			expected: rootErr,
		},
		{
			name:     "Unwrap new error (expect nil)",
			err:      New(ErrInvalidInput, "new error"),
			expected: nil,
		},
		{
			name:     "Unwrap nil error",
			err:      nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, errors.Unwrap(tt.err))
		})
	}
}

func TestIs(t *testing.T) {
	errNotFound := New(ErrNotFound, "not found")
	wrappedErr := Wrap(errNotFound, ErrInternal, "wrapped")

	tests := []struct {
		name     string
		err      error
		target   ErrorType
		expected bool
	}{
		{"Match ErrNotFound", errNotFound, ErrNotFound, true},
		{"Mismatch ErrInternal", errNotFound, ErrInternal, false},
		{"Match wrapped error type", wrappedErr, ErrInternal, true},
		{"Mismatch wrapped error cause type (AppError limitation)", wrappedErr, ErrNotFound, false},
		{"Nil error", nil, ErrNotFound, false},
		{"Standard error", errors.New("std err"), ErrNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Is(tt.err, tt.target))
		})
	}
}

func TestAs(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantMatch   bool
		expectedTyp ErrorType
	}{
		{
			name:        "Cast to AppError success",
			err:         New(ErrForbidden, "forbidden"),
			wantMatch:   true,
			expectedTyp: ErrForbidden,
		},
		{
			name:        "Cast std error to AppError fail",
			err:         errors.New("std error"),
			wantMatch:   false,
			expectedTyp: "",
		},
		{
			name:        "Cast wrapped AppError success",
			err:         Wrap(errors.New("root"), ErrSystem, "system"),
			wantMatch:   true,
			expectedTyp: ErrSystem,
		},
		{
			name:        "Cast nil error fail",
			err:         nil,
			wantMatch:   false,
			expectedTyp: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var appErr *AppError
			match := As(tt.err, &appErr)
			assert.Equal(t, tt.wantMatch, match)
			if tt.wantMatch {
				assert.NotNil(t, appErr)
				assert.Equal(t, tt.expectedTyp, appErr.Type)
			} else {
				assert.Nil(t, appErr)
			}
		})
	}
}

func TestGetType(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorType
	}{
		{"Return ErrUnauthorized", New(ErrUnauthorized, "unauthorized"), ErrUnauthorized},
		{"Return wrapped error type", Wrap(errors.New("std"), ErrSystem, "system"), ErrSystem},
		{"Standard error returns ErrUnknown", errors.New("std error"), ErrUnknown},
		{"Nil error returns ErrUnknown", nil, ErrUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetType(tt.err))
		})
	}
}

func TestCause(t *testing.T) {
	rootErr := errors.New("root error")
	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{"Cause of wrapped error", Wrap(rootErr, ErrInternal, "wrapped"), rootErr},
		{"Cause of new error is nil", New(ErrInvalidInput, "new"), nil},
		{"Cause of std error is nil", rootErr, nil},
		{"Cause of nil is nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Cause(tt.err))
		})
	}
}

func TestRootCause(t *testing.T) {
	rootErr := errors.New("root cause")

	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{
			name:     "Nil error",
			err:      nil,
			expected: nil,
		},
		{
			name:     "Single level std error",
			err:      rootErr,
			expected: rootErr,
		},
		{
			name:     "AppError (no wrap)",
			err:      New(ErrInvalidInput, "invalid"),
			expected: New(ErrInvalidInput, "invalid"),
		},
		{
			name:     "Nested AppError (2 levels)",
			err:      Wrap(rootErr, ErrInternal, "level 1"),
			expected: rootErr,
		},
		{
			name:     "Nested AppError (3 levels)",
			err:      Wrap(Wrap(rootErr, ErrInvalidInput, "level 2"), ErrInternal, "level 1"),
			expected: rootErr,
		},
		{
			name:     "Standard fmt.Errorf wrap",
			err:      fmt.Errorf("wrap: %w", rootErr),
			expected: rootErr,
		},
		{
			name:     "Mixed wrapping",
			err:      fmt.Errorf("std wrap: %w", Wrap(rootErr, ErrInternal, "app wrap")),
			expected: rootErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RootCause(tt.err)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Error(), result.Error())
			}
		})
	}
}
