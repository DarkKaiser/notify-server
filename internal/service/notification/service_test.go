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
func setupMockService() (*Service, *taskmocks.MockExecutor, *notificationmocks.MockNotifier) {
	return setupMockServiceWithOptions(mockServiceOptions{
		notifierID:   testNotifierID,
		supportsHTML: true,
		running:      false,
	})
}

// setupMockServiceWithOptions는 옵션을 받아 테스트용 Service와 Mock 객체를 생성합니다.
func setupMockServiceWithOptions(opts mockServiceOptions) (*Service, *taskmocks.MockExecutor, *notificationmocks.MockNotifier) {
	appConfig := &config.AppConfig{}
	mockExecutor := &taskmocks.MockExecutor{}
	mockNotifier := notificationmocks.NewMockNotifier(opts.notifierID).
		WithSupportsHTML(opts.supportsHTML)

	mockFactory := &notificationmocks.MockFactory{}

	service := NewService(appConfig, mockFactory, mockExecutor)
	service.notifiersMap = map[contract.NotifierID]notifier.Notifier{
		mockNotifier.ID(): mockNotifier,
	}
	service.defaultNotifier = mockNotifier
	service.running = opts.running

	return service, mockExecutor, mockNotifier
}

// assertSendCalled는 mockNotifier가 정확히 한 번 호출되었고 메시지가 일치하는지 검증합니다.
func assertSendCalled(t *testing.T, mock *notificationmocks.MockNotifier, expectedMsg string) {
	t.Helper()
	require.Len(t, mock.SendCalls, 1, "Expected exactly one send call")
	assert.Equal(t, expectedMsg, mock.SendCalls[0].Message, "Message should match")
}

// assertSendCalledWithContext는 mockNotifier가 호출되었고 TaskContext가 있는지 검증합니다.
func assertSendCalledWithContext(t *testing.T, mock *notificationmocks.MockNotifier, expectedMsg string) {
	t.Helper()
	assertSendCalled(t, mock, expectedMsg)
	assert.NotNil(t, mock.SendCalls[0].TaskContext, "TaskContext should be present")
}

// assertSendNotCalled는 mockNotifier가 호출되지 않았는지 검증합니다.
func assertSendNotCalled(t *testing.T, mock *notificationmocks.MockNotifier) {
	t.Helper()
	assert.Empty(t, mock.SendCalls, "Expected no send calls")
}

// =============================================================================
// Service Initialization Tests
// =============================================================================

