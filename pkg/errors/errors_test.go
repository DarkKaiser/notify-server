package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Constants
// =============================================================================

// Standard error for testing
var errStd = errors.New("standard error")

// =============================================================================
// Error Creation Tests
// =============================================================================

// TestNew는 New 함수로 생성된 에러의 동작을 검증합니다.
//
// 검증 항목:
//   - 다양한 ErrorType으로 에러 생성
//   - 에러 메시지 정확성
//   - ErrorType 정확성
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
		{
			name:     "Create Timeout error",
			errType:  Timeout,
			message:  "request timeout",
			expected: "request timeout",
		},
		{
			name:     "Create NotFound error",
			errType:  NotFound,
			message:  "resource not found",
			expected: "resource not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.errType, tt.message)
			require.NotNil(t, err, "Error should not be nil")
			assert.Equal(t, tt.expected, err.Error())
			assert.Equal(t, tt.errType, GetType(err))
		})
	}
}

// TestNewf는 Newf 함수로 생성된 포맷팅된 에러의 동작을 검증합니다.
//
// 검증 항목:
//   - 포맷 문자열 처리
//   - 여러 인자 처리
//   - ErrorType 정확성
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
		{
			name:     "Format with no args",
			errType:  Internal,
			format:   "simple message",
			args:     []interface{}{},
			expected: "simple message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Newf(tt.errType, tt.format, tt.args...)
			require.NotNil(t, err, "Error should not be nil")
			assert.Equal(t, tt.expected, err.Error())
			assert.Equal(t, tt.errType, GetType(err))
		})
	}
}

// =============================================================================
// Error Wrapping Tests
// =============================================================================

// TestWrap는 Wrap 함수로 생성된 에러의 동작을 검증합니다.
//
// 검증 항목:
//   - 표준 에러 래핑
//   - nil 에러 래핑
//   - AppError 중첩 래핑
//   - Cause 정확성
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
		{
			name:        "Wrap with empty message",
			cause:       errStd,
			errType:     System,
			message:     "",
			expectedMsg: ": standard error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Wrap(tt.cause, tt.errType, tt.message)
			require.NotNil(t, err, "Error should not be nil")
			assert.Equal(t, tt.expectedMsg, err.Error())
			assert.Equal(t, tt.errType, GetType(err))
			assert.Equal(t, tt.cause, Cause(err))
		})
	}
}

// TestWrapf는 Wrapf 함수로 생성된 포맷팅된 래핑 에러의 동작을 검증합니다.
//
// 검증 항목:
//   - 포맷 문자열 처리
//   - nil 에러 래핑
//   - ErrorType 정확성
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
		{
			name:        "Wrapf with multiple args",
			cause:       errStd,
			errType:     Timeout,
			format:      "timeout after %d seconds on %s",
			args:        []interface{}{30, "api.example.com"},
			expectedMsg: "timeout after 30 seconds on api.example.com: standard error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Wrapf(tt.cause, tt.errType, tt.format, tt.args...)
			require.NotNil(t, err, "Error should not be nil")
			assert.Equal(t, tt.expectedMsg, err.Error())
			assert.Equal(t, tt.errType, GetType(err))
		})
	}
}

// =============================================================================
// Error Type Checking Tests
// =============================================================================

// TestIs는 Is 함수의 에러 타입 확인 동작을 검증합니다.
//
// 검증 항목:
//   - 정확한 타입 매칭
//   - 타입 불일치
//   - 래핑된 에러의 타입 확인
//   - nil 에러 처리
//   - 표준 에러 처리
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

// =============================================================================
// Error Casting Tests
// =============================================================================

// TestAs는 As 함수의 에러 타입 캐스팅 동작을 검증합니다.
//
// 검증 항목:
//   - AppError로 캐스팅 성공
//   - 표준 에러 캐스팅 실패
//   - nil 에러 처리
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
					require.NotNil(t, *appErr, "AppError should not be nil")
					assert.NotEmpty(t, (*appErr).Type)
				}
			}
		})
	}
}

// =============================================================================
// Error Type Extraction Tests
// =============================================================================

// TestGetType는 GetType 함수의 에러 타입 추출 동작을 검증합니다.
//
// 검증 항목:
//   - AppError 타입 추출
//   - 래핑된 AppError 타입 추출
//   - 표준 에러는 Unknown 반환
//   - nil 에러는 Unknown 반환
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
		{"ExecutionFailed Error", New(ExecutionFailed, "msg"), ExecutionFailed},
		{"Unavailable Error", New(Unavailable, "msg"), Unavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetType(tt.err))
		})
	}
}

// =============================================================================
// Error Cause Tests
// =============================================================================

// TestCause는 Cause 함수의 원인 에러 추출 동작을 검증합니다.
//
// 검증 항목:
//   - nil 에러 처리
//   - 표준 에러 (Cause 없음)
//   - New로 생성된 AppError (Cause 없음)
//   - Wrap으로 생성된 AppError (Cause 있음)
//   - 다중 래핑 (직접 Cause만 반환)
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

// TestRootCause는 RootCause 함수의 최상위 원인 에러 추출 동작을 검증합니다.
//
// 검증 항목:
//   - nil 에러 처리
//   - 표준 에러 (자신 반환)
//   - 단일 래핑
//   - 다중 래핑
//   - fmt.Errorf로 래핑된 에러
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

// =============================================================================
// Unwrap Tests
// =============================================================================

// TestUnwrap는 errors.Unwrap과의 호환성을 검증합니다.
//
// 검증 항목:
//   - New로 생성된 AppError는 nil 반환
//   - Wrap으로 생성된 AppError는 Cause 반환
//   - nil 에러 처리
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

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestEdgeCases는 엣지 케이스를 검증합니다.
//
// 검증 항목:
//   - 매우 긴 메시지
//   - Unicode 메시지
//   - 특수 문자 메시지
func TestEdgeCases(t *testing.T) {
	t.Run("Very Long Message", func(t *testing.T) {
		longMsg := strings.Repeat("a", 10000)
		err := New(Internal, longMsg)
		require.NotNil(t, err)
		assert.Equal(t, longMsg, err.Error())
		assert.Equal(t, Internal, GetType(err))
		assert.Len(t, err.Error(), 10000)
	})

	t.Run("Unicode Message", func(t *testing.T) {
		unicodeMsg := "에러 발생 - エラー - 错误 - 🔥"
		err := New(InvalidInput, unicodeMsg)
		assert.Equal(t, unicodeMsg, err.Error())
		assert.Equal(t, InvalidInput, GetType(err))
	})

	t.Run("Special Characters in Message", func(t *testing.T) {
		specialMsg := "error: \n\t\"quoted\" <tag> & ampersand"
		err := New(System, specialMsg)
		assert.Equal(t, specialMsg, err.Error())
	})

	t.Run("Deep Nesting", func(t *testing.T) {
		err := errors.New("root")
		for i := 0; i < 100; i++ {
			err = Wrap(err, Internal, fmt.Sprintf("layer%d", i))
		}
		root := RootCause(err)
		assert.Equal(t, "root", root.Error())
	})
}

// =============================================================================
// Examples (Documentation)
// =============================================================================

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
