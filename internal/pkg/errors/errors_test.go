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
		{"InvalidInput", InvalidInput, "invalid input", "[InvalidInput] invalid input"},
		{"Internal", Internal, "internal server error", "[Internal] internal server error"},
		{"Empty Message", Unknown, "", "[Unknown] "},
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
	assert.Equal(t, "[NotFound] user 123", err.Error())
	assert.Equal(t, NotFound, GetType(err))
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.errType.String())
		})
	}
}

func TestWrap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		cause       error
		errType     ErrorType
		message     string
		expectedMsg string
		expectNil   bool
	}{
		{"StdError", errStd, Internal, "db failed", "[Internal] db failed: standard error", false},
		{"NilError", nil, Unknown, "unknown", "", true},
		{"Nested", New(InvalidInput, "bad"), Internal, "api failed", "[Internal] api failed: [InvalidInput] bad", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Wrap(tt.cause, tt.errType, tt.message)

			if tt.expectNil {
				assert.Nil(t, err, "nil 에러는 래핑하지 않음")
				return
			}

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
	assert.Equal(t, "[System] port 8080 failed: standard error", err.Error())
	assert.Equal(t, System, GetType(err))
}

func TestWrapf_NilError(t *testing.T) {
	t.Parallel()
	err := Wrapf(nil, System, "port %d failed", 8080)
	assert.Nil(t, err, "nil 에러는 래핑하지 않음")
}

// ... existing tests ...

func TestEdgeCases(t *testing.T) {
	t.Parallel()
	t.Run("Long Message", func(t *testing.T) {
		msg := strings.Repeat("x", 1000)
		err := New(Internal, msg)
		assert.Equal(t, "[Internal] "+msg, err.Error())
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
	// Output: [InvalidInput] invalid email
}

func ExampleWrap() {
	err := errors.New("db connection lost")
	wrapped := Wrap(err, System, "failed to query users")
	fmt.Println(wrapped)
	// Output: [System] failed to query users: db connection lost
}

// TestIs_ChainTraversal은 중첩된 AppError에서 Is 함수가 에러 체인 전체를 탐색하는지 검증합니다.
func TestIs_ChainTraversal(t *testing.T) {
	t.Parallel()

	inner := New(NotFound, "record missing")
	outer := Wrap(inner, Internal, "query failed")

	// 최상위 에러 타입 검사
	assert.True(t, Is(outer, Internal), "최상위 에러 타입(Internal)과 일치해야 함")

	// 내부 에러 타입도 검사 가능 (체인 탐색)
	assert.True(t, Is(outer, NotFound), "내부 에러 타입(NotFound)도 검사 가능해야 함")

	// 존재하지 않는 타입은 false
	assert.False(t, Is(outer, Timeout), "체인에 없는 타입(Timeout)은 false")

	// 다중 중첩 테스트
	deep := Wrap(outer, System, "system error")
	assert.True(t, Is(deep, System), "최상위 System 타입")
	assert.True(t, Is(deep, Internal), "중간 Internal 타입")
	assert.True(t, Is(deep, NotFound), "최하위 NotFound 타입")
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

func ExampleIs() {
	err := New(Timeout, "deadline exceeded")
	if Is(err, Timeout) {
		fmt.Println("It was a timeout")
	}
	// Output: It was a timeout
}

func TestAppError_Format(t *testing.T) {
	t.Parallel()

	t.Run("Basic Formatting", func(t *testing.T) {
		err := New(InvalidInput, "bad input")

		assert.Equal(t, "[InvalidInput] bad input", fmt.Sprintf("%v", err))
		assert.Equal(t, "[InvalidInput] bad input", fmt.Sprintf("%s", err))
		assert.Equal(t, `"[InvalidInput] bad input"`, fmt.Sprintf("%q", err))
	})

	t.Run("Detailed Formatting %+v", func(t *testing.T) {
		root := errors.New("root error")
		mid := Wrap(root, Internal, "middleware failed")
		top := Wrap(mid, System, "api request failed")

		output := fmt.Sprintf("%+v", top)

		// Check if output contains all parts of the error chain
		assert.Contains(t, output, "[System] api request failed")
		assert.Contains(t, output, "Caused by:")
		assert.Contains(t, output, "[Internal] middleware failed")
		assert.Contains(t, output, "root error")

		// Verify structure (indentation or order)
		lines := strings.Split(output, "\n")
		// The exact output structure depends on implementation:
		// [System] api request failed
		// Caused by:
		// [Internal] middleware failed
		// Caused by:
		// 	root error

		assert.True(t, len(lines) >= 4)
	})

	t.Run("Internal Wrap Formatting", func(t *testing.T) {
		root := New(NotFound, "user not found")
		wrapped := Wrap(root, Internal, "fetch failed")

		output := fmt.Sprintf("%+v", wrapped)

		assert.Contains(t, output, "[Internal] fetch failed")
		assert.Contains(t, output, "Caused by:")
		assert.Contains(t, output, "[NotFound] user not found")
	})
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestConcurrentErrorCreation 여러 고루틴에서 동시에 에러를 생성하고 조작할 때 안전성을 검증합니다.
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
				// 에러 생성
				err := New(Internal, fmt.Sprintf("error-%d-%d", id, j))
				require.NotNil(t, err)

				// 래핑
				wrapped := Wrap(err, System, "wrapped")
				require.NotNil(t, wrapped)

				// 타입 검사
				assert.True(t, Is(wrapped, Internal), "체인에 Internal 타입이 존재해야 함")
				assert.True(t, Is(wrapped, System), "체인에 System 타입이 존재해야 함")
				assert.Equal(t, System, GetType(wrapped), "최상위 타입은 System이어야 함")

				// RootCause 검사
				root := RootCause(wrapped)
				assert.Equal(t, err, root, "RootCause는 원본 에러를 반환해야 함")
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrentErrorChainTraversal 공유된 에러 체인을 여러 고루틴에서 동시에 읽을 때 안전성을 검증합니다.
func TestConcurrentErrorChainTraversal(t *testing.T) {
	t.Parallel()

	// 공유 에러 체인 생성 (10단계 깊이)
	err := New(NotFound, "root error")
	for i := 0; i < 10; i++ {
		err = Wrap(err, Internal, fmt.Sprintf("layer-%d", i))
	}

	// 여러 고루틴에서 동시 읽기
	const readers = 50
	var wg sync.WaitGroup
	wg.Add(readers)

	for i := 0; i < readers; i++ {
		go func(readerID int) {
			defer wg.Done()

			// 동시 읽기 작업 (여러 번 반복)
			for j := 0; j < 100; j++ {
				// Is 함수로 체인 탐색
				assert.True(t, Is(err, NotFound), "체인에 NotFound 타입이 존재해야 함")
				assert.True(t, Is(err, Internal), "체인에 Internal 타입이 존재해야 함")

				// GetType으로 최상위 타입 확인
				assert.Equal(t, Internal, GetType(err), "최상위 타입은 Internal이어야 함")

				// RootCause로 최하위 에러 확인
				root := RootCause(err)
				assert.Contains(t, root.Error(), "root error", "RootCause는 원본 메시지를 포함해야 함")

				// As 함수로 타입 변환
				var appErr *AppError
				assert.True(t, As(err, &appErr), "AppError로 변환 가능해야 함")
				assert.NotNil(t, appErr, "변환된 AppError는 nil이 아니어야 함")
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrentMixedOperations 에러 생성과 읽기를 동시에 수행할 때 안전성을 검증합니다.
func TestConcurrentMixedOperations(t *testing.T) {
	t.Parallel()

	const workers = 50
	var wg sync.WaitGroup
	wg.Add(workers * 2) // 생성자 + 읽기자

	// 공유 채널로 에러 전달
	errChan := make(chan error, workers)

	// 에러 생성 고루틴들
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				// 다양한 타입의 에러 생성
				var err error
				switch j % 4 {
				case 0:
					err = New(NotFound, fmt.Sprintf("not-found-%d-%d", id, j))
				case 1:
					err = New(InvalidInput, fmt.Sprintf("invalid-%d-%d", id, j))
					err = Wrap(err, Internal, "wrapped")
				case 2:
					err = Newf(Timeout, "timeout-%d-%d", id, j)
					err = Wrap(err, System, "system error")
					err = Wrap(err, Internal, "internal error")
				case 3:
					err = New(Unauthorized, fmt.Sprintf("unauthorized-%d-%d", id, j))
				}

				errChan <- err
			}
		}(i)
	}

	// 에러 읽기 고루틴들
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				err := <-errChan

				// 다양한 검증 작업
				assert.NotNil(t, err, "에러는 nil이 아니어야 함")

				errType := GetType(err)
				assert.NotEqual(t, Unknown, errType, "에러 타입은 Unknown이 아니어야 함")

				root := RootCause(err)
				assert.NotNil(t, root, "RootCause는 nil이 아니어야 함")

				// 타입별 검증
				if Is(err, NotFound) {
					assert.Contains(t, root.Error(), "not-found")
				} else if Is(err, InvalidInput) {
					assert.Contains(t, root.Error(), "invalid")
				} else if Is(err, Timeout) {
					assert.Contains(t, root.Error(), "timeout")
				} else if Is(err, Unauthorized) {
					assert.Contains(t, root.Error(), "unauthorized")
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)
}
