package api

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/pkg/buildinfo"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/api/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupServiceHelper는 API 서비스 테스트를 위한 공통 설정을 생성합니다.
//
// 반환값:
//   - Service: 설정된 API 서비스
//   - AppConfig: 애플리케이션 설정
//   - WaitGroup: 동기화용 WaitGroup
//   - Context: 컨텍스트
//   - CancelFunc: 취소 함수
func setupServiceHelper(t *testing.T) (*Service, *config.AppConfig, *sync.WaitGroup, context.Context, context.CancelFunc) {
	t.Helper()

	// Dynamic port to avoid conflicts
	port, err := testutil.GetFreePort()
	require.NoError(t, err, "Failed to get free port")

	appConfig := &config.AppConfig{}
	appConfig.NotifyAPI.WS.ListenPort = port
	appConfig.NotifyAPI.WS.TLSServer = false

	mockService := &testutil.MockNotificationSender{}

	service := NewService(appConfig, mockService, buildinfo.BuildInfo{
		Version:     "1.0.0",
		BuildDate:   "2024-01-01",
		BuildNumber: "100",
	})

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	return service, appConfig, wg, ctx, cancel
}

// =============================================================================
// Service Lifecycle Tests
// =============================================================================

// TestNotifyAPIService_Lifecycle는 API 서비스의 시작 및 종료를 검증합니다.
//
// 검증 항목:
//   - 서비스 정상 시작
//   - 서버 응답 확인
//   - 정상 종료
func TestNotifyAPIService_Lifecycle(t *testing.T) {
	service, appConfig, wg, ctx, cancel := setupServiceHelper(t)
	defer cancel() // Safety net

	wg.Add(1)
	go service.Start(ctx, wg)

	// Verify startup
	err := testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 2*time.Second)
	require.NoError(t, err, "Server should start within timeout")

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

// TestNotifyAPIService_DuplicateStart는 중복 시작 호출을 검증합니다.
//
// 검증 항목:
//   - 첫 번째 시작 성공
//   - 두 번째 시작 호출 처리
//   - 정상 종료
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

// TestNotifyAPIService_NilDependencies는 nil 의존성 처리를 검증합니다.
//
// 검증 항목:
//   - NotificationSender가 nil일 때 에러 반환
func TestNotifyAPIService_NilDependencies(t *testing.T) {
	appConfig := &config.AppConfig{}
	// No NotificationService
	service := NewService(appConfig, nil, buildinfo.BuildInfo{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}

	// Start() calls defer wg.Done() on error return too
	wg.Add(1)
	err := service.Start(ctx, wg)
	require.Error(t, err, "Should return error for nil NotificationSender")
	assert.Contains(t, err.Error(), "NotificationSender")
}
