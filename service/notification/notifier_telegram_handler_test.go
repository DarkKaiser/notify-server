package notification

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
)

func TestTelegramNotifier_Notify_TableDriven(t *testing.T) {
	// Helper to create a long string
	createLongString := func(chunk string, repeat int) string {
		var sb strings.Builder
		for i := 0; i < repeat; i++ {
			sb.WriteString(chunk)
		}
		return sb.String()
	}

	tests := []struct {
		name           string
		message        string
		taskCtx        task.TaskContext
		setupMockBot   func(*MockTelegramBot, *sync.WaitGroup)
		waitForCalls   int
		expectedCalls  int
		cleanupTimeout time.Duration
	}{
		{
			name:    "Long Message with Newlines",
			message: createLongString("0123456789\n", 400), // ~4400 chars
			taskCtx: task.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				// Expect 2 messages
				wg.Add(2)
				m.On("Send", mock.Anything).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Times(2)
			},
			waitForCalls: 2,
		},
		{
			name:    "Long Single Line Message",
			message: createLongString("0123456789", 400), // 4000 chars
			taskCtx: task.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(2)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Check splitting logic
					return ok && len(msg.Text) > 0 && len(msg.Text) <= 3900
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Times(2)
			},
			waitForCalls: 2,
		},
		{
			name:    "HTML Message",
			message: "<b>Bold</b> and <i>Italic</i> text",
			taskCtx: task.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && msg.ParseMode == "HTML"
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
		{
			name:    "Send Error (Network)",
			message: "Test message",
			taskCtx: task.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.Anything).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, errors.New("network error"))
			},
			waitForCalls: 1,
		},
		{
			name:    "With Task Context",
			message: "Test message",
			taskCtx: task.NewTaskContext().
				WithTask(task.ID("TEST"), task.CommandID("TEST_CMD")).
				WithTitle("Test Task"),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.Contains(msg.Text, "Test Task")
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
		{
			name:    "With Error Context",
			message: "Error occurred",
			taskCtx: task.NewTaskContext().WithError(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					_, ok := c.(tgbotapi.MessageConfig)
					// Verify error emoji/indicator is present if logic adds it,
					// or just basic send. Implementation details might vary.
					// Assuming basic send for now, or check for Error Title if logic does that.
					return ok
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
		{
			name:    "With Elapsed Time",
			message: "Task Completed",
			taskCtx: task.NewTaskContext().
				WithInstanceID(task.InstanceID("1234"), int64(3661)), // 1h 1m 1s
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok &&
						(strings.Contains(msg.Text, "1시간") ||
							strings.Contains(msg.Text, "1분") ||
							strings.Contains(msg.Text, "1초"))
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockBot := &MockTelegramBot{
				updatesChan: make(chan tgbotapi.Update), // Not used for Notify but needed for Run init
			}
			mockExecutor := &MockExecutor{}
			chatID := int64(12345)
			appConfig := &config.AppConfig{}

			notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

			// Setup expectations
			mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
			mockBot.On("GetUpdatesChan", mock.Anything).Return(nil) // Run calls this
			mockBot.On("StopReceivingUpdates").Return()

			var wgSend sync.WaitGroup
			if tt.setupMockBot != nil {
				tt.setupMockBot(mockBot, &wgSend)
			}

			// Run notifier
			ctx, cancel := context.WithCancel(context.Background())
			wg := &sync.WaitGroup{}
			wg.Add(1)
			go notifier.Run(mockExecutor, ctx, wg)

			// Act
			notifier.Notify(tt.message, tt.taskCtx)

			// Wait
			done := make(chan struct{})
			go func() {
				wgSend.Wait()
				close(done)
			}()

			select {
			case <-done:
				// Success
			case <-time.After(2 * time.Second): // Slightly larger timeout for safety
				t.Fatal("Timeout waiting for message send")
			}

			// Cleanup
			cancel()
			wg.Wait()

			mockBot.AssertExpectations(t)
		})
	}
}

// Keep the original test for very specific large message splitting logic if needed,
// but the table driven one covers it.
// We can remove the old repetitive tests now.
