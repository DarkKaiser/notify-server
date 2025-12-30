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

var errStd = errors.New("standard error")

// =============================================================================
// benchmarks
// =============================================================================

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = New(Internal, "error message")
	}
}

func BenchmarkWrap(b *testing.B) {
	err := errors.New("base error")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Wrap(err, Internal, "wrapped message")
	}
}

func BenchmarkRootCause(b *testing.B) {
	// 깊은 에러 체인 생성
	err := errors.New("root")
	for i := 0; i < 50; i++ {
		err = Wrap(err, Internal, "wrap")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RootCause(err)
	}
}

// =============================================================================
// Unit Tests (Enhanced)
// =============================================================================

func TestNew(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		errType  ErrorType
		message  string
		expected string
	}{
		{"InvalidInput", InvalidInput, "invalid input", "invalid input"},
		{"Internal", Internal, "internal server error", "internal server error"},
		{"Empty Message", Unknown, "", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := New(tt.errType, tt.message)
			require.NotNil(t, err)
			assert.Equal(t, tt.expected, err.Error())
			assert.Equal(t, tt.errType, GetType(err))
		})
	}
}

func TestNewf(t *testing.T) {
	t.Parallel()
	err := Newf(NotFound, "user %d", 123)
	require.NotNil(t, err)
	assert.Equal(t, "user 123", err.Error())
	assert.Equal(t, NotFound, GetType(err))
}

func TestWrap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		cause       error
		errType     ErrorType
		message     string
		expectedMsg string
	}{
		{"StdError", errStd, Internal, "db failed", "db failed: standard error"},
		{"NilError", nil, Unknown, "unknown", "unknown"},
		{"Nested", New(InvalidInput, "bad"), Internal, "api failed", "api failed: bad"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Wrap(tt.cause, tt.errType, tt.message)
			require.NotNil(t, err)
			assert.Equal(t, tt.expectedMsg, err.Error())
			assert.Equal(t, tt.errType, GetType(err))
			assert.Equal(t, tt.cause, Cause(err))
		})
	}
}

func TestWrapf(t *testing.T) {
	t.Parallel()
	err := Wrapf(errStd, System, "port %d failed", 8080)
	require.NotNil(t, err)
	assert.Equal(t, "port 8080 failed: standard error", err.Error())
	assert.Equal(t, System, GetType(err))
}

// TestIs_Shadowing은 중첩된 AppError에서 Is 함수가 최상위 에러의 타입만 확인하는지 검증합니다.
// 의도: 상위 레이어의 에러가 하위 레이어의 에러 타입을 "Shadowing" 하기를 기대함.
func TestIs_Shadowing(t *testing.T) {
	t.Parallel()

	inner := New(NotFound, "record missing")
	outer := Wrap(inner, Internal, "query failed")

	// Outer는 Internal이므로 True
	assert.True(t, Is(outer, Internal), "최상위 에러 타입(Internal)과는 일치해야 함")

	// Inner는 NotFound이지만, Wrap되면 상위 타입(Internal)이 우선됨 -> Shadowing
	// errors.As는 체인에서 첫 번째로 발견된 *AppError를 반환하므로 outer를 반환함.
	assert.False(t, Is(outer, NotFound), "내부 에러 타입(NotFound)은 가려져야(Shadowed) 함")

	// 표준 errors.Is와의 차이점:
	// 표준 errors.Is는 Unwrap하며 체인을 뒤지지만,
	// 우리 패키지의 Is는 'Type' 검사를 위해 errors.As를 사용하고,
	// errors.As는 가장 먼저 매칭되는(최상위) *AppError를 찾기 때문.
}

func TestIs(t *testing.T) {
	t.Parallel()
	err := New(Timeout, "timeout")
	assert.True(t, Is(err, Timeout))
	assert.False(t, Is(err, Internal))
	assert.False(t, Is(nil, Timeout))
	assert.False(t, Is(errors.New("std"), Timeout))
}

func TestAs(t *testing.T) {
	t.Parallel()
	var target *AppError
	err := New(Forbidden, "no access")
	assert.True(t, As(err, &target))
	assert.Equal(t, Forbidden, target.Type)

	assert.False(t, As(errors.New("std"), &target))
}

func TestGetType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, Unauthorized, GetType(New(Unauthorized, "")))
	assert.Equal(t, Unknown, GetType(errors.New("std")))
	assert.Equal(t, Unknown, GetType(nil))
}

func TestCause(t *testing.T) {
	t.Parallel()
	// Chain: Wrap -> New
	root := New(InvalidInput, "root")
	wrapped := Wrap(root, Internal, "wrap")

	assert.Equal(t, root, Cause(wrapped))
	assert.Nil(t, Cause(root)) // New로 만든 에러는 Cause가 없음 (nil)
	assert.Nil(t, Cause(nil))
}

func TestRootCause(t *testing.T) {
	t.Parallel()
	// Chain: Wrap(Wrap(Wrap(std)))
	std := errors.New("std root")
	l1 := Wrap(std, Internal, "l1")
	l2 := Wrap(l1, System, "l2")
	l3 := fmt.Errorf("fmt: %w", l2) // 표준 래핑 섞임

	assert.Equal(t, std, RootCause(l3))
	assert.Nil(t, RootCause(nil))
}

func TestUnwrap(t *testing.T) {
	t.Parallel()
	root := errors.New("root")
	wrapped := Wrap(root, Internal, "msg")
	assert.Equal(t, root, errors.Unwrap(wrapped))
}

func TestEdgeCases(t *testing.T) {
	t.Parallel()
	t.Run("Long Message", func(t *testing.T) {
		msg := strings.Repeat("x", 1000)
		err := New(Internal, msg)
		assert.Equal(t, msg, err.Error())
	})
	t.Run("Deep Chain Stack Overflow Check", func(t *testing.T) {
		// 매우 깊은 체인도 스택 오버플로우 없이 RootCause를 찾아야 함
		err := errors.New("base")
		for i := 0; i < 1000; i++ {
			err = Wrap(err, Internal, "wrap")
		}
		assert.Equal(t, "base", RootCause(err).Error())
	})
}

// =============================================================================
// Documentation Examples
// =============================================================================

func ExampleNew() {
	err := New(InvalidInput, "invalid email")
	fmt.Println(err)
	// Output: invalid email
}

func ExampleWrap() {
	err := errors.New("db connection lost")
	wrapped := Wrap(err, System, "failed to query users")
	fmt.Println(wrapped)
	// Output: failed to query users: db connection lost
}

func ExampleIs() {
	err := New(Timeout, "deadline exceeded")
	if Is(err, Timeout) {
		fmt.Println("It was a timeout")
	}
	// Output: It was a timeout
}
