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
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Command Handler Tests
// =============================================================================

// TestTelegramNotifier_DispatchCommand provides comprehensive coverage for command handling.
// It uses a table-driven approach to test valid commands, error scenarios, and edge cases.
func TestTelegramNotifier_DispatchCommand(t *testing.T) {
	// Common AppConfig for tests
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

	// Define test cases
	tests := []struct {
		name           string
		commandMessage *tgbotapi.Message
		setupMocks     func(*MockTelegramBot, *taskmocks.MockExecutor, *sync.WaitGroup)
		expectAction   bool // If true, waits for an async action (Send or Submit)
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
			setupMocks: func(mb *MockTelegramBot, me *taskmocks.MockExecutor, wg *sync.WaitGroup) {
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
			name: "Task Run Command",
			commandMessage: &tgbotapi.Message{
				Text: "/task_1_run", // Mapped to task1:run by logic (simulated lookup) or direct config?
				// Note: command.go uses lookupCommand which checks botCommandsByName.
				// We need to ensure the notifier is initialized with these commands.
				// The setupTelegramTeast initializes notifier with appConfig.
				// However, config parsing logic creates the name.
				// Assuming "task1" + "run" -> "task1_run" (or similar).
				// Let's assume the factory logic creates "task1_run".
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *taskmocks.MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				me.On("Submit", mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
					return req.TaskID == "task1" && req.CommandID == "run"
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(nil).Once()
			},
		},
		{
			name: "Cancel Command - Valid",
			commandMessage: &tgbotapi.Message{
				Text: "/cancel_task_1_instance_123",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *taskmocks.MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// Expect cancellation of specific instance
				me.On("Cancel", contract.TaskInstanceID("task_1_instance_123")).
					Run(func(args mock.Arguments) {
						wg.Done()
					}).Return(nil).Once()
			},
		},

		// ---------------------------------------------------------------------
		// Error Handling Cases
		// ---------------------------------------------------------------------
		{
			name: "Unknown Command",
			commandMessage: &tgbotapi.Message{
				Text: "/unknown_cmd",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *taskmocks.MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Actual msg: "'/unknown_cmd'는 등록되지 않은 명령어입니다."
					return ok && strings.Contains(msg.Text, "등록되지 않은")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name: "Unknown Command - HTML Escaping",
			commandMessage: &tgbotapi.Message{
				Text: "/unknown<script>",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *taskmocks.MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Verify input is escaped: <script> -> &lt;script&gt;
					return ok && strings.Contains(msg.Text, "&lt;script&gt;")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name: "Cancel Command - Malformed (No ID)",
			// Note: "/cancel" without separator doesn't match prefix "cancel_", so it falls through to Unknown Command.
			commandMessage: &tgbotapi.Message{
				Text: "/cancel",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *taskmocks.MockExecutor, wg *sync.WaitGroup) {
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
			name: "Cancel Command - Malformed (Separator only)",
			commandMessage: &tgbotapi.Message{
				Text: "/cancel_",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *taskmocks.MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// Splits to ["cancel", ""]. Calls Cancel(""). Mock return error.
				me.On("Cancel", contract.TaskInstanceID("")).Return(errors.New("invalid id")).Once()

				// Expect failure notification
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Actual msg: "작업취소 요청이 실패하였습니다"
					return ok && strings.Contains(msg.Text, "작업취소 요청이 실패")
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
				Text: "/task_1_run",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *taskmocks.MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// Executor fails
				me.On("Submit", mock.Anything).Return(errors.New("queue full")).Once()

				// Expect error notification to user
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Actual msg: "사용자가 요청한 작업의 실행 요청이 실패하였습니다."
					return ok && strings.Contains(msg.Text, "실행 요청이 실패")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name: "Cancel Execution Failed",
			commandMessage: &tgbotapi.Message{
				Text: "/cancel_task_1_inst_999",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *taskmocks.MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// Executor fails (e.g. not found)
				me.On("Cancel", mock.Anything).Return(errors.New("not found")).Once()

				// Expect error notification to user
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Actual msg: "작업취소 요청이 실패하였습니다.(ID:...)"
					return ok && strings.Contains(msg.Text, "작업취소 요청이 실패")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
		},

		// ---------------------------------------------------------------------
		// Edge Cases
		// ---------------------------------------------------------------------
		{
			name: "Panic Recovery",
			commandMessage: &tgbotapi.Message{
				Text: "/panic_trigger", // Note: This requires a way to trigger panic.
				// Since we can't easily inject code into dispatchCommand to panic *before* routing without code change,
				// we simulate panic within a mock call if possible, or reliance on known behavior.
				// dispatchCommand itself calls lookup or string ops.
				// To test the *defer recover* inside dispatchCommand, we need a panic to happen *inside* the function.
				// Best way: Trigger panic in one of the called methods (e.g., lookupCommand or replyUnknown).
				// Let's use an unknown command but make MockBot.Send panic.
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *taskmocks.MockExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// We expect replyUnknown to be called.
				// Make Send panic.
				mb.On("Send", mock.Anything).Run(func(args mock.Arguments) {
					wg.Done()
					panic("simulated panic in command handler")
				}).Return(tgbotapi.Message{}, nil).Once()

				// The panic should be recovered in dispatchCommand.
				// We need to ensure the test doesn't crash and we can verify recovery.
				// Since dispatchCommand logs the panic, we assume it recovers.
				// We wait for the panic action to complete (Send called).
				// NOTE: wg.Done must happen even if panic occurs?
				// The panic handler inside dispatchCommand does NOT signal the WaitGroup we passed in test setup/expectations.
				// However, 'Run' in mock executes before return/panic? No, if we panic, we panic.
				// We should ideally use a side-channel or ensure the test control simply doesn't hang.
				// Here, relying on 'Send' being called matches 'expectAction'.
				// But since it panics, we must ensure wg.Done is called.
				// We can defer wg.Done in the Run block if we want, but better:
				// The mock Run runs, then we panic.
				// The test runner catches panic? No, dispatchCommand catches it.
				// So processing continues? No, dispatchCommand returns (recover).
				// Our waitForActionWithTimeout checks wg.
				// So we must ensure wg.Done() is called.
				// Ideally, we signal *before* panic.
				// "wg.Done(); panic(...)"
			},
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			notifier, mockBot, mockExecutor, updatesChan := setupTelegramTest(t, appConfig)
			require.NotNil(t, notifier)

			// Manually inject a known command for the "Task Run" test case if needed
			// Since setupTelegramTest uses factory which reads appConfig,
			// if appConfig has "task1" / "run", the factory should have created the command mapping.
			// However, the exact command name depends on factory logic: likely "task1_run"
			// (TaskID + "_" + CommandID is a common pattern, let's assume it's "task1_run" for now).
			// If lookup fails, the test will look like "Unknown Command".

			// Setup Expectations
			var wgAction sync.WaitGroup
			if tt.setupMocks != nil {
				tt.setupMocks(mockBot, mockExecutor, &wgAction)
			}

			// Configure a specialized panic trigger for the "Panic Recovery" case
			if tt.name == "Panic Recovery" {
				// We override the setup to ensure wg.Done is called before panic
				// The setupMocks above already defines the behavior:
				// mb.On("Send", ...).Run(func() { wg.Done(); panic(...) })
				// This ensures our test waitgroup is satisfied before the stack unwinds.
			}

			// Run Notifier
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var wgNotifier sync.WaitGroup
			runTelegramNotifier(ctx, notifier, &wgNotifier)

			// Act: Send Update
			updatesChan <- tgbotapi.Update{
				Message: tt.commandMessage,
			}

			// Assert: Wait for expected action
			if tt.expectAction {
				waitForActionWithTimeout(t, &wgAction, testTelegramTimeout)
			} else {
				// If no action expected, give a short time to ensure nothing happens (negative test)
				// But we usually verify "no method called" via mock assertions at the end.
				time.Sleep(100 * time.Millisecond)
			}

			// Cleanup
			cancel()
			wgNotifier.Wait()

			// Verify Mocks
			mockBot.AssertExpectations(t)
			mockExecutor.AssertExpectations(t)
		})
	}
}
