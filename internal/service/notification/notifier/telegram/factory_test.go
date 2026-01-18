package telegram

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/constants"

	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Telegram Bot API Client Tests
// =============================================================================

// TestTelegramBotAPIClient_GetSelf verifies GetSelf method.
func TestTelegramBotAPIClient_GetSelf(t *testing.T) {
	t.Run("GetSelf function verification", func(t *testing.T) {
		mockBotAPI := &tgbotapi.BotAPI{
			Self: tgbotapi.User{
				ID:        123456,
				UserName:  "test_bot",
				FirstName: "Test",
				LastName:  "Bot",
			},
		}

		client := &defaultBotClient{BotAPI: mockBotAPI}
		user := client.GetSelf()

		assert.Equal(t, int64(123456), user.ID)
		assert.Equal(t, "test_bot", user.UserName)
		assert.Equal(t, "Test", user.FirstName)
		assert.Equal(t, "Bot", user.LastName)
	})
}

// =============================================================================
// Telegram Notifier Factory Tests
// =============================================================================

// TestNewTelegramNotifierWithBot_Table verifies Notifier creation.
func TestNewTelegramNotifierWithBot_Table(t *testing.T) {
	tests := []struct {
		name                 string
		appConfig            *config.AppConfig
		expectedCommandCount int
		expectedFirstCmd     string
	}{
		{
			name:                 "Default Config",
			appConfig:            &config.AppConfig{},
			expectedCommandCount: 1, // Only Help
			expectedFirstCmd:     "help",
		},
		{
			name: "Config with Task",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID:    "TestTask",
						Title: "Test Task",
						Commands: []config.CommandConfig{
							{
								ID:          "Run",
								Title:       "Run",
								Description: "Run task",
								Notifier: struct {
									Usable bool `json:"usable"`
								}{Usable: true},
								DefaultNotifierID: "test-notifier",
							},
						},
					},
				},
			},
			expectedCommandCount: 2, // Task Command + Help
			expectedFirstCmd:     "test_task_run",
		},
		{
			name: "Config with Disabled Task",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID:    "TestTask",
						Title: "Test Task",
						Commands: []config.CommandConfig{
							{
								ID:    "Stop",
								Title: "Stop",
								Notifier: struct {
									Usable bool `json:"usable"`
								}{Usable: false},
							},
						},
					},
				},
			},
			expectedCommandCount: 1, // Only Help (Disabled command ignored)
			expectedFirstCmd:     "help",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBot := &MockTelegramBot{
				updatesChan: make(chan tgbotapi.Update),
			}
			chatID := int64(12345)

			mockExecutor := &taskmocks.MockExecutor{}
			opts := options{
				BotToken:  "test-token",
				ChatID:    chatID,
				AppConfig: tt.appConfig,
			}
			n, err := newTelegramNotifierWithBot("test-notifier", mockBot, mockExecutor, opts)
			require.NoError(t, err)

			notifier, ok := n.(*telegramNotifier)
			require.True(t, ok, "Type assertion should succeed")
			require.NotNil(t, notifier, "Notifier should not be nil")

			assert.Len(t, notifier.botCommands, tt.expectedCommandCount)
			if tt.expectedCommandCount > 0 {
				assert.Equal(t, tt.expectedFirstCmd, notifier.botCommands[0].command)
			}

			// Buffer Size Verification
			assert.Equal(t, constants.TelegramNotifierBufferSize, cap(notifier.RequestC()), "Buffer size should match the constant")
		})
	}
}

// TestNewNotifier_CommandCollision verifies that command collisions are detected.
func TestNewNotifier_CommandCollision(t *testing.T) {
	// given
	// 충돌을 유발하는 설정 생성:
	// 1. Task: "foo_bar", Command: "baz" -> /foo_bar_baz
	// 2. Task: "foo", Command: "bar_baz" -> /foo_bar_baz
	appConfig := &config.AppConfig{
		Notifier: config.NotifierConfig{
			Telegrams: []config.TelegramConfig{
				{
					ID:       "telegram-1",
					BotToken: "test-token",
					ChatID:   12345,
				},
			},
		},
		Tasks: []config.TaskConfig{
			{
				ID:    "foo_bar",
				Title: "Task 1",
				Commands: []config.CommandConfig{
					{
						ID:       "baz",
						Title:    "Command 1",
						Notifier: config.CommandNotifierConfig{Usable: true},
					},
				},
			},
			{
				ID:    "foo",
				Title: "Task 2",
				Commands: []config.CommandConfig{
					{
						ID:       "bar_baz",
						Title:    "Command 2",
						Notifier: config.CommandNotifierConfig{Usable: true},
					},
				},
			},
		},
	}

	mockExecutor := &taskmocks.MockExecutor{}
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}

	// when
	opts := options{
		BotToken:  "test-token",
		ChatID:    12345,
		AppConfig: appConfig,
	}
	_, err := newTelegramNotifierWithBot(contract.NotifierID("telegram-1"), mockBot, mockExecutor, opts)

	// then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "텔레그램 명령어 충돌이 감지되었습니다")
	assert.Contains(t, err.Error(), "/foo_bar_baz")
}

// TestNewNotifier_Validation verifies validation logic for tasks and commands.
func TestNewNotifier_Validation(t *testing.T) {
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

			mockBot := &MockTelegramBot{
				updatesChan: make(chan tgbotapi.Update),
			}
			mockExecutor := &taskmocks.MockExecutor{}

			opts := options{
				BotToken:  "test-token",
				ChatID:    1234,
				AppConfig: cfg,
			}
			_, err := newTelegramNotifierWithBot("test_notifier", mockBot, mockExecutor, opts)

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
