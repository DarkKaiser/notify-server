package notification

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	notifierID   NotifierID
	supportsHTML bool
	running      bool
}

// setupMockService는 기본 설정으로 테스트용 Service와 Mock 객체를 생성합니다.
func setupMockService() (*Service, *MockExecutor, *mockNotifierHandler) {
	return setupMockServiceWithOptions(mockServiceOptions{
		notifierID:   testNotifierID,
		supportsHTML: true,
		running:      false,
	})
}

// setupMockServiceWithOptions는 옵션을 받아 테스트용 Service와 Mock 객체를 생성합니다.
func setupMockServiceWithOptions(opts mockServiceOptions) (*Service, *MockExecutor, *mockNotifierHandler) {
	appConfig := &config.AppConfig{}
	mockExecutor := &MockExecutor{}
	mockNotifier := &mockNotifierHandler{
		id:           opts.notifierID,
		supportsHTML: opts.supportsHTML,
	}

	service := NewService(appConfig, mockExecutor)
	service.notifiers = []NotifierHandler{mockNotifier}
	service.defaultNotifier = mockNotifier
	service.running = opts.running

	return service, mockExecutor, mockNotifier
}

// assertNotifyCalled는 mockNotifier가 정확히 한 번 호출되었고 메시지가 일치하는지 검증합니다.
func assertNotifyCalled(t *testing.T, mock *mockNotifierHandler, expectedMsg string) {
	t.Helper()
	require.Len(t, mock.notifyCalls, 1, "Expected exactly one notify call")
	assert.Equal(t, expectedMsg, mock.notifyCalls[0].message, "Message should match")
}

// assertNotifyCalledWithContext는 mockNotifier가 호출되었고 TaskContext가 있는지 검증합니다.
func assertNotifyCalledWithContext(t *testing.T, mock *mockNotifierHandler, expectedMsg string) {
	t.Helper()
	assertNotifyCalled(t, mock, expectedMsg)
	assert.NotNil(t, mock.notifyCalls[0].taskCtx, "TaskContext should be present")
}

// assertNotifyNotCalled는 mockNotifier가 호출되지 않았는지 검증합니다.
func assertNotifyNotCalled(t *testing.T, mock *mockNotifierHandler) {
	t.Helper()
	assert.Empty(t, mock.notifyCalls, "Expected no notify calls")
}

// =============================================================================
// Service Initialization Tests
// =============================================================================

// TestNotificationService_NewService는 Service 생성을 검증합니다.
//
// 검증 항목:
//   - Service 인스턴스 생성
//   - 필드 초기화 (appConfig, executor, running, notifiersStopWG)
func TestNotificationService_NewService(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockExecutor := &MockExecutor{}
	service := NewService(appConfig, mockExecutor)

	assert.NotNil(t, service)
	assert.Equal(t, appConfig, service.appConfig)
	assert.Equal(t, mockExecutor, service.executor)
	assert.False(t, service.running)
	assert.NotNil(t, service.notifiersStopWG)
}

// =============================================================================
// HTML Support Tests
// =============================================================================

// TestNotificationService_SupportsHTML은 HTML 지원 여부 확인을 검증합니다.
//
// 검증 항목:
//   - 존재하는 Notifier의 HTML 지원 여부
//   - 존재하지 않는 Notifier의 처리
func TestNotificationService_SupportsHTML(t *testing.T) {
	mockNotifier := &mockNotifierHandler{id: "test", supportsHTML: true}
	service := &Service{notifiers: []NotifierHandler{mockNotifier}}

	tests := []struct {
		name       string
		notifierID string
		want       bool
	}{
		{"Existing notifier supporting HTML", "test", true},
		{"Non-existent notifier", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, service.SupportsHTML(tt.notifierID))
		})
	}
}

// =============================================================================
// Notification Sending Tests
// =============================================================================

