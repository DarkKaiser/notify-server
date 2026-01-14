package mocks

import (
	"sync"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
)

// Interface Compliance Check
var _ notifier.NotifierFactory = (*MockNotifierFactory)(nil)

// MockNotifierFactory는 NotifierFactory 인터페이스의 Mock 구현체입니다.
//
// 이 Mock은 Notifier 생성 로직을 테스트하는 데 사용됩니다.
type MockNotifierFactory struct {
	Mu                  sync.RWMutex
	CreateNotifiersFunc func(cfg *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error)
}

// CreateNotifiers는 설정에 따라 Notifier 목록을 생성합니다.
func (m *MockNotifierFactory) CreateNotifiers(cfg *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
	m.Mu.RLock()
	defer m.Mu.RUnlock()

	if m.CreateNotifiersFunc != nil {
		return m.CreateNotifiersFunc(cfg, executor)
	}
	return []notifier.NotifierHandler{}, nil
}

// RegisterProcessor는 Notifier 설정 프로세서를 등록합니다.
func (m *MockNotifierFactory) RegisterProcessor(processor notifier.NotifierConfigProcessor) {}
