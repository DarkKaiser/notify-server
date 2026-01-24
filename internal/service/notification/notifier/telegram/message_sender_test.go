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
// Mocks & Setup
// =============================================================================

// setupSenderTest creates a harness for testing logic without external dependencies.
func setupSenderTest(t *testing.T) (*telegramNotifier, *MockTelegramBot) {
	mockBot := NewMockTelegramBot(t)
	n := &telegramNotifier{
		Base:        notifier.NewBase("test-id", true, 100, 10*time.Second),
		client:      mockBot,
		chatID:      12345,
		retryDelay:  1 * time.Millisecond,         // Fast retry for test performance
		rateLimiter: rate.NewLimiter(rate.Inf, 0), // No limit by default
	}
	return n, mockBot
}

// =============================================================================
// Unit Tests: Helper Functions
// =============================================================================

func TestSafeSplit_Korean(t *testing.T) {
	// Korean Validation: 3 bytes per char
	// "가나다" = 9 bytes
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		limit         int
		expectedChunk string
		expectedRem   string
	}{
		{"Exact Fit", "가나다", 9, "가나다", ""},
		{"Boundary Split 2chars", "가나다", 6, "가나", "다"},
		{"Cut Mid-Char (4 bytes)", "가나다", 4, "가", "나다"},               // 4 bytes -> only 1st char(3 bytes) fits
		{"Cut Mid-Char (5 bytes)", "가나다", 5, "가", "나다"},               // 5 bytes -> only 1st char(3 bytes) fits
		{"Too Small Limit", "가나다", 2, "\xea\xb0", "\x80\ub098\ub2e4"}, // Force split (2 bytes) - Broken char
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, rem := safeSplit(tt.input, tt.limit)
			assert.Equal(t, tt.expectedChunk, chunk)
			assert.Equal(t, tt.expectedRem, rem)

			// If verify valid utf8 when limit is sufficient
			if tt.limit >= 3 && utf8.ValidString(tt.expectedChunk) {
				assert.True(t, utf8.ValidString(chunk), "Chunk should be valid UTF8")
			}
		})
	}
}

func TestParseTelegramError(t *testing.T) {
	t.Parallel()

	// 1. Value Type Error
	errVal := tgbotapi.Error{
		Code: 429, Message: "Too Many Requests",
		ResponseParameters: tgbotapi.ResponseParameters{RetryAfter: 42},
	}
	code, retry := parseTelegramError(errVal)
	assert.Equal(t, 429, code)
	assert.Equal(t, 42, retry) // tgbotapi maps RetryAfter to int

	// 2. Pointer Type Error
	errPtr := &tgbotapi.Error{Code: 400}
	code, retry = parseTelegramError(errPtr)
	assert.Equal(t, 400, code)
	assert.Equal(t, 0, retry)

	// 3. Generic Error
	code, retry = parseTelegramError(errors.New("generic"))
	assert.Equal(t, 0, code)
	assert.Equal(t, 0, retry)
}

func TestShouldRetry(t *testing.T) {
	t.Parallel()

	// Retryable
	assert.True(t, shouldRetry(429), "429 should be retryable")
	assert.True(t, shouldRetry(500), "500 should be retryable")
	assert.True(t, shouldRetry(502), "502 should be retryable")
	assert.True(t, shouldRetry(0), "Network error (0) should be retryable")

	// Non-Retryable
	assert.False(t, shouldRetry(400), "400 should not be retryable")
	assert.False(t, shouldRetry(401), "401 should not be retryable")
	assert.False(t, shouldRetry(404), "404 should not be retryable")
}

func TestDelayForRetry(t *testing.T) {
	n := &telegramNotifier{
		retryDelay: 5 * time.Second,
	}

	// Case 1: Server provided Retry-After
	assert.Equal(t, 10*time.Second, n.delayForRetry(10))

	// Case 2: Default delay
	assert.Equal(t, 5*time.Second, n.delayForRetry(0))
}

