package telegram

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Handler Tests
// =============================================================================

// TestTelegramNotifier_HandleCommand verifies command handling.
func TestTelegramNotifier_HandleCommand(t *testing.T) {
	// Create common app config for command tests
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: "task1",
				Commands: []config.CommandConfig{
					{
						ID:          "run",
						Title:       "Task 1 Run",
						Description: "Run Task 1",
						Notifier: struct {
							Usable bool `json:"usable"`
						}{Usable: true},
					},
				},
			},
		},
		Debug: true,
	}

	tests := []struct {
		name          string
		commandText   string
		expectAction  bool
		setupMockBot  func(*MockTelegramBot, *sync.WaitGroup)
		setupMockExec func(*MockExecutor, *sync.WaitGroup)
	}{
		{
			name:         "Unknown Command",
			commandText:  "/unknown",
			expectAction: true,
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1) // Expect reply
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					_, ok := c.(tgbotapi.MessageConfig)
					return ok // Check content later if needed
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
		},
		{
			name:         "Help Command",
			commandText:  "/help",
			expectAction: true,
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1) // Expect reply
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.Contains(msg.Text, "도움말") // Assuming this string in code is intact or we'll update it
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
		},
		{
			name:         "Task Run Command",
			commandText:  "/task_1_run",
			expectAction: true,
			setupMockExec: func(m *MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1) // Expect run call
				m.On("SubmitTask", mock.MatchedBy(func(req *task.SubmitRequest) bool {
					return req.TaskID == "task1" && req.CommandID == "run"
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			notifier, mockBot, mockExecutor := setupTelegramTest(t, appConfig)
			require.NotNil(t, notifier)
			require.NotNil(t, mockBot)
			require.NotNil(t, mockExecutor)

			// Override GetUpdatesChan to return actual channel
			mockBot.ExpectedCalls = nil // Clear previous expectations
			mockBot.On("GetSelf").Return(tgbotapi.User{UserName: testTelegramBotUsername}).Maybe()
			mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(mockBot.updatesChan))
			mockBot.On("StopReceivingUpdates").Return()

			var wgAction sync.WaitGroup

			if tt.setupMockBot != nil {
				tt.setupMockBot(mockBot, &wgAction)
			}
			if tt.setupMockExec != nil {
				tt.setupMockExec(mockExecutor, &wgAction)
			}

			// Run
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var wg sync.WaitGroup
			runTelegramNotifier(ctx, notifier, &wg)

			// Send update
			mockBot.updatesChan <- tgbotapi.Update{
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{ID: testTelegramChatID},
					Text: tt.commandText,
				},
			}

			// Wait if action expected
			if tt.expectAction {
				waitForActionWithTimeout(t, &wgAction, testTelegramTimeout)
			}

			// Cleanup
			cancel()
			wg.Wait()

			mockBot.AssertExpectations(t)
			mockExecutor.AssertExpectations(t)
		})
	}
}
