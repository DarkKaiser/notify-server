package api

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/pkg/common"
	"github.com/darkkaiser/notify-server/service/api/testutil"
	"github.com/stretchr/testify/assert"
)

// setupServiceHelper encapsulates common setup logic
// Returns configured service, appConfig, WaitGroup, Context, and CancelFunc
// setupServiceHelper encapsulates common setup logic
// Returns configured service, appConfig, WaitGroup, Context, and CancelFunc
func setupServiceHelper(t *testing.T) (*Service, *config.AppConfig, *sync.WaitGroup, context.Context, context.CancelFunc) {
	// Dynamic port to avoid conflicts
	port, err := testutil.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	appConfig := &config.AppConfig{}
	appConfig.NotifyAPI.WS.ListenPort = port
	appConfig.NotifyAPI.WS.TLSServer = false

	mockService := &testutil.MockNotificationSender{}

	service := NewService(appConfig, mockService, common.BuildInfo{
		Version:     "1.0.0",
		BuildDate:   "2024-01-01",
		BuildNumber: "100",
	})

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	return service, appConfig, wg, ctx, cancel
}

func TestNotifyAPIService_Lifecycle(t *testing.T) {
	service, appConfig, wg, ctx, cancel := setupServiceHelper(t)
	defer cancel() // Safety net

	wg.Add(1)
	go service.Start(ctx, wg)

	// Verify startup
	err := testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 2*time.Second)
	assert.NoError(t, err, "Server should start within timeout")

	// Verify Shutdown
	shutdownStart := time.Now()
	cancel() // Trigger shutdown

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
		assert.Less(t, time.Since(shutdownStart), 5*time.Second, "Shutdown took too long")
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown timed out")
	}
}

func TestNotifyAPIService_DuplicateStart(t *testing.T) {
	service, appConfig, wg, ctx, cancel := setupServiceHelper(t)
	defer cancel()

	// First Start
	wg.Add(1)
	go service.Start(ctx, wg)

	testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 2*time.Second)

	// Second Start call
	// Since Start() calls defer wg.Done() even on early return (if checking running),
	// we MUST increment WG to prevent negative counter panics.
	wg.Add(1)
	err := service.Start(ctx, wg)
	assert.NoError(t, err)

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown timeout - possibly WaitGroup mismatch")
	}
}

func TestNotifyAPIService_NilDependencies(t *testing.T) {
	appConfig := &config.AppConfig{}
	// No NotificationService
	service := NewService(appConfig, nil, common.BuildInfo{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}

	// Start() calls defer wg.Done() on error return too
	wg.Add(1)
	err := service.Start(ctx, wg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "notificationSender")
}
