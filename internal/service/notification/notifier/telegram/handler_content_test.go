package telegram

import (
	"context"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Mocks
// =============================================================================
// We use the existing MockTelegramBot from mock_test.go if available,
// or define a minimal one if needed for specific tests here.
// Assuming MockTelegramBot is available in the package.

// =============================================================================
// HTML Validation Tests
// =============================================================================

func TestHandleNotifyRequest_HTMLSupport(t *testing.T) {
	// Setup - Manually create components
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{}

	nHandler, err := newTelegramNotifierWithBot("test-notifier", mockBot, 12345, appConfig, mockExecutor)
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
		if !ok {
			return false
		}
		// 내용 일치 및 ParseMode가 HTML인지 확인
		return msg.Text == htmlMessage && msg.ParseMode == tgbotapi.ModeHTML
	})).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return(tgbotapi.Message{}, nil)

	// Run Notifier
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Direct call to handleNotifyRequest
	n.handleNotifyRequest(ctx, &notifier.NotifyRequest{Message: htmlMessage})

	wg.Wait()
	mockBot.AssertExpectations(t)
}

// =============================================================================
// Escaping Tests
// =============================================================================

func TestTelegramNotifier_Escaping(t *testing.T) {
	t.Run("HandleNotifyRequest escapes message", func(t *testing.T) {
		mockBot := &MockTelegramBot{}
		n := &telegramNotifier{
			botAPI: mockBot,
			chatID: 12345,
		}

		req := &notifier.NotifyRequest{
			Message: "Price < 1000 & Name > Foo",
		}

		expectedMessage := "Price < 1000 & Name > Foo"

		mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
			return msg.Text == expectedMessage
		})).Return(tgbotapi.Message{}, nil).Once()

		n.handleNotifyRequest(context.Background(), req)

		mockBot.AssertExpectations(t)
	})

	t.Run("HandleNotifyRequest escapes title in context", func(t *testing.T) {
		mockBot := &MockTelegramBot{}
		n := &telegramNotifier{
			botAPI: mockBot,
			chatID: 12345,
		}

		req := &notifier.NotifyRequest{
			Message: "Body",
			TaskCtx: task.NewTaskContext().WithTitle("<Important>"),
		}

		// Expected: <b>【 &lt;Important&gt; 】</b>\n\nBody
		expectedPartial := "<b>【 &lt;Important&gt; 】</b>"

		mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
			return assert.Contains(t, msg.Text, expectedPartial)
		})).Return(tgbotapi.Message{}, nil).Once()

		n.handleNotifyRequest(context.Background(), req)

		mockBot.AssertExpectations(t)
	})
}

// =============================================================================
// SafeSplit Tests
// =============================================================================

func TestSafeSplit(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		limit         int
		expectedChunk string
		expectedRem   string
	}{
		{
			name:          "ASCII within limit",
			input:         "Hello",
			limit:         10,
			expectedChunk: "Hello",
			expectedRem:   "",
		},
		{
			name:          "ASCII exact limit",
			input:         "Hello",
			limit:         5,
			expectedChunk: "Hello",
			expectedRem:   "",
		},
		{
			name:          "ASCII exceed limit",
			input:         "HelloWorld",
			limit:         5,
			expectedChunk: "Hello",
			expectedRem:   "World",
		},
		{
			name:          "Korean exact limit (Each hangul is 3 bytes)",
			input:         "가나다", // 9 bytes
			limit:         9,
			expectedChunk: "가나다",
			expectedRem:   "",
		},
		{
			name:          "Korean within limit",
			input:         "가나다",
			limit:         10,
			expectedChunk: "가나다",
			expectedRem:   "",
		},
		{
			name:          "Korean split at boundary",
			input:         "가나다",
			limit:         6,
			expectedChunk: "가나",
			expectedRem:   "다",
		},
		{
			name:          "Korean split mid-character (1 byte)",
			input:         "가나다",
			limit:         4, // '가'(3) + 1 byte of '나' -> Should cut at '가'
			expectedChunk: "가",
			expectedRem:   "나다",
		},
		{
			name:          "Korean split mid-character (2 bytes)",
			input:         "가나다",
			limit:         5, // '가'(3) + 2 bytes of '나' -> Should cut at '가'
			expectedChunk: "가",
			expectedRem:   "나다",
		},
		{
			name:          "Mixed Content",
			input:         "A가B나C", // 1 + 3 + 1 + 3 + 1 = 9 bytes
			limit:         6,       // A(1) + 가(3) + B(1) + 1 byte of 나 -> Should cut at B
			expectedChunk: "A가B",
			expectedRem:   "나C",
		},
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
// Content Regression Tests
// =============================================================================

func TestTelegramNotifier_Regressions_Content(t *testing.T) {
	// Regression Test for: "HTML 태그 깨짐 방지"
	// Ensure that truncating a title does not break HTML entities (like &lt;)
	t.Run("Fix: appendTitle safely truncates and escapes", func(t *testing.T) {
		mockBot := &MockTelegramBot{}
		n := &telegramNotifier{
			botAPI: mockBot,
			chatID: 12345,
		}

		// Create a title that is longer than titleTruncateLength (200)
		// And ensure it has special characters at the truncation boundary.
		longTitle := strings.Repeat("A", 195) + "<Important>"

		req := &notifier.NotifyRequest{
			Message: "Body",
			TaskCtx: task.NewTaskContext().WithTitle(longTitle),
		}

		mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
			if strings.Contains(msg.Text, "<Impo") { // raw tag, bad
				return false
			}
			if !strings.Contains(msg.Text, "&lt;Impo") { // expected escaped
				return false
			}
			return true
		})).Return(tgbotapi.Message{}, nil).Once()

		n.handleNotifyRequest(context.Background(), req)

		mockBot.AssertExpectations(t)
	})
}
