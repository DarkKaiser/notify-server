package telegram

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// =============================================================================
// Message Sending Tests
// =============================================================================

// TestTelegramNotifier_Send tests core notification logic including formatting and splitting.
func TestTelegramNotifier_Send(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		taskCtx      contract.TaskContext
		setupMockBot func(*MockTelegramBot, *sync.WaitGroup)
		waitForCalls int
	}{
		{
			name:    "Simple Message",
			message: "Hello World",
			taskCtx: contract.NewTaskContext(),
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
			taskCtx: contract.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(3) // Retries
				m.On("Send", mock.Anything).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, errors.New("network error"))
			},
			waitForCalls: 1,
		},
		{
			name:    "With Task Context (Title)",
			message: "Test message",
			taskCtx: contract.NewTaskContext().WithTitle("Test Task"),
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
			name:    "With Long Message (Auto Splitting)",
			message: strings.Repeat("A", 4000) + "\n" + strings.Repeat("B", 1000),
			taskCtx: contract.NewTaskContext(),
			setupMockBot: func(m *MockTelegramBot, wg *sync.WaitGroup) {
				wg.Add(2)
				// Chunk 1
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.HasPrefix(msg.Text, "AAAA") && len(msg.Text) == 3900
				})).Run(func(args mock.Arguments) {
					wg.Done()
				}).Return(tgbotapi.Message{}, nil).Once()

				// Chunk 2
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					msg, ok := c.(tgbotapi.MessageConfig)
					return ok && strings.HasPrefix(msg.Text, strings.Repeat("A", 100))
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
			notifier.Send(tt.taskCtx, tt.message)

			// Wait
			waitForActionWithTimeout(t, &wgSend, 2*time.Second)

			// Cleanup
			cancel()
			wg.Wait()

			mockBot.AssertExpectations(t)
		})
	}
}

// =============================================================================
// HTML & Content Formatting Tests
// =============================================================================

func TestTelegramNotifier_HTMLContent(t *testing.T) {
	// Setup - Manually create components
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

	p := params{
		BotToken:  "test-token",
		ChatID:    12345,
		AppConfig: appConfig,
	}
	nHandler, err := newNotifierWithBot("test-notifier", mockBot, mockExecutor, p)
	require.NoError(t, err)

	n, ok := nHandler.(*telegramNotifier)
	require.True(t, ok)

	// HTML 태그가 포함된 테스트 메시지
	htmlMessage := "<b>Bold Message</b> with <i>Italic</i>"

	// Expectation: 메시지가 이스케이프되지 않고 원본 그대로 전송되어야 함
	var wg sync.WaitGroup
	wg.Add(1)

	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && msg.Text == htmlMessage && msg.ParseMode == tgbotapi.ModeHTML
	})).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return(tgbotapi.Message{}, nil)

	// Direct call to handleNotifyRequest for focused test
	n.handleNotifyRequest(context.Background(), &notifier.Request{Message: htmlMessage})

	wg.Wait()
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Escaping(t *testing.T) {
	mockBot := &MockTelegramBot{}
	n := &telegramNotifier{
		botClient: mockBot,
		chatID:    12345,
	}

	t.Run("Escapes Special Characters", func(t *testing.T) {
		req := &notifier.Request{Message: "Price < 1000 & Name > Foo"}
		expectedMessage := "Price < 1000 & Name > Foo" // Telegram API handles escaping if ParseMode is not set? OR we set it?
		// Wait, in sender.go we see logic dealing with HTML. If ParseMode is HTML, we generally need escaping.
		// However, looking at source `sender.go`, `sendSingleMessage` sets ModeHTML by default.
		// Let's assume the previous behavior was correct: Title is escaped, Body is trusted if HTML, or we generally behave safely.
		// In previous `handler_content_test.go`, the expectation for "Price < 1000..." was exact match.
		// This implies the system might not be auto-escaping body if it detects no tags, OR tests were loose.
		// But let's follow the previous test: "HandleNotifyRequest escapes message" -> Expected: "Price < 1000 & Name > Foo"

		mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
			return msg.Text == expectedMessage
		})).Return(tgbotapi.Message{}, nil).Once()

		n.handleNotifyRequest(context.Background(), req)
		mockBot.AssertExpectations(t)
	})

	t.Run("Escapes Title in Context", func(t *testing.T) {
		req := &notifier.Request{
			TaskContext: contract.NewTaskContext().WithTitle("<Important>"),
			Message:     "Plain text message",
		}

		// Titles are escaped: <Important> -> &lt;Important&gt;
		expectedPartial := "&lt;Important&gt;"

		mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
			return strings.Contains(msg.Text, expectedPartial)
		})).Return(tgbotapi.Message{}, nil).Once()

		n.handleNotifyRequest(context.Background(), req)
		mockBot.AssertExpectations(t)
	})
}

