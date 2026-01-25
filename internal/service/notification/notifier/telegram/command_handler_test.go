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
// It uses a table-driven approach to test valid commands, error scenarios, context timeouts, and edge cases.
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
				},
			},
		},
		Debug: true,
	}

	// Define test cases
	tests := []struct {
		name           string
		commandMessage *tgbotapi.Message
		setupMocks     func(*MockTelegramBot, *contractmocks.MockTaskExecutor, *sync.WaitGroup)
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
			name: "Task Run Command",
			commandMessage: &tgbotapi.Message{
				Text: "/taskone_run", // taskone + run -> taskone_run
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// Task submission uses the parent context (serviceStopCtx), so we don't check for short timeout deadline here
				// but verify the Submit call itself.
				me.On("Submit", mock.Anything, mock.MatchedBy(func(req *contract.TaskSubmitRequest) bool {
					return req.TaskID == "taskone" && req.CommandID == "run"
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
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// Expect cancellation of specific instance
				me.On("Cancel", contract.TaskInstanceID("task_1_instance_123")).
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
				Text: "", // Empty text
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
			name: "No Prefix Command",
			commandMessage: &tgbotapi.Message{
				Text: "hello", // No '/' prefix
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
				// Verify Send is called
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Actual msg: "입력하신 명령어 '/unknown_cmd'는 등록되지 않은 명령어입니다."
					return ok && strings.Contains(msg.Text, "등록되지 않은 명령어")
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
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
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
			name: "Cancel Command - Malformed (Separator only)",
			commandMessage: &tgbotapi.Message{
				Text: "/cancel_",
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Actual msg: "입력하신 명령어 '/cancel_'는 올바른 형식이 아닙니다."
					return ok && strings.Contains(msg.Text, "올바른 형식이 아닙니다")
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
				// Executor fails
				me.On("Submit", mock.Anything, mock.Anything).Return(errors.New("queue full")).Once()

				// Expect error notification to user
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Actual msg: "작업 실행 요청이 실패했습니다."
					return ok && strings.Contains(msg.Text, "작업 실행 요청이 실패")
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
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// Executor fails (e.g. not found)
				me.On("Cancel", mock.Anything).Return(errors.New("not found")).Once()

				// Expect error notification to user
				mb.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Actual msg: "작업 취소 요청이 실패했습니다."
					return ok && strings.Contains(msg.Text, "작업 취소 요청이 실패")
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
				Text: "/panic_trigger", // Will cause panic inside mock verification
				Chat: &tgbotapi.Chat{ID: testTelegramChatID},
			},
			expectAction: true,
			setupMocks: func(mb *MockTelegramBot, me *contractmocks.MockTaskExecutor, wg *sync.WaitGroup) {
				wg.Add(1)
				// We simulate a panic by mocking a call that panics.
				// Since dispatchCommand calls replyUnknownCommand for "/panic_trigger",
				// we mock Send to panic.
				mb.On("Send", mock.Anything).Run(func(args mock.Arguments) {
					wg.Done()
					panic("simulated panic in command handler")
				}).Return(tgbotapi.Message{}, nil).Once()
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

			// Setup Expectations
			var wgAction sync.WaitGroup
			if tt.setupMocks != nil {
				tt.setupMocks(mockBot, mockExecutor, &wgAction)
			}

			// Run Notifier
			// IMPORTANT: We use a cancellable context to simulate service stop
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
