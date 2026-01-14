package telegram

import (
	"context"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandleNotifyRequest_HTMLSupport(t *testing.T) {
	// Setup - Manually create components to avoid unused mock expectations from setupTelegramTest
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &taskmocks.MockExecutor{} // handler에서 사용하지 않더라도 초기화 필요

	// 직접 초기화 (기본 Mock Expectation 없음)
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

	// Wait
	wg.Wait()

	mockBot.AssertExpectations(t)
}