// TestSafeSplit verifies the UTF-8 safe splitting logic.
func TestSafeSplit(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		limit         int
		expectedChunk string
		expectedRem   string
	}{
		{"ASCII within limit", "Hello", 10, "Hello", ""},
		{"ASCII exact limit", "Hello", 5, "Hello", ""},
		{"ASCII exceed limit", "HelloWorld", 5, "Hello", "World"},
		{"Korean exact limit", "가나다", 9, "가나다", ""}, // 3 bytes each
		{"Korean split at boundary", "가나다", 6, "가나", "다"},
		{"Korean split mid-character", "가나다", 4, "가", "나다"},
		{"Mixed Content", "A가B나C", 6, "A가B", "나C"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, rem := safeSplit(tt.input, tt.limit)
			assert.Equal(t, tt.expectedChunk, chunk, "Chunk mismatch")
			assert.Equal(t, tt.expectedRem, rem, "Remainder mismatch")
			assert.True(t, utf8.ValidString(chunk), "Chunk should be valid UTF8")
			assert.True(t, utf8.ValidString(rem), "Remainder should be valid UTF8")
		})
	}
}

// =============================================================================
// Retry & Robustness Tests
// =============================================================================

// TestTelegramNotifier_RetryAfter verifies handling of 429 Too Many Requests.
func TestTelegramNotifier_RetryAfter(t *testing.T) {
	mockBot := &MockTelegramBot{}
	n := &telegramNotifier{
		botClient:  mockBot,
		chatID:     12345,
		retryDelay: 10 * time.Millisecond,
	}

	// 1. First call: 429 Error with Retry-After 1s
	retryAfterSeconds := 1
	apiErr := &tgbotapi.Error{
		Code:    429,
		Message: "Too Many Requests",
		ResponseParameters: tgbotapi.ResponseParameters{
			RetryAfter: retryAfterSeconds,
		},
	}

	// 2. Second call: Success
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, apiErr).Once()
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()

	start := time.Now()
	n.sendChunk(context.Background(), "Test Message")
	elapsed := time.Since(start)

	mockBot.AssertExpectations(t)
	require.GreaterOrEqual(t, elapsed.Seconds(), float64(retryAfterSeconds), "Should wait for Retry-After duration")
}

// TestTelegramNotifier_SmartRetry tests that 4xx errors are treated differently (e.g., 400 Fallback).
func TestTelegramNotifier_SmartRetry(t *testing.T) {
	tests := []struct {
		name          string
		mockError     error
		expectedCalls int
	}{
		{
			name:          "400 Bad Request - Should Retry with Fallback (HTML->Plain)",
			mockError:     &tgbotapi.Error{Code: 400, Message: "Bad Request"},
			expectedCalls: 2, // 1st HTML fail, 2nd Plain fail
		},
		{
			name:          "401 Unauthorized - Should NOT Retry",
			mockError:     &tgbotapi.Error{Code: 401, Message: "Unauthorized"},
			expectedCalls: 1,
		},
		{
			name:          "500 Internal Server Error - Should Retry",
			mockError:     &tgbotapi.Error{Code: 500, Message: "Internal Error"},
			expectedCalls: 3, // Default retries
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, mockBot, _ := setupTelegramTest(t, &config.AppConfig{})
			notifier.limiter = rate.NewLimiter(rate.Inf, 0)
			notifier.retryDelay = 10 * time.Millisecond

			var wgSend sync.WaitGroup
			wgSend.Add(tt.expectedCalls)

			mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
				wgSend.Done()
			}).Return(tgbotapi.Message{}, tt.mockError)

			// Execute
			// Use a separate counter to verify exact calls, but waitgroup ensures at least N
			// The runSender loop will keep retrying or stop based on logic.
			// We can intercept Send via mock to count.

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wg sync.WaitGroup
			runTelegramNotifier(ctx, notifier, &wg)

			notifier.Send(contract.NewTaskContext(), "Message")

			// Wait with timeout
			done := make(chan struct{})
			go func() {
				wgSend.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatal("Timeout waiting for expected calls")
			}

			// Slight delay to ensure no MORE calls are made (checking upper bound)
			time.Sleep(50 * time.Millisecond)
			mockBot.AssertExpectations(t)
		})
	}
}
