package telegram

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// =============================================================================
// Test Helper & Setup
// =============================================================================

// receiverTestEnv encapsulates the test environment for Receiver Worker
type receiverTestEnv struct {
	t            *testing.T
	ctx          context.Context
	cancel       context.CancelFunc
	notifier     *telegramNotifier
	mockBot      *MockTelegramBot
	mockExecutor *taskmocks.MockExecutor
	updatesChan  chan tgbotapi.Update
	wg           sync.WaitGroup
}

// setupReceiverTest initializes common test dependencies
func setupReceiverTest(t *testing.T) *receiverTestEnv {
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}
	appConfig := &config.AppConfig{}

	args := creationArgs{
		BotToken:  "test-token",
		ChatID:    12345,
		AppConfig: appConfig,
	}

	nHandler, err := newNotifierWithClient("test-notifier", mockBot, mockExecutor, args)
	require.NoError(t, err)
	n := nHandler.(*telegramNotifier)

	// Override rate limiter to allow unlimited for testing logic speed
	n.rateLimiter = rate.NewLimiter(rate.Inf, 0)
	n.retryDelay = 1 * time.Millisecond // Fast retry

	ctx, cancel := context.WithCancel(context.Background())

	return &receiverTestEnv{
		t:            t,
		ctx:          ctx,
		cancel:       cancel,
		notifier:     n,
		mockBot:      mockBot,
		mockExecutor: mockExecutor,
		updatesChan:  make(chan tgbotapi.Update, 100), // Buffered update channel
		wg:           sync.WaitGroup{},
	}
}

// cleanup ensures resources are released
func (e *receiverTestEnv) cleanup() {
	e.cancel()
	// Tests should wait for wg manually if they started the worker
}

// =============================================================================
// Receiver Logic Tests
// =============================================================================

// TestReceiverWorker_Process_Flow validates the standard message processing flow:
// 1. Valid update received -> 2. Semaphore acquired -> 3. Dispatch successful.
func TestReceiverWorker_Process_Flow(t *testing.T) {
	env := setupReceiverTest(t)
	defer env.cleanup()

	// Arrange: Ensure semaphore is open
	env.notifier.commandSemaphore = make(chan struct{}, 10)

	// Arrange: Mock Send behavior (as 'dispatchCommand' sends reply for unknown commands)
	// We send a command that we know triggers a reply, or we just verify dispatch happened.
	// Since dispatchCommand calls internal logic that eventually talks to Bot API or Executor,
	// let's assume "/start" might trigger something.
	// Whatever it triggers, we want to ensure the worker didn't crash.
	//
	// NOTE: Since we didn't mock 'Send' inside dispatchCommand in a granular way here,
	// we assume standard behavior. If dispatchCommand calls Send, our mockBot needs to handle it.
	// Let's make mockBot permissive.
	env.mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Maybe()

	// Act: Start Receiver Worker manually
	go env.notifier.receiveAndDispatchCommands(env.ctx, env.updatesChan, &env.wg)

	// Act: Send a valid update
	env.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 12345},
			Text:      "/ping",
		},
	}

	// Assert: Wait for processing (non-deterministic but robust enough with channel check)
	// We verify the channel is drained.
	assert.Eventually(t, func() bool {
		return len(env.updatesChan) == 0
	}, 1*time.Second, 10*time.Millisecond, "Updates channel should be consumed")

	// Cleanup will happen via defer env.cleanup()
}

// TestReceiverWorker_Ignore_Invalid_Updates verifies that invalid messages
// (Wrong ChatID, Non-text) are ignored without consuming semaphore resources.
func TestReceiverWorker_Ignore_Invalid_Updates(t *testing.T) {
	env := setupReceiverTest(t)
	defer env.cleanup()

	// Arrange: Spy on semaphore
	env.notifier.commandSemaphore = make(chan struct{}, 10)

	go env.notifier.receiveAndDispatchCommands(env.ctx, env.updatesChan, &env.wg)

	// 1. Wrong ChatID
	env.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: 99999}, // Invalid
			Text: "/ping",
		},
	}

	// 2. Non-text Message (Photo)
	env.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat:  &tgbotapi.Chat{ID: 12345},
			Photo: []tgbotapi.PhotoSize{{}}, // Photo
		},
	}

	// 3. Update without Message
	env.updatesChan <- tgbotapi.Update{
		EditedMessage: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: 12345},
			Text: "/ping",
		},
	}

	// Assert: Channel drained (consumed), but Semaphore should be empty (no dispatch)
	assert.Eventually(t, func() bool {
		return len(env.updatesChan) == 0
	}, 1*time.Second, 10*time.Millisecond, "All invalid updates should be consumed (ignored)")

	assert.Equal(t, 0, len(env.notifier.commandSemaphore), "Semaphore should remain empty as no tasks were dispatched")
}

