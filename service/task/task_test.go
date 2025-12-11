package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTask_BasicMethods(t *testing.T) {
	testTask := &Task{
		ID:         ID("TEST_TASK"),
		CommandID:  CommandID("TEST_COMMAND"),
		InstanceID: InstanceID("test_instance_123"),
		NotifierID: "test_notifier",
		Canceled:   false,
		Storage:    &MockTaskResultStorage{},
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

func TestTask_Run(t *testing.T) {
	// defaultRegistry 백업 및 복원 (테스트 격리)
	originalRegistry := defaultRegistry
	defaultRegistry = newRegistry()
	defer func() {
		defaultRegistry = originalRegistry
	}()

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
			Storage: &MockTaskResultStorage{},
		}
		taskInstance.Storage.(*MockTaskResultStorage).On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		// Register 사용 (NewTaskFn 더미 추가)
		Register("ErrorTask", &Config{
			NewTaskFn: func(InstanceID, *RunRequest, *config.AppConfig) (TaskHandler, error) { return nil, nil },
			Commands: []*CommandConfig{
				{
					ID: "ErrorCommand",
					NewTaskResultDataFn: func() interface{} {
						return map[string]interface{}{}
					},
				},
			},
		})

		wg.Add(1)
		go taskInstance.Run(NewTaskContext(), mockSender, wg, doneC)

		select {
		case id := <-doneC:
			assert.Equal(t, InstanceID("ErrorInstance"), id)
		case <-time.After(1 * time.Second):
			t.Fatal("Task did not complete in time")
		}
		wg.Wait()

		assert.Equal(t, 1, mockSender.GetNotifyCallCount())
		assert.Equal(t, "test-notifier", mockSender.NotifyCalls[0].NotifierID)
		assert.Contains(t, mockSender.NotifyCalls[0].Message, "Run Error")
		assert.Contains(t, mockSender.NotifyCalls[0].Message, "작업이 실패하였습니다")
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
			Storage: &MockTaskResultStorage{},
		}
		taskInstance.Storage.(*MockTaskResultStorage).On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		Register("CancelTask", &Config{
			NewTaskFn: func(InstanceID, *RunRequest, *config.AppConfig) (TaskHandler, error) { return nil, nil },
			Commands: []*CommandConfig{
				{
					ID: "CancelCommand",
					NewTaskResultDataFn: func() interface{} {
						return &map[string]interface{}{}
					},
				},
			},
		})

		wg.Add(1)
		go taskInstance.Run(NewTaskContext(), mockSender, wg, doneC)

		select {
		case id := <-doneC:
			assert.Equal(t, InstanceID("CancelInstance"), id)
		case <-time.After(1 * time.Second):
			t.Fatal("Task did not complete in time")
		}
		wg.Wait()

		// Should not send	// No notification should be sent for duplicate
		assert.Equal(t, 0, mockSender.GetNotifyCallCount())
	})
}
