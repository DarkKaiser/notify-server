package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Standard error for testing
var errStd = errors.New("standard error")

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		errType  ErrorType
		message  string
		expected string
	}{
		{
			name:     "Create InvalidInput error",
			errType:  InvalidInput,
			message:  "invalid input",
			expected: "invalid input",
		},
		{
			name:     "Create Internal error",
			errType:  Internal,
			message:  "internal server error",
			expected: "internal server error",
		},
		{
			name:     "Create error with empty message",
			errType:  Unknown,
			message:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.errType, tt.message)
			assert.Equal(t, tt.expected, err.Error())
			assert.Equal(t, tt.errType, GetType(err))
		})
	}
}

func TestNewf(t *testing.T) {
	tests := []struct {
		name     string
		errType  ErrorType
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "Format simple string",
			errType:  Conflict,
			format:   "resource %s already exists",
			args:     []interface{}{"user-123"},
			expected: "resource user-123 already exists",
		},
		{
			name:     "Format with multiple args",
			errType:  System,
			format:   "failed to connect to %s:%d",
			args:     []interface{}{"localhost", 8080},
			expected: "failed to connect to localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Newf(tt.errType, tt.format, tt.args...)
			assert.Equal(t, tt.expected, err.Error())
			assert.Equal(t, tt.errType, GetType(err))
		})
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name        string
		cause       error
		errType     ErrorType
		message     string
		expectedMsg string
	}{
		{
			name:        "Wrap standard error",
			cause:       errStd,
			errType:     Internal,
			message:     "db query failed",
			expectedMsg: "db query failed: standard error",
		},
		{
			name:        "Wrap nil error",
			cause:       nil,
			errType:     Unknown,
			message:     "unknown error",
			expectedMsg: "unknown error", // Cause가 nil이면 메시지만 출력
		},
		{
			name:        "Wrap AppError (nested)",
			cause:       New(InvalidInput, "bad request"),
			errType:     Internal,
			message:     "controller failed",
			expectedMsg: "controller failed: bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Wrap(tt.cause, tt.errType, tt.message)
			assert.Equal(t, tt.expectedMsg, err.Error())
			assert.Equal(t, tt.errType, GetType(err))
			assert.Equal(t, tt.cause, Cause(err))
		})
	}
}

func TestWrapf(t *testing.T) {
	tests := []struct {
		name        string
		cause       error
		errType     ErrorType
		format      string
		args        []interface{}
		expectedMsg string
	}{
		{
			name:        "Wrapf with format",
			cause:       errStd,
			errType:     NotFound,
			format:      "user %s not found",
			args:        []interface{}{"alice"},
			expectedMsg: "user alice not found: standard error",
		},
		{
			name:        "Wrapf nil error",
			cause:       nil,
			errType:     System,
			format:      "system error code %d",
			args:        []interface{}{500},
			expectedMsg: "system error code 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Wrapf(tt.cause, tt.errType, tt.format, tt.args...)
			assert.Equal(t, tt.expectedMsg, err.Error())
			assert.Equal(t, tt.errType, GetType(err))
		})
	}
}

func TestIs(t *testing.T) {
	errNotFound := New(NotFound, "not found")
	wrappedErr := Wrap(errNotFound, Internal, "wrapped")
	multiWrapped := Wrap(wrappedErr, System, "outer")

	tests := []struct {
		name     string
		err      error
		target   ErrorType
		expected bool
	}{
		{"Match exact type", errNotFound, NotFound, true},
		{"Mismatch type", errNotFound, Internal, false},
		{"Match wrapped error type (direct parent)", wrappedErr, Internal, true},
		{"Match nested error type (limitation: Is only checks the top-level AppError)", wrappedErr, NotFound, false}, // 현재 구현상 Is는 unwrap하지 않고 최상위 AppError의 타입만 확인합니다.
		{"Match multi-wrapped outer", multiWrapped, System, true},
		{"Nil error", nil, NotFound, false},
		{"Standard error", errors.New("std err"), NotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Is(tt.err, tt.target))
		})
	}
}

