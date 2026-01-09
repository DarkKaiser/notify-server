package errors

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Constants & Variables
// =============================================================================

var errStd = errors.New("standard error")

// =============================================================================
// Construction Tests
// =============================================================================

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		errType ErrorType
		message string
	}{
		{
			name:    "Normal Creation",
			errType: InvalidInput,
			message: "invalid input parameter",
		},
		{
			name:    "Empty Message",
			errType: Internal,
			message: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.errType, tt.message)

			assert.NotNil(t, err)
			expectedMsg := fmt.Sprintf("[%s] %s", tt.errType, tt.message)
			assert.Equal(t, expectedMsg, err.Error(), "Error message should match formatted string")

			// Type Check
			assert.Equal(t, tt.errType, GetType(err))

			// Stack Check
			assert.NotEmpty(t, err.(*AppError).Stack(), "Stack trace should be captured")
		})
	}
}

func TestNewf(t *testing.T) {
	t.Parallel()

	err := Newf(NotFound, "user %d not found", 123)

	assert.NotNil(t, err)
	assert.Equal(t, "[NotFound] user 123 not found", err.Error())
	assert.Equal(t, NotFound, GetType(err))
}

// =============================================================================
// Wrapping Tests
// =============================================================================

func TestWrap(t *testing.T) {
	t.Parallel()

	t.Run("Wrap Standard Error", func(t *testing.T) {
		err := Wrap(errStd, System, "wrapper message")

		assert.NotNil(t, err)
		assert.Equal(t, System, GetType(err))
		assert.Contains(t, err.Error(), "wrapper message")
		assert.Contains(t, err.Error(), "standard error")
		assert.Equal(t, errStd, errors.Unwrap(err))
	})

	t.Run("Wrap Nil Error", func(t *testing.T) {
		err := Wrap(nil, Internal, "should be nil")
		assert.Nil(t, err)
	})

	t.Run("Nested Wrapping", func(t *testing.T) {
		err1 := New(NotFound, "root error")
		err2 := Wrap(err1, Internal, "first wrap")
		err3 := Wrap(err2, ExecutionFailed, "second wrap")

		// Check Chain
		assert.True(t, Is(err3, ExecutionFailed))
		assert.True(t, Is(err3, Internal))
		assert.True(t, Is(err3, NotFound))

		// Check Root Cause
		assert.Equal(t, err1, RootCause(err3))
	})
}

func TestWrapf(t *testing.T) {
	t.Parallel()

	t.Run("Wrapf Standard Error", func(t *testing.T) {
		err := Wrapf(errStd, Unauthorized, "access denied: %s", "admin")

		assert.NotNil(t, err)
		assert.Equal(t, Unauthorized, GetType(err))
		assert.Contains(t, err.Error(), "access denied: admin")
		assert.Contains(t, err.Error(), "standard error")
	})

	t.Run("Wrapf Nil Error", func(t *testing.T) {
		err := Wrapf(nil, Internal, "should be nil")
		assert.Nil(t, err)
	})
}

// =============================================================================
// Inspection Tests (Is, As, GetType, RootCause)
// =============================================================================

func TestIs(t *testing.T) {
	t.Parallel()

	errInvalid := New(InvalidInput, "invalid")
	errWrapped := Wrap(errInvalid, System, "system")

	// Standard checks
	assert.True(t, Is(errInvalid, InvalidInput))
	assert.False(t, Is(errInvalid, Internal))

	// Chain checks
	assert.True(t, Is(errWrapped, System))
	assert.True(t, Is(errWrapped, InvalidInput)) // Nested check
	assert.False(t, Is(errWrapped, NotFound))

	// Nil checks
	assert.False(t, Is(nil, Internal))
}

func TestIs_StandardCompatibility(t *testing.T) {
	t.Parallel()

	// Ensure AppError works with standard errors.Is
	err := New(NotFound, "missing")
	assert.True(t, errors.Is(err, NotFound))

	wrapped := Wrap(err, Internal, "wrapped")
	assert.True(t, errors.Is(wrapped, NotFound))
}

func TestAs(t *testing.T) {
	t.Parallel()

	err := New(Conflict, "conflict occurred")
	wrapped := Wrap(err, Internal, "wrapped")

	var appErr *AppError
	if assert.True(t, As(wrapped, &appErr)) {
		assert.Equal(t, Internal, appErr.errType)
		assert.Equal(t, "wrapped", appErr.message)
	}

	// Verify we can allow extracting the inner error via As if needed,
	// though As typically matches the first compatible error in chain.
	// Since AppError is the type for both wrapper and cause, it matches wrapper first.
}

