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
		{
			name:    "With Long Title (Truncated)",
			message: "Message with long title",
			taskCtx: task.NewTaskContext().
				WithTitle(strings.Repeat("가", 300)), // 300 unicode chars > 200 limit
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(1)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// Check if title contains "..." indicating truncation
					// And check length is not excessively long (approx 200 chars + overhead)
					// "가" is 3 bytes, so 200 "가" is 600 bytes.
					// We just check if it contains "..." and does NOT contain 300 "가"s.
					hasEllipsis := strings.Contains(msg.Text, "...")
					fullTitle := strings.Repeat("가", 300)
					hasFullTitle := strings.Contains(msg.Text, fullTitle)

					return ok && hasEllipsis && !hasFullTitle
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil)
			},
			waitForCalls: 1,
		},
		{
			name:    "With Long Message (Auto Splitting)",
			message: strings.Repeat("A", 4000) + "\n" + strings.Repeat("B", 1000), // Total > 4096. "A"*4000 + "\n" + "B"*1000
			// Actual behavior with limit 3900:
			// Chunk 1: "A"*3900 (limit applied)
			// Chunk 2: "A"*100 + "\n" + "B"*1000 (remaining 'A's + newline + 'B's)
			taskCtx: task.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(2) // Expecting 2 calls
				// First call (Chunk A - Truncated at 3900)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.HasPrefix(msg.Text, "AAAA") && len(msg.Text) == 3900
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()

				// Second call (Remainder A + Chunk B)
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					// "A"*100 + "\n" + "B"*1000 = 100 + 1 + 1000 = 1101
					expectedPrefix := strings.Repeat("A", 100) + "\n" + "BBBB"
					return ok && strings.HasPrefix(msg.Text, expectedPrefix) && len(msg.Text) == 1101
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()
			},
			waitForCalls: 2,
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

// TestTelegramNotifier_SendTimeout_Log verifies that the function handles send timeouts correctly.
func TestTelegramNotifier_SendTimeout_Log(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{}
	n := &telegramNotifier{
		botAPI:     mockBot,
		chatID:     12345,
		retryDelay: 10 * time.Millisecond,
	}

	// Mock 설정: Send 호출 시 즉시 에러 반환 (재시도 로직 진입 유도)
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, &tgbotapi.Error{Code: 500, Message: "Internal Server Error"}).Maybe()

	// 짧은 타임아웃 컨텍스트 생성 (50ms)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// 재시도 대기 시간을 타임아웃보다 길게 설정 (100ms)
	n.retryDelay = 100 * time.Millisecond

	start := time.Now()

	// 실행
	// sendSingleMessage 내부에서 ctx.Done()을 체크하므로 50ms 후 종료되어야 함
	n.sendSingleMessage(ctx, "Test Timeout Message")

	elapsed := time.Since(start)

	// 검증
	// 50ms (타임아웃) + 알파 내에 종료되어야 함.
	// 너무 빨리 끝나면(지연 없이 바로 리턴) Fail, 너무 오래 걸리면(타임아웃 무시) Fail.
	require.GreaterOrEqual(t, elapsed.Milliseconds(), int64(50), "타임아웃 시간보다 너무 빨리 종료되었습니다.")
	require.Less(t, elapsed.Milliseconds(), int64(150), "타임아웃이 발생했음에도 함수가 즉시 종료되지 않았습니다.")
}

// TestTelegramNotifier_RetryAfter_Compliance verifies handling of 429 Too Many Requests with Retry-After header.
func TestTelegramNotifier_RetryAfter_Compliance(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{}
	// Notifier 수동 생성
	n := &telegramNotifier{
		botAPI:     mockBot,
		chatID:     12345,
		retryDelay: 10 * time.Millisecond, // 기본 재시도 대기 시간 (짧게 설정)
		// RateLimiter는 nil로 두어 로직 간소화
	}

	// 1. 첫 번째 호출: 429 에러 + Retry-After 1초 반환
	retryAfterSeconds := 1
	apiErr := &tgbotapi.Error{
		Code:    429,
		Message: "Too Many Requests: retry after 1",
		ResponseParameters: tgbotapi.ResponseParameters{
			RetryAfter: retryAfterSeconds,
		},
	}

	// 2. 두 번째 호출: 성공
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, apiErr).Once()
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()

	// 시간 측정 시작
	start := time.Now()

	// 실행
	ctx := context.Background()
	n.sendSingleMessage(ctx, "Test Message")

	// 시간 측정 종료
	elapsed := time.Since(start)

	// 검증
	mockBot.AssertExpectations(t)
	require.GreaterOrEqual(t, elapsed.Seconds(), float64(retryAfterSeconds), "Retry-After 시간만큼 대기하지 않았습니다.")
}

// TestTelegramNotifier_400_FallbackRetry verifies fallback to plain text on 400 Bad Request (HTML error).
func TestTelegramNotifier_400_FallbackRetry(t *testing.T) {
	mockBot := &MockTelegramBot{}
	n := &telegramNotifier{
		botAPI:     mockBot,
		chatID:     12345,
		retryDelay: 100 * time.Millisecond,
	}

	// 400 에러
	apiErr := &tgbotapi.Error{
		Code:    400,
		Message: "Bad Request",
	}

	// 1. 첫 번째 호출 (HTML 모드) -> 400 에러
	mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
		return msg.ParseMode == tgbotapi.ModeHTML
	})).Return(tgbotapi.Message{}, apiErr).Once()

	// 2. 두 번째 호출 (Fallback: Plain Text 모드) -> 여전히 400 에러
	mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
		return msg.ParseMode == ""
	})).Return(tgbotapi.Message{}, apiErr).Once()

	start := time.Now()
	// Default uses HTML=true
	n.sendSingleMessage(context.Background(), "Test Message")
	elapsed := time.Since(start)

	mockBot.AssertExpectations(t)

	// 재시도 대기(100ms) 없이 즉시 리턴해야 함 (Fallback은 즉시 실행되므로)
	require.Less(t, elapsed.Milliseconds(), int64(50), "Fallback 로직은 대기 시간 없이 즉시 실행되어야 합니다.")
}