func TestAs(t *testing.T) {
	targetAppErr := &AppError{}

	tests := []struct {
		name      string
		err       error
		target    interface{}
		wantMatch bool
	}{
		{
			name:      "Cast New() AppError",
			err:       New(Forbidden, "forbidden"),
			target:    &targetAppErr,
			wantMatch: true,
		},
		{
			name:      "Cast Wrap() AppError",
			err:       Wrap(errStd, System, "system"),
			target:    &targetAppErr,
			wantMatch: true,
		},
		{
			name:      "Cast failed for std error",
			err:       errStd,
			target:    &targetAppErr,
			wantMatch: false,
		},
		{
			name:      "Cast failed for nil error",
			err:       nil,
			target:    &targetAppErr,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := As(tt.err, tt.target)
			assert.Equal(t, tt.wantMatch, match)
			if tt.wantMatch {
				// Type assertion to access fields
				if appErr, ok := tt.target.(**AppError); ok && *appErr != nil {
					assert.NotEmpty(t, (*appErr).Type)
				}
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
		{"AppError", New(Unauthorized, "msg"), Unauthorized},
		{"Wrapped AppError", Wrap(errStd, Timeout, "msg"), Timeout},
		{"Standard Error", errStd, Unknown},
		{"Nil Error", nil, Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetType(tt.err))
		})
	}
}

func TestCause(t *testing.T) {
	root := errors.New("root")
	wrapped := Wrap(root, Internal, "wrapped")
	doubleWrapped := Wrap(wrapped, System, "double wrapped")

	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{"Nil error", nil, nil},
		{"Standard error (no cause)", root, nil}, // AppError가 아니면 Cause는 nil
		{"AppError New (no cause)", New(Internal, "msg"), nil},
		{"AppError Wrap (has cause)", wrapped, root},
		{"Double wrapped (direct cause)", doubleWrapped, wrapped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Cause(tt.err))
		})
	}
}

func TestRootCause(t *testing.T) {
	root := errors.New("root")
	wrapped1 := Wrap(root, Internal, "layer1")
	wrapped2 := Wrap(wrapped1, System, "layer2")
	fmtWrapped := fmt.Errorf("fmt wrap: %w", wrapped2)

	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{"Nil error", nil, nil},
		{"Standard error", root, root},
		{"Wrappped Once", wrapped1, root},
		{"Wrapped Twice", wrapped2, root},
		{"Fmt Wrapped", fmtWrapped, root},
		{"New AppError", New(Internal, "new"), New(Internal, "new")}, // Cause가 없으면 자신을 반환
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RootCause(tt.err)
			// assert.Equal compares deep equality. For errors created with New(), pointers are different.
			// Compare error messages for simple check, or specific logic logic
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Error(), result.Error())
			}
		})
	}
}

func TestUnwrap(t *testing.T) {
	root := errors.New("root")
	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{"New AppError (nil)", New(Internal, "msg"), nil},
		{"Wrap AppError", Wrap(root, Internal, "msg"), root},
		{"Nil error", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// AppError implements Unwrap()
			// errors.Unwrap calls the Unwrap method if available
			assert.Equal(t, tt.expected, errors.Unwrap(tt.err))
		})
	}
}

// ----------------------------------------------------------------------------
// Examples (Documentation)
// ----------------------------------------------------------------------------

func ExampleNew() {
	err := New(InvalidInput, "email is invalid")
	fmt.Println(err)
	// Output: email is invalid
}

func ExampleNewf() {
	err := Newf(NotFound, "user %d not found", 101)
	fmt.Println(err)
	// Output: user 101 not found
}

func ExampleWrap() {
	originalErr := errors.New("eof")
	err := Wrap(originalErr, Internal, "failed to read file")
	fmt.Println(err)
	// Output: failed to read file: eof
}

func ExampleIs() {
	err := New(Timeout, "request timed out")
	if Is(err, Timeout) {
		fmt.Println("Error is Timeout")
	}
	// Output: Error is Timeout
}

func ExampleGetType() {
	err := New(Forbidden, "access denied")
	switch GetType(err) {
	case Forbidden:
		fmt.Println("Handle forbidden error")
	case Internal:
		fmt.Println("Handle internal error")
	}
	// Output: Handle forbidden error
}
