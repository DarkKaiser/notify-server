package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
	assert.Equal(t, commandID, ctx.GetCommandID(), "TaskCommandID가 설정되어야 합니다")
}

func TestTaskContext_WithInstanceID(t *testing.T) {
	ctx := NewTaskContext()

	instanceID := InstanceID("test_instance_123")
	elapsedTime := int64(42)

	ctx = ctx.WithInstanceID(instanceID, elapsedTime)

	assert.Equal(t, instanceID, ctx.GetInstanceID(), "TaskInstanceID가 설정되어야 합니다")
	assert.Equal(t, elapsedTime, ctx.GetElapsedTimeAfterRun(), "경과 시간이 설정되어야 합니다")
}

func TestTaskContext_WithError(t *testing.T) {
	ctx := NewTaskContext()

	ctx = ctx.WithError()

	assert.Equal(t, true, ctx.IsErrorOccurred(), "에러 상태가 설정되어야 합니다")
}

func TestTask_BasicMethods(t *testing.T) {
	testTask := &Task{
		ID:         ID("TEST_TASK"),
		CommandID:  CommandID("TEST_COMMAND"),
		InstanceID: InstanceID("test_instance_123"),
		NotifierID: "test_notifier",
		Canceled:   false,
	}

	t.Run("ID 반환 테스트", func(t *testing.T) {
		assert.Equal(t, ID("TEST_TASK"), testTask.GetID(), "TaskID가 올바르게 반환되어야 합니다")
	})

	t.Run("CommandID 반환 테스트", func(t *testing.T) {
		assert.Equal(t, CommandID("TEST_COMMAND"), testTask.GetCommandID(), "TaskCommandID가 올바르게 반환되어야 합니다")
	})

	t.Run("InstanceID 반환 테스트", func(t *testing.T) {
		assert.Equal(t, InstanceID("test_instance_123"), testTask.GetInstanceID(), "TaskInstanceID가 올바르게 반환되어야 합니다")
	})

	t.Run("NotifierID 반환 테스트", func(t *testing.T) {
		assert.Equal(t, "test_notifier", testTask.GetNotifierID(), "NotifierID가 올바르게 반환되어야 합니다")
	})

	t.Run("Cancel 및 IsCanceled 테스트", func(t *testing.T) {
		assert.False(t, testTask.IsCanceled(), "초기 상태에서는 취소되지 않아야 합니다")

		testTask.Cancel()
		assert.True(t, testTask.IsCanceled(), "Cancel 호출 후에는 취소 상태여야 합니다")
	})

	t.Run("ElapsedTimeAfterRun 테스트", func(t *testing.T) {
		// runTime을 현재 시간으로 설정
		testTask.RunTime = time.Now()

		// 짧은 대기
		time.Sleep(100 * time.Millisecond)

		elapsed := testTask.ElapsedTimeAfterRun()
		assert.GreaterOrEqual(t, elapsed, int64(0), "경과 시간은 0 이상이어야 합니다")
		assert.LessOrEqual(t, elapsed, int64(2), "경과 시간은 2초 이하여야 합니다")
	})
}

func TestRunBy_Values(t *testing.T) {
	t.Run("RunBy 상수 값 테스트", func(t *testing.T) {
		assert.Equal(t, RunBy(0), RunByUnknown, "RunByUnknown은 0이어야 합니다")
		assert.Equal(t, RunBy(1), RunByUser, "RunByUser는 1이어야 합니다")
		assert.Equal(t, RunBy(2), RunByScheduler, "RunByScheduler는 2이어야 합니다")
	})

	t.Run("RunBy 비교 테스트", func(t *testing.T) {
		testTask := Task{
			RunBy: RunByUser,
		}

		assert.Equal(t, RunByUser, testTask.RunBy, "Task의 runBy가 RunByUser여야 합니다")
		assert.NotEqual(t, RunByScheduler, testTask.RunBy, "Task의 runBy가 RunByScheduler가 아니어야 합니다")
	})
}
