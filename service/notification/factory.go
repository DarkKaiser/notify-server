package notification

import "github.com/darkkaiser/notify-server/config"

// NotifierFactory 알림 핸들러 생성을 담당하는 팩토리 인터페이스입니다.
type NotifierFactory interface {
	CreateNotifiers(cfg *config.AppConfig) []NotifierHandler
}

// DefaultNotifierFactory 기본 NotifierFactory 구현체입니다.
type DefaultNotifierFactory struct{}

// NewNotifierFactory 새로운 DefaultNotifierFactory를 생성합니다.
func NewNotifierFactory() NotifierFactory {
	return &DefaultNotifierFactory{}
}

// telegramNotifierCreator는 테스트 시 Mocking을 위해 변수로 분리합니다.
var telegramNotifierCreator = newTelegramNotifier

// CreateNotifiers 설정을 기반으로 활성화된 모든 Notifier를 생성하여 반환합니다.
func (f *DefaultNotifierFactory) CreateNotifiers(appConfig *config.AppConfig) []NotifierHandler {
	var handlers []NotifierHandler

	// Telegram Notifier 생성
	for _, telegram := range appConfig.Notifiers.Telegrams {
		h := telegramNotifierCreator(NotifierID(telegram.ID), telegram.BotToken, telegram.ChatID, appConfig)
		handlers = append(handlers, h)
	}

	// 추후 다른 Notifier (예: Slack, Discord) 추가 시 여기에 생성 로직 추가

	return handlers
}
