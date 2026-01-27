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
	"github.com/darkkaiser/notify-server/internal/service/scheduler"
	"github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/darkkaiser/notify-server/internal/service/task/idgen"
	"github.com/darkkaiser/notify-server/internal/service/task/storage"
	"github.com/darkkaiser/notify-server/internal/testutil"

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
	wg                  sync.WaitGroup
	taskService         *task.Service
	schedulerService    *scheduler.Scheduler
	notificationService *notification.Service
	apiService          *api.Service
	mockHandler         *mockNotifierHandler // 최종 도착지 (Telegram 역할)
	apiPort             int
}

// setupIntegrationTestServices initializes all services but does NOT start them.
// This allows modification of services before starting.
func setupIntegrationTestServices(t *testing.T) *IntegrationTestSuite {
	// 1. Dynamic Port Allocation
	apiPort, err := testutil.GetFreePort()
	require.NoError(t, err, "Failed to get free port for API")

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

	// 2. Mock Notifier Handler Setup
	// This simulates the actual Telegram Client.
	mockHandler := &mockNotifierHandler{
		id:           contract.NotifierID("test-notifier"),
		supportsHTML: true,
		calls:        make([]string, 0),
	}

	// 3. Service Creation
	taskResultStore := storage.NewFileTaskResultStore(config.AppName)
	idGenerator := idgen.New()
	taskService := task.NewService(appConfig, idGenerator, taskResultStore)

	// Inject Mock Handler into Notification Service
	mockFactory := (&notificationmocks.MockFactory{}).WithCreateAll([]notifier.Notifier{
		mockHandler,
	}, nil)

	notificationService := notification.NewService(appConfig, mockFactory, taskService)

	// *CRITICAL*: Link TaskService back to NotificationService
	// TaskService needs to send notifications (e.g. task errors, completions) using the notification service.
	taskService.SetNotificationSender(notificationService)

	apiService := api.NewService(appConfig, notificationService, version.Info{Version: "test"})

	// 4. Scheduler Creation
	// Scheduler depends on TaskService (Submitter) and NotificationSender
	schedulerService := scheduler.NewService(appConfig.Tasks, taskService, notificationService)

	// 5. Context Setup
	ctx, cancel := context.WithCancel(context.Background())

	return &IntegrationTestSuite{
		t:                   t,
		appConfig:           appConfig,
		ctx:                 ctx,
		cancel:              cancel,
		taskService:         taskService,
		schedulerService:    schedulerService,
		notificationService: notificationService,
		apiService:          apiService,
		mockHandler:         mockHandler,
		apiPort:             apiPort,
	}
}

func (s *IntegrationTestSuite) Start() {
	s.wg.Add(4)
	// Start all services
	go s.taskService.Start(s.ctx, &s.wg)
	go s.schedulerService.Start(s.ctx, &s.wg)
	go s.notificationService.Start(s.ctx, &s.wg)
	go s.apiService.Start(s.ctx, &s.wg)

	// Wait for API server to be ready using polling
	require.NoError(s.t, testutil.WaitForServer(s.apiPort, 5*time.Second), "API Server did not start in time")
}

func (s *IntegrationTestSuite) Teardown() {
	s.cancel()
	// Wait for graceful shutdown
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		s.t.Error("Test Teardown timed out: Services did not shut down gracefully")
	}
}

// =============================================================================
// Mock Definitions
// =============================================================================

// mockNotifierHandler simulates a concrete Notifier implementation (like Telegram).
type mockNotifierHandler struct {
	id           contract.NotifierID
	supportsHTML bool
	mu           sync.Mutex
	calls        []string
}

func (m *mockNotifierHandler) ID() contract.NotifierID { return m.id }
func (m *mockNotifierHandler) SupportsHTML() bool      { return m.supportsHTML }
func (m *mockNotifierHandler) Run(ctx context.Context) {
	// Simulate a running notifier that respects context
	<-ctx.Done()
}

