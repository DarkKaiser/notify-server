package errors

import (
	"errors"
	"fmt"
	"strings"
	"sync"
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

func BenchmarkIs(b *testing.B) {
	// 깊은 에러 체인 생성
	err := New(NotFound, "not found")
	for i := 0; i < 10; i++ {
		err = Wrap(err, Internal, "wrap")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Is(err, NotFound)
	}
}

// =============================================================================
// Basic Error Creation Tests
// =============================================================================

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		errType ErrorType
		message string
	}{
		{
			name:    "InvalidInput",
			errType: InvalidInput,
			message: "invalid input",
		},
		{
			name:    "Internal",
			errType: Internal,
			message: "internal error",
		},
		{
			name:    "Empty Message",
			errType: NotFound,
			message: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.errType, tt.message)

			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tt.message)
			assert.True(t, Is(err, tt.errType))
		})
	}
}

func TestNewf(t *testing.T) {
	t.Parallel()

	err := Newf(InvalidInput, "error code: %d", 404)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "error code: 404")
	assert.True(t, Is(err, InvalidInput))
}

func TestErrorType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		errType  ErrorType
		expected string
	}{
		{"Unknown", Unknown, "Unknown"},
		{"Internal", Internal, "Internal"},
		{"System", System, "System"},
		{"Unauthorized", Unauthorized, "Unauthorized"},
		{"Forbidden", Forbidden, "Forbidden"},
		{"InvalidInput", InvalidInput, "InvalidInput"},
		{"Conflict", Conflict, "Conflict"},
		{"NotFound", NotFound, "NotFound"},
		{"ExecutionFailed", ExecutionFailed, "ExecutionFailed"},
		{"Timeout", Timeout, "Timeout"},
		{"Unavailable", Unavailable, "Unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.errType.String())
		})
	}
}

// =============================================================================
// Wrapping Tests
// =============================================================================

func TestWrap(t *testing.T) {
	t.Parallel()

	t.Run("StdError", func(t *testing.T) {
		wrapped := Wrap(errStd, Internal, "wrapped message")

		assert.NotNil(t, wrapped)
		assert.Contains(t, wrapped.Error(), "wrapped message")
		assert.Contains(t, wrapped.Error(), "standard error")
		assert.True(t, Is(wrapped, Internal))
	})

	t.Run("NilError", func(t *testing.T) {
		wrapped := Wrap(nil, Internal, "should be nil")
		assert.Nil(t, wrapped)
	})

	t.Run("Nested", func(t *testing.T) {
		err1 := New(NotFound, "not found")
		err2 := Wrap(err1, Internal, "internal error")
		err3 := Wrap(err2, System, "system error")

		assert.True(t, Is(err3, System))
		assert.True(t, Is(err3, Internal))
		assert.True(t, Is(err3, NotFound))
	})
}

func TestWrapf(t *testing.T) {
	t.Parallel()

	wrapped := Wrapf(errStd, Internal, "error code: %d", 500)

	assert.NotNil(t, wrapped)
	assert.Contains(t, wrapped.Error(), "error code: 500")
	assert.Contains(t, wrapped.Error(), "standard error")
}

func TestWrapf_NilError(t *testing.T) {
	t.Parallel()

	wrapped := Wrapf(nil, Internal, "should be nil")
	assert.Nil(t, wrapped)
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("Long Message", func(t *testing.T) {
		longMsg := strings.Repeat("a", 10000)
		err := New(Internal, longMsg)

		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), longMsg)
	})

	t.Run("Deep Chain Stack Overflow Check", func(t *testing.T) {
		err := New(Internal, "base")
		for i := 0; i < 1000; i++ {
			err = Wrap(err, Internal, "wrap")
		}

		// RootCause should not stack overflow
		root := RootCause(err)
		assert.NotNil(t, root)
	})
}

// =============================================================================
// Is Function Tests
// =============================================================================

func TestIs_ChainTraversal(t *testing.T) {
	t.Parallel()

	err1 := New(NotFound, "not found")
	err2 := Wrap(err1, Internal, "internal")
	err3 := Wrap(err2, System, "system")

	assert.True(t, Is(err3, System))
	assert.True(t, Is(err3, Internal))
	assert.True(t, Is(err3, NotFound))
	assert.False(t, Is(err3, InvalidInput))
}

func TestIs(t *testing.T) {
	t.Parallel()

	err := New(InvalidInput, "test")

	assert.True(t, Is(err, InvalidInput))
	assert.False(t, Is(err, Internal))
	assert.False(t, Is(nil, InvalidInput))
}

// =============================================================================
// As Function Tests
// =============================================================================

