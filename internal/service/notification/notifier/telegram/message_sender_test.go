package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/time/rate"
)

// =============================================================================
// Mock dependencies
// =============================================================================

// setupSenderTest creates a harness for testing logic without external dependencies.
func setupSenderTest(t *testing.T) (*telegramNotifier, *MockTelegramBot) {
	mockBot := NewMockTelegramBot(t)
	n := &telegramNotifier{
		Base:        notifier.NewBase("test-id", true, 100, 10*time.Second),
		client:      mockBot,
		chatID:      12345,
		retryDelay:  1 * time.Millisecond,         // Fast retry for testing
		rateLimiter: rate.NewLimiter(rate.Inf, 0), // Infinite rate limit by default
	}
	return n, mockBot
}

// =============================================================================
// Unit Tests: Helper Functions
// =============================================================================

func TestSafeSplit(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		limit         int
		expectedChunk string
		expectedRem   string
	}{
		// ASCII Cases
		{"ASCII Short", "Hello", 10, "Hello", ""},
		{"ASCII Exact", "Hello", 5, "Hello", ""},
		{"ASCII Long", "HelloWorld", 5, "Hello", "World"},

		// Multi-byte Cases (Korean: 3 bytes per char)
		{"Korean Short", "안녕하세요", 20, "안녕하세요", ""},
		{"Korean Exact", "가나다", 9, "가나다", ""},                                 // 3*3 = 9 bytes
		{"Korean Boundary Split", "가나다", 6, "가나", "다"},                        // 3*2 = 6 bytes
		{"Korean Mid-char Split", "가나다", 4, "가", "나다"},                        // 4 bytes -> cut at 3 ("가")
		{"Korean Complex Mid", "가나다", 5, "가", "나다"},                           // 5 bytes -> cut at 3 ("가")
		{"Korean Very Small Limit", "가나다", 2, "\xea\xb0", "\x80\ub098\ub2e4"}, // Limit 2 -> returns first 2 bytes of '가(EA B0 80)', remainder starts with 3rd byte(80) + '나다'
		{"Korean Force Split", "가", 2, "\xea\xb0", "\x80"},                    // Force split generates broken bytes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, rem := safeSplit(tt.input, tt.limit)
			assert.Equal(t, tt.expectedChunk, chunk, "Chunk mismatch")
			assert.Equal(t, tt.expectedRem, rem, "Remainder mismatch")

			if utf8.ValidString(tt.expectedChunk) && tt.limit >= 3 {
				assert.True(t, utf8.ValidString(chunk), "Chunk should be valid UTF8")
			}
		})
	}
}

func TestParseTelegramError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantCode       int
		wantRetryAfter int
	}{
		{
			name:     "Generic Error",
			err:      errors.New("network error"),
			wantCode: 0, wantRetryAfter: 0,
		},
		{
			name: "Telegram Error (Value)",
			err: tgbotapi.Error{
				Code: 429, Message: "Too Many Requests",
				ResponseParameters: tgbotapi.ResponseParameters{RetryAfter: 42},
			},
			wantCode: 429, wantRetryAfter: 42,
		},
		{
			name: "Telegram Error (Pointer)",
			err: &tgbotapi.Error{
				Code: 400, Message: "Bad Request",
			},
			wantCode: 400, wantRetryAfter: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, retryAfter := parseTelegramError(tt.err)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantRetryAfter, retryAfter)
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"400 Bad Request", 400, false},
		{"401 Unauthorized", 401, false},
		{"403 Forbidden", 403, false},
		{"404 Not Found", 404, false},
		{"429 Too Many Requests", 429, true}, // Special case!
		{"500 Internal Server Error", 500, true},
		{"502 Bad Gateway", 502, true},
		{"504 Gateway Timeout", 504, true},
		{"0 Network Error", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldRetry(tt.statusCode))
		})
	}
}

func TestDelayForRetry(t *testing.T) {
	n := &telegramNotifier{
		Base:       notifier.NewBase("test-id", true, 100, 10*time.Second),
		retryDelay: 5 * time.Second,
	}

	t.Run("Use Retry-After Header", func(t *testing.T) {
		assert.Equal(t, 10*time.Second, n.delayForRetry(10))
	})

	t.Run("Use Default Delay", func(t *testing.T) {
		assert.Equal(t, 5*time.Second, n.delayForRetry(0))
	})
}

