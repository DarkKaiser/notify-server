package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/pkg/version"
	"github.com/darkkaiser/notify-server/internal/service/api"
	v1request "github.com/darkkaiser/notify-server/internal/service/api/v1/model/request"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Integration Test Suite & Helpers
// =============================================================================

type IntegrationTestSuite struct {
	t                   *testing.T
	appConfig           *config.AppConfig
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  *sync.WaitGroup
	taskService         *task.Service
	notificationService *notification.Service
	apiService          *api.Service
	mockSender          *mockNotificationSender
	apiPort             int
}

func setupIntegrationTest(t *testing.T) *IntegrationTestSuite {
	// 1. Config Setup (랜덤 포트 또는 테스트 포트 사용)
	// 포트 0을 사용하면 OS가 가용 포트를 할당하지만, api.Service에서 할당된 포트를 알 방법이 없음 (Start 후 로그 외엔).
	// 따라서 충돌 가능성이 낮고 제어 가능한 포트를 사용하거나, 포트 재사용 옵션을 믿고 진행.
	// 여기서는 테스트 편의상 18080 ~ 18090 사이 포트를 사용하거나 고정 포트 사용.
	// *주의*: CI 환경 등에서 병렬 실행 시 충돌 가능. 실제 프로덕션급 테스트에선 랜덤 포트 할당 후 바인딩된 포트를 조회하는 기능이 api.Service에 필요함.
	// 현재 api.Service 수정 불가 제약이 있다면 고정 포트 사용.
	apiPort := 18088

	appConfig := &config.AppConfig{
		Debug: true,
		NotifyAPI: config.NotifyAPIConfig{
			WS: config.WSConfig{
				ListenPort: apiPort,
			},
			Applications: []config.ApplicationConfig{
				{
					ID:                "test-app",
					Title:             "Test Application",
					DefaultNotifierID: "test-notifier",
					AppKey:            "valid-key",
				},
			},
		},
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "test-notifier",
			Telegrams: []config.TelegramConfig{
				{ID: "test-notifier", BotToken: "token", ChatID: 12345},
			},
		},
	}

	// 2. Mock Setup
	mockSender := &mockNotificationSender{
		notifyCalls: make([]notifyCall, 0),
	}

	// 3. Service Creation
	taskService := task.NewService(appConfig)

	// Notification Service needs a factory that returns our mock handler
	mockFactory := (&notificationmocks.MockFactory{}).WithCreateNotifiers([]notifier.NotifierHandler{
		&mockNotifierHandler{id: contract.NotifierID("test-notifier"), supportsHTML: true},
	}, nil)
	notificationService := notification.NewService(appConfig, mockFactory, taskService)

	// Inject Mock Sender to TaskService effectively bridging the loop for verification
	// *중요*: 실제로는 NotificationService가 Sender이지만,
	// 우리는 NotificationService가 *실제로* 메시지를 보내는 'Notifier' 부분을 Mocking하거나,
	// TaskService -> NotificationService -> (Notifier) -> Telegram API 호출 과정을 검증해야 함.
	// 현재 mockSender는 `notification.Sender` 인터페이스를 구현함.
	// TaskService는 `notification.Sender`를 사용함.
	// 통합 테스트에서는 TaskService가 *진짜* NotificationService를 사용하게 하고,
	// NotificationService가 *가짜* Notifier(Telegram 등)를 사용하게 하는 것이 나음.
	// 위에서 MockFactory를 통해 NotificationService에 가짜 Notifier를 주입했으므로,
	// TaskService에는 *진짜* NotificationService를 연결해야 함.
	taskService.SetNotificationSender(notificationService)

	apiService := api.NewService(appConfig, notificationService, version.Info{Version: "test"})

	// 4. Start Services
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	wg.Add(3)
	go taskService.Start(ctx, wg)
	go notificationService.Start(ctx, wg)
	go apiService.Start(ctx, wg)

	// Wait for services to be ready (naive sleep)
	time.Sleep(200 * time.Millisecond)

	return &IntegrationTestSuite{
		t:                   t,
		appConfig:           appConfig,
		ctx:                 ctx,
		cancel:              cancel,
		wg:                  wg,
		taskService:         taskService,
		notificationService: notificationService,
		apiService:          apiService,
		mockSender:          mockSender, // This might not be populated if we use NotificationService directly.
		// But we need a way to verify *final* delivery.
		// The mockNotifierHandler (in factory) serves this purpose.
		apiPort: apiPort,
	}
}

func (s *IntegrationTestSuite) Teardown() {
	s.cancel()
	s.wg.Wait()
}

// =============================================================================
// Mock Definitions
// =============================================================================

type mockNotifierHandler struct {
	id           contract.NotifierID
	supportsHTML bool
	mu           sync.Mutex
	calls        []string
}

