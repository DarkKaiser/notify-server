package contract

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ensure TaskContext implements context.Context at compile time
var _ context.Context = (*taskContext)(nil)
var _ TaskContext = (*taskContext)(nil)

// TestTaskContext_Immutability verifies that With* methods return new instances
// and do not modify the original context. This effectively tests the "Copy-on-Write" behavior.
func TestTaskContext_Immutability(t *testing.T) {
	t.Parallel()

	baseCtx := NewTaskContext()

	// 1. Create independent branches
	ctxA := baseCtx.WithTask("ID_A", "CMD_A")
	ctxB := baseCtx.WithTask("ID_B", "CMD_B")

	// 2. Verify Base remains empty
	assert.Empty(t, baseCtx.GetTaskID())
	assert.Empty(t, baseCtx.GetTaskCommandID())

	// 3. Verify Branch A
	assert.Equal(t, TaskID("ID_A"), ctxA.GetTaskID())
	assert.Equal(t, TaskCommandID("CMD_A"), ctxA.GetTaskCommandID())

	// 4. Verify Branch B (Should not be affected by A)
	assert.Equal(t, TaskID("ID_B"), ctxB.GetTaskID())
	assert.Equal(t, TaskCommandID("CMD_B"), ctxB.GetTaskCommandID())
}

// TestTaskContext_Accessors verifies all setters and getters using table-driven tests.
func TestTaskContext_Accessors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		transform    func(TaskContext) TaskContext
		verify       func(*testing.T, TaskContext)
		expectFields bool // true if we expect struct fields to be populated
	}{
		{
			name: "WithTask Sets ID and CommandID",
			transform: func(ctx TaskContext) TaskContext {
				return ctx.WithTask("TASK_001", "CMD_START")
			},
			verify: func(t *testing.T, ctx TaskContext) {
				assert.Equal(t, TaskID("TASK_001"), ctx.GetTaskID())
				assert.Equal(t, TaskCommandID("CMD_START"), ctx.GetTaskCommandID())
				// Standard Context Value Check
				assert.Equal(t, TaskID("TASK_001"), ctx.Value(ctxKeyTaskID))
				assert.Equal(t, TaskCommandID("CMD_START"), ctx.Value(ctxKeyCommandID))
			},
		},
		{
			name: "WithTaskInstanceID Sets Instance Metadata",
			transform: func(ctx TaskContext) TaskContext {
				return ctx.WithTaskInstanceID("INST_999", 500)
			},
			verify: func(t *testing.T, ctx TaskContext) {
				assert.Equal(t, TaskInstanceID("INST_999"), ctx.GetTaskInstanceID())
				assert.Equal(t, int64(500), ctx.GetElapsedTimeAfterRun())
				// Standard Context Value Check
				assert.Equal(t, TaskInstanceID("INST_999"), ctx.Value(ctxKeyInstanceID))
				assert.Equal(t, int64(500), ctx.Value(ctxKeyElapsedTimeAfterRun))
			},
		},
		{
			name: "WithTitle Sets Title",
			transform: func(ctx TaskContext) TaskContext {
				return ctx.WithTitle("Critical Alert")
			},
			verify: func(t *testing.T, ctx TaskContext) {
				assert.Equal(t, "Critical Alert", ctx.GetTitle())
				assert.Equal(t, "Critical Alert", ctx.Value(ctxKeyTitle))
			},
		},
		{
			name: "WithCancelable Sets Flag",
			transform: func(ctx TaskContext) TaskContext {
				return ctx.WithCancelable(true)
			},
			verify: func(t *testing.T, ctx TaskContext) {
				assert.True(t, ctx.IsCancelable())
				assert.Equal(t, true, ctx.Value(ctxKeyCancelable))
			},
		},
		{
			name: "WithError Sets Error Flag",
			transform: func(ctx TaskContext) TaskContext {
				return ctx.WithError()
			},
			verify: func(t *testing.T, ctx TaskContext) {
				assert.True(t, ctx.IsErrorOccurred())
				assert.Equal(t, true, ctx.Value(ctxKeyErrorOccurred))
			},
		},
		{
			name: "Chaining Multiple Methods",
			transform: func(ctx TaskContext) TaskContext {
				return ctx.WithTask("CHAIN_T", "CHAIN_C").
					WithTaskInstanceID("CHAIN_I", 123).
					WithTitle("Chained").
					WithCancelable(true).
					WithError()
			},
			verify: func(t *testing.T, ctx TaskContext) {
				assert.Equal(t, TaskID("CHAIN_T"), ctx.GetTaskID())
				assert.Equal(t, TaskCommandID("CHAIN_C"), ctx.GetTaskCommandID())
				assert.Equal(t, TaskInstanceID("CHAIN_I"), ctx.GetTaskInstanceID())
				assert.Equal(t, "Chained", ctx.GetTitle())
				assert.True(t, ctx.IsCancelable())
				assert.True(t, ctx.IsErrorOccurred())
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := NewTaskContext()
			ctx = tt.transform(ctx)
			tt.verify(t, ctx)
		})
	}
}

