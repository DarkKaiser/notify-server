package mocks

import (
	"context"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/stretchr/testify/mock"
)

// Interface Compliance Check
var _ notifier.Notifier = (*MockNotifier)(nil)

// NewMockNotifier creates a new MockNotifier with default ID behavior.
func NewMockNotifier(id contract.NotifierID) *MockNotifier {
	m := &MockNotifier{}
	// Setup default behavior for ID as it's immutable context and used everywhere
	m.On("ID").Return(id)
	return m
}

// MockNotifier is a mock implementation of the Notifier interface using testify/mock.
type MockNotifier struct {
	mock.Mock
}

// ID returns the notifier's ID.
func (m *MockNotifier) ID() contract.NotifierID {
	args := m.Called()
	return args.Get(0).(contract.NotifierID)
}

// Send sends a notification.
func (m *MockNotifier) Send(taskCtx contract.TaskContext, message string) error {
	args := m.Called(taskCtx, message)
	return args.Error(0)
}

// Run runs the notifier.
func (m *MockNotifier) Run(ctx context.Context) {
	m.Called(ctx)
}

// SupportsHTML returns whether the notifier supports HTML.
func (m *MockNotifier) SupportsHTML() bool {
	args := m.Called()
	return args.Bool(0)
}

// Done returns a channel that is closed when the notifier is done.
func (m *MockNotifier) Done() <-chan struct{} {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(<-chan struct{})
}