func TestFormatParseMode(t *testing.T) {
	assert.Equal(t, "HTML", formatParseMode(tgbotapi.ModeHTML))
	assert.Equal(t, "PlainText", formatParseMode(""))
	assert.Equal(t, "PlainText", formatParseMode("Markdown"))
}

// =============================================================================
// Logic Tests: sendMessage (Splitting & Chunking)
// =============================================================================

func TestSendMessage_SplittingLogic(t *testing.T) {
	n, mockCli := setupSenderTest(t)

	// Capture all sent messages
	var sentMessages []string
	mockCli.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		if ok {
			sentMessages = append(sentMessages, msg.Text)
		}
		return ok
	})).Return(tgbotapi.Message{}, nil)

	const limit = 3900

	t.Run("Short Message", func(t *testing.T) {
		sentMessages = nil
		msg := "Short message"
		n.sendMessage(context.Background(), msg)
		assert.Len(t, sentMessages, 1)
		assert.Equal(t, msg, sentMessages[0])
	})

	t.Run("Exact Limit Message", func(t *testing.T) {
		sentMessages = nil
		msg := strings.Repeat("A", limit)
		n.sendMessage(context.Background(), msg)
		assert.Len(t, sentMessages, 1)
		assert.Equal(t, msg, sentMessages[0])
	})

	t.Run("Limit + 1 Message (Logical Split)", func(t *testing.T) {
		sentMessages = nil
		msg := strings.Repeat("A", limit) + "B"
		n.sendMessage(context.Background(), msg)

		assert.Len(t, sentMessages, 2)
		assert.Equal(t, strings.Repeat("A", limit), sentMessages[0])
		assert.Equal(t, "B", sentMessages[1])
	})

	t.Run("Line Based Splitting", func(t *testing.T) {
		sentMessages = nil
		line1 := strings.Repeat("A", 2000)
		line2 := strings.Repeat("B", 2000)
		msg := line1 + "\n" + line2
		n.sendMessage(context.Background(), msg)

		assert.Len(t, sentMessages, 2)
		assert.Equal(t, line1, sentMessages[0])
		assert.Equal(t, line2, sentMessages[1])
	})

	t.Run("Multiple Short Lines Accumulation", func(t *testing.T) {
		sentMessages = nil
		lines := make([]string, 10)
		for i := 0; i < 10; i++ {
			lines[i] = strings.Repeat("A", 100)
		}
		msg := strings.Join(lines, "\n")
		n.sendMessage(context.Background(), msg)

		assert.Len(t, sentMessages, 1)
		assert.Equal(t, msg, sentMessages[0])
	})

	t.Run("Context Cancel During Loop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		mockCli.ExpectedCalls = nil // Reset calls

		msg := strings.Repeat("A\n", 1000)

		// Cancel after first send
		mockCli.On("Send", mock.Anything).Run(func(args mock.Arguments) {
			cancel()
		}).Return(tgbotapi.Message{}, nil).Once()

		n.sendMessage(ctx, msg)
	})
}

// =============================================================================
// Logic Tests: attemptSendWithRetry (Retry, Fallback, Context)
// =============================================================================

