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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// =============================================================================
// Sender Worker Tests (Lifecycle & Drain)
// =============================================================================

// TestSenderWorker_Run_Drain 정상적인 종료 상황(Graceful Shutdown)에서
// 큐에 남은 메시지들이 모두 처리(Drain)되는지 검증합니다.
func TestSenderWorker_Run_Drain(t *testing.T) {
	// Setup
	appConfig := &config.AppConfig{}
	notifier, mockBot, _, _ := setupTelegramTest(t, appConfig)
	require.NotNil(t, notifier)

	// Expectation: 5 messages will be sent
	var wgSend sync.WaitGroup
	wgSend.Add(5)

	mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		wgSend.Done()
	}).Return(tgbotapi.Message{}, nil).Times(5)

	// Run notifier (Start Sender Loop)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	runTelegramNotifier(ctx, notifier, &wg)

	// Act: Send 5 messages
	for i := 0; i < 5; i++ {
		err := notifier.Send(context.Background(), contract.NewNotification("Drain Message"))
		assert.NoError(t, err)
	}

	// Trigger Shutdown immediately
	// 메시지가 채널에 들어갈 시간을 아주 잠깐 줍니다.
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Wait for shutdown and drain
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

// TestSenderWorker_GracefulShutdown_InFlightMessage 셧다운 시점에
// 이미 처리 중(In-Flight)인 메시지가 손실되지 않고 완료되는지 검증합니다.
func TestSenderWorker_GracefulShutdown_InFlightMessage(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

	args := creationArgs{
		BotToken:  "test-token",
		ChatID:    12345,
		AppConfig: appConfig,
	}
	nHandler, err := newNotifierWithClient("test-notifier", mockBot, mockExecutor, args)
	require.NoError(t, err)
	n := nHandler.(*telegramNotifier)

	msg := "In-Flight Message"
	var wgSender sync.WaitGroup
	wgSender.Add(1)
	sendCalled := make(chan struct{})

	// Send가 호출되면 신호를 보내고, 약간의 지연을 주어 처리 중임을 시뮬레이션
	mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		close(sendCalled)
		time.Sleep(50 * time.Millisecond) // 처리 지연 시뮬레이션
	}).Return(tgbotapi.Message{}, nil)

	ctx, cancel := context.WithCancel(context.Background())

	// Start only SendNotifications (Sender Worker) manually for detailed control
	go func() {
		defer wgSender.Done()
		n.sendNotifications(ctx)
	}()

	// Act
	n.Send(context.Background(), contract.NewNotification(msg))

	// Wait until Send is called
	<-sendCalled

	// Trigger Shutdown while Send is still running (sleeping)
	cancel()

	// Wait for Sender Worker to finish
	wgSender.Wait()

	n.Close()
	mockBot.AssertExpectations(t)
}

// TestSenderWorker_PanicRecovery Sender 루프 내에서 패닉이 발생했을 때
// 워커가 죽지 않고 복구되어 다음 메시지를 계속 처리하는지 검증합니다.
func TestSenderWorker_PanicRecovery(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

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

	// 1. Trigger Panic via Mock
	// 첫 번째 메시지 전송 시 패닉 발생
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		if msgConfig, ok := c.(tgbotapi.MessageConfig); ok {
			return msgConfig.Text == "Panic Message"
		}
		return false
	})).Run(func(args mock.Arguments) {
		panic("simulated panic")
	}).Return(tgbotapi.Message{}, nil).Once() // Once ensures it's expected only once

	// 2. Resume Normal Ops
	// 두 번째 메시지는 정상 처리되어야 함
	processedNormal := make(chan struct{})
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		if msgConfig, ok := c.(tgbotapi.MessageConfig); ok {
			return msgConfig.Text == "Normal Message"
		}
		return false
	})).Run(func(args mock.Arguments) {
		close(processedNormal)
	}).Return(tgbotapi.Message{}, nil).Once()

	// Act
	notifier.Send(context.Background(), contract.NewNotification("Panic Message"))

	// 패닉 복구 후 루프가 살아있는지 확인하기 위해 즉시 다음 메시지 전송
	time.Sleep(10 * time.Millisecond)
	notifier.Send(context.Background(), contract.NewNotification("Normal Message"))

	// Validation
	select {
	case <-processedNormal:
		// Success: 패닉 이후 메시지가 정상 처리됨
	case <-time.After(1 * time.Second):
		t.Fatal("Sender worker did not recover from panic")
	}

	mockBot.AssertExpectations(t)
}

// TestSenderWorker_Drain_Timeout Drain 프로세스가 타임아웃 내에 완료되지 못할 경우
// 강제로 종료되는지 검증합니다. (무한 대기 방지)
// 주의: types.go의 shutdownTimeout 상수는 60초이므로, 실제 60초를 기다리는 대신
// 로직 검증을 위해 Context 타임아웃 동작을 시뮬레이션하거나, 별도의 테스트용 메서드가 필요할 수 있습니다.
// 하지만 블랙박스 테스트 원칙상 내부 상수를 변경하기 어려우므로,
// 여기서는 drainRemainingNotifications가 context cancellation에 반응하는지를 간접적으로 확인합니다.
//
// 더 정확한 테스트를 위해 제한적인 환경(느린 전송)을 조성합니다.
func TestSenderWorker_Drain_Timeout(t *testing.T) {
	// Drain 타임아웃 테스트 (기본 동작 검증)

	appConfig := &config.AppConfig{}
	// Note: Buffer size is fixed to 30 in factory.go, so we can't change it via config for now.

	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

	args := creationArgs{
		BotToken:  "test-token",
		ChatID:    12345,
		AppConfig: appConfig,
	}
	nHandler, err := newNotifierWithClient("test-notifier", mockBot, mockExecutor, args)
	require.NoError(t, err)
	n := nHandler.(*telegramNotifier)

	// Initialize essential fields
	n.retryDelay = 1 * time.Millisecond
	n.rateLimiter = rate.NewLimiter(rate.Inf, 0)

	// Mock Send with delay
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	})

	// Start Sender Worker
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		n.sendNotifications(ctx)
	}()

	// Send messages (less than buffer size 30)
	count := 10
	for i := 0; i < count; i++ {
		// We ignore error here as we just want to fill buffer
		_ = n.Send(context.Background(), contract.NewNotification("Drain Check"))
	}
	t.Logf("Sent %d messages", count)

	// Stop Sender -> Trigger Drain
	t.Log("Cancelling context...")
	cancel()

	// Wait for finish
	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()

	select {
	case <-finished:
		t.Log("Drain finished successfully")
	case <-time.After(3 * time.Second):
		t.Fatal("Drain did not finish in time")
	}
}
