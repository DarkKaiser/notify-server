package telegram

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
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

		client := &telegramBotAPIClient{BotAPI: mockBotAPI}
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
			n := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, tt.appConfig, mockExecutor)

			notifier, ok := n.(*telegramNotifier)
			require.True(t, ok, "Type assertion should succeed")
			require.NotNil(t, notifier, "Notifier should not be nil")

			assert.Len(t, notifier.botCommands, tt.expectedCommandCount)
			if tt.expectedCommandCount > 0 {
				assert.Equal(t, tt.expectedFirstCmd, notifier.botCommands[0].command)
			}
		})
	}
}