func TestAs(t *testing.T) {
	t.Parallel()

	err := New(Internal, "test error")

	var appErr *AppError
	assert.True(t, As(err, &appErr))
	assert.Equal(t, Internal, appErr.Type)
	assert.Equal(t, "test error", appErr.Message)
}

// =============================================================================
// GetType Tests
// =============================================================================

func TestGetType(t *testing.T) {
	t.Parallel()

	err := New(NotFound, "not found")
	assert.Equal(t, NotFound, GetType(err))

	assert.Equal(t, Unknown, GetType(nil))
	assert.Equal(t, Unknown, GetType(errStd))
}

// =============================================================================
// RootCause Tests
// =============================================================================

func TestRootCause(t *testing.T) {
	t.Parallel()

	err1 := New(NotFound, "not found")
	err2 := Wrap(err1, Internal, "internal")
	err3 := Wrap(err2, System, "system")

	root := RootCause(err3)
	assert.Equal(t, err1, root)

	assert.Nil(t, RootCause(nil))
}

// =============================================================================
// Unwrap Tests
// =============================================================================

func TestUnwrap(t *testing.T) {
	t.Parallel()

	err1 := New(NotFound, "not found")
	err2 := Wrap(err1, Internal, "internal")

	var appErr *AppError
	require.True(t, As(err2, &appErr))

	unwrapped := appErr.Unwrap()
	assert.Equal(t, err1, unwrapped)
}

// =============================================================================
// Format Tests
// =============================================================================

func TestAppError_Format(t *testing.T) {
	t.Parallel()

	t.Run("Basic Formatting", func(t *testing.T) {
		err := New(Internal, "test error")

		// %s, %v
		assert.Contains(t, fmt.Sprintf("%s", err), "[Internal] test error")
		assert.Contains(t, fmt.Sprintf("%v", err), "[Internal] test error")

		// %q
		quoted := fmt.Sprintf("%q", err)
		assert.Contains(t, quoted, "Internal")
		assert.Contains(t, quoted, "test error")
	})

	t.Run("Detailed Formatting %+v", func(t *testing.T) {
		err := New(InvalidInput, "validation failed")
		detailed := fmt.Sprintf("%+v", err)

		assert.Contains(t, detailed, "[InvalidInput] validation failed")
	})

	t.Run("Internal Wrap Formatting", func(t *testing.T) {
		err1 := New(NotFound, "not found")
		err2 := Wrap(err1, Internal, "query failed")

		detailed := fmt.Sprintf("%+v", err2)
		assert.Contains(t, detailed, "[Internal] query failed")
		assert.Contains(t, detailed, "Caused by:")
		assert.Contains(t, detailed, "[NotFound] not found")
	})
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestConcurrentErrorCreation(t *testing.T) {
	t.Parallel()

	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				err := New(Internal, fmt.Sprintf("error %d-%d", id, j))
				assert.NotNil(t, err)
				assert.True(t, Is(err, Internal))
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentErrorChainTraversal(t *testing.T) {
	t.Parallel()

	err := New(NotFound, "base")
	for i := 0; i < 10; i++ {
		err = Wrap(err, Internal, fmt.Sprintf("wrap %d", i))
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			assert.True(t, Is(err, NotFound))
			assert.True(t, Is(err, Internal))
			root := RootCause(err)
			assert.NotNil(t, root)
		}()
	}

	wg.Wait()
}

func TestConcurrentMixedOperations(t *testing.T) {
	t.Parallel()

	baseErr := New(System, "system error")

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			err := New(InvalidInput, fmt.Sprintf("new %d", id))
			assert.NotNil(t, err)
		}(i)

		go func(id int) {
			defer wg.Done()
			err := Wrap(baseErr, Internal, fmt.Sprintf("wrap %d", id))
			assert.True(t, Is(err, System))
		}(i)

		go func() {
			defer wg.Done()
			_ = GetType(baseErr)
			_ = RootCause(baseErr)
		}()
	}

	wg.Wait()
}

// =============================================================================
// Stack Trace Tests
// =============================================================================

// TestStackTrace는 스택 트레이스가 올바르게 캡처되는지 검증합니다.
func TestStackTrace(t *testing.T) {
	t.Parallel()

	err := New(InvalidInput, "test error")

	var appErr *AppError
	require.True(t, As(err, &appErr), "Should be AppError")

	// 스택이 캡처되었는지 확인
	assert.NotEmpty(t, appErr.Stack, "Stack should be captured")

	// 최대 5개 프레임으로 제한되는지 확인
	assert.LessOrEqual(t, len(appErr.Stack), 5, "Stack should be limited to 5 frames")

	// 첫 번째 프레임이 현재 테스트 함수인지 확인
	if len(appErr.Stack) > 0 {
		assert.Equal(t, "errors_test.go", appErr.Stack[0].File)
		assert.Contains(t, appErr.Stack[0].Function, "TestStackTrace")
	}
}

