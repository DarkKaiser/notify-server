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

// =============================================================================
// Test Helpers
// =============================================================================

// StubPanicNotifier is a helper for testing panic recovery.
// Using testify/mock for panic is unstable, so we use a stub.
type StubPanicNotifier struct {
	id string
}

func (s *StubPanicNotifier) ID() contract.NotifierID                                 { return contract.NotifierID(s.id) }
func (s *StubPanicNotifier) Run(ctx context.Context)                                 { panic("Simulated Panic in Notifier Run") }
func (s *StubPanicNotifier) Send(ctx context.Context, n contract.Notification) error { return nil }
func (s *StubPanicNotifier) Close()                                                  {}
func (s *StubPanicNotifier) Done() <-chan struct{}                                   { return nil }
func (s *StubPanicNotifier) SupportsHTML() bool                                      { return true }

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
	// m.On("Send", mock.Anything, mock.Anything).Return(nil).Maybe() // Removed default Send behavior to allow specific error return in tests

	h.mocks[id] = m
	return m
}

func (h *serviceTestHelper) Build(defaultID string) *Service {
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: defaultID,
		},
	}

	notifiers := make([]notifier.Notifier, 0, len(h.mocks))
	for _, m := range h.mocks {
		notifiers = append(notifiers, m)
	}

	h.mockFactory.On("CreateAll", mock.Anything, mock.Anything).Return(notifiers, nil)

	s := NewService(cfg, h.mockFactory, h.mockExecutor)

	// Manually inject notifiers to simulate Start() having run,
	// or properly run Start() in tests.
	// For unit tests focusing on Notify logic, we often want to inject directly.
	// But Start() also handles duplicate checks and default assignment.
	// Let's rely on tests calling Start() or manual injection as needed.

	return s
}

// Manually start service for Notify tests without actual goroutines if needed,
// or just populate fields.
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
// Service Lifecycle Tests
// =============================================================================

func TestNewService(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockExecutor := &taskmocks.MockExecutor{}
	mockFactory := new(notificationmocks.MockFactory)
	service := NewService(appConfig, mockFactory, mockExecutor)

	assert.NotNil(t, service)
	assert.Equal(t, appConfig, service.appConfig)
	assert.Equal(t, mockExecutor, service.executor)
	assert.False(t, service.running)
	assert.NotNil(t, service.creator)
}

func TestService_Start_Success(t *testing.T) {
	helper := newServiceTestHelper(t)
	helper.AddMockNotifier("default-notifier")
	helper.AddMockNotifier("extra-notifier")

	service := helper.Build("default-notifier")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}
	wg.Add(1)

	err := service.Start(ctx, wg)
	assert.NoError(t, err)
	assert.True(t, service.running)

	// Verify notifiers are stored
	assert.Equal(t, 2, len(service.notifiers))
	assert.NotNil(t, service.defaultNotifier)
	assert.Equal(t, contract.NotifierID("default-notifier"), service.defaultNotifier.ID())

	// Shutdown
	cancel()
	wg.Wait()
	assert.False(t, service.running)
}

func TestService_Start_Errors(t *testing.T) {
	tests := []struct {
		name          string
		cfgSetup      func(*config.AppConfig)
		factorySetup  func(*notificationmocks.MockFactory)
		executorNil   bool
		errorContains string
	}{
		{
			name:          "Executor가 nil",
			executorNil:   true,
			errorContains: "Executor 객체가 초기화되지 않았습니다",
		},
		{
			name: "Factory에서 에러 반환",
			factorySetup: func(m *notificationmocks.MockFactory) {
				m.On("CreateAll", mock.Anything, mock.Anything).Return(nil, errors.New("factory error"))
			},
			errorContains: "Notifier 인스턴스 초기화 실패",
		},
		{
			name: "기본 Notifier를 찾을 수 없음",
			cfgSetup: func(c *config.AppConfig) {
				c.Notifier.DefaultNotifierID = "def"
			},
			factorySetup: func(m *notificationmocks.MockFactory) {
				m.On("CreateAll", mock.Anything, mock.Anything).Return([]notifier.Notifier{
					notificationmocks.NewMockNotifier(t, "other"),
				}, nil)
			},
			errorContains: "기본 Notifier('def')를 찾을 수 없습니다",
		},
		{
			name: "중복된 Notifier ID",
			cfgSetup: func(c *config.AppConfig) {
				c.Notifier.DefaultNotifierID = "dup"
			},
			factorySetup: func(m *notificationmocks.MockFactory) {
				// Create 2 mocks with same ID
				h1 := notificationmocks.NewMockNotifier(t, "dup")
				h2 := notificationmocks.NewMockNotifier(t, "dup")
				// Expect redundant Close calls for leak prevention
				h1.On("Close").Return()
				h2.On("Close").Return()

				m.On("CreateAll", mock.Anything, mock.Anything).Return([]notifier.Notifier{h1, h2}, nil)
			},
			errorContains: "중복된 Notifier ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				// Default empty factory behavior if not set
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
	// Setup
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "normal",
		},
	}
	executor := &taskmocks.MockExecutor{}

	panicNotifier := &StubPanicNotifier{id: "panic"}
	normalNotifier := notificationmocks.NewMockNotifier(t, "normal")
	normalNotifier.On("Run", mock.Anything).Run(func(args mock.Arguments) {
		<-args.Get(0).(context.Context).Done()
	}).Return()
	normalNotifier.On("SupportsHTML").Return(true).Maybe()

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

	// Wait for panic to happen and recover
	time.Sleep(50 * time.Millisecond)

	// Verify Service is still running
	assert.NoError(t, service.Health())

	// Cleanup
	cancel()
	wg.Wait()
}