// TestReceiverWorker_Backpressure verifies that when the semaphore is full:
// 1. Request is dropped (channel consumed).
// 2. TrySend is called to send a "System Busy" message.
func TestReceiverWorker_Backpressure(t *testing.T) {
	env := setupReceiverTest(t)
	defer env.cleanup()

	// Arrange: Fill the semaphore completely
	semaphoreSize := 1
	env.notifier.commandSemaphore = make(chan struct{}, semaphoreSize)
	env.notifier.commandSemaphore <- struct{}{} // Occupy the only slot

	// Arrange: Mock TrySend behavior verification
	// Since we can't easily mock internal TrySend of notifier struct (it calls Base.TrySend),
	// we rely on the fact that TrySend will push to env.notifier.NotificationC().
	// But Wait... telegramNotifier embeds Base. Base has NotificationC channel.
	// If TrySend succeeds, it puts message into that channel.
	// If TrySend fails (Queue Full), it returns error.
	//
	// In the worker code:
	// if err := n.TrySend(...); err != nil { Log Error }
	//
	// We want to verify TrySend was called.
	// If the NotificationC has capacity, TrySend succeeds and we see a message in it.
	// Let's ensure Base has buffer.
	// newNotifierWithClient creates Base with buffer.

	// Ensure there is space in Notifications Queue to receive the "Busy" message
	require.True(t, cap(env.notifier.NotificationC()) > 0)

	// Act: Start Worker
	go env.notifier.receiveAndDispatchCommands(env.ctx, env.updatesChan, &env.wg)

	// Act: Send a command while semaphore is full
	env.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: 12345},
			Text: "/heavy_task",
		},
	}

	// Assert:
	// 1. Update consumed (processed)
	assert.Eventually(t, func() bool {
		return len(env.updatesChan) == 0
	}, 1*time.Second, 10*time.Millisecond, "Update should be dropped/consumed")

	// 2. Semaphore still full (no new task started)
	assert.Equal(t, 1, len(env.notifier.commandSemaphore))

	// 3. "System Busy" notification sent via TrySend -> NotificationC
	select {
	case req := <-env.notifier.NotificationC():
		assert.Contains(t, req.Notification.Message, "시스템 이용자가 많아", "Should send busy message")
	case <-time.After(1 * time.Second):
		t.Fatal("Expected 'System Busy' notification in queue, but found none")
	}
}

// TestReceiverWorker_Shutdown_ChannelClosed verifies loop exit when Update Channel closes.
func TestReceiverWorker_Shutdown_ChannelClosed(t *testing.T) {
	env := setupReceiverTest(t)
	defer env.cleanup()

	done := make(chan struct{})
	go func() {
		env.notifier.receiveAndDispatchCommands(env.ctx, env.updatesChan, &env.wg)
		close(done)
	}()

	// Act: Close updates channel
	close(env.updatesChan)

	// Assert: Loop exits
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Worker loop did not exit on channel close")
	}
}

// TestReceiverWorker_Shutdown_ContextCancel verifies loop exit when Context is canceled.
func TestReceiverWorker_Shutdown_ContextCancel(t *testing.T) {
	env := setupReceiverTest(t)
	// No defer cleanup here, we call cancel explicitly

	done := make(chan struct{})
	go func() {
		env.notifier.receiveAndDispatchCommands(env.ctx, env.updatesChan, &env.wg)
		close(done)
	}()

	// Act: Cancel Context
	env.cancel()

	// Assert: Loop exits
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Worker loop did not exit on context cancel")
	}
}