// =============================================================================
// Logic Tests: sendMessage (Splitting & Context)
// =============================================================================

func TestSendMessage_SplittingAndContext(t *testing.T) {
	n, mockCli := setupSenderTest(t)
	const limit = 3900

	t.Run("Chunking Logic", func(t *testing.T) {
		// Mock: Capture sent messages
		var capturedMsgs []string
		mockCli.ExpectedCalls = nil
		mockCli.On("Send", mock.Anything).Run(func(args mock.Arguments) {
			cfg := args.Get(0).(tgbotapi.MessageConfig)
			capturedMsgs = append(capturedMsgs, cfg.Text)
		}).Return(tgbotapi.Message{}, nil)

		// Input: A really long line that forces hard split + normal lines
		longLine := strings.Repeat("A", limit+100) // 4000 chars -> Split into 3900 + 100
		normalLine := "Normal Line"
		fullMsg := longLine + "\n" + normalLine

		// Execute
		n.sendMessage(context.Background(), fullMsg)

		// Verify
		// Chunk 1: 3900 'A's
		// Chunk 2: 100 'A's + \n + "Normal Line" (Total 112 chars)
		assert.Len(t, capturedMsgs, 2)
		assert.Equal(t, strings.Repeat("A", limit), capturedMsgs[0])
		assert.Equal(t, strings.Repeat("A", 100)+"\n"+normalLine, capturedMsgs[1])
	})

	t.Run("Context Cancellation During Chunking", func(t *testing.T) {
		mockCli.ExpectedCalls = nil
		// Fail on second call? Or just cancel context
		// We can test if it stops sending after context cancel.

		ctx, cancel := context.WithCancel(context.Background())

		// Send 1st chunk successfully, then cancel
		mockCli.On("Send", mock.Anything).Run(func(args mock.Arguments) {
			cancel() // Cancel context after first send
		}).Return(tgbotapi.Message{}, nil).Once()

		// If logic is wrong, it might try to send 2nd chunk
		// We expect NO more calls after cancellation
		// But testify doesn't have "MaxCalls" easily, so we rely on strict expectation violation or count.

		chunks := strings.Repeat("A\n", 4000) // Many short lines, will result in multiple chunks
		n.sendMessage(ctx, chunks)

		// Verify that it stopped. If it continued, it would likely try to send more or panic if we didn't mock enough returns.
		// Detailed verification: check calling count manually if needed, but .Once() implies if called twice it errors?
		// No, testify mock objects by default return default values if not matched, OR error if strict.
		// Let's rely on Mock's AssertExpectations.
		mockCli.AssertExpectations(t)
	})
}

// =============================================================================
// Logic Tests: attemptSendWithRetry (Core Logic)
// =============================================================================

