package telegram

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// TestTelegramNotifier_SmartRetry tests that 4xx errors are not retried, while others are.
func TestTelegramNotifier_SmartRetry(t *testing.T) {
	appConfig := &config.AppConfig{}

	tests := []struct {
		name          string
		mockError     error
		expectedCalls int
		waitDuration  time.Duration
	}{
		{
			name: "400 Bad Request - Should Retry (Fallback to PlainText)",
			mockError: &tgbotapi.Error{
				Code:    400,
				Message: "Bad Request",
			},
			expectedCalls: 2, // 1st: HTML (fail), 2nd: PlainText (fail) -> Stop
			waitDuration:  500 * time.Millisecond,
		},
		{
			name: "401 Unauthorized - Should NOT Retry",
			mockError: &tgbotapi.Error{
				Code:    401,
				Message: "Unauthorized",
			},
			expectedCalls: 1,
			waitDuration:  500 * time.Millisecond,
		},
		{
			name: "500 Internal Server Error - Should Retry",
			mockError: &tgbotapi.Error{
				Code:    500,
				Message: "Internal Server Error",
			},
			expectedCalls: 3, // Default maxRetries is 3
			waitDuration:  2 * time.Second,
		},
		{
			name: "429 Too Many Requests - Should Retry",
			mockError: &tgbotapi.Error{
				Code:    429,
				Message: "Too Many Requests",
			},
			expectedCalls: 3,
			waitDuration:  2 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, mockBot, _ := setupTelegramTest(t, appConfig)

			// Disable Rate Limiter for speed
			notifier.limiter = rate.NewLimiter(rate.Inf, 0)
			// Fast retry delay for testing
			notifier.retryDelay = 50 * time.Millisecond

			var wgSend sync.WaitGroup
			wgSend.Add(tt.expectedCalls)

			// Setup Mock
			// The Run method on Mock call allows us to count valid calls
			callCount := 0
			mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
				callCount++
				wgSend.Done()
			}).Return(tgbotapi.Message{}, tt.mockError)

			// Run notifier
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var wg sync.WaitGroup
			runTelegramNotifier(ctx, notifier, &wg)

			// Act
			notifier.Notify(contract.NewTaskContext(), "Test Message")

			// Wait for calls
			// Note: We use a channel to wait with timeout
			done := make(chan struct{})
			go func() {
				wgSend.Wait()
				close(done)
			}()

			select {
			case <-done:
				// Success
			case <-time.After(tt.waitDuration):
				if callCount != tt.expectedCalls {
					t.Fatalf("Expected %d calls, but got %d (Timeout)", tt.expectedCalls, callCount)
				}
			}

			// Verify total calls
			// If we expected 1 call (no retry), checking logic ensures we didn't wait for retries that shouldn't happen.
			// However, ensuring NO MORE calls happened is tricky without a sleep.
			// But the mock expectations + waitgroup handles the "at least" part.
			// To ensure "at most", we can check mock assertions.

			// Wait a bit more to ensure no extra calls are made in case of failure logic bug
			if tt.expectedCalls == 1 {
				time.Sleep(100 * time.Millisecond)
			}

			require.Equal(t, tt.expectedCalls, callCount, "Call count mismatch")
		})
	}
}

// TestTelegramNotifier_PanicRecovery tests that the notifier recovers from panics
// and continues to process subsequent messages.
// TestTelegramNotifier_PanicRecovery tests that the notifier recovers from panics
// and continues to process subsequent messages.
// TestTelegramNotifier_PanicRecovery tests that the notifier recovers from panics
// and continues to process subsequent messages.
func TestTelegramNotifier_PanicRecovery(t *testing.T) {
	// Manual Setup to avoid conflicting expectations from setupTelegramTest
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

	// Create Notifier manually
	notifierHandler, err := newTelegramNotifierWithBot(testTelegramNotifierID, mockBot, testTelegramChatID, appConfig, mockExecutor)
	require.NoError(t, err)
	notifier := notifierHandler.(*telegramNotifier)
	notifier.retryDelay = 1 * time.Millisecond
	notifier.limiter = rate.NewLimiter(rate.Inf, 0)

	// Setup Expectations
	initDone := make(chan struct{})
	mockBot.On("GetSelf").Run(func(args mock.Arguments) {
		close(initDone)
	}).Return(tgbotapi.User{UserName: testTelegramBotUsername}).Once()

	// Return a valid channel so Run doesn't block on nil channel if that's what happens
	updatesCh := make(chan tgbotapi.Update, 1)
	mockBot.On("GetUpdatesChan", mock.Anything).Return(tgbotapi.UpdatesChannel(updatesCh)).Once()

	// StopReceivingUpdates will be called when context is cancelled
	mockBot.On("StopReceivingUpdates").Return().Maybe()

	// Run notifier
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	runTelegramNotifier(ctx, notifier, &wg)

	// Wait for Run to initialize and call GetSelf
	select {
	case <-initDone:
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for GetSelf")
	}

	// Give a tiny bit more time for Run to enter the select loop after GetSelf returns
	time.Sleep(10 * time.Millisecond)

	// 1. Trigger Panic
	// Temporarily set botAPI to nil to cause panic in sendSingleMessage
	originalBotAPI := notifier.botAPI
	notifier.botAPI = nil

	// Act 1: Send message that will cause panic
	notifier.Notify(contract.NewTaskContext(), "Panic Message")

	// Wait a bit to ensure panic handling
	time.Sleep(100 * time.Millisecond)

	// 2. Recovery & Resume
	// Restore botAPI
	notifier.botAPI = originalBotAPI

	// Setup Mock for success
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()

	// Act 2: Send normal message
	success := notifier.Notify(contract.NewTaskContext(), "Normal Message")
	require.True(t, success, "Notify should succeed")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	mockBot.AssertExpectations(t)
}
