package notification

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sender Compliance Check
var _ notifier.Sender = (*Service)(nil)

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
	notifierID   notifier.NotifierID
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

	service := NewService(appConfig, mockExecutor, mockFactory)
	service.notifiersMap = map[notifier.NotifierID]notifier.NotifierHandler{
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
	service := NewService(appConfig, mockExecutor, mockFactory)

	assert.NotNil(t, service)
	assert.Equal(t, appConfig, service.appConfig)
	assert.Equal(t, mockExecutor, service.executor)
	assert.False(t, service.running)
	assert.NotNil(t, service.notifiersStopWG)
	assert.NotNil(t, service.notifierFactory)
}

// =============================================================================
// HTML Support Tests
// =============================================================================

// TestSupportsHTML은 HTML 지원 여부 확인을 검증합니다.
func TestSupportsHTML(t *testing.T) {
	mockNotifier := &notificationmocks.MockNotifierHandler{IDValue: "test", SupportsHTMLValue: true}
	service := &Service{
		notifiersMap: map[notifier.NotifierID]notifier.NotifierHandler{
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
			assert.Equal(t, tt.want, service.SupportsHTML(tt.notifierID))
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

			err := service.NotifyWithTitle(tt.notifierID, "title", tt.message, tt.isError)

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

// TestNotifyWithTitle는 NotifyWithTitle 메서드를 검증합니다.
func TestNotifyWithTitle(t *testing.T) {
	tests := []struct {
		name          string
		title         string
		message       string
		errorOccurred bool
		expectError   bool
	}{
		{
			name:          "성공: 일반 알림",
			title:         "Test Title",
			message:       "Test Message",
			errorOccurred: false,
			expectError:   false,
		},
		{
			name:          "성공: 에러 알림",
			title:         "Error Title",
			message:       "Error Message",
			errorOccurred: true,
			expectError:   false,
		},
		{
			name:          "성공: 빈 제목",
			title:         "",
			message:       "Message",
			errorOccurred: false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockNotifier := setupMockServiceWithOptions(mockServiceOptions{
				notifierID:   testNotifierID,
				supportsHTML: true,
				running:      true,
			})

			err := service.NotifyWithTitle(testNotifierID, tt.title, tt.message, tt.errorOccurred)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			require.Len(t, mockNotifier.NotifyCalls, 1)
			assert.Equal(t, tt.message, mockNotifier.NotifyCalls[0].Message)

			// TaskContext 검증
			ctx := mockNotifier.NotifyCalls[0].TaskCtx
			require.NotNil(t, ctx)
			if tt.errorOccurred {
				assert.True(t, ctx.IsErrorOccurred())
			}
		})
	}
}

// TestNotifyDefault는 기본 알림 메서드들을 검증합니다.
func TestNotifyDefault(t *testing.T) {
	tests := []struct {
		name            string
		method          string // "Default", "DefaultError"
		message         string
		expectError     bool
		expectedDefCall bool
	}{
		{
			name:            "NotifyDefault 성공",
			method:          "Default",
			message:         "msg",
			expectError:     false,
			expectedDefCall: true,
		},
		{
			name:            "NotifyDefaultWithError 성공",
			method:          "DefaultError",
			message:         "errorMsg",
			expectError:     false,
			expectedDefCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockNotifier := setupMockServiceWithOptions(mockServiceOptions{
				notifierID:   defaultNotifierID,
				supportsHTML: true,
				running:      true,
			})

			var err error
			switch tt.method {
			case "Default":
				err = service.NotifyDefault(tt.message)
			case "DefaultError":
				err = service.NotifyDefaultWithError(tt.message)
			}

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectedDefCall {
				require.NotEmpty(t, mockNotifier.NotifyCalls)
				lastCall := mockNotifier.NotifyCalls[len(mockNotifier.NotifyCalls)-1]
				assert.Equal(t, tt.message, lastCall.Message)
			} else {
				assertNotifyNotCalled(t, mockNotifier)
			}
		})
	}
}

// TestNotify_NotRunning은 서비스가 실행 중이 아닐 때의 동작을 검증합니다.
func TestNotify_NotRunning(t *testing.T) {
	service, _, mockNotifier := setupMockServiceWithOptions(mockServiceOptions{
		notifierID:   testNotifierID,
		supportsHTML: true,
		running:      false, // 실행 중이 아님
	})

	err := service.Notify(task.NewTaskContext(), testNotifierID, "test")

	assert.Error(t, err)
	assert.Equal(t, notifier.ErrServiceStopped, err)
	assertNotifyNotCalled(t, mockNotifier)
}

// TestNotifyDefault_NilNotifier는 defaultNotifier가 nil일 때의 동작을 검증합니다.
func TestNotifyDefault_NilNotifier(t *testing.T) {
	service := &Service{
		defaultNotifier: nil,
		running:         true,
	}

	err := service.NotifyDefault("test")

	assert.Error(t, err)
	assert.Equal(t, notifier.ErrServiceStopped, err)
}

// =============================================================================
// Multiple Notifiers Tests
// =============================================================================

// TestMultipleNotifiers는 여러 Notifier 처리를 검증합니다.
func TestMultipleNotifiers(t *testing.T) {
	mockNotifier1 := &notificationmocks.MockNotifierHandler{IDValue: "n1", SupportsHTMLValue: true}
	mockNotifier2 := &notificationmocks.MockNotifierHandler{IDValue: "n2", SupportsHTMLValue: false}

	service := &Service{
		notifiersMap: map[notifier.NotifierID]notifier.NotifierHandler{
			mockNotifier1.IDValue: mockNotifier1,
			mockNotifier2.IDValue: mockNotifier2,
		},
		running: true,
	}

	// n2로 전송
	err := service.Notify(task.NewTaskContext(), "n2", "msg")
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
		notifiersMap: map[notifier.NotifierID]notifier.NotifierHandler{
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
			service.NotifyWithTitle(testNotifierID, "title", "stress test", false)
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
			CreateNotifiersFunc: func(c *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
				return []notifier.NotifierHandler{mockNotifier}, nil
			},
		}
		service.SetNotifierFactory(mockFactory)
		service.appConfig = cfg

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
				m.CreateNotifiersFunc = func(c *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
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
				m.CreateNotifiersFunc = func(c *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
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

			var executor task.Executor = &taskmocks.MockExecutor{}
			if tt.executorNil {
				executor = nil
			}

			factory := &notificationmocks.MockNotifierFactory{}
			if tt.factorySetup != nil {
				tt.factorySetup(factory)
			} else {
				factory.CreateNotifiersFunc = func(c *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
					return []notifier.NotifierHandler{}, nil
				}
			}

			service := NewService(cfg, executor, factory)

			ctx := context.Background()
			wg := &sync.WaitGroup{}
			wg.Add(1)

			err := service.Start(ctx, wg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}