func TestAttemptSendWithRetry_Scenarios(t *testing.T) {
	errNet := errors.New("net err")
	err500 := &tgbotapi.Error{Code: 500, Message: "Server Error"}
	err400 := &tgbotapi.Error{Code: 400, Message: "Bad Request"}
	err429 := &tgbotapi.Error{Code: 429, Message: "Rate Limit", ResponseParameters: tgbotapi.ResponseParameters{RetryAfter: 0}}

	type testCase struct {
		name        string
		msg         string
		useHTML     bool
		mockSetup   func(*MockTelegramBot)
		expectedErr error
		checkLog    bool // Optional: check if logs contain specific entries? (Not implemented here generally)
	}

	tests := []testCase{
		{
			name:        "Success on First Try",
			msg:         "hello",
			useHTML:     true,
			expectedErr: nil,
			mockSetup: func(m *MockTelegramBot) {
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					return c.(tgbotapi.MessageConfig).ParseMode == tgbotapi.ModeHTML
				})).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name:        "Retry Success (Fail -> Success)",
			msg:         "retry",
			useHTML:     false,
			expectedErr: nil,
			mockSetup: func(m *MockTelegramBot) {
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, err500).Once()
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name:        "Max Retries Exhausted",
			msg:         "fail",
			useHTML:     false,
			expectedErr: errNet, // Should return the last error
			mockSetup: func(m *MockTelegramBot) {
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, errNet).Times(3)
			},
		},
		{
			name:        "Fatal Error (4xx) - No Retry",
			msg:         "fatal",
			useHTML:     false,
			expectedErr: err400,
			mockSetup: func(m *MockTelegramBot) {
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, err400).Once()
			},
		},
		{
			name:        "HTML Fallback on 400",
			msg:         "<b>BadHTML",
			useHTML:     true,
			expectedErr: nil, // Success after fallback
			mockSetup: func(m *MockTelegramBot) {
				// 1. HTML Attempt -> 400 Error
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					return c.(tgbotapi.MessageConfig).ParseMode == tgbotapi.ModeHTML
				})).Return(tgbotapi.Message{}, err400).Once()

				// 2. Fallback PlainText Attempt -> Success
				m.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
					return c.(tgbotapi.MessageConfig).ParseMode == ""
				})).Return(tgbotapi.Message{}, nil).Once()
			},
		},
		{
			name:        "Retry on 429 Rate Limit",
			msg:         "fast",
			useHTML:     false,
			expectedErr: nil,
			mockSetup: func(m *MockTelegramBot) {
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, err429).Once()
				m.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, mockCli := setupSenderTest(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mockCli)
			}

			err := n.attemptSendWithRetry(context.Background(), tt.msg, tt.useHTML)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				// Basic error check implies strict equality for mocked errors
				assert.Equal(t, tt.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}
			mockCli.AssertExpectations(t)
		})
	}
}

// TestRateLimit_Retry_Timing verifies that the Rate Limiter is applied *during retries*.
// This is the regression test for the bug where retries ignored the rate limiter.
func TestRateLimit_Retry_Timing(t *testing.T) {
	mockCli := NewMockTelegramBot(t)
	// Rate Limit: 1 event per 500ms
	// Burst: 1
	limitInterval := 200 * time.Millisecond
	limiter := rate.NewLimiter(rate.Every(limitInterval), 1)

	n := &telegramNotifier{
		Base:        notifier.NewBase("test-id", true, 100, 10*time.Second),
		client:      mockCli,
		chatID:      12345,
		rateLimiter: limiter,
		retryDelay:  1 * time.Millisecond, // Instant retry delay (we only want to measure Rate Limit delay)
	}

	// Scenario:
	// 1. First Send (consumes token) -> Fails
	// 2. Retry Send (bucket empty, should wait ~200ms) -> Success
	mockCli.On("Send", mock.Anything).Return(tgbotapi.Message{}, errors.New("fail")).Once()
	mockCli.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()

	// Drain the initial token so the first send also might take time?
	// Actually, Allow() consumes 1.
	// Let's ensure bucket is full at start.
	// Wait logic:
	// - Attempt 1: Wait() -> Returns immediately (consumes 1) -> Client.Send fails
	// - Attempt 2: Wait() -> Bucket empty -> Sleeps 200ms -> Returns -> Client.Send succeeds

	ctx := context.Background()
	start := time.Now()

	err := n.attemptSendWithRetry(ctx, "msg", false)
	assert.NoError(t, err)

	elapsed := time.Since(start)

	// Verification:
	// If bug exists (no Wait on retry), elapsed would be very fast (close to 0 + retryDelay).
	// If fix works, elapsed should be > limitInterval (200ms).
	assert.Greater(t, elapsed, limitInterval, "Execution time should reflect rate limiter wait on retry")

	mockCli.AssertExpectations(t)
}