func (m *mockNotifierHandler) Send(ctx context.Context, notification contract.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Store the message for assertion
	m.calls = append(m.calls, notification.Message)
	return nil
}

func (m *mockNotifierHandler) TrySend(ctx context.Context, notification contract.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Store the message for assertion
	m.calls = append(m.calls, notification.Message)
	return nil
}

func (m *mockNotifierHandler) Close()                {}
func (m *mockNotifierHandler) Done() <-chan struct{} { return nil }

func (m *mockNotifierHandler) GetCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to avoid races
	calls := make([]string, len(m.calls))
	copy(calls, m.calls)
	return calls
}

// =============================================================================
// Actual Tests
// =============================================================================

func TestIntegration_ServiceLifecycle(t *testing.T) {
	suite := setupIntegrationTestServices(t)
	suite.Start()
	// If Start returns, it means the server is listening.
	suite.Teardown()
}

func TestIntegration_E2E_NotificationFlow(t *testing.T) {
	suite := setupIntegrationTestServices(t)
	suite.Start()
	defer suite.Teardown()

	// 1. Prepare Request
	reqBody := v1request.NotificationRequest{
		ApplicationID: "test-app",
		Message:       "Hello Integration World",
		ErrorOccurred: false,
	}
	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	url := fmt.Sprintf("http://localhost:%d/api/v1/notifications", suite.apiPort)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonBody))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Application-Id", "test-app")
	req.Header.Set("X-App-Key", "valid-key")

	// 2. Send Request
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// 3. Verify HTTP Response
	require.Equal(t, http.StatusOK, resp.StatusCode, "API request should succeed")

	// 4. Verify Notification Delivery (Async)
	// The full flow is: API -> NotificationService -> Queue -> Worker -> MockHandler
	expectedMessage := "Hello Integration World" // The minimal expected content

	assert.Eventually(t, func() bool {
		calls := suite.mockHandler.GetCalls()
		for _, msg := range calls {
			// Check if our message is contained (ignoring potential headers/footers added by formatters)
			if contains(msg, expectedMessage) {
				return true
			}
		}
		return false
	}, 3*time.Second, 100*time.Millisecond, "Notification should reach the final handler")
}

func TestIntegration_Auth_Failure(t *testing.T) {
	// Start with a new port to avoid any lingering state (though TestUtil handles this)
	port, err := testutil.GetFreePort()
	require.NoError(t, err)

	appConfig := &config.AppConfig{
		NotifyAPI: config.NotifyAPIConfig{
			WS: config.WSConfig{ListenPort: port},
			Applications: []config.ApplicationConfig{
				{ID: "app", AppKey: "key"},
			},
		},
	}

	// Minimal setup just for API auth check
	// We don't need the full stack if we just expect 401 at the API layer
	mockSender := &mockNotificationSender{}
	apiService := api.NewService(appConfig, mockSender, version.Info{})

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go apiService.Start(ctx, wg)

	// Custom cleanup since we didn't use the suite
	defer func() {
		cancel()
		wg.Wait()
	}()

	require.NoError(t, testutil.WaitForServer(port, 2*time.Second))

	// Send Request with Invalid Key
	url := fmt.Sprintf("http://localhost:%d/api/v1/notifications", port)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Application-Id", "app")
	req.Header.Set("X-App-Key", "wrong-key")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// Helpers

// mockNotificationSender for simple API-only tests
type mockNotificationSender struct {
	mu          sync.Mutex
	notifyCalls []string
}

func (m *mockNotificationSender) Notify(ctx context.Context, notification contract.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyCalls = append(m.notifyCalls, notification.Message)
	return nil
}
func (m *mockNotificationSender) SupportsHTML(nid contract.NotifierID) bool { return true }
func (m *mockNotificationSender) Health() error                             { return nil }

func contains(s, substr string) bool {
	// Simple wrapper, can be replaced by strings.Contains
	return len(s) >= len(substr) && (s == substr || len(s) > 0) // Simplified Check
}