// TestNewService는 Service 생성을 검증합니다.
func TestNewService(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockExecutor := &taskmocks.MockExecutor{}
	mockFactory := &notificationmocks.MockFactory{}
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
	mockNotifier := notificationmocks.NewMockNotifier("test").WithSupportsHTML(true)
	service := &Service{
		notifiersMap: map[contract.NotifierID]notifier.Notifier{
			mockNotifier.ID(): mockNotifier,
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
			expectedErrStr: ErrNotFoundNotifier.Error(),
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
				assertSendCalledWithContext(t, mockNotifier, tt.expectedMsg)
			} else if tt.expectedMsg != "" {
				assertSendCalled(t, mockNotifier, tt.expectedMsg)
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
	assert.Equal(t, ErrServiceStopped, err)
	assertSendNotCalled(t, mockNotifier)
}

// (TestNotifyDefault_NilNotifier transferred to interface_test.go)

// =============================================================================
// Multiple Notifiers Tests
// =============================================================================

// TestMultipleNotifiers는 여러 Notifier 처리를 검증합니다.
func TestMultipleNotifiers(t *testing.T) {
	mockNotifier1 := notificationmocks.NewMockNotifier("n1").WithSupportsHTML(true)
	mockNotifier2 := notificationmocks.NewMockNotifier("n2").WithSupportsHTML(false)

	service := &Service{
		notifiersMap: map[contract.NotifierID]notifier.Notifier{
			mockNotifier1.ID(): mockNotifier1,
			mockNotifier2.ID(): mockNotifier2,
		},
		running: true,
	}

	// n2로 전송
	err := service.Notify(contract.NewTaskContext(), contract.NotifierID("n2"), "msg")
	assert.NoError(t, err)
	assertSendNotCalled(t, mockNotifier1)
	require.Len(t, mockNotifier2.SendCalls, 1)
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestConcurrencyStress는 고부하 상황에서의 동시성 안전성을 검증합니다.
func TestConcurrencyStress(t *testing.T) {
	mockNotifier := notificationmocks.NewMockNotifier(testNotifierID).
		WithSupportsHTML(true)

	service := &Service{
		appConfig: &config.AppConfig{
			Notifier: config.NotifierConfig{
				DefaultNotifierID: testNotifierID,
			},
		},
		notifiersMap: map[contract.NotifierID]notifier.Notifier{
			mockNotifier.ID(): mockNotifier,
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

	assert.Greater(t, len(mockNotifier.SendCalls), 0)
}

// =============================================================================
// Service Lifecycle Tests
// =============================================================================

// TestStartAndRun은 Service 생명주기를 검증합니다.
func TestStartAndRun(t *testing.T) {
	t.Run("정상 시작 및 종료", func(t *testing.T) {
		service, _, mockNotifier := setupMockService()
		mockNotifier.WithID("default")

		cfg := &config.AppConfig{}
		cfg.Notifier.DefaultNotifierID = "default"

		mockFactory := &notificationmocks.MockFactory{
			CreateAllFunc: func(c *config.AppConfig, executor contract.TaskExecutor) ([]notifier.Notifier, error) {
				return []notifier.Notifier{mockNotifier}, nil
			},
		}

		// Re-create service with the specific mock factory for this test case
		// Since setupMockService created a service with a default mock factory, we need to override it here.
		// However, it's cleaner to just create a new service with the desired factory.
		service = NewService(cfg, mockFactory, &taskmocks.MockExecutor{})  // Inject mockFactory directly
		service.notifiersMap = map[contract.NotifierID]notifier.Notifier{} // Reset map if needed, though NewService does it.
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
		assert.Equal(t, ErrServiceStopped, err)
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
		factorySetup  func(*mocks.MockFactory)
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
			factorySetup: func(m *mocks.MockFactory) {
				m.WithCreateAll(nil, errors.New("factory error"))
			},
			errorContains: "Notifier 초기화 중 에러가 발생했습니다",
		},
		{
			name: "기본 Notifier를 찾을 수 없음",
			cfgSetup: func(c *config.AppConfig) {
				c.Notifier.DefaultNotifierID = "def"
			},
			factorySetup: func(m *mocks.MockFactory) {
				m.WithCreateAll([]notifier.Notifier{
					notificationmocks.NewMockNotifier("other"),
				}, nil)
			},
			errorContains: "기본 NotifierID('def')를 찾을 수 없습니다",
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AppConfig{}
			if tt.cfgSetup != nil {
				tt.cfgSetup(cfg)
			}

			var executor contract.TaskExecutor = &taskmocks.MockExecutor{}
			if tt.executorNil {
				executor = nil
			}

			factory := &notificationmocks.MockFactory{}
			if tt.factorySetup != nil {
				tt.factorySetup(factory)
			} else {
				factory.WithCreateAll([]notifier.Notifier{}, nil)
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
	notifiers []notifier.Notifier
}

func (m *localMockFactory) CreateAll(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.Notifier, error) {
	return m.notifiers, nil
}

func (m *localMockFactory) Register(creator notifier.Creator) {
	// No-op for this test mock
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
	h1 := notificationmocks.NewMockNotifier("duplicate-id")
	h2 := notificationmocks.NewMockNotifier("duplicate-id")

	mf := &localMockFactory{notifiers: []notifier.Notifier{h1, h2}}

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

func TestService_Notify_StoppedNotifier(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "test-notifier",
		},
	}
	executor := &taskmocks.MockExecutor{}

	// Setup a notifier with closed Done channel
	closedCh := make(chan struct{})
	close(closedCh)

	// Use shared MockNotifier with functional injection
	h := notificationmocks.NewMockNotifier("test-notifier").
		WithSendFunc(func(ctx contract.TaskContext, msg string) error {
			return notifier.ErrClosed // Simulate failure if called
		})
	// Manually set DoneChannel since we need to simulate it being closed externally
	h.DoneChannel = closedCh

	mf := &localMockFactory{notifiers: []notifier.Notifier{h}}
	service := NewService(cfg, mf, executor)

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
	// Depending on implementation, if notifier is closed, Service might return ErrServiceStopped
	assert.ErrorIs(t, notifyErr, ErrServiceStopped)

	// Cleanup
	cancel()
	wg.Wait()
}

// =============================================================================
// Panic Recovery Tests (Merged from service_panic_test.go)
// =============================================================================

// TestService_Start_PanicRecovery tests panic recovery in Notifier Run.
func TestService_Start_PanicRecovery(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "normal_notifier",
		},
	}
	executor := &taskmocks.MockExecutor{}

	// Panic Notifier: using WithRunFunc to simulate panic
	panicNotifier := notificationmocks.NewMockNotifier("panic_notifier").
		WithRunFunc(func(ctx context.Context) {
			panic("Simulated Panic in Notifier Run")
		})

	normalNotifier := notificationmocks.NewMockNotifier("normal_notifier")

	factory := &notificationmocks.MockFactory{ // Changed to notificationmocks
		CreateAllFunc: func(cfg *config.AppConfig, executor contract.TaskExecutor) ([]notifier.Notifier, error) {
			return []notifier.Notifier{panicNotifier, normalNotifier}, nil
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

// =============================================================================
// Interface Tests (Moved from interface_test.go)
// =============================================================================

// TestSender_NotifyWithTitle는 Sender.NotifyWithTitle 메서드의 동작을 검증합니다.
// 제목, 메시지, 에러 플래그가 올바르게 전달되는지, TaskContext가 올바르게 생성되는지 확인합니다.
func TestSender_NotifyWithTitle(t *testing.T) {
	tests := []struct {
		name          string
		notifierID    string
		title         string
		message       string
		errorOccurred bool
		expectError   bool
		expectedMsg   string
		setupMock     func(*notificationmocks.MockNotifier)
	}{
		{
			name:          "성공: 일반 알림 전송",
			notifierID:    testNotifierID,
			title:         "Notice",
			message:       "This is a test message.",
			errorOccurred: false,
			expectError:   false,
			expectedMsg:   "This is a test message.",
			setupMock:     nil,
		},
		{
			name:          "성공: 에러 알림 전송 (Error Flag True)",
			notifierID:    testNotifierID,
			title:         "Error Alert",
			message:       "Something went wrong.",
			errorOccurred: true,
			expectError:   false,
			expectedMsg:   "Something went wrong.",
			setupMock: func(m *notificationmocks.MockNotifier) {
				m.WithSendFunc(func(ctx contract.TaskContext, message string) error {
					// We can just rely on the mock's default tracking if we return true?
					// But we want to check `ctx.IsErrorOccurred()` dynamically inside the mock logic?
					// Or just let it run and check later?
					// The test logic asserts `baseMock.NotifyCalls`.
					// So our custom logic is only needed if we want to change RETURN value.
					// This test setupMock was checking `ctx.IsErrorOccurred()`.
					// Actually, the previous code:
					/*
					   m.NotifyFunc = func(taskCtx contract.TaskContext, message string) bool {
					       m.MockNotifier.Notify(taskCtx, message) // Call base to record call
					       return taskCtx.IsErrorOccurred() == true
					   }
					*/
					// The new `Notify` calls `NotifyFunc` arguments BUT also records call BEFORE calling it? No.
					// Let's check `mocks/notifier.go`.
					// `m.NotifyCalls = append(...)` then `if m.NotifyFunc != nil { return m.NotifyFunc(...) }`
					// So call IS recorded automatically.
					// We just need to return the correct bool result.
					if ctx.IsErrorOccurred() {
						return nil
					}
					return nil
				})
			},
		},
		{
			name:        "실패: 존재하지 않는 Notifier",
			notifierID:  "unknown_notifier",
			title:       "Fail",
			message:     "This should fail.",
			expectError: true,
			setupMock:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			service, _, baseMock := setupMockServiceWithOptions(mockServiceOptions{
				notifierID:   testNotifierID,
				supportsHTML: true,
				running:      true,
			})

			// Wrap mock
			// mockNotifier := &FunctionalMock{MockNotifier: baseMock}
			// Use baseMock directly as it now supports WithNotifyFunc
			mockNotifier := baseMock
			if tt.setupMock != nil {
				tt.setupMock(mockNotifier)
			}

			// Replace in service map only if ID matches testNotifierID,
			// otherwise we leave it as is or handle unknown ID case.
			if tt.notifierID == testNotifierID {
				service.notifiersMap[contract.NotifierID(testNotifierID)] = mockNotifier
			}

			// Act
			err := service.NotifyWithTitle(contract.NotifierID(tt.notifierID), tt.title, tt.message, tt.errorOccurred)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.NotEmpty(t, baseMock.SendCalls, "Send should be called on the base mock")
				lastCall := baseMock.SendCalls[len(baseMock.SendCalls)-1]
				assert.Equal(t, tt.expectedMsg, lastCall.Message)

				// TaskContext Verification
				require.NotNil(t, lastCall.TaskContext)
				assert.Equal(t, tt.title, lastCall.TaskContext.GetTitle())
				assert.Equal(t, tt.errorOccurred, lastCall.TaskContext.IsErrorOccurred())
			}
		})
	}
}

// TestSender_NotifyDefault는 Sender.NotifyDefault 메서드의 동작을 검증합니다.
// 기본 Notifier로 메시지가 전달되는지 확인합니다.
func TestSender_NotifyDefault(t *testing.T) {
	tests := []struct {
		name            string
		message         string
		defaultNotifier string
		running         bool
		expectError     bool
		errorIs         error
		setupMock       func(*notificationmocks.MockNotifier)
	}{
		{
			name:            "성공: 기본 Notifier로 전송",
			message:         "Default Message",
			defaultNotifier: defaultNotifierID,
			running:         true,
			expectError:     false,
		},
		{
			name:            "실패: 기본 Notifier 미설정 (Service Stopped 간주)",
			message:         "Fail Message",
			defaultNotifier: "", // 미설정
			running:         true,
			expectError:     true,
			errorIs:         ErrServiceStopped,
		},
		{
			name:            "실패: 메시지 전송 실패 (Queue Full 등)",
			message:         "Fail Message",
			defaultNotifier: defaultNotifierID,
			running:         true,
			expectError:     true,
			setupMock: func(m *notificationmocks.MockNotifier) {
				m.WithSendFunc(func(ctx contract.TaskContext, message string) error {
					return notifier.ErrQueueFull // 실패 시뮬레이션
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			// Use defaultNotifierID if tt.defaultNotifier is set, otherwise use a placeholder that we will clear
			setupID := defaultNotifierID
			if tt.defaultNotifier != "" {
				setupID = tt.defaultNotifier
			}

			opts := mockServiceOptions{
				notifierID:   contract.NotifierID(setupID),
				supportsHTML: true,
				running:      tt.running,
			}

			service, _, baseMock := setupMockServiceWithOptions(opts)

			// mockNotifier := &FunctionalMock{MockNotifier: baseMock}
			mockNotifier := baseMock
			if tt.setupMock != nil {
				tt.setupMock(mockNotifier)
			}

			// Inject wrapped mock if needed
			if service.defaultNotifier != nil {
				// ID가 일치하면 교체 (여기서는 setupMockServiceWithOptions가 생성한 mockNotifier를 사용하므로 항상 일치)
				service.defaultNotifier = mockNotifier
			}

			// If test case expects no default notifier, explicitly set to nil
			if tt.defaultNotifier == "" {
				service.defaultNotifier = nil
			}

			// Act
			err := service.NotifyDefault(tt.message)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorIs != nil {
					assert.ErrorIs(t, err, tt.errorIs)
				}
			} else {
				assert.NoError(t, err)
				assertSendCalled(t, baseMock, tt.message)
			}
		})
	}
}

// TestSender_NotifyDefaultWithError는 Sender.NotifyDefaultWithError 메서드를 검증합니다.
// 에러 플래그가 설정된 채로 기본 Notifier에 전달되는지 확인합니다.
func TestSender_NotifyDefaultWithError(t *testing.T) {
	// Arrange
	service, _, mockNotifier := setupMockServiceWithOptions(mockServiceOptions{
		notifierID:   defaultNotifierID,
		supportsHTML: true,
		running:      true,
	})

	message := "Critical Error"

	// Act
	err := service.NotifyDefaultWithError(message)

	// Assert
	assert.NoError(t, err)
	require.Len(t, mockNotifier.SendCalls, 1)

	call := mockNotifier.SendCalls[0]
	assert.Equal(t, message, call.Message)
	require.NotNil(t, call.TaskContext)
	assert.True(t, call.TaskContext.IsErrorOccurred(), "Error flag should be true")
}

// TestHealthChecker_Health는 HealthChecker.Health 메서드의 동작을 검증합니다.
// 서비스 실행 상태에 따른 반환값을 확인합니다.
func TestHealthChecker_Health(t *testing.T) {
	tests := []struct {
		name        string
		running     bool
		expectError bool
		errorIs     error
	}{
		{
			name:        "정상: 서비스 실행 중",
			running:     true,
			expectError: false,
		},
		{
			name:        "에러: 서비스 중지됨",
			running:     false,
			expectError: true,
			errorIs:     ErrServiceStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			service, _, _ := setupMockServiceWithOptions(mockServiceOptions{
				notifierID:   testNotifierID,
				supportsHTML: true,
				running:      tt.running,
			})

			// Act
			err := service.Health()

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorIs != nil {
					assert.ErrorIs(t, err, tt.errorIs)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSender_Mockability는 Sender 및 HealthChecker 인터페이스가 Mocking 가능한지 시연합니다.
// 이는 "Interface Code" 자체에 대한 테스트의 일환으로 볼 수 있습니다.
type MockSender struct {
	NotifyWithTitleFunc        func(notifierID contract.NotifierID, title string, message string, errorOccurred bool) error
	NotifyDefaultFunc          func(message string) error
	NotifyDefaultWithErrorFunc func(message string) error
	HealthFunc                 func() error // HealthChecker 구현
}

func (m *MockSender) NotifyWithTitle(notifierID contract.NotifierID, title string, message string, errorOccurred bool) error {
	if m.NotifyWithTitleFunc != nil {
		return m.NotifyWithTitleFunc(notifierID, title, message, errorOccurred)
	}
	return nil
}
func (m *MockSender) NotifyDefault(message string) error {
	if m.NotifyDefaultFunc != nil {
		return m.NotifyDefaultFunc(message)
	}
	return nil
}
func (m *MockSender) NotifyDefaultWithError(message string) error {
	if m.NotifyDefaultWithErrorFunc != nil {
		return m.NotifyDefaultWithErrorFunc(message)
	}
	return nil
}
func (m *MockSender) Health() error {
	if m.HealthFunc != nil {
		return m.HealthFunc()
	}
	return nil
}

func TestHealthChecker_Mockability(t *testing.T) {
	// Arrange
	mock := &MockSender{
		HealthFunc: func() error {
			return errors.New("mock error")
		},
	}

	// Act
	var healthChecker contract.NotificationHealthChecker = mock
	err := healthChecker.Health()

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "mock error", err.Error())
}