func TestContextCancellation_During_RetryWait(t *testing.T) {
	// Verifies that if context is canceled while waiting for retry delay, it exits immediately.
	n, mockCli := setupSenderTest(t)
	// Long retry delay
	n.retryDelay = 5 * time.Second

	mockCli.On("Send", mock.Anything).Return(tgbotapi.Message{}, errors.New("fail")).Once()

	ctx, cancel := context.WithCancel(context.Background())

	// Start a goroutine to cancel context after small delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := n.attemptSendWithRetry(ctx, "msg", false)
	elapsed := time.Since(start)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, elapsed, 2*time.Second, "Should exit immediately after cancellation, not wait full retry delay")
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	// Verifies context cancellation processing within RateLimiter.Wait
	mockCli := NewMockTelegramBot(t)
	// Very slow limiter: 1 request every hour
	limiter := rate.NewLimiter(rate.Every(1*time.Hour), 1)

	n := &telegramNotifier{
		client:      mockCli,
		rateLimiter: limiter,
	}

	// Consume the only token
	limiter.Allow()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Attempt to send -> Wait() blocked -> Context Timeout
	err := n.attemptSendWithRetry(ctx, "msg", false)

	assert.Error(t, err)
	// rate.Wait can return its own error if the wait exceeds context deadline immediately
	// or context.DeadlineExceeded if it waits and then times out.
	// Since we set a short timeout and long limit interval, it likely returns "would exceed context deadline".
	isDeadline := errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "exceed context deadline")
	assert.True(t, isDeadline, "Error should be DeadlineExceeded or rate limit exceeded message, got: %v", err)
}

func TestHTMLFallback_Recursion_MaxRetries(t *testing.T) {
	// Verifies that when falling back from HTML -> PlainText, the retry count logic is safe.
	// Current implementation: Fallback calls attemptSendWithRetry recursively.
	// This "could" reset retry count effectively doubling it (3 HTML retries -> 3 Plain retries).
	// This test observes behavior. If desired behavior is shared budget, implementation might need check.
	// But based on current code, recursive call starts a NEW retry loop.
	// Let's verify this behavior is "as implemented" (recurse = fresh retries for plaintext).

	n, mockCli := setupSenderTest(t)
	err400 := &tgbotapi.Error{Code: 400, Message: "Bad HTML"}
	err500 := &tgbotapi.Error{Code: 500, Message: "Server Error"}

	// Scenario:
	// 1. HTML Send -> 400 (Bad Request)
	// 2. Recursive Call (PlainText)
	//    - Attempt 1: 500
	//    - Attempt 2: 500
	//    - Attempt 3: 500 -> Fail

	// HTML Failure
	mockCli.On("Send", mock.MatchedBy(func(c tgbotapi.MessageConfig) bool {
		return c.ParseMode == tgbotapi.ModeHTML
	})).Return(tgbotapi.Message{}, err400).Once()

	// PlainText Retries
	mockCli.On("Send", mock.MatchedBy(func(c tgbotapi.MessageConfig) bool {
		return c.ParseMode == ""
	})).Return(tgbotapi.Message{}, err500).Times(3)

	err := n.attemptSendWithRetry(context.Background(), "msg", true)

	assert.Error(t, err)
	assert.Equal(t, err500, err) // Should return the error from PlainText attempts
	mockCli.AssertExpectations(t)
}

func TestSendNotification_Enrichment(t *testing.T) {
	n, mockCli := setupSenderTest(t)

	noti := &contract.Notification{
		Message: "Body",
		Title:   "Title",
		TaskID:  "Task-1",
	}

	// Verify that the sent message contains all parts
	mockCli.On("Send", mock.MatchedBy(func(c tgbotapi.MessageConfig) bool {
		text := c.Text
		return strings.Contains(text, "Body") &&
			strings.Contains(text, "Title") &&
			c.ParseMode == tgbotapi.ModeHTML
	})).Return(tgbotapi.Message{}, nil).Once()

	n.sendNotification(context.Background(), noti)
	mockCli.AssertExpectations(t)
}
