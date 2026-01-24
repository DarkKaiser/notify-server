package notification

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"

	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// Sender Compliance Check
var _ contract.NotificationSender = (*Service)(nil)

// =============================================================================
// Test Constants
// =============================================================================

const (
	testNotifierID    = "test-notifier"
	defaultNotifierID = "default-notifier"
)

// TestMain runs tests and checks for goroutine leaks.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// =============================================================================
// Test Helpers & Stubs
// =============================================================================

// StubPanicNotifier simulates a notifier that panics during Run().
// It implements the Notifier interface.
type StubPanicNotifier struct {
	id string
}

func (s *StubPanicNotifier) ID() contract.NotifierID                                    { return contract.NotifierID(s.id) }
func (s *StubPanicNotifier) Run(ctx context.Context)                                    { panic("Simulated Panic in Notifier Run") }
func (s *StubPanicNotifier) Send(ctx context.Context, n contract.Notification) error    { return nil }
func (s *StubPanicNotifier) TrySend(ctx context.Context, n contract.Notification) error { return nil }
func (s *StubPanicNotifier) Close()                                                     {}
func (s *StubPanicNotifier) Done() <-chan struct{}                                      { return nil }
func (s *StubPanicNotifier) SupportsHTML() bool                                         { return true }

// serviceTestHelper simplifies test setup
type serviceTestHelper struct {
	t            *testing.T
	service      *Service
	mockExecutor *taskmocks.MockExecutor
	mockFactory  *notificationmocks.MockFactory
	// Key: NotifierID, Value: MockNotifier
	mocks map[string]*notificationmocks.MockNotifier
}

func newServiceTestHelper(t *testing.T) *serviceTestHelper {
	return &serviceTestHelper{
		t:            t,
		mockExecutor: &taskmocks.MockExecutor{},
		mockFactory:  new(notificationmocks.MockFactory),
		mocks:        make(map[string]*notificationmocks.MockNotifier),
	}
}

// AddMockNotifier creates and registers a mock notifier for the factory.
func (h *serviceTestHelper) AddMockNotifier(id string) *notificationmocks.MockNotifier {
	m := notificationmocks.NewMockNotifier(h.t, contract.NotifierID(id))
	// Default behaviors to avoid unexpected calls failing tests unless overridden
	m.On("ID").Return(contract.NotifierID(id)).Maybe()
	m.On("SupportsHTML").Return(true).Maybe()
	m.On("Run", mock.Anything).Run(func(args mock.Arguments) {
		ctx := args.Get(0).(context.Context)
		<-ctx.Done()
	}).Return().Maybe()
	m.On("Done").Return(nil).Maybe()
	m.On("Close").Return().Maybe()

	h.mocks[id] = m
	return m
}

// Build creates the Service instance. It does NOT start it.
func (h *serviceTestHelper) Build(defaultID string) *Service {
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: defaultID,
		},
	}

	// Prepare factory response
	notifiers := make([]notifier.Notifier, 0, len(h.mocks))
	for _, m := range h.mocks {
		notifiers = append(notifiers, m)
	}

	// Only setup CreateAll expectation if there are mocks or logic requires it
	h.mockFactory.On("CreateAll", mock.Anything, mock.Anything).Return(notifiers, nil).Maybe()

	s := NewService(cfg, h.mockFactory, h.mockExecutor)
	return s
}

// DirtyStart manually sets the service to running state with injected mocks.
// This is useful for unit testing Notify() logic without spinning up goroutines.
func (h *serviceTestHelper) DirtyStart(s *Service, defaultID string) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	s.running = true

	for id, m := range h.mocks {
		s.notifiers[contract.NotifierID(id)] = m
		if id == defaultID {
			s.defaultNotifier = m
		}
	}
}

// =============================================================================
// 1. Service Lifecycle Tests (Start & Shutdown)
// =============================================================================

func TestNewService(t *testing.T) {
	t.Parallel()
	appConfig := &config.AppConfig{}
	mockExecutor := &taskmocks.MockExecutor{}
	mockFactory := new(notificationmocks.MockFactory)
	service := NewService(appConfig, mockFactory, mockExecutor)

	assert.NotNil(t, service)
	assert.False(t, service.running)
}

