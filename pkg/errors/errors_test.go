package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err := New(ErrInvalidInput, "invalid input")
	assert.Error(t, err)
	assert.Equal(t, "invalid input", err.Error())
	assert.Equal(t, ErrInvalidInput, GetType(err))
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	err := Wrap(originalErr, ErrInternal, "internal error")

	assert.Error(t, err)
	assert.Equal(t, "internal error: original error", err.Error())
	assert.Equal(t, ErrInternal, GetType(err))
	assert.Equal(t, originalErr, Cause(err))
	assert.Equal(t, originalErr, errors.Unwrap(err))
}

func TestIs(t *testing.T) {
	err := New(ErrNotFound, "not found")
	assert.True(t, Is(err, ErrNotFound))
	assert.False(t, Is(err, ErrInternal))

	wrappedErr := Wrap(err, ErrInternal, "internal error")
	// Is checks the type of the *AppError* itself, not the wrapped error's type if the outer is AppError
	// In our implementation, Is checks if the error is an AppError and if its Type matches.
	assert.True(t, Is(wrappedErr, ErrInternal))
	assert.False(t, Is(wrappedErr, ErrNotFound))
}

func TestAs(t *testing.T) {
	err := New(ErrForbidden, "forbidden")
	var appErr *AppError
	assert.True(t, As(err, &appErr))
	assert.Equal(t, ErrForbidden, appErr.Type)
}

func TestGetType(t *testing.T) {
	err := New(ErrUnauthorized, "unauthorized")
	assert.Equal(t, ErrUnauthorized, GetType(err))

	stdErr := errors.New("std error")
	assert.Equal(t, ErrUnknown, GetType(stdErr))
}

func TestCause(t *testing.T) {
	rootErr := errors.New("root error")
	err := Wrap(rootErr, ErrInternal, "wrapped")
	assert.Equal(t, rootErr, Cause(err))

	assert.Nil(t, Cause(rootErr))
}

func TestRootCause(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{
			name:     "nil 에러",
			err:      nil,
			expected: nil,
		},
		{
			name:     "단일 레벨 에러",
			err:      errors.New("single error"),
			expected: errors.New("single error"),
		},
		{
			name:     "AppError (wrapping 없음)",
			err:      New(ErrInvalidInput, "invalid"),
			expected: New(ErrInvalidInput, "invalid"),
		},
		{
			name: "2단계 중첩 에러",
			err: func() error {
				rootErr := errors.New("root cause")
				return Wrap(rootErr, ErrInternal, "wrapped once")
			}(),
			expected: errors.New("root cause"),
		},
		{
			name: "3단계 중첩 에러",
			err: func() error {
				rootErr := errors.New("root cause")
				level1 := Wrap(rootErr, ErrInternal, "level 1")
				return Wrap(level1, ErrInvalidInput, "level 2")
			}(),
			expected: errors.New("root cause"),
		},
		{
			name: "4단계 중첩 에러",
			err: func() error {
				rootErr := errors.New("root cause")
				level1 := Wrap(rootErr, ErrInternal, "level 1")
				level2 := Wrap(level1, ErrInvalidInput, "level 2")
				return Wrap(level2, ErrNotFound, "level 3")
			}(),
			expected: errors.New("root cause"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RootCause(tt.err)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				// 에러 메시지로 비교 (errors.New로 생성된 에러는 포인터가 다름)
				assert.Equal(t, tt.expected.Error(), result.Error())
			}
		})
	}
}

func TestRootCause_WithStandardErrors(t *testing.T) {
	// 표준 라이브러리 에러 체인 테스트
	rootErr := errors.New("root")
	wrapped1 := fmt.Errorf("wrap1: %w", rootErr)
	wrapped2 := fmt.Errorf("wrap2: %w", wrapped1)

	result := RootCause(wrapped2)
	assert.Equal(t, rootErr, result)
}

func TestRootCause_MixedErrorTypes(t *testing.T) {
	// AppError와 표준 에러가 섞인 체인
	rootErr := errors.New("standard root")
	appErr := Wrap(rootErr, ErrInternal, "app error")
	stdWrapped := fmt.Errorf("standard wrap: %w", appErr)

	result := RootCause(stdWrapped)
	assert.Equal(t, rootErr, result)
}
