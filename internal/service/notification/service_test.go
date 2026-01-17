package notification

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"

	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	"github.com/stretchr/testify/assert"
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
	testMessage       = "test message"
)

// =============================================================================
// Test Helpers
// =============================================================================

// mockServiceOptions는 setupMockServiceWithOptions의 옵션입니다.
type mockServiceOptions struct {
	notifierID   contract.NotifierID
	supportsHTML bool
	running      bool
}

// setupMockService는 기본 설정으로 테스트용 Service와 Mock 객체를 생성합니다.
func setupMockService() (*Service, *taskmocks.MockExecutor, *notificationmocks.MockNotifierHandler) {
	return setupMockServiceWithOptions(mockServiceOptions{
		notifierID:   testNotifierID,
		supportsHTML: true,
		running:      false,
	})
}

// setupMockServiceWithOptions는 옵션을 받아 테스트용 Service와 Mock 객체를 생성합니다.
func setupMockServiceWithOptions(opts mockServiceOptions) (*Service, *taskmocks.MockExecutor, *notificationmocks.MockNotifierHandler) {
	appConfig := &config.AppConfig{}
	mockExecutor := &taskmocks.MockExecutor{}
	mockNotifier := &notificationmocks.MockNotifierHandler{
		IDValue:           opts.notifierID,
		SupportsHTMLValue: opts.supportsHTML,
	}

	mockFactory := &notificationmocks.MockNotifierFactory{}

	service := NewService(appConfig, mockFactory, mockExecutor)
	service.notifiersMap = map[contract.NotifierID]notifier.NotifierHandler{
		mockNotifier.IDValue: mockNotifier,
	}
	service.defaultNotifier = mockNotifier
	service.running = opts.running

	return service, mockExecutor, mockNotifier
}

// assertNotifyCalled는 mockNotifier가 정확히 한 번 호출되었고 메시지가 일치하는지 검증합니다.
func assertNotifyCalled(t *testing.T, mock *notificationmocks.MockNotifierHandler, expectedMsg string) {
	t.Helper()
	require.Len(t, mock.NotifyCalls, 1, "Expected exactly one notify call")
	assert.Equal(t, expectedMsg, mock.NotifyCalls[0].Message, "Message should match")
}

// assertNotifyCalledWithContext는 mockNotifier가 호출되었고 TaskContext가 있는지 검증합니다.
func assertNotifyCalledWithContext(t *testing.T, mock *notificationmocks.MockNotifierHandler, expectedMsg string) {
	t.Helper()
	assertNotifyCalled(t, mock, expectedMsg)
	assert.NotNil(t, mock.NotifyCalls[0].TaskCtx, "TaskContext should be present")
}

// assertNotifyNotCalled는 mockNotifier가 호출되지 않았는지 검증합니다.
func assertNotifyNotCalled(t *testing.T, mock *notificationmocks.MockNotifierHandler) {
	t.Helper()
	assert.Empty(t, mock.NotifyCalls, "Expected no notify calls")
}

// =============================================================================
// Service Initialization Tests
// =============================================================================

// TestNewService는 Service 생성을 검증합니다.
func TestNewService(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockExecutor := &taskmocks.MockExecutor{}
	mockFactory := &notificationmocks.MockNotifierFactory{}
	service := NewService(appConfig, mockFactory, mockExecutor)

	assert.NotNil(t, service)
	assert.Equal(t, appConfig, service.appConfig)
	assert.Equal(t, mockExecutor, service.executor)
	assert.False(t, service.running)
	// assert.NotNil(t, service.notifiersStopWG) // sync.WaitGroup is a struct, not a pointer, so it's never nil. Checking it causes lock copy lint.
	assert.NotNil(t, service.notifierFactory)
}

// =============================================================================
// HTML Support Tests
// =============================================================================

// TestSupportsHTML은 HTML 지원 여부 확인을 검증합니다.
func TestSupportsHTML(t *testing.T) {
	mockNotifier := &notificationmocks.MockNotifierHandler{IDValue: "test", SupportsHTMLValue: true}
	service := &Service{
		notifiersMap: map[contract.NotifierID]notifier.NotifierHandler{
			mockNotifier.IDValue: mockNotifier,
		},
	}

	tests := []struct {
		name       string
		notifierID string
		want       bool
	}{
		{"존재하는 Notifier (HTML 지원)", "test", true},
		{"존재하지 않는 Notifier", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, service.SupportsHTML(contract.NotifierID(tt.notifierID)))
		})
	}
}

