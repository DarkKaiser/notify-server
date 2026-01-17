package mocks

import (
	"sync"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
)

// Interface Compliance Check
var _ notifier.NotifierFactory = (*MockNotifierFactory)(nil)

// MockNotifierFactory는 NotifierFactory 인터페이스의 Mock 구현체입니다.
//
// 이 Mock은 Notifier 생성 로직을 테스트하는 데 사용됩니다.
type MockNotifierFactory struct {
	Mu sync.Mutex

	CreateNotifiersFunc   func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error)
	RegisterProcessorFunc func(processor notifier.NotifierConfigProcessor)

	// Call Tracking
	CreateNotifiersCallCount   int
	RegisterProcessorCalled    bool
	RegisterProcessorCallCount int
	RegisteredProcessors       []notifier.NotifierConfigProcessor
}

// CreateNotifiers는 설정에 따라 Notifier 목록을 생성합니다.
func (m *MockNotifierFactory) CreateNotifiers(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.CreateNotifiersCallCount++

	if m.CreateNotifiersFunc != nil {
		return m.CreateNotifiersFunc(cfg, executor)
	}
	return []notifier.NotifierHandler{}, nil
}

// RegisterProcessor는 Notifier 설정 프로세서를 등록합니다.
func (m *MockNotifierFactory) RegisterProcessor(processor notifier.NotifierConfigProcessor) {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.RegisterProcessorCalled = true
	m.RegisterProcessorCallCount++
	m.RegisteredProcessors = append(m.RegisteredProcessors, processor)

	if m.RegisterProcessorFunc != nil {
		m.RegisterProcessorFunc(processor)
	}
}

// Reset 상태를 초기화합니다.
func (m *MockNotifierFactory) Reset() {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.CreateNotifiersCallCount = 0
	m.RegisterProcessorCalled = false
	m.RegisterProcessorCallCount = 0
	m.RegisteredProcessors = make([]notifier.NotifierConfigProcessor, 0)
}
