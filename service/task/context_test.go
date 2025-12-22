package task

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Ensure TaskContext implements context.Context at compile time
var _ context.Context = (*taskContext)(nil)
var _ TaskContext = (*taskContext)(nil)

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
	assert.Equal(t, "Test", ctx.Value(ctxKeyTitle))

	// 2. Verify cancellation propagation
	cancel()
	select {
	case <-ctx.Done():
		assert.Equal(t, context.Canceled, ctx.Err())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should be canceled")
	}

	// 3. Verify Deadline propagation
	deadlineCtx, cancel := context.WithTimeout(tCtx, 100*time.Millisecond)
	defer cancel()

	dl, ok := deadlineCtx.Deadline()
	assert.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(100*time.Millisecond), dl, 10*time.Millisecond)
}

// TestTaskContext_Accessors verifies all setters and getters using table-driven tests.
func TestTaskContext_Accessors(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(TaskContext) TaskContext
		verification func(*testing.T, TaskContext)
	}{
		{
			name: "WithTask",
			setup: func(ctx TaskContext) TaskContext {
				return ctx.WithTask("TASK_01", "CMD_01")
			},
			verification: func(t *testing.T, ctx TaskContext) {
				assert.Equal(t, ID("TASK_01"), ctx.GetID())
				assert.Equal(t, CommandID("CMD_01"), ctx.GetCommandID())
			},
		},
		{
			name: "WithInstanceID",
			setup: func(ctx TaskContext) TaskContext {
				return ctx.WithInstanceID("INST_01", 12345)
			},
			verification: func(t *testing.T, ctx TaskContext) {
				assert.Equal(t, InstanceID("INST_01"), ctx.GetInstanceID())
				assert.Equal(t, int64(12345), ctx.GetElapsedTimeAfterRun())
			},
		},
		{
			name: "WithTitle",
			setup: func(ctx TaskContext) TaskContext {
				return ctx.WithTitle("My Notification")
			},
			verification: func(t *testing.T, ctx TaskContext) {
				assert.Equal(t, "My Notification", ctx.GetTitle())
			},
		},
		{
			name: "WithError",
			setup: func(ctx TaskContext) TaskContext {
				return ctx.WithError()
			},
			verification: func(t *testing.T, ctx TaskContext) {
				assert.True(t, ctx.IsErrorOccurred())
			},
		},
		{
			name: "WithCancelable",
			setup: func(ctx TaskContext) TaskContext {
				return ctx.WithCancelable(true)
			},
			verification: func(t *testing.T, ctx TaskContext) {
				assert.True(t, ctx.IsCancelable())
			},
		},
		{
			name: "Chained Calls",
			setup: func(ctx TaskContext) TaskContext {
				return ctx.WithTask("T1", "C1").WithInstanceID("I1", 100).WithTitle("Chained").WithError().WithCancelable(true)
			},
			verification: func(t *testing.T, ctx TaskContext) {
				assert.Equal(t, ID("T1"), ctx.GetID())
				assert.Equal(t, InstanceID("I1"), ctx.GetInstanceID())
				assert.Equal(t, "Chained", ctx.GetTitle())
				assert.True(t, ctx.IsErrorOccurred())
				assert.True(t, ctx.IsCancelable())
			},
		},
		{
			name: "Override Values",
			setup: func(ctx TaskContext) TaskContext {
				return ctx.WithTitle("Old").WithTitle("New")
			},
			verification: func(t *testing.T, ctx TaskContext) {
				assert.Equal(t, "New", ctx.GetTitle(), "Latest value should override previous ones")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewTaskContext()
			ctx = tt.setup(ctx)
			tt.verification(t, ctx)
		})
	}
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
	assert.False(t, ctx.IsCancelable())
}

// TestTaskContext_Concurrency verifies thread safety of the immutable design.
func TestTaskContext_Concurrency(t *testing.T) {
	baseCtx := NewTaskContext().WithTitle("Base")
	var wg sync.WaitGroup
	workers := 100

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()

			// Each worker derives a new context without affecting baseCtx
			ctx := baseCtx.WithTitle("Worker")

			// Introduce rigorous checking
			assert.Equal(t, "Worker", ctx.GetTitle())
			assert.Equal(t, "Base", baseCtx.GetTitle()) // Must remain unchanged
		}(i)
	}

	wg.Wait()
}

func TestTaskContext_With(t *testing.T) {
	ctx := NewTaskContext()

	// 값 설정
	ctx = ctx.With("key1", "value1")
	ctx = ctx.With("key2", 123)

	// 값 조회
	assert.Equal(t, "value1", ctx.Value("key1"), "설정한 값을 조회할 수 있어야 합니다")
	assert.Equal(t, 123, ctx.Value("key2"), "설정한 값을 조회할 수 있어야 합니다")
	assert.Nil(t, ctx.Value("key3"), "설정하지 않은 키는 nil을 반환해야 합니다")
}

func TestTaskContext_WithTask(t *testing.T) {
	ctx := NewTaskContext()

	taskID := ID("TEST_TASK")
	commandID := CommandID("TEST_COMMAND")

	ctx = ctx.WithTask(taskID, commandID)

	assert.Equal(t, taskID, ctx.GetID(), "TaskID가 설정되어야 합니다")
	assert.Equal(t, commandID, ctx.GetCommandID(), "CommandID가 설정되어야 합니다")
}

func TestTaskContext_WithInstanceID(t *testing.T) {
	ctx := NewTaskContext()

	instanceID := InstanceID("test_instance_123")
	elapsedTime := int64(42)

	ctx = ctx.WithInstanceID(instanceID, elapsedTime)

	assert.Equal(t, instanceID, ctx.GetInstanceID(), "InstanceID가 설정되어야 합니다")
	assert.Equal(t, elapsedTime, ctx.GetElapsedTimeAfterRun(), "경과 시간이 설정되어야 합니다")
}

func TestTaskContext_WithError(t *testing.T) {
	ctx := NewTaskContext()

	ctx = ctx.WithError()

	assert.Equal(t, true, ctx.IsErrorOccurred(), "에러 상태가 설정되어야 합니다")
}
