package notification

import (
	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/task"
)

// NotifierFactory 알림 핸들러 생성을 담당하는 팩토리 인터페이스입니다.
type NotifierFactory interface {
	CreateNotifiers(appConfig *config.AppConfig, executor task.Executor) ([]NotifierHandler, error)
}

// NotifierConfigProcessor 설정에서 특정 알림 타입(예: Telegram)을 생성하는 함수 타입입니다.
type NotifierConfigProcessor func(appConfig *config.AppConfig, executor task.Executor) ([]NotifierHandler, error)

// DefaultNotifierFactory 기본 NotifierFactory 구현체입니다.
type DefaultNotifierFactory struct {
	processors []NotifierConfigProcessor
}

// NewNotifierFactory 새로운 DefaultNotifierFactory를 생성합니다.
func NewNotifierFactory() *DefaultNotifierFactory {
	return &DefaultNotifierFactory{
		processors: make([]NotifierConfigProcessor, 0),
	}
}

// RegisterProcessor 새로운 설정 프로세서를 등록합니다.
func (f *DefaultNotifierFactory) RegisterProcessor(processor NotifierConfigProcessor) {
	if processor != nil {
		f.processors = append(f.processors, processor)
	}
}

// CreateNotifiers 설정을 기반으로 활성화된 모든 Notifier를 생성하여 반환합니다.
func (f *DefaultNotifierFactory) CreateNotifiers(appConfig *config.AppConfig, executor task.Executor) ([]NotifierHandler, error) {
	var allHandlers []NotifierHandler

	for _, processor := range f.processors {
		handlers, err := processor(appConfig, executor)
		if err != nil {
			return nil, err
		}
		allHandlers = append(allHandlers, handlers...)
	}

	return allHandlers, nil
}
