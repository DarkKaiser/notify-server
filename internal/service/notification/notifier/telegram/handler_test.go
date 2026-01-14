package telegram

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Handler Tests
// =============================================================================

// TestTelegramNotifier_Notify tests notification messages.
func TestTelegramNotifier_Notify(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		taskCtx      task.TaskContext
		setupMockBot func(*MockTelegramBot, *sync.WaitGroup)
		waitForCalls int
	}{
		{
			name:    "Simple Message",
			message: "Hello World",
			taskCtx: task.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && msg.Text == "Hello World"
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
		{
			name:    "Message Send Error",
			message: "Fail Message",
			taskCtx: task.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(3)
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
					return ok
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
			appConfig := &config.AppConfig{}
			notifier, mockBot, _ := setupTelegramTest(t, appConfig)
			require.NotNil(t, notifier)
			require.NotNil(t, mockBot)

			// Setup expectations
			var wgSend sync.WaitGroup
			if tt.setupMockBot != nil {
				tt.setupMockBot(mockBot, &wgSend)
			}

			// Run notifier
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var wg sync.WaitGroup
			runTelegramNotifier(ctx, notifier, &wg)

			// Act
			notifier.Notify(tt.taskCtx, tt.message)

			// Wait
			waitForActionWithTimeout(t, &wgSend, 2*time.Second)

			// Cleanup
			cancel()
			wg.Wait()

			mockBot.AssertExpectations(t)
		})
	}
}