// =============================================================================
// Notification Sending Tests
// =============================================================================

// TestServiceNotify는 Service의 Notify 메서드를 검증합니다.
func TestServiceNotify(t *testing.T) {
	tests := []struct {
		name           string
		notifierID     string
		message        string
		isError        bool
		expectError    bool
		expectedErrStr string
		expectedMsg    string
		expectedErrCtx bool
	}{
		{
			name:        "성공: 일반 메시지",
			notifierID:  testNotifierID,
			message:     "test msg",
			isError:     false,
			expectError: false,
			expectedMsg: "test msg",
		},
		{
			name:           "성공: 에러 메시지",
			notifierID:     testNotifierID,
			message:        "error msg",
			isError:        true,
			expectError:    false,
			expectedMsg:    "error msg",
			expectedErrCtx: true,
		},
		{
			name:           "실패: 존재하지 않는 Notifier (기본 Notifier로 폴백)",
			notifierID:     "unknown",
			message:        "msg",
			expectError:    true,
			expectedErrStr: notifier.ErrNotFoundNotifier.Error(),
			expectedMsg:    "등록되지 않은 Notifier ID('unknown')입니다. 메시지 발송이 거부되었습니다. 원본 메시지: msg",
			expectedErrCtx: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockNotifier := setupMockServiceWithOptions(mockServiceOptions{
				notifierID:   testNotifierID,
				supportsHTML: true,
				running:      true,
			})

			err := service.NotifyWithTitle(contract.NotifierID(tt.notifierID), "title", tt.message, tt.isError)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrStr != "" {
					assert.Contains(t, err.Error(), tt.expectedErrStr)
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.expectedErrCtx {
				assertNotifyCalledWithContext(t, mockNotifier, tt.expectedMsg)
			} else if tt.expectedMsg != "" {
				assertNotifyCalled(t, mockNotifier, tt.expectedMsg)
			}
		})
	}
}

// (TestNotifyWithTitle transferred to interface_test.go)

// (TestNotifyDefault transferred to interface_test.go)

// TestNotify_NotRunning은 서비스가 실행 중이 아닐 때의 동작을 검증합니다.
func TestNotify_NotRunning(t *testing.T) {
	service, _, mockNotifier := setupMockServiceWithOptions(mockServiceOptions{
		notifierID:   testNotifierID,
		supportsHTML: true,
		running:      false, // 실행 중이 아님
	})

	err := service.Notify(contract.NewTaskContext(), contract.NotifierID(testNotifierID), "test")

	assert.Error(t, err)
	assert.Equal(t, notifier.ErrServiceStopped, err)
	assertNotifyNotCalled(t, mockNotifier)
}

// (TestNotifyDefault_NilNotifier transferred to interface_test.go)

// =============================================================================
// Multiple Notifiers Tests
// =============================================================================

// TestMultipleNotifiers는 여러 Notifier 처리를 검증합니다.
func TestMultipleNotifiers(t *testing.T) {
	mockNotifier1 := &notificationmocks.MockNotifierHandler{IDValue: "n1", SupportsHTMLValue: true}
	mockNotifier2 := &notificationmocks.MockNotifierHandler{IDValue: "n2", SupportsHTMLValue: false}

	service := &Service{
		notifiersMap: map[contract.NotifierID]notifier.NotifierHandler{
			mockNotifier1.IDValue: mockNotifier1,
			mockNotifier2.IDValue: mockNotifier2,
		},
		running: true,
	}

	// n2로 전송
	err := service.Notify(contract.NewTaskContext(), contract.NotifierID("n2"), "msg")
	assert.NoError(t, err)
	assertNotifyNotCalled(t, mockNotifier1)
	require.Len(t, mockNotifier2.NotifyCalls, 1)
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestConcurrencyStress는 고부하 상황에서의 동시성 안전성을 검증합니다.
func TestConcurrencyStress(t *testing.T) {
	mockNotifier := &notificationmocks.MockNotifierHandler{
		IDValue:           testNotifierID,
		SupportsHTMLValue: true,
	}

	service := &Service{
		appConfig: &config.AppConfig{
			Notifier: config.NotifierConfig{
				DefaultNotifierID: testNotifierID,
			},
		},
		notifiersMap: map[contract.NotifierID]notifier.NotifierHandler{
			mockNotifier.IDValue: mockNotifier,
		},
		defaultNotifier: mockNotifier,
		running:         true,
	}

	concurrency := 100
	wg := sync.WaitGroup{}
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			service.NotifyWithTitle(contract.NotifierID(testNotifierID), "title", "stress test", false)
			service.NotifyDefault("default stress")
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 성공
	case <-time.After(5 * time.Second):
		t.Fatal("Deadlock 감지 또는 타임아웃 발생")
	}

	assert.Greater(t, len(mockNotifier.NotifyCalls), 0)
}

