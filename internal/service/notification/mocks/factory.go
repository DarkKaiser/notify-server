package mocks

import (
	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/stretchr/testify/mock"
)

// Interface Compliance Check
var _ notifier.Factory = (*MockFactory)(nil)

// MockFactory is a mock implementation of the Factory interface using testify/mock.
type MockFactory struct {
	mock.Mock
}

// CreateAll creates notifiers based on configuration.
func (m *MockFactory) CreateAll(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.Notifier, error) {
	args := m.Called(cfg, executor)
	// Handle nil return safely
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]notifier.Notifier), args.Error(1)
}

// Register registers a notifier creator.
func (m *MockFactory) Register(creator notifier.Creator) {
	m.Called(creator)
}

// WithCreateAll configures the mock to return specific notifiers for CreateAll calls.
func (m *MockFactory) WithCreateAll(notifiers []notifier.Notifier, err error) *MockFactory {
	m.On("CreateAll", mock.Anything, mock.Anything).Return(notifiers, err)
	return m
}

// WithRegister configures the mock to expect Register calls.
func (m *MockFactory) WithRegister() *MockFactory {
	m.On("Register", mock.Anything).Return()
	return m
}
