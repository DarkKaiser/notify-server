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
// Helper Functions
// =============================================================================

// setupTestNotifier initializes a fully functional telegramNotifier for testing.
// It uses the actual factory function to ensure all internal fields (including Base) are correctly set up.
func setupTestNotifier(t *testing.T) (*telegramNotifier, *MockTelegramBot, *taskmocks.MockExecutor) {
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

	args := creationArgs{
		BotToken:  "test-token",
		ChatID:    12345,
		AppConfig: appConfig,
	}

	// Use the factory to ensure proper initialization
	nHandler, err := newNotifierWithClient("test-notifier", mockBot, mockExecutor, args)
	require.NoError(t, err)

	n := nHandler.(*telegramNotifier)

	// Custom adjustments for testing speed
	n.retryDelay = time.Microsecond
	n.rateLimiter = rate.NewLimiter(rate.Inf, 0)

	// Expect GetSelf to be called during startup logging/checks
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"}, nil).Maybe()

	return n, mockBot, mockExecutor
}

// =============================================================================
// Sender Worker Tests (Lifecycle & Happy Path)
// =============================================================================

func TestSenderWorker_Start_Lifecycle(t *testing.T) {
	n, _, _ := setupTestNotifier(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		n.sendNotifications(ctx)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Worker did not exit gracefully on context cancel")
	}
}

func TestSenderWorker_ProcessSendQueue(t *testing.T) {
	n, mockBot, _ := setupTestNotifier(t)

	var wg sync.WaitGroup
	wg.Add(3)

	mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return(tgbotapi.Message{}, nil).Times(3)

	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		n.sendNotifications(ctx)
		close(workerDone)
	}()

	n.Send(context.Background(), contract.NewNotification("Msg 1"))
	n.Send(context.Background(), contract.NewNotification("Msg 2"))
	n.Send(context.Background(), contract.NewNotification("Msg 3"))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for messages to be processed")
	}

	cancel()
	<-workerDone
}

// =============================================================================
// Exception Handling (Panic Recovery)
// =============================================================================

func TestSenderWorker_ProcessSendQueue_PanicRecovery(t *testing.T) {
	n, mockBot, _ := setupTestNotifier(t)

	var wg sync.WaitGroup
	wg.Add(1)

	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && msg.Text == "Panic"
	})).Run(func(args mock.Arguments) {
		panic("Intentional Panic")
	}).Return(tgbotapi.Message{}, nil).Once()

	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && msg.Text == "Normal"
	})).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return(tgbotapi.Message{}, nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		n.sendNotifications(ctx)
		close(workerDone)
	}()

	n.Send(context.Background(), contract.NewNotification("Panic"))
	n.Send(context.Background(), contract.NewNotification("Normal"))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Worker died or failed to process subsequent message after panic")
	}

	cancel()
	<-workerDone
	mockBot.AssertExpectations(t)
}

func TestSenderWorker_PanicRecovery_OuterLoop(t *testing.T) {
	n, _, _ := setupTestNotifier(t)

	// 테스트 훅 주입: 루프 시작 시 강제 패닉
	n.testHookSenderPanic = func() {
		panic("Outer Loop Panic")
	}

	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		n.sendNotifications(ctx)
		close(workerDone)
	}()

	// 패닉 후 루프가 종료될 때까지 대기
	select {
	case <-workerDone:
		// 성공: 루프 탈출
	case <-time.After(1 * time.Second):
		t.Fatal("Worker did not exit after panic")
	}

	// 검증: Notifier가 명시적으로 닫혔는지 확인 (Silent Failure 방지 확인)
	assert.True(t, n.isClosed(), "Notifier should be closed after sender panic")

	cancel()
}

// =============================================================================
// Graceful Shutdown (Drain) Tests
// =============================================================================

func TestSenderWorker_Drain_Success(t *testing.T) {
	n, mockBot, _ := setupTestNotifier(t)

	var wg sync.WaitGroup
	wg.Add(5)
	mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return(tgbotapi.Message{}, nil).Times(5)

	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		n.sendNotifications(ctx)
		close(workerDone)
	}()

	for i := 0; i < 5; i++ {
		n.Send(context.Background(), contract.NewNotification("Drain Msg"))
	}

	cancel()

	select {
	case <-workerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Drain timed out or stuck")
	}

	mockBot.AssertExpectations(t)
}