// TestStackTraceFormat은 %+v 포맷으로 스택 트레이스가 출력되는지 검증합니다.
func TestStackTraceFormat(t *testing.T) {
	t.Parallel()

	err := New(ExecutionFailed, "operation failed")

	// %v 포맷: 스택 미포함
	simpleOutput := fmt.Sprintf("%v", err)
	assert.Contains(t, simpleOutput, "[ExecutionFailed] operation failed")
	assert.NotContains(t, simpleOutput, "Stack trace")

	// %+v 포맷: 스택 포함
	detailedOutput := fmt.Sprintf("%+v", err)
	assert.Contains(t, detailedOutput, "[ExecutionFailed] operation failed")
	assert.Contains(t, detailedOutput, "Stack trace:")
	assert.Contains(t, detailedOutput, "errors_test.go")
	assert.Contains(t, detailedOutput, "TestStackTraceFormat")
}

// TestStackTraceChain은 에러 체인에서 각 레벨의 스택이 올바르게 캡처되는지 검증합니다.
func TestStackTraceChain(t *testing.T) {
	t.Parallel()

	// 3단계 에러 체인 생성
	err1 := New(System, "database error")
	err2 := Wrap(err1, Internal, "query failed")
	err3 := Wrap(err2, ExecutionFailed, "operation failed")

	// 각 레벨의 스택 확인
	var appErr *AppError
	require.True(t, As(err3, &appErr))
	assert.NotEmpty(t, appErr.Stack, "Top level should have stack")

	// Cause의 스택도 확인
	var causeErr *AppError
	require.True(t, As(appErr.Cause, &causeErr))
	assert.NotEmpty(t, causeErr.Stack, "Cause should have stack")

	// %+v 출력에 모든 스택이 포함되는지 확인
	output := fmt.Sprintf("%+v", err3)
	assert.Contains(t, output, "[ExecutionFailed] operation failed")
	assert.Contains(t, output, "[Internal] query failed")
	assert.Contains(t, output, "[System] database error")

	// 여러 개의 "Stack trace:" 섹션이 있어야 함
	stackTraceCount := 0
	for i := 0; i < len(output)-12; i++ {
		if output[i:i+12] == "Stack trace:" {
			stackTraceCount++
		}
	}
	assert.GreaterOrEqual(t, stackTraceCount, 2, "Should have multiple stack traces in chain")
}

// TestStackTraceNilError는 nil 에러를 Wrap할 때 스택이 캡처되지 않는지 확인합니다.
func TestStackTraceNilError(t *testing.T) {
	t.Parallel()

	err := Wrap(nil, Internal, "should be nil")
	assert.Nil(t, err, "Wrapping nil should return nil")
}

// TestStackTraceWrapf는 Wrapf 함수에서도 스택이 올바르게 캡처되는지 확인합니다.
func TestStackTraceWrapf(t *testing.T) {
	t.Parallel()

	baseErr := New(System, "base error")
	err := Wrapf(baseErr, Internal, "wrapped: %s", "test")

	var appErr *AppError
	require.True(t, As(err, &appErr))
	assert.NotEmpty(t, appErr.Stack, "Wrapf should capture stack")
	assert.Contains(t, appErr.Message, "wrapped: test")
}

// TestStackTraceNewf는 Newf 함수에서도 스택이 올바르게 캡처되는지 확인합니다.
func TestStackTraceNewf(t *testing.T) {
	t.Parallel()

	err := Newf(InvalidInput, "error code: %d", 404)

	var appErr *AppError
	require.True(t, As(err, &appErr))
	assert.NotEmpty(t, appErr.Stack, "Newf should capture stack")
	assert.Contains(t, appErr.Message, "error code: 404")
}

// =============================================================================
// Example Tests
// =============================================================================

func ExampleNew() {
	err := New(NotFound, "user not found")
	fmt.Println(err)
	// Output: [NotFound] user not found
}

func ExampleWrap() {
	baseErr := New(System, "database connection failed")
	wrappedErr := Wrap(baseErr, Internal, "failed to fetch user")
	fmt.Println(wrappedErr)
	// Output: [Internal] failed to fetch user: [System] database connection failed
}

func ExampleIs() {
	err := New(NotFound, "resource not found")
	wrapped := Wrap(err, Internal, "operation failed")

	if Is(wrapped, NotFound) {
		fmt.Println("NotFound error detected in chain")
	}
	// Output: NotFound error detected in chain
}
