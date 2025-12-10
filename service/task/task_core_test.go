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
		mockSender := NewMockNotificationSender()
		wg := &sync.WaitGroup{}
		doneC := make(chan InstanceID, 1)

		taskInstance := &Task{
			ID:         "ErrorTask",
			CommandID:  "ErrorCommand",
			InstanceID: "ErrorInstance",
			NotifierID: "test-notifier",
			RunFn: func(data interface{}, supportHTML bool) (string, interface{}, error) {
				return "", nil, errors.New("Run Error")
			},
		}

		supportedTasks["ErrorTask"] = &TaskConfig{
			CommandConfigs: []*TaskCommandConfig{
				{
					TaskCommandID: "ErrorCommand",
					NewTaskResultDataFn: func() interface{} {
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
			assert.Equal(t, InstanceID("ErrorInstance"), id)
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
		mockSender := NewMockNotificationSender()
		wg := &sync.WaitGroup{}
		doneC := make(chan InstanceID, 1)

		taskInstance := &Task{
			ID:         "CancelTask",
			CommandID:  "CancelCommand",
			InstanceID: "CancelInstance",
			NotifierID: "test-notifier",
			Canceled:   true, // Already canceled
			RunFn: func(data interface{}, supportHTML bool) (string, interface{}, error) {
				return "Should Not Send", nil, nil
			},
		}

		supportedTasks["CancelTask"] = &TaskConfig{
			CommandConfigs: []*TaskCommandConfig{
				{
					TaskCommandID: "CancelCommand",
					NewTaskResultDataFn: func() interface{} {
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
			assert.Equal(t, InstanceID("CancelInstance"), id)
		case <-time.After(1 * time.Second):
			t.Fatal("Task did not complete in time")
		}
		wg.Wait()

		// Should not send notification if canceled
		assert.Equal(t, 0, mockSender.GetNotifyWithTaskContextCallCount())
	})
}

func TestTask_Cancel(t *testing.T) {
	taskInstance := &Task{}
	assert.False(t, taskInstance.IsCanceled())

	taskInstance.Cancel()
	assert.True(t, taskInstance.IsCanceled())
}

func TestTaskContext(t *testing.T) {
	ctx := NewTaskContext()

	ctx = ctx.With("key", "value")
	assert.Equal(t, "value", ctx.Value("key"))

	ctx = ctx.WithTask("TaskID", "CommandID")
	assert.Equal(t, ID("TaskID"), ctx.GetID())
	assert.Equal(t, CommandID("CommandID"), ctx.GetCommandID())

	ctx = ctx.WithInstanceID("InstanceID", 100)
	assert.Equal(t, InstanceID("InstanceID"), ctx.GetInstanceID())
	assert.Equal(t, int64(100), ctx.GetElapsedTimeAfterRun())

	ctx = ctx.WithError()
	assert.True(t, ctx.IsErrorOccurred())
}
