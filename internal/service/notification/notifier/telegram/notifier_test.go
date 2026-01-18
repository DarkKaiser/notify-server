package telegram

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// =============================================================================
// Test Constants
// =============================================================================

const (
	testTelegramChatID      = int64(12345)
	testTelegramBotUsername = "test_bot"
	testTelegramNotifierID  = "test-notifier"
	testTelegramTimeout     = 1 * time.Second
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupTelegramTest sets up common test objects.
func setupTelegramTest(t *testing.T, appConfig *config.AppConfig) (*telegramNotifier, *MockTelegramBot, *taskmocks.MockExecutor) {
	t.Helper()

	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockExecutor := &taskmocks.MockExecutor{}

	opts := options{
		BotToken:  "test-token",
		ChatID:    testTelegramChatID,
		AppConfig: appConfig,
	}
	notifierHandler, err := newTelegramNotifierWithBot(testTelegramNotifierID, mockBot, mockExecutor, opts)
	require.NoError(t, err)
	notifier := notifierHandler.(*telegramNotifier)
	notifier.retryDelay = 1 * time.Millisecond
	notifier.limiter = rate.NewLimiter(rate.Inf, 0)

	// Common expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: testTelegramBotUsername}).Maybe()
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil).Maybe()
	mockBot.On("StopReceivingUpdates").Return().Maybe()

	return notifier, mockBot, mockExecutor
}

// runTelegramNotifier runs the notifier in a goroutine.
func runTelegramNotifier(ctx context.Context, notifier *telegramNotifier, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		notifier.Run(ctx)
	}()
}

// waitForActionWithTimeout waits for a waitgroup with timeout.
func waitForActionWithTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(timeout):
		t.Fatal("Timeout waiting for action")
	}
}

// =============================================================================
// Lifecycle & Run Loop Tests
// =============================================================================

// TestTelegramNotifier_Run_Drain tests that the notifier processes remaining messages
// after the context is cancelled (Graceful Shutdown).
func TestTelegramNotifier_Run_Drain(t *testing.T) {
	// Setup
	appConfig := &config.AppConfig{}
	notifier, mockBot, _ := setupTelegramTest(t, appConfig)
	require.NotNil(t, notifier)

	// Expectation: 5 messages will be sent
	var wgSend sync.WaitGroup
	wgSend.Add(5)

	mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		wgSend.Done()
	}).Return(tgbotapi.Message{}, nil).Times(5)

	// Run notifier
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	runTelegramNotifier(ctx, notifier, &wg)

	// Act: Send 5 messages
	taskCtx := contract.NewTaskContext()
	for i := 0; i < 5; i++ {
		notifier.Notify(taskCtx, "Drain Message")
	}

	// Trigger Shutdown immediately
	time.Sleep(100 * time.Millisecond) // Ensure messages are queued
	cancel()

	// Wait for shutdown and drain
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

// TestTelegramNotifier_Run_ChannelClosed_Fix checks loop exit on closed channel.
func TestTelegramNotifier_Run_ChannelClosed_Fix(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

	opts := options{
		BotToken:  "test-token",
		ChatID:    11111,
		AppConfig: appConfig,
	}
	nHandler, err := newTelegramNotifierWithBot("test-notifier", mockBot, mockExecutor, opts)
	require.NoError(t, err)
	n := nHandler.(*telegramNotifier)

	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "TestBot"})
	updatesChan := make(chan tgbotapi.Update)
	mockBot.updatesChan = updatesChan
	mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(updatesChan))
	mockBot.On("StopReceivingUpdates").Return()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runResult := make(chan struct{})
	go func() {
		n.Run(ctx)
		close(runResult)
	}()

	close(updatesChan)

	select {
	case <-runResult:
	case <-time.After(2 * time.Second):
		t.Fatal("Run loop did not exit")
	}
}

