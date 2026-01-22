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
// Test Constants
// =============================================================================

const (
	testTelegramChatID      = int64(12345)
	testTelegramBotUsername = "test_bot"
	testTelegramNotifierID  = "test-notifier"
	testTelegramTimeout     = 5 * time.Second
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupTelegramTest sets up common test objects.
// 이 헬퍼는 다른 테스트 파일(sender_worker_test.go 등)에서도 사용될 수 있습니다.
func setupTelegramTest(t *testing.T, appConfig *config.AppConfig) (*telegramNotifier, *MockTelegramBot, *taskmocks.MockExecutor, chan tgbotapi.Update) {
	t.Helper()

	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}
	updatesChan := make(chan tgbotapi.Update, 100)

	args := creationArgs{
		BotToken:  "test-token",
		ChatID:    testTelegramChatID,
		AppConfig: appConfig,
	}
	notifierHandler, err := newNotifierWithClient(testTelegramNotifierID, mockBot, mockExecutor, args)
	require.NoError(t, err)
	notifier := notifierHandler.(*telegramNotifier)
	notifier.retryDelay = 1 * time.Millisecond
	notifier.rateLimiter = rate.NewLimiter(rate.Inf, 0)

	// Common expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: testTelegramBotUsername}).Maybe()
	mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(updatesChan)).Maybe()
	mockBot.On("StopReceivingUpdates").Return().Maybe()

	return notifier, mockBot, mockExecutor, updatesChan
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
// Used in other tests as well.
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
// Notifier Lifecycle Tests
// =============================================================================

// TestNotifier_Lifecycle_Integration 통합 테스트: 시작 -> 실행 -> 종료
// Run 메서드가 Sender와 Receiver를 정상적으로 시작하고,
// 컨텍스트 취소 시 Graceful Shutdown이 수행되는지 전체 흐름을 검증합니다.
func TestNotifier_Lifecycle_Integration(t *testing.T) {
	// 1. Setup
	appConfig := &config.AppConfig{}
	notifier, mockBot, _, updatesChan := setupTelegramTest(t, appConfig)

	// Shutdown 시 호출되는 StopReceivingUpdates 검증을 위해 명확한 Expectation 설정
	// setupTelegramTest의 Maybe()를 덮어쓰거나, 호출 확인을 위해 새로 설정하지 않고
	// Mock 객체의 호출 기록을 나중에 검증합니다.

	// 2. Start Notifier
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	runTelegramNotifier(ctx, notifier, &wg)

	// 3. Verify Running State
	// 잠시 대기하여 내부 고루틴들이 시작되게 함
	time.Sleep(100 * time.Millisecond)

	assert.False(t, notifier.isClosed(), "Notifier should be running (not closed)")

	// Receiver 동작 확인: 채널이 열려있어야 함 (Long Polling)
	select {
	case _, ok := <-updatesChan:
		if !ok {
			t.Fatal("Updates channel closed unexpectedly")
		}
	default:
		// Channel is open and empty, good.
	}

	// 4. Trigger Shutdown
	cancel()

	// 5. Wait for Graceful Shutdown
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Notifier did not shut down gracefully in time")
	}

	// 6. Verify Post-Shutdown State
	assert.True(t, notifier.isClosed(), "Notifier should be closed after shutdown")
	mockBot.AssertCalled(t, "StopReceivingUpdates")
}

// TestNotifier_IsClosed 상태 조회 메서드 검증
func TestNotifier_IsClosed(t *testing.T) {
	appConfig := &config.AppConfig{}
	notifier, _, _, _ := setupTelegramTest(t, appConfig)

	// Initial State
	assert.False(t, notifier.isClosed(), "Initially correctly open")

	// Close Manually (Mocking what Run/cleanup does)
	notifier.Close()

	// Final State
	assert.True(t, notifier.isClosed(), "Closed after Close() called")
}