// =============================================================================
// Service Lifecycle Tests
// =============================================================================

// TestStartAndRun은 Service 생명주기를 검증합니다.
func TestStartAndRun(t *testing.T) {
	t.Run("정상 시작 및 종료", func(t *testing.T) {
		service, _, mockNotifier := setupMockService()
		mockNotifier.IDValue = "default"

		cfg := &config.AppConfig{}
		cfg.Notifier.DefaultNotifierID = "default"

		mockFactory := &notificationmocks.MockNotifierFactory{
			CreateNotifiersFunc: func(c *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
				return []notifier.NotifierHandler{mockNotifier}, nil
			},
		}

		// Re-create service with the specific mock factory for this test case
		// Since setupMockService created a service with a default mock factory, we need to override it here.
		// However, it's cleaner to just create a new service with the desired factory.
		service = NewService(cfg, mockFactory, &taskmocks.MockExecutor{})         // Inject mockFactory directly
		service.notifiersMap = map[contract.NotifierID]notifier.NotifierHandler{} // Reset map if needed, though NewService does it.
		// We need to re-apply the mock state if setupMockService did meaningful setup besides creation.
		// setupMockService sets notifiersMap and defaultNotifier, but Start() will overwrite them using the Factory.
		// So for TestStartAndRun, we mainly need the Factory to behave correctly.

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)

		err := service.Start(ctx, wg)
		assert.NoError(t, err)
		assert.True(t, service.running)

		err = service.NotifyDefault("test")
		assert.NoError(t, err)

		cancel()
		wg.Wait()

		assert.False(t, service.running)
		err = service.NotifyDefault("fail")
		assert.Error(t, err)
		assert.Equal(t, notifier.ErrServiceStopped, err)
	})
}

// =============================================================================
// Service Start Error Tests
// =============================================================================