func TestGetType(t *testing.T) {
	t.Parallel()

	assert.Equal(t, NotFound, GetType(New(NotFound, "err")))
	assert.Equal(t, Unknown, GetType(errStd))
	assert.Equal(t, Unknown, GetType(nil))

	// Wrapper type takes precedence
	wrapped := Wrap(New(NotFound, "err"), Internal, "wrap")
	assert.Equal(t, Internal, GetType(wrapped))
}

func TestRootCause(t *testing.T) {
	t.Parallel()

	assert.Nil(t, RootCause(nil))
	assert.Equal(t, errStd, RootCause(errStd))

	err := New(System, "root")
	wrapped := Wrap(err, Internal, "wrap")
	assert.Equal(t, err, RootCause(wrapped))

	// Multi-level
	wrapped2 := Wrap(wrapped, Conflict, "wrap2")
	assert.Equal(t, err, RootCause(wrapped2))
}

// =============================================================================
// Interface Implementation Tests
// =============================================================================

func TestAppError_Error_Interface(t *testing.T) {
	t.Parallel()

	err := New(Timeout, "timeout")
	assert.Equal(t, "[Timeout] timeout", err.Error())

	wrapped := Wrap(err, Internal, "fail")
	assert.Equal(t, "[Internal] fail: [Timeout] timeout", wrapped.Error())
}

func TestAppError_Unwrap_Interface(t *testing.T) {
	t.Parallel()

	err1 := New(InvalidInput, "err1")
	err2 := Wrap(err1, Internal, "err2")

	// Direct cast to check Unwrap() method existence and behavior
	appErr, ok := err2.(*AppError)
	require.True(t, ok)
	assert.Equal(t, err1, appErr.Unwrap())
}

func TestAppError_Format(t *testing.T) {
	t.Parallel()

	t.Run("Simple Format %v", func(t *testing.T) {
		err := New(Internal, "simple")
		assert.Equal(t, "[Internal] simple", fmt.Sprintf("%v", err))
		assert.Equal(t, "[Internal] simple", fmt.Sprintf("%s", err))
	})

	t.Run("Quote Format %q", func(t *testing.T) {
		err := New(Internal, "quote")
		assert.Equal(t, "\"[Internal] quote\"", fmt.Sprintf("%q", err))
	})

	t.Run("Detail Format %+v", func(t *testing.T) {
		err := New(Internal, "detail")
		output := fmt.Sprintf("%+v", err)

		assert.Contains(t, output, "[Internal] detail")
		assert.Contains(t, output, "Stack trace:")
		// Function name check - assuming test file name is errors_test.go
		assert.Contains(t, output, "errors_test.go")
	})

	t.Run("Detail Format Chain", func(t *testing.T) {
		root := New(NotFound, "root")
		wrapped := Wrap(root, Internal, "wrapper")
		output := fmt.Sprintf("%+v", wrapped)

		// Should contain both messages
		assert.Contains(t, output, "[Internal] wrapper")
		assert.Contains(t, output, "[NotFound] root")

		// Stack trace should be present
		assert.Contains(t, output, "Stack trace:")
	})
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestConcurrency(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	count := 100
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func(idx int) {
			defer wg.Done()

			// Concurrent Create
			err := Newf(Internal, "concurrent %d", idx)
			assert.NotNil(t, err)

			// Concurrent Wrap
			wrapped := Wrap(err, System, "wrapped")
			assert.True(t, Is(wrapped, Internal))

			// Concurrent Inspect
			_ = RootCause(wrapped)
			_ = fmt.Sprintf("%+v", wrapped)
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// Accessor Tests (Coverage for Message, Stack methods)
// =============================================================================

func TestAccessors(t *testing.T) {
	t.Parallel()

	err := New(Unauthorized, "access denied")
	appErr, ok := err.(*AppError)
	require.True(t, ok)

	assert.Equal(t, "access denied", appErr.Message())
	assert.NotEmpty(t, appErr.Stack())

	// Test nil stack case (manually crafted)
	emptyErr := &AppError{errType: Internal}
	assert.Nil(t, emptyErr.Stack())
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
	baseErr := New(System, "db error")
	err := Wrap(baseErr, Internal, "query failed")
	fmt.Println(err)
	// Output: [Internal] query failed: [System] db error
}

func ExampleIs() {
	err := New(NotFound, "missing")
	wrapped := Wrap(err, Internal, "process failed")

	if Is(wrapped, NotFound) {
		fmt.Println("Is NotFound")
	}
	// Output: Is NotFound
}
