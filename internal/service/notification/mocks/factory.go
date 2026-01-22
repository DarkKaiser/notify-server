package mocks

import (
	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/stretchr/testify/mock"
)

// Interface Compliance Check (컴파일 타임 검증)
var _ notifier.Factory = (*MockFactory)(nil)

// MockFactory는 Factory 인터페이스의 Mock 구현체입니다.
// "github.com/stretchr/testify/mock" 패키지를 사용하여 동작을 정의하고 검증할 수 있습니다.
type MockFactory struct {
	mock.Mock
}

// NewMockFactory는 새로운 MockFactory 인스턴스를 생성하고, 테스트 객체(t)와 연결합니다.
// 테스트가 종료될 때 모든 Mock 기대치(Expectations)가 충족되었는지 자동으로 검증됩니다.
func NewMockFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockFactory {
	m := &MockFactory{}
	m.Mock.Test(t)

	t.Cleanup(func() {
		m.AssertExpectations(t)
	})

	return m
}

// =============================================================================
// Interface Implementation
// =============================================================================

// CreateAll은 설정된 Notifier 목록을 반환합니다.
func (m *MockFactory) CreateAll(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.Notifier, error) {
	args := m.Called(cfg, executor)

	// 첫 번째 반환값이 nil일 경우를 안전하게 처리합니다.
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]notifier.Notifier), args.Error(1)
}

// Register는 새로운 Creator를 등록합니다.
func (m *MockFactory) Register(creator notifier.Creator) {
	m.Called(creator)
}

// =============================================================================
// Fluent Helpers (Expectation Setup)
// =============================================================================

// OnCreateAll은 CreateAll 메서드 호출에 대한 기대치를 설정하는 헬퍼입니다.
// 반환된 *mock.Call을 통해 Return, Run, Once 등을 체이닝할 수 있습니다.
func (m *MockFactory) OnCreateAll(cfg interface{}, executor interface{}) *mock.Call {
	if cfg == nil {
		cfg = mock.Anything
	}
	if executor == nil {
		executor = mock.Anything
	}
	return m.On("CreateAll", cfg, executor)
}

// ExpectCreateAllSuccess는 CreateAll 호출 시 지정된 Notifier 목록을 반환하도록 설정합니다.
func (m *MockFactory) ExpectCreateAllSuccess(notifiers []notifier.Notifier) *mock.Call {
	return m.OnCreateAll(mock.Anything, mock.Anything).Return(notifiers, nil)
}

// ExpectCreateAllFailure는 CreateAll 호출 시 에러를 반환하도록 설정합니다.
func (m *MockFactory) ExpectCreateAllFailure(err error) *mock.Call {
	return m.OnCreateAll(mock.Anything, mock.Anything).Return(nil, err)
}

// OnRegister는 Register 메서드 호출에 대한 기대치를 설정하는 헬퍼입니다.
func (m *MockFactory) OnRegister(creator interface{}) *mock.Call {
	if creator == nil {
		creator = mock.Anything
	}
	return m.On("Register", creator)
}

// ExpectRegister는 Register 호출을 기대하고, 아무런 동작도 하지 않도록 설정합니다.
func (m *MockFactory) ExpectRegister() *mock.Call {
	return m.OnRegister(mock.Anything).Return()
}

// =============================================================================
// Deprecated / Legacy Helpers (Backward Compatibility)
// =============================================================================

// WithCreateAll configures the mock to return specific notifiers for CreateAll calls.
// Deprecated: Use OnCreateAll or ExpectCreateAllSuccess instead.
func (m *MockFactory) WithCreateAll(notifiers []notifier.Notifier, err error) *MockFactory {
	m.On("CreateAll", mock.Anything, mock.Anything).Return(notifiers, err)
	return m
}

// WithRegister configures the mock to expect Register calls.
// Deprecated: Use OnRegister or ExpectRegister instead.
func (m *MockFactory) WithRegister() *MockFactory {
	m.On("Register", mock.Anything).Return()
	return m
}
