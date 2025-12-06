package notification

import "github.com/darkkaiser/notify-server/config"

// NotifierFactory 알림 핸들러 생성을 담당하는 팩토리 인터페이스입니다.
type NotifierFactory interface {
	CreateNotifiers(cfg *config.AppConfig) ([]NotifierHandler, error)
}

// telegramNotifierCreatorFunc Telegram Notifier 생성 함수 타입
type telegramNotifierCreatorFunc func(id NotifierID, botToken string, chatID int64, appConfig *config.AppConfig) (NotifierHandler, error)

// DefaultNotifierFactory 기본 NotifierFactory 구현체입니다.
type DefaultNotifierFactory struct {
	createTelegramNotifier telegramNotifierCreatorFunc
}

// NewNotifierFactory 새로운 DefaultNotifierFactory를 생성합니다.
func NewNotifierFactory() NotifierFactory {
	return &DefaultNotifierFactory{
		createTelegramNotifier: newTelegramNotifier,
	}
}

// CreateNotifiers 설정을 기반으로 활성화된 모든 Notifier를 생성하여 반환합니다.
func (f *DefaultNotifierFactory) CreateNotifiers(appConfig *config.AppConfig) ([]NotifierHandler, error) {
	var handlers []NotifierHandler

	// Telegram Notifier 생성
	for _, telegram := range appConfig.Notifiers.Telegrams {
		h, err := f.createTelegramNotifier(NotifierID(telegram.ID), telegram.BotToken, telegram.ChatID, appConfig)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, h)
	}

	// 추후 다른 Notifier (예: Slack, Discord) 추가 시 여기에 생성 로직 추가

	return handlers, nil
}
