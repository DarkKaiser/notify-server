package telegram

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Handler Tests
// =============================================================================

// TestTelegramNotifier_DispatchCommand provides comprehensive coverage for command handling.
func TestTelegramNotifier_DispatchCommand(t *testing.T) {
	// Common AppConfig for tests
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: "taskone",
				Commands: []config.CommandConfig{
					{
						ID:          "run",
						Title:       "Task 1 Run",
						Description: "Run Task 1",
						Notifier: struct {
							Usable bool `json:"usable"`
						}{Usable: true},
					},
					{
						ID:          "stop", // Command with mixed case ID in config? usually lowercase
						Title:       "Task 1 Stop",
						Description: "Stop Task 1",
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
		name           string
		commandMessage *tgbotapi.Message
		setupMocks     func(*MockTelegramBot, *contractmocks.MockTaskExecutor, *sync.WaitGroup)
		expectAction   bool
	}{
		// ---------------------------------------------------------------------
		// Valid Command Cases
		// ---------------------------------------------------------------------
		{
			name: "Help Command",
			commandMessage: &tgbotapi.Message{
				Text: "/help",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.Contains(msg.Text, "도움말")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name: "Task Run Command (Valid)",
			commandMessage: &tgbotapi.Message{
				Text: "/taskone_run",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				me.On("Submit", mock.Anything, mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
					return req.TaskID == "taskone" && req.CommandID == "run"
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(nil).Once()
			},
		},
		{
			name: "Cancel Command (Valid Simple ID)",
			commandMessage: &tgbotapi.Message{
				Text: "/cancel_task_1_instance_123",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				me.On("Cancel", contract.TaskInstanceID("task_1_instance_123")).
					Run(func(args mock.Arguments) {
						wg.Done()
					}).Return(nil).Once()
			},
		},
		{
			name: "Cancel Command (Valid Complex ID with Separators)",
			commandMessage: &tgbotapi.Message{
				Text: "/cancel_group_subgroup_id_123", // Separators in ID
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// Code uses SplitN(..., 2), so "cancel" is prefix, "group_subgroup_id_123" is ID
				me.On("Cancel", contract.TaskInstanceID("group_subgroup_id_123")).
					Run(func(args mock.Arguments) {
						wg.Done()
					}).Return(nil).Once()
			},
		},

		// ---------------------------------------------------------------------
		// Invalid Input & Routing Cases
		// ---------------------------------------------------------------------
		{
			name: "Empty Message",
			commandMessage: &tgbotapi.Message{
				Text: "",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.Contains(msg.Text, "등록되지 않은")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name: "Unknown Command",
			commandMessage: &tgbotapi.Message{
				Text: "/unknown_cmd",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.Contains(msg.Text, "등록되지 않은 명령어")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name: "Unknown Command with HTML chars",
			commandMessage: &tgbotapi.Message{
				Text: "/foo<i>bar</i>",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Should be escaped
					return ok && strings.Contains(msg.Text, "&lt;i&gt;bar&lt;/i&gt;")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name: "Cancel Command Malformed (No ID)",
			commandMessage: &tgbotapi.Message{
				Text: "/cancel_",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.Contains(msg.Text, "올바른 형식이 아닙니다")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name: "Check Case Sensitivity (Mismatch)",
			commandMessage: &tgbotapi.Message{
				Text: "/TASKONE_RUN", // Uppercase input
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// If exact match is required, this is "Unknown Command"
				// Command mapping is manually created in factory.go. Assuming case-sensitive map keys.
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.Contains(msg.Text, "등록되지 않은 명령어")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
		},

		// ---------------------------------------------------------------------
		// System Error Cases
		// ---------------------------------------------------------------------
		{
			name: "Task Submit Failed",
			commandMessage: &tgbotapi.Message{
				Text: "/taskone_run",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				me.On("Submit", mock.Anything, mock.Anything).Return(errors.New("queue full")).Once()
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.Contains(msg.Text, "작업 실행 요청이 실패") && strings.Contains(msg.Text, "과부하")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name: "Task Submit Timeout Check",
			commandMessage: &tgbotapi.Message{
				Text: "/taskone_run",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// Verify context deadline ~3s
				me.On("Submit", mock.MatchedBy(func(ctx context.Context) bool {
					deadline, ok := ctx.Deadline()
					if !ok {
						return false
					}
					// Verify that a deadline is set and it is reasonably close to 3s.
					// We just check if it's within [100ms, 10s] to avoid flakiness in slow CI environments.
					timeLeft := time.Until(deadline)
					return timeLeft > 100*time.Millisecond && timeLeft < 10*time.Second
				}), mock.Anything).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(nil).Once()
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			notifier, mockBot, mockExecutor, updatesChan := setupTelegramTest(t, appConfig)
			require.NotNil(t, notifier)

			var wgAction sync.WaitGroup
			if tt.setupMocks != nil {
				tt.setupMocks(mockBot, mockExecutor, &wgAction)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var wgNotifier sync.WaitGroup
			runTelegramNotifier(ctx, notifier, &wgNotifier)

			// Act
			updatesChan <- tgbotapi.Update{
				Message: tt.commandMessage,
			}

			// Assert
			if tt.expectAction {
				waitForActionWithTimeout(t, &wgAction, testTelegramTimeout)
			} else {
				// No action expected, wait brief moment
				time.Sleep(50 * time.Millisecond)
			}

			// Cleanup
			cancel()
			close(updatesChan)
			wgNotifier.Wait()

			// Verify
			mockBot.AssertExpectations(t)
			mockExecutor.AssertExpectations(t)
		})
	}
}

// TestTelegramNotifier_Resilience_PanicRecovery tests that the notifier continues to work
// after recovering from a panic in the command handler.
func TestTelegramNotifier_Resilience_PanicRecovery(t *testing.T) {
	appConfig := &config.AppConfig{Debug: true}
	notifier, mockBot, _, updatesChan := setupTelegramTest(t, appConfig)

	// Context for runner
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wgNotifier sync.WaitGroup
	runTelegramNotifier(ctx, notifier, &wgNotifier)

	var wg sync.WaitGroup

	// Step 1: Trigger Panic
	// We'll use a "Send" mock that panics.
	// Since invalid messages trigger "Send" (Unknown command), we use that.
	wg.Add(1)
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && strings.Contains(msg.Text, "등록되지 않은")
	})).Run(func(args mock.Arguments) {
		wg.Done() // Signal we reached the panic point
		panic("artificial panic in handler")
	}).Return(tgbotapi.Message{}, nil).Once()

	// Act 1
	updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: "/bad_command_triggers_panic",
			Chat: &tgbotapi.Chat{ID: testTelegramChatID},
		},
	}
	waitForActionWithTimeout(t, &wg, testTelegramTimeout)

	// Ensure service is still alive by sending a valid command (Help)
	// Step 2: Valid Request
	wg.Add(1)
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && strings.Contains(msg.Text, "도움말")
	})).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return(tgbotapi.Message{}, nil).Once()

	// Act 2
	updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: "/help",
			Chat: &tgbotapi.Chat{ID: testTelegramChatID},
		},
	}
	waitForActionWithTimeout(t, &wg, testTelegramTimeout)

	// Cleanup
	cancel()
	// Ensure updatesChan is closed to unblock any potential readers not using context (though worker does)
	// and to signal cleanup.
	close(updatesChan)
	wgNotifier.Wait()
	mockBot.AssertExpectations(t)
}
