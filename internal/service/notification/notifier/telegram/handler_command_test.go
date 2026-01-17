package telegram

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
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
		setupMockExec func(*taskmocks.MockExecutor, *sync.WaitGroup)
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
			setupMockExec: func(m *taskmocks.MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1) // Expect run call
				m.On("Submit", mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
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

			// Assertions
			mockBot.AssertExpectations(t)
			mockExecutor.AssertExpectations(t)
		})
	}
}

// =============================================================================
// Command Regression Tests
// =============================================================================

func TestTelegramNotifier_Regressions_Command(t *testing.T) {
	// Regression Test for: "Cancel 명령어 파싱 오류 수정"
	// Ensure that /cancel_task_1_inst_1 is parsed correctly as InstanceID="task_1_inst_1"
	t.Run("Fix: handleCancelCommand supports underscores in InstanceID", func(t *testing.T) {
		mockExec := new(taskmocks.MockExecutor)
		n := &telegramNotifier{}

		ctx := context.Background()
		commandWithUnderscores := "/cancel_task_1_instance_123"
		expectedInstanceID := contract.TaskInstanceID("task_1_instance_123")

		mockExec.On("Cancel", expectedInstanceID).Return(nil).Once()

		n.handleCancelCommand(ctx, mockExec, commandWithUnderscores)

		mockExec.AssertExpectations(t)
	})

	t.Run("Fix: handleCancelCommand fails gracefully for bad format", func(t *testing.T) {
		mockExec := new(taskmocks.MockExecutor)
		// We need a mockBot here because it sends a message on error
		mockBot := &MockTelegramBot{}
		n := &telegramNotifier{
			botAPI: mockBot,
			chatID: 12345,
		}

		ctx := context.Background()
		// Only one part
		badCommand := "/cancel"

		mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
			msg, ok := c.(tgbotapi.MessageConfig)
			return ok && strings.Contains(msg.Text, "잘못된 취소 명령어 형식")
		})).Return(tgbotapi.Message{}, nil).Once()

		// Should NOT call Cancel
		n.handleCancelCommand(ctx, mockExec, badCommand)

		mockExec.AssertExpectations(t)
		mockBot.AssertExpectations(t)
	})
}