// TestRunSender_GracefulShutdown_InFlightMessage checks message delivery on shutdown.
func TestRunSender_GracefulShutdown_InFlightMessage(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

	opts := options{
		BotToken:  "test-token",
		ChatID:    12345,
		AppConfig: appConfig,
	}
	nHandler, err := newTelegramNotifierWithBot("test-notifier", mockBot, mockExecutor, opts)
	require.NoError(t, err)
	n := nHandler.(*telegramNotifier)

	msg := "In-Flight Message"
	var wgSender sync.WaitGroup
	wgSender.Add(1)
	sendCalled := make(chan struct{})

	mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		close(sendCalled)
	}).Return(tgbotapi.Message{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	// TestRunSender_GracefulShutdown_InFlightMessage
	// ...
	go func() {
		defer wgSender.Done()
		n.runSender(ctx)
	}()

	n.Notify(nil, msg)
	time.Sleep(1 * time.Millisecond)
	cancel()

	select {
	case <-sendCalled:
	case <-time.After(1 * time.Second):
		t.Fatal("In-flight message lost")
	}

	n.Close()
	wgSender.Wait()
	mockBot.AssertExpectations(t)
}

// TestTelegramNotifier_PanicRecovery tests that the notifier recovers from panics.
func TestTelegramNotifier_PanicRecovery(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

	// Create Notifier manually
	opts := options{
		BotToken:  "test-token",
		ChatID:    testTelegramChatID,
		AppConfig: appConfig,
	}
	notifierHandler, err := newTelegramNotifierWithBot(testTelegramNotifierID, mockBot, mockExecutor, opts)
	require.NoError(t, err)
	notifier := notifierHandler.(*telegramNotifier)
	notifier.retryDelay = 1 * time.Millisecond
	notifier.limiter = rate.NewLimiter(rate.Inf, 0)

	// Setup Expectations
	initDone := make(chan struct{})
	mockBot.On("GetSelf").Run(func(args mock.Arguments) {
		close(initDone)
	}).Return(tgbotapi.User{UserName: testTelegramBotUsername}).Once()

	updatesCh := make(chan tgbotapi.Update, 1)
	mockBot.On("GetUpdatesChan", mock.Anything).Return(tgbotapi.UpdatesChannel(updatesCh)).Once()
	mockBot.On("StopReceivingUpdates").Return().Maybe()

	// Run notifier
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	runTelegramNotifier(ctx, notifier, &wg)

	// Wait for Run to initialize
	select {
	case <-initDone:
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for GetSelf")
	}
	time.Sleep(10 * time.Millisecond)

	// 1. Trigger Panic
	originalBotAPI := notifier.botAPI
	notifier.botAPI = nil // Cause nil pointer panic

	notifier.Notify(contract.NewTaskContext(), "Panic Message")
	time.Sleep(100 * time.Millisecond) // Wait for panic and recovery

	// 2. Recovery & Resume
	notifier.botAPI = originalBotAPI
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()

	success := notifier.Notify(contract.NewTaskContext(), "Normal Message")
	require.True(t, success, "Notify should succeed")

	time.Sleep(100 * time.Millisecond)
	mockBot.AssertExpectations(t)
}

// TestTelegramNotifier_Concurrency checks correct concurrent processing.
func TestTelegramNotifier_Concurrency(t *testing.T) {
	mockBot := &MockTelegramBot{}
	updateC := make(chan tgbotapi.Update, 100)

	mockBot.updatesChan = updateC
	mockBot.On("GetUpdatesChan", mock.AnythingOfType("tgbotapi.UpdateConfig")).Return(tgbotapi.UpdatesChannel(updateC))
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("StopReceivingUpdates").Return()

	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Run(func(args mock.Arguments) {
		time.Sleep(50 * time.Millisecond) // Faster than 100ms for test speed
	})

	n := &telegramNotifier{
		Base:   notifier.NewBase("test", true, 10, time.Second),
		botAPI: mockBot,
		chatID: 12345,
		botCommands: []telegramBotCommand{
			{command: "help"},
		},
		notifierSemaphore: make(chan struct{}, 100),
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		n.Run(ctx)
	}()

	for i := 0; i < 5; i++ {
		n.Notify(contract.NewTaskContext(), "Slow Message")
	}

	cmdUpdate := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: 12345},
			Text: "/help",
		},
	}
	updateC <- cmdUpdate

	time.Sleep(200 * time.Millisecond)
	cancel()
	wg.Wait()

	mockBot.AssertExpectations(t)
}
