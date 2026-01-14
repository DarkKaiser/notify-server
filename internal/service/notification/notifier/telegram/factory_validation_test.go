package telegram

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
)

// mockBotAPI is a minimal mock for telegramBotAPI interface to avoid network calls
type mockBotAPI struct{}

func (m *mockBotAPI) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return make(tgbotapi.UpdatesChannel)
}
func (m *mockBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return tgbotapi.Message{}, nil
}
func (m *mockBotAPI) StopReceivingUpdates() {}
func (m *mockBotAPI) GetSelf() tgbotapi.User {
	return tgbotapi.User{UserName: "test_bot"}
}

func TestNewNotifier_Validation(t *testing.T) {
	// mocks.handler.go is in "internal/service/notification/mocks"
	// mocks.MockExecutor is likely not there, based on previous errors.
	// But let's check imports.
	// Ah, previous test had "taskmocks" for executor.
	// internal/service/notification/mocks does NOT have MockExecutor?
	// Let's assume we don't need a real executor for this validation test
	// because validation happens BEFORE executor is used.
	// But `newTelegramNotifierWithBot` takes `executor task.Executor`.
	// So we can pass nil if it's not used in validation, or a mock.

	// Let's define a local minimal mock executor if needed, or pass nil if valid.
	// But wait, Go is strict. We need to pass something implementing task.Executor.
	// I'll assume nil is fine for validation phase if the code doesn't check it early.
	// Code: `notifier := &telegramNotifier{ ..., executor: executor, ... }`
	// It just assigns it.

	tests := []struct {
		name        string
		tasks       []config.TaskConfig
		expectError bool
		errContains string
	}{
		{
			name: "성공: 유효한 ID",
			tasks: []config.TaskConfig{
				{
					ID: "valid_task",
					Commands: []config.CommandConfig{
						{ID: "valid_command", Notifier: config.CommandNotifierConfig{Usable: true}},
					},
				},
			},
			expectError: false,
		},
		{
			name: "실패: TaskID 누락",
			tasks: []config.TaskConfig{
				{
					ID: "", // Missing ID
					Commands: []config.CommandConfig{
						{ID: "valid_command", Notifier: config.CommandNotifierConfig{Usable: true}},
					},
				},
			},
			expectError: true,
			errContains: "TaskID 또는 CommandID는 비어있을 수 없습니다",
		},
		{
			name: "실패: CommandID 누락",
			tasks: []config.TaskConfig{
				{
					ID: "valid_task",
					Commands: []config.CommandConfig{
						{ID: "", Notifier: config.CommandNotifierConfig{Usable: true}}, // Missing ID
					},
				},
			},
			expectError: true,
			errContains: "TaskID 또는 CommandID는 비어있을 수 없습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AppConfig{
				Tasks: tt.tasks,
			}

			botAPI := &mockBotAPI{}

			// Passing nil for executor as it is not used during validation logic
			_, err := newTelegramNotifierWithBot("test_notifier", botAPI, 1234, cfg, nil)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