func TestAttemptSendWithRetry(t *testing.T) {
	errNet := errors.New("network")
	err500 := &tgbotapi.Error{Code: 500, Message: "Internal Server Error"}
	err429 := &tgbotapi.Error{Code: 429, Message: "Too Many Requests", ResponseParameters: tgbotapi.ResponseParameters{RetryAfter: 0}}
	err400 := &tgbotapi.Error{Code: 400, Message: "Bad Request"}
	err401 := &tgbotapi.Error{Code: 401, Message: "Unauthorized"}

	type testCase struct {
		name        string
		msg         string
		useHTML     bool
		mockSetup   func(*MockTelegramBot)
		ctxSetup    func() (context.Context, context.CancelFunc)
		expectError bool
		targetErr   error
	}

	tests := []testCase{
		{
			name:    "Success Immediate",
			msg:     "test",
			useHTML: true,
			mockSetup: func(m *MockTelegramBot) {
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					return c.(tgbotapi.MessageConfig).ParseMode == tgbotapi.ModeHTML
				})).Return(tgbotapi.Message{}, nil).Once()
			},
			expectError: false,
		},
		{
			name:    "Retry Success (Fail -> Success)",
			msg:     "test",
			useHTML: true,
			mockSetup: func(m *MockTelegramBot) {
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, err500).Once()
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()
			},
			expectError: false,
		},
		{
			name:    "Max Retries Exceeded",
			msg:     "test",
			useHTML: true,
			mockSetup: func(m *MockTelegramBot) {
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, err500).Times(3)
			},
			expectError: true,
			targetErr:   err500,
		},
		{
			name:    "No Retry on 4xx Error",
			msg:     "test",
			useHTML: true,
			mockSetup: func(m *MockTelegramBot) {
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, err401).Once()
			},
			expectError: true,
			targetErr:   err401,
		},
		{
			name:    "Retry on 429",
			msg:     "test",
			useHTML: true,
			mockSetup: func(m *MockTelegramBot) {
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, err429).Once()
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()
			},
			expectError: false,
		},
		{
			name:    "HTML Fallback on 400",
			msg:     "<b>Broken",
			useHTML: true,
			mockSetup: func(m *MockTelegramBot) {
				// 1. HTML attempt fails with 400
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					return c.(tgbotapi.MessageConfig).ParseMode == tgbotapi.ModeHTML
				})).Return(tgbotapi.Message{}, err400).Once()

				// 2. PlainText attempt succeeds
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					return c.(tgbotapi.MessageConfig).ParseMode == ""
				})).Return(tgbotapi.Message{}, nil).Once()
			},
			expectError: false,
		},
		{
			name:    "Context Cancelled Before Start",
			msg:     "test",
			useHTML: true,
			ctxSetup: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			mockSetup: func(m *MockTelegramBot) {
				// Expect no calls
			},
			expectError: true,
			targetErr:   context.Canceled,
		},
		{
			name:    "Network Error Retry",
			msg:     "test",
			useHTML: false,
			mockSetup: func(m *MockTelegramBot) {
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, errNet).Twice() // Fail twice
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()     // Succeed
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, mockCli := setupSenderTest(t)

			// Setup Expectations
			if tt.mockSetup != nil {
				tt.mockSetup(mockCli)
			}

			// Setup Context
			var ctx context.Context
			var cancel context.CancelFunc
			if tt.ctxSetup != nil {
				ctx, cancel = tt.ctxSetup()
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
			}
			defer cancel()

			// Execute
			err := n.attemptSendWithRetry(ctx, tt.msg, tt.useHTML)

			// Verify
			if tt.expectError {
				assert.Error(t, err)
				if tt.targetErr != nil {
					assert.True(t, errors.Is(err, tt.targetErr) || strings.Contains(err.Error(), tt.targetErr.Error()))
				}
			} else {
				assert.NoError(t, err)
			}
			mockCli.AssertExpectations(t)
		})
	}
}

func TestRateLimiterIntegration(t *testing.T) {
	mockCli := NewMockTelegramBot(t)
	limiter := rate.NewLimiter(rate.Limit(1), 1)

	n := &telegramNotifier{
		Base:        notifier.NewBase("test-id", true, 100, 10*time.Second),
		client:      mockCli,
		chatID:      12345,
		rateLimiter: limiter,
		retryDelay:  1 * time.Millisecond,
	}

	mockCli.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil)

	ctx := context.Background()

	start := time.Now()
	_ = n.attemptSendWithRetry(ctx, "msg1", false)
	duration1 := time.Since(start)

	start = time.Now()
	_ = n.attemptSendWithRetry(ctx, "msg2", false)
	duration2 := time.Since(start)

	assert.Less(t, duration1, 100*time.Millisecond, "First call should be instant")
	assert.Greater(t, duration2, 800*time.Millisecond, "Second call should wait for rate limit")
}

func TestSendNotification_IntegrationFlow(t *testing.T) {
	n, mockCli := setupSenderTest(t)

	notification := &contract.Notification{
		Message: "Hello",
		Title:   "Test Title",
		TaskID:  "task-123",
	}

	// Expectation: client.Send is called with enriched message content (HTML mode, Title + Message)
	mockCli.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok &&
			msg.ParseMode == tgbotapi.ModeHTML &&
			strings.Contains(msg.Text, "Test Title") &&
			strings.Contains(msg.Text, "Hello")
	})).Return(tgbotapi.Message{}, nil).Once()

	n.sendNotification(context.Background(), notification)
	mockCli.AssertExpectations(t)
}