// TestNotificationService_Notify_Table은 알림 발송 기능을 검증합니다.
//
// 검증 항목:
//   - 정상 메시지 발송
//   - 에러 메시지 발송
//   - 존재하지 않는 Notifier 처리 (기본 Notifier로 폴백)
func TestNotificationService_Notify_Table(t *testing.T) {
	tests := []struct {
		name           string
		notifierID     string
		message        string
		isError        bool
		expectSuccess  bool
		expectedMsg    string
		expectedErrCtx bool
	}{
		{
			name:          "Success normal message",
			notifierID:    testNotifierID,
			message:       "test msg",
			isError:       false,
			expectSuccess: true,
			expectedMsg:   "test msg",
		},
		{
			name:           "Success error message",
			notifierID:     testNotifierID,
			message:        "error msg",
			isError:        true,
			expectSuccess:  true,
			expectedMsg:    "error msg",
			expectedErrCtx: true,
		},
		{
			name:           "Unknown notifier (fallback to default)",
			notifierID:     "unknown",
			message:        "msg",
			expectSuccess:  false,
			expectedMsg:    "알 수 없는 Notifier('unknown')입니다. 알림메시지 발송이 실패하였습니다.(Message:msg)",
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

			result := service.NotifyWithTitle(tt.notifierID, "title", tt.message, tt.isError)

			assert.Equal(t, tt.expectSuccess, result)
			if tt.expectedErrCtx {
				assertNotifyCalledWithContext(t, mockNotifier, tt.expectedMsg)
			} else if tt.expectedMsg != "" {
				assertNotifyCalled(t, mockNotifier, tt.expectedMsg)
			}
		})
	}
}

// TestNotificationService_NotifyMethods_Table은 다양한 알림 메서드를 검증합니다.
//
// 검증 항목:
//   - NotifyDefault
//   - NotifyDefaultWithError
//   - NotifyWithTaskContext
func TestNotificationService_NotifyMethods_Table(t *testing.T) {
	tests := []struct {
		name            string
		method          string // "Default", "DefaultError", "WithContext"
		targetID        string // Used for WithContext
		message         string
		expectSuccess   bool
		isError         bool // Added missing isError field
		expectedCall    bool // on the targeted notifier
		expectedDefCall bool // on default notifier (fallback or direct)
	}{
		{
			name:            "NotifyDefault Success",
			method:          "Default",
			message:         "msg",
			expectSuccess:   true,
			expectedDefCall: true,
		},
		{
			name:            "NotifyWithErrorToDefault Success",
			method:          "DefaultError",
			message:         "errorMsg",
			expectSuccess:   true,
			isError:         true, // Should be true for NotifyDefaultWithError
			expectedCall:    true,
			expectedDefCall: true,
		},
		{
			name:            "NotifyWithTaskContext Success",
			method:          "WithContext",
			targetID:        defaultNotifierID,
			message:         "ctx msg",
			expectSuccess:   true,
			expectedDefCall: true,
		},
		{
			name:            "NotifyWithTaskContext Unknown Notifier",
			method:          "WithContext",
			targetID:        "unknown",
			message:         "fail msg",
			expectSuccess:   false,
			expectedCall:    false,
			expectedDefCall: true, // Should fall back to default for error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockNotifier := setupMockServiceWithOptions(mockServiceOptions{
				notifierID:   defaultNotifierID,
				supportsHTML: true,
				running:      true,
			})

			var result bool
			switch tt.method {
			case "Default":
				result = service.NotifyDefault(tt.message)
			case "DefaultError":
				result = service.NotifyDefaultWithError(tt.message)
			case "WithContext":
				result = service.Notify(task.NewTaskContext(), tt.targetID, tt.message)
			}

			assert.Equal(t, tt.expectSuccess, result)

			if tt.expectedDefCall {
				require.NotEmpty(t, mockNotifier.notifyCalls, "Expected at least one notify call")
				lastCall := mockNotifier.notifyCalls[len(mockNotifier.notifyCalls)-1]
				if !tt.expectSuccess && tt.method == "WithContext" {
					assert.Contains(t, lastCall.message, "알 수 없는 Notifier")
				} else {
					assert.Equal(t, tt.message, lastCall.message)
				}
			} else {
				assertNotifyNotCalled(t, mockNotifier)
			}
		})
	}
}

// TestNotificationService_MultipleNotifiers는 여러 Notifier 처리를 검증합니다.
//
// 검증 항목:
//   - 특정 Notifier로 메시지 발송
//   - 다른 Notifier는 호출되지 않음
func TestNotificationService_MultipleNotifiers(t *testing.T) {
	mockNotifier1 := &mockNotifierHandler{id: "n1", supportsHTML: true}
	mockNotifier2 := &mockNotifierHandler{id: "n2", supportsHTML: false}

	service := &Service{
		notifiers: []NotifierHandler{mockNotifier1, mockNotifier2},
		running:   true,
	}

	// Notify n2
	result := service.Notify(task.NewTaskContext(), "n2", "msg")
	assert.True(t, result)
	assertNotifyNotCalled(t, mockNotifier1)
	require.Len(t, mockNotifier2.notifyCalls, 1)
}