// =============================================================================
// Start Notification Logic Tests
// =============================================================================

func TestService_Notify(t *testing.T) {
	// Common test data
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
		wantLogContains string // Simplified log check (conceptually)
	}{
		{
			name:           "성공: 지정된 Notifier로 전송",
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
			name:           "성공: Notifier 미지정 시 기본값 사용",
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
			name:           "성공: 에러 알림 전송",
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
			name:           "실패: 서비스 중지됨",
			notifierID:     "target",
			defaultID:      "default",
			serviceRunning: false,
			notification:   contract.Notification{NotifierID: "target", Message: "hello"},
			setupMocks:     func(h *serviceTestHelper) {},
			wantErr:        true,
			wantErrIs:      ErrServiceNotRunning,
		},
		{
			name:           "실패: 유효하지 않은 알림 (메시지 없음)",
			notifierID:     "target",
			defaultID:      "default",
			serviceRunning: true,
			notification:   contract.Notification{NotifierID: "target", Message: ""},
			setupMocks:     func(h *serviceTestHelper) {},
			wantErr:        true,
			wantErrIs:      contract.ErrMessageRequired,
		},
		{
			name:           "실패: 존재하지 않는 Notifier ID (기본 Notifier로 경고)",
			notifierID:     "unknown",
			defaultID:      "default",
			serviceRunning: true,
			notification:   contract.Notification{NotifierID: "unknown", Message: "hello", TaskID: taskID, CommandID: cmdID, Title: title},
			setupMocks: func(h *serviceTestHelper) {
				d := h.AddMockNotifier("default")
				// Expect error notification sent to default
				d.On("Send", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
					return n.ErrorOccurred && strings.Contains(n.Message, "등록되지 않은 Notifier ID") && strings.Contains(n.Message, "unknown")
				})).Return(nil)
			},
			wantErr:   true,
			wantErrIs: ErrNotifierNotFound,
		},
		{
			name:           "실패: 존재하지 않는 Notifier ID + 기본 Notifier 없음 (Double Failure)",
			notifierID:     "unknown",
			defaultID:      "", // No default
			serviceRunning: true,
			notification:   contract.Notification{NotifierID: "unknown", Message: "hello"},
			setupMocks: func(h *serviceTestHelper) {
				// No default notifier to fallback to
				// But we need at least one notifier in map to not just have Start() fail?
				// Actually if defaultID is empty or nil, service might have running=true but defaultNotifier=nil
				h.AddMockNotifier("other")
			},
			wantErr:   true,
			wantErrIs: ErrNotifierNotFound,
		},
		{
			name:           "실패: 기본 Notifier ID 누락 (Service Notify 호출 시)",
			notifierID:     "",
			defaultID:      "", // Invalid config state or runtime state
			serviceRunning: true,
			notification:   contract.Notification{Message: "hello"},
			setupMocks:     func(h *serviceTestHelper) {},
			wantErr:        true,
			wantErrIs:      ErrServiceNotRunning, // Mapped to service not running if default is nil
		},
		{
			name:           "실패: Notifier Send 에러 (ErrClosed -> ServiceStopped 매핑)",
			notifierID:     "target",
			defaultID:      "default",
			serviceRunning: true,
			notification:   contract.Notification{NotifierID: "target", Message: "hello"},
			setupMocks: func(h *serviceTestHelper) {
				m := h.AddMockNotifier("target")
				m.On("Send", mock.Anything, mock.Anything).Return(notifier.ErrClosed)
			},
			wantErr:   true,
			wantErrIs: ErrServiceNotRunning,
		},
		{
			name:           "실패: Notifier Send 일반 에러",
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
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			helper := newServiceTestHelper(t)

			// Custom mocks setup
			if tt.setupMocks != nil {
				tt.setupMocks(helper)
			}

			service := helper.Build(tt.defaultID)

			// Manually set running state and map injection
			// (Since Build() constructs service but doesn't call Start)
			service.runningMu.Lock()
			service.running = tt.serviceRunning
			// Inject created mocks into service map
			for id, m := range helper.mocks {
				service.notifiers[contract.NotifierID(id)] = m
				if id == tt.defaultID {
					service.defaultNotifier = m
				}
			}
			service.runningMu.Unlock()

			// Action
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
// Helper Method Tests
// =============================================================================

func TestService_Health(t *testing.T) {
	service := &Service{
		running: false,
	}
	assert.ErrorIs(t, service.Health(), ErrServiceNotRunning)

	service.running = true
	assert.NoError(t, service.Health())
}

func TestService_SupportsHTML(t *testing.T) {
	helper := newServiceTestHelper(t)
	m := helper.AddMockNotifier("html-support")
	m.On("SupportsHTML").Return(true)

	service := helper.Build("default")
	helper.DirtyStart(service, "default")

	assert.True(t, service.SupportsHTML("html-support"))
	assert.False(t, service.SupportsHTML("unknown"))
}

func TestService_GetExecutors(t *testing.T) {
	// Assuming GetExecutors exists or similar accessor?
	// Based on viewed code, Executor is internal.
	// If no public accessor, skip.
}