func (m *mockNotifierHandler) ID() contract.NotifierID { return m.id }
func (m *mockNotifierHandler) SupportsHTML() bool      { return m.supportsHTML }
func (m *mockNotifierHandler) Run(ctx context.Context) { <-ctx.Done() }
func (m *mockNotifierHandler) Notify(ctx contract.TaskContext, msg string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, msg)
	return true
}
func (m *mockNotifierHandler) Done() <-chan struct{} { return nil }

func (m *mockNotifierHandler) GetCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// mockNotificationSender is kept for tests that want to bypass NotificationService
type mockNotificationSender struct {
	mu          sync.Mutex
	notifyCalls []notifyCall
}
type notifyCall struct {
	notifierID string
	message    string
}

func (m *mockNotificationSender) NotifyDefault(msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyCalls = append(m.notifyCalls, notifyCall{"default", msg})
	return nil
}
func (m *mockNotificationSender) Notify(ctx contract.TaskContext, nid contract.NotifierID, msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyCalls = append(m.notifyCalls, notifyCall{string(nid), msg})
	return nil
}
func (m *mockNotificationSender) SupportsHTML(nid contract.NotifierID) bool { return true }
func (m *mockNotificationSender) NotifyDefaultWithError(msg string) error {
	return m.NotifyDefault(msg)
} // Added for interface compliance
func (m *mockNotificationSender) NotifyWithTitle(nid contract.NotifierID, title string, message string, errorOccurred bool) error {
	return nil
} // Added for interface compliance

func (m *mockNotificationSender) Health() error { return nil }

// =============================================================================
// Actual Tests
// =============================================================================

func TestIntegration_ServiceLifecycle(t *testing.T) {
	// Setup creates and starts services
	suite := setupIntegrationTest(t)

	// Just verify they are running by waiting a bit
	time.Sleep(100 * time.Millisecond)

	// Teardown stops them
	suite.Teardown()

	// If no panic/deadlock, pass
}

func TestIntegration_E2E_NotificationFlow(t *testing.T) {
	// Setup
	// 1. Config Setup (랜덤 포트 또는 테스트 포트 사용)
	appPort := 18089

	appConfig := &config.AppConfig{
		Debug: true,
		NotifyAPI: config.NotifyAPIConfig{
			WS: config.WSConfig{ListenPort: appPort},
			Applications: []config.ApplicationConfig{
				{ID: "test-app", AppKey: "valid-key", DefaultNotifierID: "test-notifier", Title: "TestApp"},
			},
		},
		Notifier: config.NotifierConfig{
			DefaultNotifierID: "test-notifier",
		},
	}

	// Mock Notifier Handler (The final destination)
	finalHandler := &mockNotifierHandler{
		id:           contract.NotifierID("test-notifier"),
		supportsHTML: true,
		calls:        make([]string, 0),
	}

	mockFactory := (&notificationmocks.MockFactory{}).WithCreateNotifiers([]notifier.NotifierHandler{finalHandler}, nil)

	// Services
	taskService := task.NewService(appConfig)
	notificationService := notification.NewService(appConfig, mockFactory, taskService)
	taskService.SetNotificationSender(notificationService)
	apiService := api.NewService(appConfig, notificationService, version.Info{})

	// Start
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(3)
	go taskService.Start(ctx, wg)
	go notificationService.Start(ctx, wg)
	go apiService.Start(ctx, wg)

	defer func() {
		cancel()
		wg.Wait()
	}()

	time.Sleep(200 * time.Millisecond) // Wait for server start

	// 2. HTTP Request (Simulate Client)
	reqBody := v1request.NotificationRequest{
		ApplicationID: "test-app", // Required by validator
		Message:       "Hello Integration World",
		ErrorOccurred: false,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/api/v1/notifications", appPort), bytes.NewBuffer(jsonBody))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	// Auth Headers
	req.Header.Set("X-Application-Id", "test-app")
	req.Header.Set("X-App-Key", "valid-key")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// 3. Verify HTTP Response
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// 4. Verify Notification Received (Async)
	// Task -> Notification -> Notifier (Mock)
	// Wait for async processing
	assert.Eventually(t, func() bool {
		calls := finalHandler.GetCalls()
		if len(calls) == 0 {
			return false
		}
		// Expect message format: "[TestApp] Hello Integration World" (Title used in formatter)
		// Or whatever the logic is. The simple formatter just joins them.
		return true
	}, 2*time.Second, 100*time.Millisecond, "Notification should reach the handler")
}

func TestIntegration_Auth_Failure(t *testing.T) {
	appPort := 18090
	appConfig := &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{WS: config.WSConfig{ListenPort: appPort}, Applications: []config.ApplicationConfig{{ID: "app", AppKey: "key"}}},
	}

	// Minimal setup just for API
	// We need nil-safe mocks because API initializes with NotificationSender.
	mockSender := &mockNotificationSender{}
	apiService := api.NewService(appConfig, mockSender, version.Info{})

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go apiService.Start(ctx, wg)
	defer func() { cancel(); wg.Wait() }()

	time.Sleep(100 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:%d/api/v1/notifications", appPort), bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Application-Id", "app")
	req.Header.Set("X-App-Key", "wrong-key")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
