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
	// Default behaviors to avoid boilerplate in every test
	m.On("Run", mock.Anything).Return()
	m.On("Done").Return(nil)
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

// WithSend configures the mock to return a specific error for Send calls.
func (m *MockNotifier) WithSend(err error) *MockNotifier {
	m.On("Send", mock.Anything, mock.Anything).Return(err)
	return m
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

// WithSupportsHTML configures the mock to return a specific boolean for SupportsHTML calls.
func (m *MockNotifier) WithSupportsHTML(supported bool) *MockNotifier {
	m.On("SupportsHTML").Return(supported)
	return m
}

// Done returns a channel that is closed when the notifier is done.
func (m *MockNotifier) Done() <-chan struct{} {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(<-chan struct{})
}
