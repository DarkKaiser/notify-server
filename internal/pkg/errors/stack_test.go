package errors

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Stack Trace Tests
// =============================================================================

// TestCaptureStack_Direct verifies captureStack implementation details directly.
// Since captureStack is unexported, we test it within the package.
func TestCaptureStack_Direct(t *testing.T) {
	t.Parallel()

	// 1. Verify basic capture
	frames := captureStack(0)
	assert.NotEmpty(t, frames, "Should capture at least one frame")

	// The stack should contain this function
	found := false
	for _, frame := range frames {
		if frame.File == "stack_test.go" && strings.Contains(frame.Function, "TestCaptureStack_Direct") {
			found = true
			break
		}
	}
	assert.True(t, found, "Stack trace should contain TestCaptureStack_Direct in stack_test.go")

	// 2. Verify skip count
	// Calling captureStack(1) should skip this frame and show the caller (runtime runner)
	framesSkipped := captureStack(1)
	assert.NotEmpty(t, framesSkipped)
	if len(framesSkipped) > 0 {
		assert.NotEqual(t, "TestCaptureStack_Direct", framesSkipped[0].Function)
	}
}

// TestCaptureStack_DepthLimit verifies that stack capture respects the max depth.
func TestCaptureStack_DepthLimit(t *testing.T) {
	t.Parallel()

	// Recursive function to generate deep stack
	var recurse func(n int) []StackFrame
	recurse = func(n int) []StackFrame {
		if n == 0 {
			return captureStack(0)
		}
		return recurse(n - 1)
	}

	// Call with depth > 5 (maxFrames in stack.go)
	frames := recurse(10)

	// Since stack.go defines maxFrames = 5, we verify this limit
	const expectedMaxFrames = 5
	assert.LessOrEqual(t, len(frames), expectedMaxFrames, "Stack should be limited to max frames")
}

// TestStackTrace_Integration verifies high-level behavior via public API.
func TestStackTrace_Integration(t *testing.T) {
	t.Parallel()

	err := New(InvalidInput, "test error")

	var appErr *AppError
	require.True(t, As(err, &appErr), "Should be AppError")

	stack := appErr.Stack()
	assert.NotEmpty(t, stack, "Stack should be captured")
	assert.LessOrEqual(t, len(stack), 5, "Stack should be limited to 5 frames")

	if len(stack) > 0 {
		// Verify file path is base name only
		assert.NotContains(t, stack[0].File, "/", "File should be base name only")
		assert.NotContains(t, stack[0].File, "\\", "File should be base name only")
		assert.Equal(t, "stack_test.go", stack[0].File)

		// Verify function name
		assert.Contains(t, stack[0].Function, "TestStackTrace_Integration")
	}
}

// TestStackTrace_Format verifies %+v formatting for stack traces.
func TestStackTrace_Format(t *testing.T) {
	t.Parallel()

	err := New(ExecutionFailed, "operation failed")

	// %v format: no stack
	simpleOutput := fmt.Sprintf("%v", err)
	assert.Contains(t, simpleOutput, "[ExecutionFailed] operation failed")
	assert.NotContains(t, simpleOutput, "Stack trace")

	// %+v format: with stack
	detailedOutput := fmt.Sprintf("%+v", err)
	assert.Contains(t, detailedOutput, "[ExecutionFailed] operation failed")
	assert.Contains(t, detailedOutput, "Stack trace:")
	assert.Contains(t, detailedOutput, "stack_test.go")
	assert.Contains(t, detailedOutput, "TestStackTrace_Format")
}

// TestStackTrace_Chain verifies stack handling in error chains.
func TestStackTrace_Chain(t *testing.T) {
	t.Parallel()

	// Create a chain: err3 -> err2 -> err1
	err1 := New(System, "database error")
	err2 := Wrap(err1, Internal, "query failed")
	err3 := Wrap(err2, ExecutionFailed, "operation failed")

	// %+v should show usage stack trace once (typically for the root or wherever printed)
	// Our current implementation prints stack only if cause is NOT AppError or nil.
	// But wait, the standard library behavior is to print stack if available.
	// Let's check the implementation logic in errors.go:
	// if e.cause == nil || !errors.As(e.cause, &target) { print stack }
	// This means stack is printed only for the 'leaf' AppError in the chain (the one wrapping a non-AppError or nothing).

	// In this case:
	// err3 wraps err2 (AppError) -> Stack NOT printed
	// err2 wraps err1 (AppError) -> Stack NOT printed
	// err1 wraps nothing -> Stack printed

	output := fmt.Sprintf("%+v", err3)

	// Check content presence
	assert.Contains(t, output, "[ExecutionFailed] operation failed")
	assert.Contains(t, output, "[Internal] query failed")
	assert.Contains(t, output, "[System] database error")

	// Count "Stack trace:" occurrences
	// It should appear exactly once (for err1)
	stackTraceCount := 0
	for i := 0; i < len(output)-11; i++ { // "Stack trace:" is 12 chars? No, "Stack trace:" is 12 chars.
		if output[i:i+12] == "Stack trace:" {
			stackTraceCount++
		}
	}
	assert.Equal(t, 1, stackTraceCount, "Should have exactly one stack trace in chain (for the root cause)")
}

// TestInitialStack_Helper verifies helpers like Newf/Wrapf correct stack skip.
func TestInitialStack_Helper(t *testing.T) {
	t.Parallel()

	// Newf
	err := Newf(InvalidInput, "error code: %d", 404)
	var appErr *AppError
	require.True(t, As(err, &appErr))
	if len(appErr.Stack()) > 0 {
		assert.Equal(t, "stack_test.go", appErr.Stack()[0].File)
		assert.Contains(t, appErr.Stack()[0].Function, "TestInitialStack_Helper")
	}

	// Wrapf
	baseErr := New(System, "base")
	errWrap := Wrapf(baseErr, Internal, "wrapped: %s", "test")
	require.True(t, As(errWrap, &appErr))
	if len(appErr.Stack()) > 0 {
		assert.Equal(t, "stack_test.go", appErr.Stack()[0].File)
		assert.Contains(t, appErr.Stack()[0].Function, "TestInitialStack_Helper")
	}
}

// TestCaptureStack_Concurrency ensures thread safety (though stateless).
func TestCaptureStack_Concurrency(t *testing.T) {
	t.Parallel()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			frames := captureStack(0)
			assert.NotEmpty(t, frames)
			assert.LessOrEqual(t, len(frames), 5)
		}()
	}
	wg.Wait()
}

// TestCaptureStack_NilSafety ensures no panic on edge cases (though unlikely with current impl).
func TestCaptureStack_NilSafety(t *testing.T) {
	t.Parallel()

	// Just ensuring it doesn't crash with weird skip values
	frames := captureStack(1000)
	assert.Empty(t, frames)
}
