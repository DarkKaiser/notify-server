package mocks

import (
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
)

// Interface Compliance Check
var _ notifier.Factory = (*MockFactory)(nil)

// MockFactory는 Factory 인터페이스의 Mock 구현체입니다.
//
// 이 Mock은 Notifier 생성 로직을 테스트하는 데 사용됩니다.
type MockFactory struct {
	Mu sync.Mutex

	CreateNotifiersFunc func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error)
	RegisterFunc        func(creator notifier.Creator)

	// Call Tracking
	CreateNotifiersCallCount int
	RegisterCalled           bool
	RegisterCallCount        int
	RegisteredCreators       []notifier.Creator
}

// CreateNotifiers는 설정에 따라 Notifier 목록을 생성합니다.
func (m *MockFactory) CreateNotifiers(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
	m.Mu.Lock()
	m.CreateNotifiersCallCount++
	m.Mu.Unlock()

	if m.CreateNotifiersFunc != nil {
		return m.CreateNotifiersFunc(cfg, executor)
	}
	return nil, nil // Default behavior: success with empty list
}

// WithCreateNotifiers CreateNotifiers 호출 시 반환할 값을 설정합니다.
func (m *MockFactory) WithCreateNotifiers(handlers []notifier.NotifierHandler, err error) *MockFactory {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.CreateNotifiersFunc = func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
		return handlers, err
	}
	return m
}

// WithCreateFunc CreateNotifiers 호출 시 실행할 커스텀 함수를 설정합니다.
func (m *MockFactory) WithCreateFunc(fn func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error)) *MockFactory {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.CreateNotifiersFunc = fn
	return m
}

// VerifyCreateCalled CreateNotifiers가 정확히 expected 횟수만큼 호출되었는지 검증합니다.
func (m *MockFactory) VerifyCreateCalled(t *testing.T, expected int) {
	t.Helper()
	m.Mu.Lock()
	defer m.Mu.Unlock()

	if m.CreateNotifiersCallCount != expected {
		t.Errorf("MockFactory.CreateNotifiers called %d times, expected %d", m.CreateNotifiersCallCount, expected)
	}
}

// VerifyRegisterCalled Register가 적어도 한 번 호출되었는지 검증합니다.
func (m *MockFactory) VerifyRegisterCalled(t *testing.T, expectedCalled bool) {
	t.Helper()
	m.Mu.Lock()
	defer m.Mu.Unlock()

	if m.RegisterCalled != expectedCalled {
		t.Errorf("MockFactory.Register called = %v, expected %v", m.RegisterCalled, expectedCalled)
	}
}

// Register는 Notifier 생성 크리에이터를 등록합니다.
func (m *MockFactory) Register(creator notifier.Creator) {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	if m.RegisteredCreators == nil {
		m.RegisteredCreators = make([]notifier.Creator, 0)
	}
	m.RegisteredCreators = append(m.RegisteredCreators, creator)
	m.RegisterCalled = true
	m.RegisterCallCount++

	if m.RegisterFunc != nil {
		m.RegisterFunc(creator)
	}
}

// Reset 상태를 초기화합니다.
func (m *MockFactory) Reset() {
	m.Mu.Lock()
	defer m.Mu.Unlock()

	m.CreateNotifiersFunc = nil
	m.RegisterFunc = nil
	m.CreateNotifiersCallCount = 0
	m.RegisterCalled = false
	m.RegisterCallCount = 0
	m.RegisteredCreators = nil
}
