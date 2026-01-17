package notifier

import (
	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// ConfigProcessor 설정 정보를 바탕으로 Notifier 목록을 생성하는 함수 타입입니다.
type ConfigProcessor func(appConfig *config.AppConfig, executor contract.TaskExecutor) ([]NotifierHandler, error)

// Factory Notifier 생성을 담당하는 팩토리 인터페이스입니다.
type Factory interface {
	RegisterProcessor(processor ConfigProcessor)

	CreateNotifiers(appConfig *config.AppConfig, executor contract.TaskExecutor) ([]NotifierHandler, error)
}

// defaultFactory Processor 패턴을 사용하여 Notifier를 생성하는 기본 Factory 구현체입니다.
type defaultFactory struct {
	processors []ConfigProcessor
}

// NewFactory 새로운 Factory 인스턴스를 생성합니다.
func NewFactory() Factory {
	return &defaultFactory{
		processors: make([]ConfigProcessor, 0),
	}
}

// RegisterProcessor Notifier 생성을 담당할 새로운 Processor를 등록합니다.
func (f *defaultFactory) RegisterProcessor(processor ConfigProcessor) {
	if processor != nil {
		f.processors = append(f.processors, processor)
	}
}

// CreateNotifiers 등록된 모든 Processor를 실행하여 사용 가능한 Notifier 목록을 생성합니다.
func (f *defaultFactory) CreateNotifiers(appConfig *config.AppConfig, executor contract.TaskExecutor) ([]NotifierHandler, error) {
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
