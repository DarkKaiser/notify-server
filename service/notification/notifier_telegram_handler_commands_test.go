package notification

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
)

func TestTelegramNotifier_HandleCommand(t *testing.T) {
	chatID := int64(12345)

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
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.Contains(msg.Text, "등록되지 않은 명령어")
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
					return ok && strings.Contains(msg.Text, "/help") && strings.Contains(msg.Text, "/task1_run")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
		},
		{
			name:         "Task Run Command",
			commandText:  "/task1_run",
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
			mockBot := &MockTelegramBot{
				updatesChan: make(chan tgbotapi.Update, 1),
			}
			mockExecutor := &MockExecutor{}

			// Setup notifier
			// Using type assertion to access internal method if needed, but we test public Run loop interaction
			// Just like the previous file, but focusing on different logic aspects?
			// Actually this file seems to duplicate the Run loop testing but focuses on logic.
			// The original file used `createTestNotifier` and ran `Run`.
			// We will follow the same pattern: Run the bot in a goroutine and send updates.

			notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig, mockExecutor)

			// Common Mock Expectations
			mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"}).Maybe()
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
			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				notifier.Run(ctx)
			}()

			// Send update
			mockBot.updatesChan <- tgbotapi.Update{
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{ID: chatID},
					Text: tt.commandText,
				},
			}

			// Wait if action expected
			if tt.expectAction {
				done := make(chan struct{})
				go func() {
					wgAction.Wait()
					close(done)
				}()

				select {
				case <-done:
					// Success
				case <-time.After(1 * time.Second):
					t.Fatal("Timeout waiting for command action")
				}
			}

			// Cleanup
			cancel()
			wg.Wait()

			mockBot.AssertExpectations(t)
			mockExecutor.AssertExpectations(t)
		})
	}
}
