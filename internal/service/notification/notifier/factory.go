package notifier

import (
	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// Creator Notifier 목록을 생성하는 역할을 정의한 인터페이스입니다.
type Creator interface {
	CreateNotifiers(appConfig *config.AppConfig, executor contract.TaskExecutor) ([]Notifier, error)
}

// FactoryFunc 설정 정보를 바탕으로 Notifier 목록을 생성하는 함수 타입입니다.
type FactoryFunc func(appConfig *config.AppConfig, executor contract.TaskExecutor) ([]Notifier, error)

// CreateNotifiers 함수 f를 호출하여 Notifier 생성을 위임합니다.
func (f FactoryFunc) CreateNotifiers(appConfig *config.AppConfig, executor contract.TaskExecutor) ([]Notifier, error) {
	return f(appConfig, executor)
}

// Factory Notifier 생성을 담당하는 팩토리 인터페이스입니다.
type Factory interface {
	Creator

	Register(creator Creator)
}

// defaultFactory Processor 패턴을 사용하여 Notifier를 생성하는 기본 Factory 구현체입니다.
type defaultFactory struct {
	creators []Creator
}

// NewFactory 새로운 Factory 인스턴스를 생성합니다.
func NewFactory() Factory {
	return &defaultFactory{
		creators: make([]Creator, 0),
	}
}

// Register Notifier 생성을 담당할 새로운 Creator를 등록합니다.
func (f *defaultFactory) Register(creator Creator) {
	if creator != nil {
		f.creators = append(f.creators, creator)
	}
}

// CreateNotifiers 등록된 모든 Creator를 실행하여 사용 가능한 Notifier 목록을 생성합니다.
func (f *defaultFactory) CreateNotifiers(appConfig *config.AppConfig, executor contract.TaskExecutor) ([]Notifier, error) {
	var allNotifiers []Notifier

	for _, creator := range f.creators {
		notifiers, err := creator.CreateNotifiers(appConfig, executor)
		if err != nil {
			return nil, err
		}
		allNotifiers = append(allNotifiers, notifiers...)
	}

	return allNotifiers, nil
}