func TestSenderWorker_Drain_Timeout(t *testing.T) {
	// Drain 타임아웃 테스트: 전역 변수 shutdownTimeout을 조작하여 검증
	// 대량의 메시지(50개)를 보내고, 타임아웃(1ms) 내에 극히 일부만 처리됨을 확인

	originalTimeout := shutdownTimeout
	shutdownTimeout = 1 * time.Millisecond
	defer func() { shutdownTimeout = originalTimeout }()

	n, mockBot, _ := setupTestNotifier(t)

	// 각 메시지 처리 시간을 10ms로 설정 (타임아웃은 1ms)
	mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	}).Return(tgbotapi.Message{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		n.sendNotifications(ctx)
		close(workerDone)
	}()

	// 메시지 50개 푸시 (총 500ms 소요 예상)
	for i := 0; i < 50; i++ {
		n.Send(context.Background(), contract.NewNotification("Slow Msg"))
	}

	time.Sleep(20 * time.Millisecond)
	cancel()

	start := time.Now()
	<-workerDone
	duration := time.Since(start)

	// 검증 전략:
	// 50개를 처리하려면 최소 500ms(50 * 10ms)가 필요함.
	// 타임아웃이 1ms이므로, 200ms 내에 종료되어야 함 (즉, 모든 메시지를 처리하지 못하고 중단됨).
	assert.Less(t, duration, 200*time.Millisecond, "Drain should have timed out early")

	// 추가 검증: 실제로 처리되지 못한 메시지가 있어야 함 (Mock 호출 횟수 체크가 이상적이나 예제상 시간으로 검증)

}

func TestSenderWorker_Drain_PanicRecovery(t *testing.T) {
	n, mockBot, _ := setupTestNotifier(t)

	var wg sync.WaitGroup
	wg.Add(2)

	callCount := 0
	mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		callCount++
		if callCount == 2 {
			panic("Panic in Drain")
		}
		wg.Done()
	}).Return(tgbotapi.Message{}, nil).Times(3)

	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		n.sendNotifications(ctx)
		close(workerDone)
	}()

	n.Send(context.Background(), contract.NewNotification("1"))
	n.Send(context.Background(), contract.NewNotification("2")) // Will Panic
	n.Send(context.Background(), contract.NewNotification("3"))

	cancel()
	<-workerDone

	mockBot.AssertExpectations(t)
}

// =============================================================================
// Shutdown Triggers (Context vs Close)
// =============================================================================

func TestSenderWorker_Shutdown_Triggers(t *testing.T) {
	t.Run("Context Cancel Trigger", func(t *testing.T) {
		n, mockBot, _ := setupTestNotifier(t)
		mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()

		ctx, cancel := context.WithCancel(context.Background())
		workerDone := make(chan struct{})
		go func() {
			n.sendNotifications(ctx)
			close(workerDone)
		}()

		n.Send(context.Background(), contract.NewNotification("Msg"))
		time.Sleep(10 * time.Millisecond)

		cancel() // 트리거: Context Cancel

		select {
		case <-workerDone:
		case <-time.After(1 * time.Second):
			t.Fatal("Worker failed to shutdown on context cancel")
		}
		mockBot.AssertExpectations(t)
	})

	t.Run("Notifier Close Trigger", func(t *testing.T) {
		n, mockBot, _ := setupTestNotifier(t)
		mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()

		// Context는 취소하지 않음 (Background)
		workerDone := make(chan struct{})
		go func() {
			n.sendNotifications(context.Background())
			close(workerDone)
		}()

		n.Send(context.Background(), contract.NewNotification("Msg"))
		time.Sleep(10 * time.Millisecond)

		n.Close() // 트리거: Notifier Close (n.Done() closed)

		select {
		case <-workerDone:
		case <-time.After(1 * time.Second):
			t.Fatal("Worker failed to shutdown on Notifier.Close()")
		}
		mockBot.AssertExpectations(t)
	})
}

// =============================================================================
// Pending Sends (Wait for Enqueue)
// =============================================================================

func TestSenderWorker_WaitForPendingSends(t *testing.T) {
	n, mockBot, _ := setupTestNotifier(t)
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil)

	// 1. 버퍼 가득 채우기 (30개)
	for i := 0; i < 30; i++ {
		_ = n.Send(context.Background(), contract.NewNotification("Fill"))
	}

	// 2. 31번째 요청 (Blocking) - 별도 고루틴
	pendingDone := make(chan struct{})
	blockEntered := make(chan struct{})

	go func() {
		close(blockEntered) // 진입 신호
		_ = n.Send(context.Background(), contract.NewNotification("Pending"))
		close(pendingDone) // 완료 신호
	}()

	<-blockEntered
	// Send가 내부적으로 Block 될 때까지 조금만 기다립니다 (이 부분은 채널로 감지 불가하므로 짧은 sleep 필요)
	// 하지만 base_test.go와 달리 SenderWorker가 동작해야 풀리므로, 여기선 여전히 살짝 기다려야 함.
	// 순서를 확실히 하기 위해:
	//   Buffer Full -> Send 호출 (Block) -> Worker Start -> Consume -> Space Available -> Send Unblock
	time.Sleep(10 * time.Millisecond)

	// 3. Worker 시작 (메시지 소비 시작)
	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})

	// 소비가 시작되면 버퍼 공간이 생겨 Pending Send가 해제됨
	go func() {
		n.sendNotifications(ctx)
		close(workerDone)
	}()

	// 4. Pending Send가 완료될 때까지 대기 (Worker가 소비했으므로 가능해야 함)
	select {
	case <-pendingDone:
		// 성공: 버퍼가 비워져서 메시지가 들어감
	case <-time.After(1 * time.Second):
		t.Fatal("Blocking send was not processed (Worker not consuming?)")
	}

	// 5. 종료
	cancel()
	<-workerDone
}
