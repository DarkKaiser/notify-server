package notification

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/stretchr/testify/assert"
)

func TestDefaultNotifierFactory_CreateNotifiers(t *testing.T) {
	// 기존 생성자 함수 백업 및 복원
	originalCreator := telegramNotifierCreator
	defer func() {
		telegramNotifierCreator = originalCreator
	}()

	t.Run("Telegram Notifier 생성 확인", func(t *testing.T) {
		// 설정 준비
		cfg := &config.AppConfig{}
		cfg.Notifiers.Telegrams = []config.TelegramConfig{
			{ID: "telegram-1", BotToken: "token-1", ChatID: 111},
			{ID: "telegram-2", BotToken: "token-2", ChatID: 222},
		}

		// Mock Creator 설정
		createdCount := 0
		telegramNotifierCreator = func(id NotifierID, botToken string, chatID int64, appConfig *config.AppConfig) NotifierHandler {
			createdCount++
			return &mockNotifierHandler{id: id}
		}

		// Factory 실행
		factory := NewNotifierFactory()
		handlers := factory.CreateNotifiers(cfg)

		// 검증
		assert.Equal(t, 2, len(handlers), "2개의 핸들러가 생성되어야 합니다")
		assert.Equal(t, 2, createdCount, "Creator가 2번 호출되어야 합니다")
		assert.Equal(t, NotifierID("telegram-1"), handlers[0].ID())
		assert.Equal(t, NotifierID("telegram-2"), handlers[1].ID())
	})

	t.Run("설정이 비어있는 경우", func(t *testing.T) {
		cfg := &config.AppConfig{}
		cfg.Notifiers.Telegrams = []config.TelegramConfig{}

		createdCount := 0
		telegramNotifierCreator = func(id NotifierID, botToken string, chatID int64, appConfig *config.AppConfig) NotifierHandler {
			createdCount++
			return &mockNotifierHandler{id: id}
		}

		factory := NewNotifierFactory()
		handlers := factory.CreateNotifiers(cfg)

		assert.Empty(t, handlers, "핸들러가 생성되지 않아야 합니다")
		assert.Equal(t, 0, createdCount, "Creator가 호출되지 않아야 합니다")
	})
}
