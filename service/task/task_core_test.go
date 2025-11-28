package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTask_Run(t *testing.T) {
	t.Run("실행 중 에러 발생", func(t *testing.T) {
		mockSender := NewMockTaskNotificationSender()
		wg := &sync.WaitGroup{}
		doneC := make(chan TaskInstanceID, 1)

		taskInstance := &task{
			id:         "ErrorTask",
			commandID:  "ErrorCommand",
			instanceID: "ErrorInstance",
			notifierID: "test-notifier",
			runFn: func(data interface{}, supportHTML bool) (string, interface{}, error) {
				return "", nil, errors.New("Run Error")
			},
		}

		supportedTasks["ErrorTask"] = &supportedTaskConfig{
			commandConfigs: []*supportedTaskCommandConfig{
				{
					taskCommandID: "ErrorCommand",
					newTaskResultDataFn: func() interface{} {
						return map[string]interface{}{}
					},
				},
			},
		}
		defer delete(supportedTasks, "ErrorTask")

		wg.Add(1)
		go taskInstance.Run(mockSender, wg, doneC)

		select {
		case id := <-doneC:
			assert.Equal(t, TaskInstanceID("ErrorInstance"), id)
		case <-time.After(1 * time.Second):
			t.Fatal("Task did not complete in time")
		}
		wg.Wait()

		assert.Equal(t, 1, mockSender.GetNotifyWithTaskContextCallCount())
		call := mockSender.NotifyWithTaskContextCalls[0]
		assert.Contains(t, call.Message, "Run Error")
		assert.Contains(t, call.Message, "작업이 실패하였습니다")
	})

	t.Run("취소된 작업", func(t *testing.T) {
		mockSender := NewMockTaskNotificationSender()
		wg := &sync.WaitGroup{}
		doneC := make(chan TaskInstanceID, 1)

		taskInstance := &task{
			id:         "CancelTask",
			commandID:  "CancelCommand",
			instanceID: "CancelInstance",
			notifierID: "test-notifier",
			canceled:   true, // Already canceled
			runFn: func(data interface{}, supportHTML bool) (string, interface{}, error) {
				return "Should Not Send", nil, nil
			},
		}

		supportedTasks["CancelTask"] = &supportedTaskConfig{
			commandConfigs: []*supportedTaskCommandConfig{
				{
					taskCommandID: "CancelCommand",
					newTaskResultDataFn: func() interface{} {
						return &map[string]interface{}{}
					},
				},
			},
		}
		defer delete(supportedTasks, "CancelTask")

		wg.Add(1)
		go taskInstance.Run(mockSender, wg, doneC)

		select {
		case id := <-doneC:
			assert.Equal(t, TaskInstanceID("CancelInstance"), id)
		case <-time.After(1 * time.Second):
			t.Fatal("Task did not complete in time")
		}
		wg.Wait()

		// Should not send notification if canceled
		assert.Equal(t, 0, mockSender.GetNotifyWithTaskContextCallCount())
	})
}

func TestTask_Cancel(t *testing.T) {
	taskInstance := &task{}
	assert.False(t, taskInstance.IsCanceled())

	taskInstance.Cancel()
	assert.True(t, taskInstance.IsCanceled())
}

func TestTaskContext(t *testing.T) {
	ctx := NewContext()

	ctx.With("key", "value")
	assert.Equal(t, "value", ctx.Value("key"))

	ctx.WithTask("TaskID", "CommandID")
	assert.Equal(t, TaskID("TaskID"), ctx.Value(TaskCtxKeyTaskID))
	assert.Equal(t, TaskCommandID("CommandID"), ctx.Value(TaskCtxKeyTaskCommandID))

	ctx.WithInstanceID("InstanceID", 100)
	assert.Equal(t, TaskInstanceID("InstanceID"), ctx.Value(TaskCtxKeyTaskInstanceID))
	assert.Equal(t, int64(100), ctx.Value(TaskCtxKeyElapsedTimeAfterRun))

	ctx.WithError()
	assert.Equal(t, true, ctx.Value(TaskCtxKeyErrorOccurred))
}