func TestService_Start_Success(t *testing.T) {
	t.Parallel()
	helper := newServiceTestHelper(t)
	helper.AddMockNotifier("default-notifier")
	helper.AddMockNotifier("extra-notifier")

	service := helper.Build("default-notifier")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}
	wg.Add(1) // Main Stop WG

	// Act
	err := service.Start(ctx, wg)
	assert.NoError(t, err)
	assert.True(t, service.running)

	// Verify internal state
	assert.Equal(t, 2, len(service.notifiers))
	assert.NotNil(t, service.defaultNotifier)
	assert.Equal(t, contract.NotifierID("default-notifier"), service.defaultNotifier.ID())

	// Shutdown
	cancel()  // Signal shutdown
	wg.Wait() // Wait for all goroutines to finish
	assert.False(t, service.running)
}

func TestService_Start_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cfgSetup      func(*config.AppConfig)
		factorySetup  func(*notificationmocks.MockFactory)
		executorNil   bool
		errorContains string
	}{
		{
			name:          "Executor Not Initialized",
			executorNil:   true,
			errorContains: "Executor 객체가 초기화되지 않았습니다",
		},
		{
			name: "Factory Error",
			factorySetup: func(m *notificationmocks.MockFactory) {
				m.On("CreateAll", mock.Anything, mock.Anything).Return(nil, errors.New("factory error"))
			},
			errorContains: "Notifier 인스턴스 초기화 실패",
		},
		{
			name: "Default Notifier Not Found",
			cfgSetup: func(c *config.AppConfig) {
				c.Notifier.DefaultNotifierID = "missing-def"
			},
			factorySetup: func(m *notificationmocks.MockFactory) {
				m.On("CreateAll", mock.Anything, mock.Anything).Return([]notifier.Notifier{
					notificationmocks.NewMockNotifier(t, "other"),
				}, nil)
			},
			errorContains: "기본 Notifier('missing-def')를 찾을 수 없습니다",
		},
		{
			name: "Duplicate Notifier ID",
			cfgSetup: func(c *config.AppConfig) {
				c.Notifier.DefaultNotifierID = "dup"
			},
			factorySetup: func(m *notificationmocks.MockFactory) {
				// Create 2 mocks with same ID
				h1 := notificationmocks.NewMockNotifier(t, "dup")
				h2 := notificationmocks.NewMockNotifier(t, "dup")
				// Must ensure they are closed to prevent leaks during error handling in Start
				h1.On("Close").Return()
				h2.On("Close").Return()

				m.On("CreateAll", mock.Anything, mock.Anything).Return([]notifier.Notifier{h1, h2}, nil)
			},
			errorContains: "중복된 Notifier ID",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.AppConfig{}
			if tt.cfgSetup != nil {
				tt.cfgSetup(cfg)
			}

			var executor contract.TaskExecutor = &taskmocks.MockExecutor{}
			if tt.executorNil {
				executor = nil
			}

			factory := new(notificationmocks.MockFactory)
			if tt.factorySetup != nil {
				tt.factorySetup(factory)
			} else {
				// Default empty factory behavior
				factory.On("CreateAll", mock.Anything, mock.Anything).Return([]notifier.Notifier{}, nil)
			}

			service := NewService(cfg, factory, executor)

			ctx := context.Background()
			wg := &sync.WaitGroup{}
			wg.Add(1)

			err := service.Start(ctx, wg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

func TestService_Start_PanicRecovery(t *testing.T) {
	t.Parallel()

	// Setup
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "normal",
		},
	}
	executor := &taskmocks.MockExecutor{}

	// StubPanicNotifier will panic on Run()
	panicNotifier := &StubPanicNotifier{id: "panic"}

	// Normal notifier
	normalNotifier := notificationmocks.NewMockNotifier(t, "normal")
	normalNotifier.On("Run", mock.Anything).Run(func(args mock.Arguments) {
		<-args.Get(0).(context.Context).Done()
	}).Return()
	normalNotifier.On("SupportsHTML").Return(true).Maybe()
	normalNotifier.On("Close").Return().Maybe()

	factory := new(notificationmocks.MockFactory)
	factory.On("CreateAll", mock.Anything, mock.Anything).Return([]notifier.Notifier{panicNotifier, normalNotifier}, nil)

	service := NewService(cfg, factory, executor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Action
	err := service.Start(ctx, wg)
	assert.NoError(t, err)

	// Wait briefly for panic to happen and recover
	// Since panic happens in a goroutine, we can't assert it directly,
	// but we verify the service keeps running and normal notifier works.
	time.Sleep(50 * time.Millisecond)

	// Verify Service is still healthy
	assert.NoError(t, service.Health())

	// Cleanup
	cancel()
	wg.Wait()
}

// =============================================================================
// 2. Notify Logic Tests (Table Driven)
// =============================================================================

func TestService_Notify(t *testing.T) {
	t.Parallel()

	taskID := contract.TaskID("TaskA")
	cmdID := contract.TaskCommandID("CmdA")
	title := "MyTitle"

	tests := []struct {
		name           string
		notifierID     string
		defaultID      string
		serviceRunning bool

		// Input
		notification contract.Notification

		// Mocks setup
		setupMocks func(h *serviceTestHelper)

		// Expectations
		wantErr         bool
		wantErrIs       error
		wantErrContains string
	}{
		{
			name:           "Success: Specific Notifier",
			notifierID:     "target",
			defaultID:      "default",
			serviceRunning: true,
			notification:   contract.Notification{NotifierID: "target", Message: "hello"},
			setupMocks: func(h *serviceTestHelper) {
				m := h.AddMockNotifier("target")
				m.On("Send", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
					return n.Message == "hello"
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name:           "Success: Default Notifier Fallback",
			notifierID:     "", // Empty
			defaultID:      "default",
			serviceRunning: true,
			notification:   contract.Notification{Message: "hello defaults"},
			setupMocks: func(h *serviceTestHelper) {
				m := h.AddMockNotifier("default")
				m.On("Send", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
					return n.Message == "hello defaults"
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name:           "Success: Error Notification",
			notifierID:     "target",
			defaultID:      "default",
			serviceRunning: true,
			notification:   contract.Notification{NotifierID: "target", Message: "fail", ErrorOccurred: true},
			setupMocks: func(h *serviceTestHelper) {
				m := h.AddMockNotifier("target")
				m.On("Send", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
					return n.ErrorOccurred == true
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name:           "Failure: Service Not Running",
			notifierID:     "target",
			defaultID:      "default",
			serviceRunning: false,
			notification:   contract.Notification{NotifierID: "target", Message: "hello"},
			setupMocks:     func(h *serviceTestHelper) {},
			wantErr:        true,
			wantErrIs:      ErrServiceNotRunning,
		},
		{
			name:           "Failure: Invalid Notification",
			notifierID:     "target",
			defaultID:      "default",
			serviceRunning: true,
			notification:   contract.Notification{NotifierID: "target", Message: ""},
			setupMocks:     func(h *serviceTestHelper) {},
			wantErr:        true,
			wantErrIs:      contract.ErrMessageRequired,
		},
		{
			name:           "Failure: Notifier Not Found (Send Error to Default)",
			notifierID:     "unknown",
			defaultID:      "default",
			serviceRunning: true,
			notification:   contract.Notification{NotifierID: "unknown", Message: "hello", TaskID: taskID, CommandID: cmdID, Title: title},
			setupMocks: func(h *serviceTestHelper) {
				d := h.AddMockNotifier("default")
				// Expect error notification sent to default about unknown notifier
				d.On("Send", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
					return n.ErrorOccurred && strings.Contains(n.Message, "등록되지 않은 Notifier ID")
				})).Return(nil)
			},
			wantErr:   true,
			wantErrIs: ErrNotifierNotFound,
		},
		{
			name:           "Failure: Notifier Not Found & Default Missing",
			notifierID:     "unknown",
			defaultID:      "", // No default configured
			serviceRunning: true,
			notification:   contract.Notification{NotifierID: "unknown", Message: "hello"},
			setupMocks: func(h *serviceTestHelper) {
				// We add 'other' just so map isn't empty, but default is unset
				h.AddMockNotifier("other")
			},
			wantErr:   true,
			wantErrIs: ErrNotifierNotFound,
		},
		{
			name:           "Failure: Default Notifier Missing (Runtime)",
			notifierID:     "",
			defaultID:      "", // Invalid state
			serviceRunning: true,
			notification:   contract.Notification{Message: "hello"},
			setupMocks:     func(h *serviceTestHelper) {},
			wantErr:        true,
			wantErrIs:      ErrServiceNotRunning,
		},
		{
			name:           "Failure: Notifier Closed (Service Running) -> ErrNotifierUnavailable",
			notifierID:     "target",
			defaultID:      "default",
			serviceRunning: true,
			notification:   contract.Notification{NotifierID: "target", Message: "hello"},
			setupMocks: func(h *serviceTestHelper) {
				m := h.AddMockNotifier("target")
				m.On("Send", mock.Anything, mock.Anything).Return(notifier.ErrClosed)
			},
			wantErr:   true,
			wantErrIs: ErrNotifierUnavailable,
		},
		{
			name:           "Failure: Notifier Closed (Service Stopping) -> ErrServiceNotRunning",
			notifierID:     "target",
			defaultID:      "default",
			serviceRunning: true, // Initially running to pass first check
			notification:   contract.Notification{NotifierID: "target", Message: "hello"},
			setupMocks: func(h *serviceTestHelper) {
				m := h.AddMockNotifier("target")
				m.On("Send", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
					// Simulate service shutdown occurring during Send
					// check h.service is set (requires helper update in loop)
					if h.service != nil {
						h.service.runningMu.Lock()
						h.service.running = false
						h.service.runningMu.Unlock()
					}
				}).Return(notifier.ErrClosed)
			},
			wantErr:   true,
			wantErrIs: ErrServiceNotRunning,
		},
		{
			name:           "Failure: Notifier Send Generic Error",
			notifierID:     "target",
			defaultID:      "default",
			serviceRunning: true,
			notification:   contract.Notification{NotifierID: "target", Message: "hello"},
			setupMocks: func(h *serviceTestHelper) {
				m := h.AddMockNotifier("target")
				m.On("Send", mock.Anything, mock.Anything).Return(errors.New("network error"))
			},
			wantErr:         true,
			wantErrContains: "network error",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			helper := newServiceTestHelper(t)

			if tt.setupMocks != nil {
				tt.setupMocks(helper)
			}

			// Instantiate Service
			service := helper.Build(tt.defaultID)
			helper.service = service

			// Manually inject state (DirtyStart) for unit testing Notify
			helper.DirtyStart(service, tt.defaultID)

			// Override running state if test requires service to be stopped
			if !tt.serviceRunning {
				service.runningMu.Lock()
				service.running = false
				service.runningMu.Unlock()
			}

			// Act
			err := service.Notify(context.Background(), tt.notification)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// 3. Helper Method Tests
// =============================================================================

func TestService_Start_Idempotency(t *testing.T) {
	t.Parallel()
	helper := newServiceTestHelper(t)
	helper.AddMockNotifier("default")

	service := helper.Build("default")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}

	// 1st Start
	wg.Add(1)
	err := service.Start(ctx, wg)
	assert.NoError(t, err)

	// 2nd Start (Idempotent - should return nil but log warning)
	wg.Add(1)
	err = service.Start(ctx, wg)
	assert.NoError(t, err)

	// Clean up
	cancel()
	wg.Wait()
}

func TestService_Cleanup_Resources(t *testing.T) {
	t.Parallel()
	helper := newServiceTestHelper(t)
	helper.AddMockNotifier("default")

	service := helper.Build("default")
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Start
	require.NoError(t, service.Start(ctx, wg))
	require.True(t, service.running)

	// Stop
	cancel()
	wg.Wait()

	// Verify Cleanup
	assert.False(t, service.running)
	assert.Nil(t, service.executor, "Executor should be nil after shutdown")
	assert.Nil(t, service.notifiers, "Notifiers map should be nil after shutdown")
	assert.Nil(t, service.defaultNotifier, "DefaultNotifier should be nil after shutdown")
}

func TestService_Notify_Concurrent(t *testing.T) {
	t.Parallel()
	helper := newServiceTestHelper(t)
	m := helper.AddMockNotifier("default")
	// Allow Send to be called multiple times
	m.On("Send", mock.Anything, mock.Anything).Return(nil).Maybe()

	service := helper.Build("default")
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	require.NoError(t, service.Start(ctx, wg))

	// Run concurrent notifications
	var notifyWg sync.WaitGroup
	workers := 20
	requests := 50

	notifyWg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer notifyWg.Done()
			for j := 0; j < requests; j++ {
				// We don't check error here strictly because service might stop in middle
				// We just want to ensure no panics (race detector will catch races)
				_ = service.Notify(context.Background(), contract.Notification{
					Message: "load test",
				})
				time.Sleep(time.Millisecond)
			}
		}()
	}

	// Trigger shutdown in middle of processing
	time.Sleep(10 * time.Millisecond)
	cancel()
	wg.Wait()

	// Wait for notify workers
	notifyWg.Wait()

	// Ensure service is stopped
	assert.False(t, service.running)
}

func TestService_Health(t *testing.T) {
	t.Parallel()
	service := &Service{
		running: false,
	}
	assert.ErrorIs(t, service.Health(), ErrServiceNotRunning)

	service.running = true
	assert.NoError(t, service.Health())
}

func TestService_SupportsHTML(t *testing.T) {
	t.Parallel()
	helper := newServiceTestHelper(t)
	m := helper.AddMockNotifier("html-support")
	m.On("SupportsHTML").Return(true)

	service := helper.Build("default")
	helper.DirtyStart(service, "default")

	assert.True(t, service.SupportsHTML("html-support"))
	assert.False(t, service.SupportsHTML("unknown"))
}
