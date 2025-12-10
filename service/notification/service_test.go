package notification

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
)

// -- Mocks and Setup Helpers --

// setupMockService creates a service with mocks for testing
func setupMockService() (*NotificationService, *MockExecutor, *mockNotifierHandler) {
	appConfig := &config.AppConfig{}
	mockExecutor := &MockExecutor{}
	mockNotifier := &mockNotifierHandler{
		id:           NotifierID("test-notifier"),
		supportsHTML: true,
	}

	service := NewService(appConfig, mockExecutor)
	service.notifierHandlers = []NotifierHandler{mockNotifier}
	// For convenience in some tests
	service.defaultNotifierHandler = mockNotifier

	return service, mockExecutor, mockNotifier
}

// -- Tests --

func TestNotificationService_SupportsHTML(t *testing.T) {
	mockNotifier := &mockNotifierHandler{id: "test", supportsHTML: true}
	service := &NotificationService{notifierHandlers: []NotifierHandler{mockNotifier}}

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

func TestNotificationService_NewService(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockExecutor := &MockExecutor{}
	service := NewService(appConfig, mockExecutor)

	assert.NotNil(t, service)
	assert.Equal(t, appConfig, service.appConfig)
	assert.Equal(t, mockExecutor, service.executor)
	assert.False(t, service.running)
	assert.NotNil(t, service.notificationStopWaiter)
}

func TestNotificationService_Notify_Table(t *testing.T) {
	tests := []struct {
		name           string
		notifierID     string
		message        string
		isError        bool
		mockSetup      func(*mockNotifierHandler)
		expectSuccess  bool
		expectedCall   bool
		expectedMsg    string
		expectedErrCtx bool
	}{
		{
			name:       "Success normal message",
			notifierID: "test-notifier",
			message:    "test msg",
			isError:    false,
			mockSetup: func(m *mockNotifierHandler) {
				// No special setup needed for simple success in our mock
			},
			expectSuccess: true,
			expectedCall:  true,
			expectedMsg:   "test msg",
		},
		{
			name:           "Success error message",
			notifierID:     "test-notifier",
			message:        "error msg",
			isError:        true,
			mockSetup:      func(m *mockNotifierHandler) {},
			expectSuccess:  true, // Notify return value
			expectedCall:   true,
			expectedMsg:    "error msg",
			expectedErrCtx: true,
		},

		{
			name:           "Unknown notifier (fallback to default)",
			notifierID:     "unknown",
			message:        "msg",
			expectSuccess:  false,
			expectedCall:   true, // Falls back to default
			expectedMsg:    "알 수 없는 Notifier('unknown')입니다. 알림메시지 발송이 실패하였습니다.(Message:msg)",
			expectedErrCtx: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockNotifier := setupMockService()
			// Override running state to allow notification
			service.running = true

			if tt.mockSetup != nil {
				tt.mockSetup(mockNotifier)
			}

			result := service.NotifyWithTitle(tt.notifierID, "title", tt.message, tt.isError)

			assert.Equal(t, tt.expectSuccess, result)
			if tt.expectedCall {
				assert.Len(t, mockNotifier.notifyCalls, 1)
				assert.Equal(t, tt.expectedMsg, mockNotifier.notifyCalls[0].message)
				if tt.expectedErrCtx {
					assert.NotNil(t, mockNotifier.notifyCalls[0].taskCtx)
					// Verify error context if we had a way to check deeper,
					// but existence is what we check for now based on original test
				}
			} else {
				assert.Len(t, mockNotifier.notifyCalls, 0)
			}
		})
	}
}

func TestNotificationService_NotifyMethods_Table(t *testing.T) {
	// Tests for NotifyDefault, NotifyDefaultWithError, NotifyWithTaskContext

	defaultID := "default-notifier"

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
			targetID:        defaultID,
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
			service, _, mockNotifier := setupMockService()
			service.running = true

			// Setup default notifier specifically with correct ID
			mockNotifier.id = NotifierID(defaultID)
			service.defaultNotifierHandler = mockNotifier
			service.notifierHandlers = []NotifierHandler{mockNotifier}

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
				assert.GreaterOrEqual(t, len(mockNotifier.notifyCalls), 1)
				// Check content of last call
				lastCall := mockNotifier.notifyCalls[len(mockNotifier.notifyCalls)-1]
				if !tt.expectSuccess && tt.method == "WithContext" {
					// Fallback error message usually contains info about failure
					assert.Contains(t, lastCall.message, "알 수 없는 Notifier")
				} else {
					assert.Equal(t, tt.message, lastCall.message)
				}
			} else {
				assert.Len(t, mockNotifier.notifyCalls, 0)
			}
		})
	}
}

func TestNotificationService_MultipleNotifiers(t *testing.T) {
	mockNotifier1 := &mockNotifierHandler{id: "n1", supportsHTML: true}
	mockNotifier2 := &mockNotifierHandler{id: "n2", supportsHTML: false}

	service := &NotificationService{
		notifierHandlers: []NotifierHandler{mockNotifier1, mockNotifier2},
		running:          true,
	}

	// Notify n2
	result := service.Notify(task.NewTaskContext(), "n2", "msg")
	assert.True(t, result)
	assert.Len(t, mockNotifier1.notifyCalls, 0)
	assert.Len(t, mockNotifier2.notifyCalls, 1)
}

func TestNotificationService_StartAndRun(t *testing.T) {
	// Consolidating Start/Run tests
	t.Run("Lifecycle", func(t *testing.T) {
		service, _, mockNotifier := setupMockService()
		mockNotifier.id = "default"

		cfg := &config.AppConfig{}
		cfg.Notifiers.DefaultNotifierID = "default"

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
				c.Notifiers.DefaultNotifierID = "def"
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

// -- Mocks Implementation (Local to package test) --
// Moved to mock_test.go: mockNotifierHandler, mockExecutor (already there), mockNotifierFactory