// =============================================================================
// Service Lifecycle Tests
// =============================================================================

// TestNotificationService_StartAndRun은 Service 생명주기를 검증합니다.
//
// 검증 항목:
//   - Service 시작
//   - 알림 발송 동작
//   - Service 종료
func TestNotificationService_StartAndRun(t *testing.T) {
	// Consolidating Start/Run tests
	t.Run("Lifecycle", func(t *testing.T) {
		service, _, mockNotifier := setupMockService()
		mockNotifier.id = "default"

		cfg := &config.AppConfig{}
		cfg.Notifier.DefaultNotifierID = "default"

		// Mock factory
		mockFactory := &mockNotifierFactory{
			createNotifiersFunc: func(c *config.AppConfig, executor task.Executor) ([]NotifierHandler, error) {
				return []NotifierHandler{mockNotifier}, nil
			},
		}
		service.SetNotifierFactory(mockFactory)
		service.appConfig = cfg

		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}
		wg.Add(1)

		// Start
		err := service.Start(ctx, wg)
		assert.NoError(t, err)
		assert.True(t, service.running)

		// Verify working
		assert.True(t, service.NotifyDefault("test"))

		// Shutdown
		cancel()
		wg.Wait()

		assert.False(t, service.running)
		assert.False(t, service.NotifyDefault("fail"))
	})
}

// =============================================================================
// Service Start Error Tests
// =============================================================================

// TestNotificationService_StartErrors는 Service 시작 시 에러 처리를 검증합니다.
//
// 검증 항목:
//   - Executor가 nil일 때
//   - Factory에서 에러 반환
//   - 기본 Notifier를 찾을 수 없을 때
func TestNotificationService_StartErrors(t *testing.T) {
	tests := []struct {
		name          string
		cfgSetup      func(*config.AppConfig)
		factorySetup  func(*mockNotifierFactory)
		executorNil   bool
		errorContains string
	}{
		{
			name:          "Executor Nil",
			executorNil:   true,
			errorContains: "Executor 객체가 초기화되지 않았습니다",
		},
		{
			name: "Factory return error",
			factorySetup: func(m *mockNotifierFactory) {
				m.createNotifiersFunc = func(c *config.AppConfig, executor task.Executor) ([]NotifierHandler, error) {
					return nil, errors.New("factory error")
				}
			},
			errorContains: "Notifier 초기화 중 에러가 발생했습니다",
		},
		{
			name: "Missing Default Notifier",
			cfgSetup: func(c *config.AppConfig) {
				c.Notifier.DefaultNotifierID = "def"
			},
			factorySetup: func(m *mockNotifierFactory) {
				m.createNotifiersFunc = func(c *config.AppConfig, executor task.Executor) ([]NotifierHandler, error) {
					return []NotifierHandler{
						&mockNotifierHandler{id: "other"},
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

			var executor task.Executor = &MockExecutor{}
			if tt.executorNil {
				executor = nil
			}

			service := NewService(cfg, executor)

			factory := &mockNotifierFactory{}
			if tt.factorySetup != nil {
				tt.factorySetup(factory)
			} else {
				// Default success factory if not specified but needed
				factory.createNotifiersFunc = func(c *config.AppConfig, executor task.Executor) ([]NotifierHandler, error) {
					return []NotifierHandler{}, nil
				}
			}
			service.SetNotifierFactory(factory)

			ctx := context.Background()
			wg := &sync.WaitGroup{}
			wg.Add(1)

			err := service.Start(ctx, wg)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)

			// If start failed, we might need to manually decrement wg if the service didn't launch the goroutine
			// In Start() implementation, if it fails before go routine, wg is not touched (or should not be)
			// But the test case added 1. Start() is supposed to run goroutine which calls done.
			// If Start returns error, it means goroutine wasn't started usually.
			// So we don't wait for wg in error cases here.
		})
	}
}

// =============================================================================
// Mock Implementations
// =============================================================================
// Moved to mock_test.go: mockNotifierHandler, mockExecutor (already there), mockNotifierFactory