// TestStartErrors는 Service 시작 시 에러 처리를 검증합니다.
func TestStartErrors(t *testing.T) {
	tests := []struct {
		name          string
		cfgSetup      func(*config.AppConfig)
		factorySetup  func(*mocks.MockNotifierFactory)
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
			factorySetup: func(m *mocks.MockNotifierFactory) {
				m.CreateNotifiersFunc = func(c *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
					return nil, errors.New("factory error")
				}
			},
			errorContains: "Notifier 초기화 중 에러가 발생했습니다",
		},
		{
			name: "기본 Notifier를 찾을 수 없음",
			cfgSetup: func(c *config.AppConfig) {
				c.Notifier.DefaultNotifierID = "def"
			},
			factorySetup: func(m *mocks.MockNotifierFactory) {
				m.CreateNotifiersFunc = func(c *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
					return []notifier.NotifierHandler{
						&notificationmocks.MockNotifierHandler{IDValue: "other"},
					}, nil
				}
			},
			errorContains: "기본 NotifierID('def')를 찾을 수 없습니다",
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

			factory := &notificationmocks.MockNotifierFactory{}
			if tt.factorySetup != nil {
				tt.factorySetup(factory)
			} else {
				factory.CreateNotifiersFunc = func(c *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
					return []notifier.NotifierHandler{}, nil
				}
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

// =============================================================================
// Improvement Tests (Merged from service_improvement_test.go)
// =============================================================================

// localMockFactory for creating duplicate notifiers in tests
type localMockFactory struct {
	handlers []notifier.NotifierHandler
}

func (m *localMockFactory) CreateNotifiers(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
	return m.handlers, nil
}

func TestService_Start_DuplicateID(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "test-notifier",
		},
	}
	executor := &taskmocks.MockExecutor{}

	// Create 2 notifiers with SAME ID
	h1 := &notificationmocks.MockNotifierHandler{IDValue: "duplicate-id"} // Changed to notificationmocks
	h2 := &notificationmocks.MockNotifierHandler{IDValue: "duplicate-id"} // Changed to notificationmocks

	mf := &localMockFactory{handlers: []notifier.NotifierHandler{h1, h2}}

	service := NewService(cfg, mf, executor) // Changed to NewService

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

// controllableMockHandler to control Notify return value
type controllableMockHandler struct {
	notificationmocks.MockNotifierHandler // Changed to notificationmocks
	notifyResult                          bool
}

func (m *controllableMockHandler) Notify(taskCtx contract.TaskContext, message string) bool {
	return m.notifyResult
}

func TestService_Notify_StoppedNotifier(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "test-notifier",
		},
	}
	executor := &taskmocks.MockExecutor{}

	// Setup a notifier with closed Done channel and Notify returning false
	closedCh := make(chan struct{})
	close(closedCh)

	h := &controllableMockHandler{
		MockNotifierHandler: notificationmocks.MockNotifierHandler{ // Changed to notificationmocks
			IDValue:     "test-notifier",
			DoneChannel: closedCh,
		},
		notifyResult: false, // Simulate full queue/failure
	}

	mf := &localMockFactory{handlers: []notifier.NotifierHandler{h}}
	service := NewService(cfg, mf, executor) // Changed to NewService

	// Start service
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Start normally
	err := service.Start(ctx, wg)
	assert.NoError(t, err)

	// Action
	notifyErr := service.Notify(contract.NewTaskContext(), "test-notifier", "hello")

	// Assert
	// Should return ErrServiceStopped because Done() is closed
	assert.ErrorIs(t, notifyErr, notifier.ErrServiceStopped)

	// Cleanup
	cancel()
	wg.Wait()
}

// =============================================================================
// Panic Recovery Tests (Merged from service_panic_test.go)
// =============================================================================

// PanicMockNotifierHandler Run 메서드에서 패닉을 발생시키는 Mock Notifier
type PanicMockNotifierHandler struct {
	notificationmocks.MockNotifierHandler // Changed to notificationmocks
	PanicOnRun                            bool
}

func (m *PanicMockNotifierHandler) Run(ctx context.Context) {
	if m.PanicOnRun {
		panic("Simulated Panic in Notifier Run")
	}
	m.MockNotifierHandler.Run(ctx)
}

func TestService_Start_PanicRecovery(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "normal_notifier",
		},
	}
	executor := &taskmocks.MockExecutor{}

	// 패닉을 발생시키는 Notifier와 정상적인 Notifier 준비
	panicNotifier := &PanicMockNotifierHandler{
		MockNotifierHandler: notificationmocks.MockNotifierHandler{ // Changed to notificationmocks
			IDValue: "panic_notifier",
		},
		PanicOnRun: true,
	}
	normalNotifier := &notificationmocks.MockNotifierHandler{ // Changed to notificationmocks
		IDValue: "normal_notifier",
	}

	factory := &notificationmocks.MockNotifierFactory{ // Changed to notificationmocks
		CreateNotifiersFunc: func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.NotifierHandler, error) {
			return []notifier.NotifierHandler{panicNotifier, normalNotifier}, nil
		},
	}

	service := NewService(cfg, factory, executor) // Changed to NewService

	// Test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Service Termination WaitGroup (Service Start uses this to signal service stop)
	serviceStopWG := &sync.WaitGroup{}
	serviceStopWG.Add(1)

	// Start Service
	err := service.Start(ctx, serviceStopWG)
	assert.NoError(t, err)

	// service.Start launches goroutines for notifiers.
	// One of them will panic immediately.
	// We wait a bit to ensure panic happens and is recovered.
	time.Sleep(100 * time.Millisecond)

	// Verify Service is still running
	assert.NoError(t, service.Health())

	// Terminate Service
	cancel()
	serviceStopWG.Wait()
}
