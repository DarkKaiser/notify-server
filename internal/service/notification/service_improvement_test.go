package notification_test

import (
	"context"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/notification"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockExecutor minimal implementation for testing
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) SubmitTask(req *task.SubmitRequest) error {
	args := m.Called(req)
	return args.Error(0)
}
func (m *MockExecutor) CancelTask(instanceID task.InstanceID) error { return nil }

// mockFactory for creating duplicate notifiers
type mockFactory struct {
	handlers []notifier.NotifierHandler
}

func (m *mockFactory) CreateNotifiers(cfg *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
	return m.handlers, nil
}

func TestService_Start_DuplicateID(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "test-notifier",
		},
	}
	executor := &MockExecutor{}

	// Create 2 notifiers with SAME ID
	h1 := &mocks.MockNotifierHandler{IDValue: "duplicate-id"}
	h2 := &mocks.MockNotifierHandler{IDValue: "duplicate-id"}

	mf := &mockFactory{handlers: []notifier.NotifierHandler{h1, h2}}

	service := notification.NewService(cfg, executor, mf)

	// Action
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Verify
	err := service.Start(ctx, wg)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "중복된 Notifier ID")
	assert.Contains(t, err.Error(), "duplicate-id")
}

// Local mock to control Notify return value
type controllableMockHandler struct {
	mocks.MockNotifierHandler
	notifyResult bool
}

func (m *controllableMockHandler) Notify(taskCtx task.TaskContext, message string) bool {
	return m.notifyResult
}

func TestService_Notify_StoppedNotifier(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "test-notifier",
		},
	}
	executor := &MockExecutor{}

	// Setup a notifier with closed Done channel and Notify returning false
	closedCh := make(chan struct{})
	close(closedCh)

	h := &controllableMockHandler{
		MockNotifierHandler: mocks.MockNotifierHandler{
			IDValue:     "test-notifier",
			DoneChannel: closedCh,
		},
		notifyResult: false, // Simulate full queue/failure
	}

	mf := &mockFactory{handlers: []notifier.NotifierHandler{h}}
	service := notification.NewService(cfg, executor, mf)

	// Start service (ignoring duplicate check passed, just need map population)
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Start normally
	err := service.Start(ctx, wg)
	assert.NoError(t, err)

	// Action
	notifyErr := service.Notify(nil, "test-notifier", "hello")

	// Assert
	// Should return ErrServiceStopped because Done() is closed
	assert.ErrorIs(t, notifyErr, notifier.ErrServiceStopped)

	// Cleanup
	cancel()
	wg.Wait()
}
