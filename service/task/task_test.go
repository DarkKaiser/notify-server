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
		assert.Equal(t, CommandID("TEST_COMMAND"), testTask.GetCommandID(), "CommandID가 올바르게 반환되어야 합니다")
	})

	t.Run("InstanceID 반환 테스트", func(t *testing.T) {
		assert.Equal(t, InstanceID("test_instance_123"), testTask.GetInstanceID(), "InstanceID가 올바르게 반환되어야 합니다")
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
		assert.LessOrEqual(t, elapsed, int64(10), "경과 시간은 오차 범위 내여야 합니다")
	})
}

func TestTask_Run(t *testing.T) {
	// defaultRegistry 백업 및 복원 (테스트 격리)
	originalRegistry := defaultRegistry
	defaultRegistry = newRegistry()
	defer func() {
		defaultRegistry = originalRegistry
	}()

	mockSender := NewMockNotificationSender()

	tests := []struct {
		name                 string
		taskID               string
		commandID            string
		canceled             bool
		runFn                RunFunc
		storageSetup         func(*MockTaskResultStorage)
		configSetup          func()
		expectedNotifyCount  int
		expectedMessageParts []string
	}{
		{
			name:      "실행 중 에러 발생 (Run Error)",
			taskID:    "ErrorTask",
			commandID: "ErrorCommand",
			runFn: func(data interface{}, supportHTML bool) (string, interface{}, error) {
				return "", nil, errors.New("Run Error")
			},
			storageSetup: func(m *MockTaskResultStorage) {
				m.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			configSetup: func() {
				Register("ErrorTask", &Config{
					NewTaskFn: func(InstanceID, *RunRequest, *config.AppConfig) (Handler, error) { return nil, nil },
					Commands: []*CommandConfig{
						{
							ID: "ErrorCommand",
							NewTaskResultDataFn: func() interface{} {
								return map[string]interface{}{}
							},
						},
					},
				})
			},
			expectedNotifyCount:  1,
			expectedMessageParts: []string{"Run Error", "작업이 실패하였습니다"},
		},
		{
			name:      "이미 취소된 작업 (Canceled)",
			taskID:    "CancelTask",
			commandID: "CancelCommand",
			canceled:  true,
			runFn: func(data interface{}, supportHTML bool) (string, interface{}, error) {
				return "Should Not Send", nil, nil
			},
			storageSetup: func(m *MockTaskResultStorage) {
				m.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			configSetup: func() {
				Register("CancelTask", &Config{
					NewTaskFn: func(InstanceID, *RunRequest, *config.AppConfig) (Handler, error) { return nil, nil },
					Commands: []*CommandConfig{
						{
							ID: "CancelCommand",
							NewTaskResultDataFn: func() interface{} {
								return &map[string]interface{}{}
							},
						},
					},
				})
			},
			expectedNotifyCount: 0,
		},
		{
			name:      "정상 실행 및 알림 발송 (Success)",
			taskID:    "SuccessTask",
			commandID: "SuccessCommand",
			runFn: func(data interface{}, supportHTML bool) (string, interface{}, error) {
				return "Success Message", map[string]interface{}{"key": "value"}, nil
			},
			storageSetup: func(m *MockTaskResultStorage) {
				m.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				m.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			configSetup: func() {
				Register("SuccessTask", &Config{
					NewTaskFn: func(InstanceID, *RunRequest, *config.AppConfig) (Handler, error) { return nil, nil },
					Commands: []*CommandConfig{
						{
							ID: "SuccessCommand",
							NewTaskResultDataFn: func() interface{} {
								return map[string]interface{}{}
							},
						},
					},
				})
			},
			expectedNotifyCount:  1,
			expectedMessageParts: []string{"Success Message"},
		},
		{
			name:      "Storage 저장 실패 (Storage Save Error)",
			taskID:    "StorageFailTask",
			commandID: "StorageFailCommand",
			runFn: func(data interface{}, supportHTML bool) (string, interface{}, error) {
				return "Success Message", map[string]interface{}{"key": "value"}, nil
			},
			storageSetup: func(m *MockTaskResultStorage) {
				m.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				m.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("Save Error"))
			},
			configSetup: func() {
				Register("StorageFailTask", &Config{
					NewTaskFn: func(InstanceID, *RunRequest, *config.AppConfig) (Handler, error) { return nil, nil },
					Commands: []*CommandConfig{
						{
							ID: "StorageFailCommand",
							NewTaskResultDataFn: func() interface{} {
								return map[string]interface{}{}
							},
						},
					},
				})
			},
			// 성공 메시지 1회 + 저장 실패 에러 메시지 1회 = 총 2회
			expectedNotifyCount:  2,
			expectedMessageParts: []string{"Success Message", "Save Error", "작업결과데이터의 저장이 실패"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSender.NotifyCalls = nil // Reset mock calls

			if tt.configSetup != nil {
				tt.configSetup()
			}

			mockStorage := &MockTaskResultStorage{}
			if tt.storageSetup != nil {
				tt.storageSetup(mockStorage)
			}

			taskInstance := &Task{
				ID:         ID(tt.taskID),
				CommandID:  CommandID(tt.commandID),
				InstanceID: InstanceID(tt.taskID + "_Instance"),
				NotifierID: "test-notifier",
				Canceled:   tt.canceled,
				RunFn:      tt.runFn,
				Storage:    mockStorage,
			}

			wg := &sync.WaitGroup{}
			doneC := make(chan InstanceID, 1)

			wg.Add(1)
			go taskInstance.Run(NewTaskContext(), mockSender, wg, doneC)

			select {
			case id := <-doneC:
				assert.Equal(t, taskInstance.InstanceID, id)
			case <-time.After(1 * time.Second):
				t.Fatal("Task did not complete in time")
			}
			wg.Wait()

			assert.Equal(t, tt.expectedNotifyCount, mockSender.GetNotifyCallCount(), "알림 발송 횟수가 일치해야 합니다")

			if len(tt.expectedMessageParts) > 0 {
				// 모든 메시지를 합쳐서 검사 (간소화)
				allMessages := ""
				for _, call := range mockSender.NotifyCalls {
					allMessages += call.Message
				}

				for _, part := range tt.expectedMessageParts {
					assert.Contains(t, allMessages, part, "메시지에 예상 문구가 포함되어야 합니다")
				}
			}

			mockStorage.AssertExpectations(t)
		})
	}
}