// TestTaskContext_FieldPreservation ensures that using the generic With() method
// does not wipe out the specialized struct fields (taskID, etc.)
func TestTaskContext_FieldPreservation(t *testing.T) {
	t.Parallel()

	// 1. Setup context with explicit fields
	baseCtx := NewTaskContext().
		WithTask("PRESERVED_TASK", "PRESERVED_CMD").
		WithTaskInstanceID("PRESERVED_INST", 100)

	// 2. Use Generic With() to add arbitrary metadata
	derivedCtx := baseCtx.With("CustomKey", "CustomValue")

	// 3. Verify fields are preserved in derived context
	assert.Equal(t, TaskID("PRESERVED_TASK"), derivedCtx.GetTaskID(), "TaskID field must be preserved after With()")
	assert.Equal(t, TaskCommandID("PRESERVED_CMD"), derivedCtx.GetTaskCommandID(), "CommandID field must be preserved after With()")
	assert.Equal(t, TaskInstanceID("PRESERVED_INST"), derivedCtx.GetTaskInstanceID(), "InstanceID field must be preserved after With()")

	// 4. Verify new value exists
	assert.Equal(t, "CustomValue", derivedCtx.Value("CustomKey"))
}

// TestTaskContext_FallbackToContextValue verifies that getters retrieve values
// from context.Value() if the struct fields are empty. This ensures compatibility
// with contexts created via With() or wrapping.
func TestTaskContext_FallbackToContextValue(t *testing.T) {
	t.Parallel()

	// 1. Create a Base Context (empty fields)
	baseCtx := NewTaskContext()

	// 2. Inject values using Generic With() only (simulating external context wrapping or generic usage)
	// Note: keys must match the ones defined in task_context.go
	ctx := baseCtx.
		With(ctxKeyTaskID, TaskID("FALLBACK_TASK")).
		With(ctxKeyCommandID, TaskCommandID("FALLBACK_CMD")).
		With(ctxKeyInstanceID, TaskInstanceID("FALLBACK_INST")).
		With(ctxKeyTitle, "Fallback Title").
		With(ctxKeyCancelable, true).
		With(ctxKeyErrorOccurred, true)

	// 3. Verify Getters find these values even though fields are theoretically empty
	// (Note: The current implementation of With() copies fields, but NewTaskContext fields are empty initially.
	// So current fields are empty, but Value() has data.)
	assert.Equal(t, TaskID("FALLBACK_TASK"), ctx.GetTaskID())
	assert.Equal(t, TaskCommandID("FALLBACK_CMD"), ctx.GetTaskCommandID())
	assert.Equal(t, TaskInstanceID("FALLBACK_INST"), ctx.GetTaskInstanceID())
	assert.Equal(t, "Fallback Title", ctx.GetTitle())
	assert.True(t, ctx.IsCancelable())
	assert.True(t, ctx.IsErrorOccurred())
}

// TestTaskContext_StandardInterop verifies compatibility with standard library context functions.
func TestTaskContext_StandardInterop(t *testing.T) {
	t.Parallel()

	tCtx := NewTaskContext().WithTask("MAIN_TASK", "MAIN_CMD").WithTitle("Interop Test")

	t.Run("WithCancel", func(t *testing.T) {
		// Wrap with std context
		ctx, cancel := context.WithCancel(tCtx)
		defer cancel()

		// Verify values are still accessible via Value interface
		assert.Equal(t, TaskID("MAIN_TASK"), ctx.Value(ctxKeyTaskID))
		assert.Equal(t, "Interop Test", ctx.Value(ctxKeyTitle))

		// Verify cancellation works
		cancel()
		select {
		case <-ctx.Done():
			assert.ErrorIs(t, ctx.Err(), context.Canceled)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Context should be canceled")
		}
	})

	t.Run("WithTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(tCtx, 50*time.Millisecond)
		defer cancel()

		// Verify Deadline
		dl, ok := ctx.Deadline()
		require.True(t, ok)
		assert.WithinDuration(t, time.Now().Add(50*time.Millisecond), dl, 20*time.Millisecond)

		// Verify Value access
		assert.Equal(t, TaskCommandID("MAIN_CMD"), ctx.Value(ctxKeyCommandID))
	})
}

// TestTaskContext_ConcurrencyStress performs a stress test to ensure thread safety.
func TestTaskContext_ConcurrencyStress(t *testing.T) {
	t.Parallel()

	baseCtx := NewTaskContext().WithTask("ROOT", "ROOT_CMD")
	const workers = 50
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Create derived context (Read & Write simulation)
				derived := baseCtx.WithTaskInstanceID(TaskInstanceID("Worker"), int64(j))

				// Verify Derived
				assert.Equal(t, int64(j), derived.GetElapsedTimeAfterRun())
				assert.Equal(t, TaskID("ROOT"), derived.GetTaskID()) // Should read base correctly

				// Verify Base is untouched (Read Check)
				assert.Equal(t, TaskID("ROOT"), baseCtx.GetTaskID())
			}
		}(i)
	}

	wg.Wait()
}
