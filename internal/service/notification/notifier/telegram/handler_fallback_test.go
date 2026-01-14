package telegram

import (
	"context"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/notification/types"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTelegramNotifier_FallbackToPlainText(t *testing.T) {
	// 1. Setup
	mockBot := new(MockTelegramBot)
	notifierID := types.NotifierID("telegram-test")
	chatID := int64(123456789)

	// Create notifier with mock bot and empty config
	appConfig := &config.AppConfig{}

	// Create notifier instance using the internal constructor
	// This helps initializing internal fields like botCommands map properly
	nHandler, err := newTelegramNotifierWithBot(notifierID, mockBot, chatID, appConfig, nil)
	assert.NoError(t, err)

	notifier, ok := nHandler.(*telegramNotifier)
	assert.True(t, ok)

	// Disable rate limiter and retry delay for fast testing
	notifier.limiter = nil
	notifier.retryDelay = 0

	ctx := context.Background()
	// An intentionally broken HTML message
	message := "<b>Broken HTML Message"

	// 2. Define Expectations

	// First call: Expect HTML mode, return 400 Bad Request
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		// ParseMode should be HTML
		return ok && msg.ParseMode == tgbotapi.ModeHTML && msg.Text == message
	})).Return(tgbotapi.Message{}, &tgbotapi.Error{
		Code:    400,
		Message: "Bad Request: can't parse entities",
	}).Once()

	// Second call: Expect Plain Text mode (empty ParseMode), return success
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		// ParseMode should be empty (Plain Text) due to fallback
		return ok && msg.ParseMode == "" && msg.Text == message
	})).Return(tgbotapi.Message{}, nil).Once()

	// 3. Execute
	// We call sendSingleMessage directly.
	// Note: sendSingleMessage is an internal method (lowercase) but we are in the same package 'telegram'.
	notifier.sendSingleMessage(ctx, message)

	// 4. Verify
	mockBot.AssertExpectations(t)
}
