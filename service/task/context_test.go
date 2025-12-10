package task

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestTaskContext_Immutability verifies that With* methods return new instances
// and do not modify the original context. This is crucial for thread safety.
func TestTaskContext_Immutability(t *testing.T) {
	baseCtx := NewTaskContext()

	// Create derived context
	ctx1 := baseCtx.WithTask("ID1", "CMD1")

	// Check base context is unchanged
	assert.Empty(t, baseCtx.GetID())
	assert.Empty(t, baseCtx.GetCommandID())

	// Check derived context has values
	assert.Equal(t, ID("ID1"), ctx1.GetID())
	assert.Equal(t, CommandID("CMD1"), ctx1.GetCommandID())

	// Create another derived context from base
	ctx2 := baseCtx.WithTask("ID2", "CMD2")

	// Check ctx1 is unaffected by ctx2 creation
	assert.Equal(t, ID("ID1"), ctx1.GetID())

	// Check ctx2 has its own values
	assert.Equal(t, ID("ID2"), ctx2.GetID())
}

// TestTaskContext_StandardCompliance verifies that TaskContext behaves like a standard context.Context
// and works correctly with standard library functions.
func TestTaskContext_StandardCompliance(t *testing.T) {
	tCtx := NewTaskContext().WithTitle("Test")

	// 1. Wrap with standard context.WithCancel
	ctx, cancel := context.WithCancel(tCtx)

	// Should preserve TaskContext values even when wrapped
	// Note: standard wrapper hides specialized methods, so we access via Value() or type assertion if supported by wrapper (std wrapper doesn't support type assertion back to inner usually, but Value passes through)
	// TaskContext's functional accessors (GetTitle) won't work on `ctx` directly because `ctx` is `*cancelCtx`.
	// But `tCtx.Value` should still work if we passed `ctx` to something consuming context.Context.
	assert.Equal(t, "Test", ctx.Value(taskCtxKeyTitle))

	// 2. Verify cancellation propagation
	cancel()
	select {
	case <-ctx.Done():
		assert.Equal(t, context.Canceled, ctx.Err())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should be canceled")
	}

	// 3. Verify Deadline propagation
	deadlineCtx, _ := context.WithTimeout(tCtx, 100*time.Millisecond)
	dl, ok := deadlineCtx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(100*time.Millisecond), dl, 10*time.Millisecond)
}

// TestTaskContext_Accessors verifies all setters and getters.
func TestTaskContext_Accessors(t *testing.T) {
	ctx := NewTaskContext().
		WithTask("TASK_01", "CMD_01").
		WithInstanceID("INST_01", 12345).
		WithTitle("My Notification").
		WithError()

	assert.Equal(t, ID("TASK_01"), ctx.GetID())
	assert.Equal(t, CommandID("CMD_01"), ctx.GetCommandID())
	assert.Equal(t, InstanceID("INST_01"), ctx.GetInstanceID())
	assert.Equal(t, int64(12345), ctx.GetElapsedTimeAfterRun())
	assert.Equal(t, "My Notification", ctx.GetTitle())
	assert.True(t, ctx.IsErrorOccurred())
}

// TestTaskContext_Defaults verifies default values when keys are missing.
func TestTaskContext_Defaults(t *testing.T) {
	ctx := NewTaskContext()

	assert.Empty(t, ctx.GetID())
	assert.Empty(t, ctx.GetCommandID())
	assert.Empty(t, ctx.GetInstanceID())
	assert.Equal(t, int64(0), ctx.GetElapsedTimeAfterRun())
	assert.Empty(t, ctx.GetTitle())
	assert.False(t, ctx.IsErrorOccurred())
}

// TestTaskContext_Concurrency verifies thread safety of the immutable design.
func TestTaskContext_Concurrency(t *testing.T) {
	baseCtx := NewTaskContext()
	wg := sync.WaitGroup{}

	// Create 100 concurrent goroutines deriving contexts
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Each goroutine creates its own derived context
			myCtx := baseCtx.WithTitle("Title").WithTask(ID("ID"), CommandID("CMD"))

			// Verify value immediately
			assert.Equal(t, "Title", myCtx.GetTitle())
		}(i)
	}

	wg.Wait()
}
