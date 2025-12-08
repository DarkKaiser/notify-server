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
			name:     "단일 에러 메시지",
			err:      New(ErrInvalidInput, "invalid input"),
			expected: "invalid input",
		},
		{
			name:     "Wrap된 에러 메시지",
			err:      Wrap(errors.New("root cause"), ErrInternal, "internal error"),
			expected: "internal error: root cause",
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
			name:     "Wrapping된 에러 Unwrap",
			err:      Wrap(rootErr, ErrInternal, "wrapped"),
			expected: rootErr,
		},
		{
			name:     "New로 생성된 에러 Unwrap (nil 기대)",
			err:      New(ErrInvalidInput, "new error"),
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
		{"ErrNotFound 매칭", errNotFound, ErrNotFound, true},
		{"ErrInternal 불일치", errNotFound, ErrInternal, false},
		{"Wrapped 에러의 겉 타입 매칭", wrappedErr, ErrInternal, true},
		// 주의: 현재 구현상 Is는 AppError의 Type만 확인하므로, 내부 에러의 타입을 확인하지 않음
		{"Wrapped 에러의 원인 타입 불일치 (AppError 동작)", wrappedErr, ErrNotFound, false},
		{"nil 에러", nil, ErrNotFound, false},
		{"표준 에러", errors.New("std err"), ErrNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Is(tt.err, tt.target))
		})
	}
}

func TestAs(t *testing.T) {
	t.Run("AppError로 캐스팅 성공", func(t *testing.T) {
		err := New(ErrForbidden, "forbidden")
		var appErr *AppError
		assert.True(t, As(err, &appErr))
		assert.Equal(t, ErrForbidden, appErr.Type)
	})

	t.Run("AppError로 캐스팅 실패 (표준 에러)", func(t *testing.T) {
		err := errors.New("std error")
		var appErr *AppError
		assert.False(t, As(err, &appErr))
		assert.Nil(t, appErr)
	})
}

func TestGetType(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorType
	}{
		{"ErrUnauthorized 반환", New(ErrUnauthorized, "unauthorized"), ErrUnauthorized},
		{"Wrapped 에러의 타입 반환", Wrap(errors.New("std"), ErrSystem, "system"), ErrSystem},
		{"표준 에러는 ErrUnknown", errors.New("std error"), ErrUnknown},
		{"nil 에러는 ErrUnknown", nil, ErrUnknown},
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
		{"Wrap된 에러의 원인", Wrap(rootErr, ErrInternal, "wrapped"), rootErr},
		{"New로 생성된 에러의 원인은 nil", New(ErrInvalidInput, "new"), nil},
		{"표준 에러의 원인은 nil (AppError 아님)", rootErr, nil},
		{"nil 에러는 nil", nil, nil},
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
			name:     "nil 에러",
			err:      nil,
			expected: nil,
		},
		{
			name:     "단일 레벨 표준 에러",
			err:      rootErr,
			expected: rootErr,
		},
		{
			name:     "AppError (wrapping 없음)",
			err:      New(ErrInvalidInput, "invalid"),
			expected: New(ErrInvalidInput, "invalid"), // 자체 반환
		},
		{
			name:     "중첩된 AppError (2단계)",
			err:      Wrap(rootErr, ErrInternal, "level 1"),
			expected: rootErr,
		},
		{
			name:     "중첩된 AppError (3단계: AppError -> AppError -> std)",
			err:      Wrap(Wrap(rootErr, ErrInvalidInput, "level 2"), ErrInternal, "level 1"),
			expected: rootErr,
		},
		{
			name:     "표준 wrapping (fmt.Errorf)",
			err:      fmt.Errorf("wrap: %w", rootErr),
			expected: rootErr,
		},
		{
			name:     "혼합 wrapping (fmt.Errorf -> AppError -> std)",
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
				// 에러 메시지 비교를 통해 동일 에러인지 확인 (포인터 비교가 안될 수 있는 상황 대비)
				// 단, rootErr 변수를 직접 사용하는 케이스는 포인터 비교도 가능하나,
				// New() 호출 결과는 새로운 포인터이므로 Error() 문자열 비교가 안전함.
				assert.Equal(t, tt.expected.Error(), result.Error())
			}
		})
	}
}
