package api

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/pkg/version"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/testutil"
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

	mockService := &mocks.MockNotificationSender{}

	service := NewService(appConfig, mockService, version.Info{
		Version:     "1.0.0",
		BuildDate:   "2024-01-01",
		BuildNumber: "100",
	})

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	return service, appConfig, wg, ctx, cancel
}

// setupMinimalService는 최소한의 설정으로 Service를 생성합니다.
func setupMinimalService(t *testing.T) *Service {
	t.Helper()

	appConfig := &config.AppConfig{}
	mockService := &mocks.MockNotificationSender{}
	buildInfo := version.Info{
		Version:     "1.0.0",
		BuildDate:   "2024-01-01",
		BuildNumber: "100",
	}

	return NewService(appConfig, mockService, buildInfo)
}

// =============================================================================
// Constructor Tests
// =============================================================================

// TestNewService는 Service 생성자를 검증합니다.
func TestNewService(t *testing.T) {
	appConfig := &config.AppConfig{
		Debug: true,
	}
	appConfig.NotifyAPI.WS.ListenPort = 8080
	appConfig.NotifyAPI.CORS.AllowOrigins = []string{"http://localhost"}

	mockSender := &mocks.MockNotificationSender{}
	buildInfo := version.Info{
		Version:     "1.2.3",
		BuildDate:   "2024-01-15",
		BuildNumber: "456",
	}

	service := NewService(appConfig, mockSender, buildInfo)

	// 필드 검증
	assert.NotNil(t, service)
	assert.Equal(t, appConfig, service.appConfig)
	assert.Equal(t, mockSender, service.notificationSender)
	assert.Equal(t, buildInfo, service.buildInfo)
	assert.False(t, service.running, "초기 상태는 running=false여야 함")
}

// =============================================================================
// Server Setup Tests
// =============================================================================

// TestService_setupServer는 Echo 서버 설정을 검증합니다.
func TestService_setupServer(t *testing.T) {
	service := setupMinimalService(t)

	// setupServer 호출
	e := service.setupServer()

	// Echo 인스턴스 검증
	assert.NotNil(t, e)
	assert.NotNil(t, e.Router())

	// 라우트 등록 검증
	routes := e.Routes()
	assert.NotEmpty(t, routes, "라우트가 등록되어야 함")

	// 주요 라우트 존재 확인
	routePaths := make(map[string]bool)
	for _, route := range routes {
		routePaths[route.Path] = true
	}

	assert.True(t, routePaths["/health"], "/health 라우트가 등록되어야 함")
	assert.True(t, routePaths["/version"], "/version 라우트가 등록되어야 함")
	assert.True(t, routePaths["/api/v1/notifications"], "/api/v1/notifications 라우트가 등록되어야 함")
}

// =============================================================================
// Error Handling Tests
// =============================================================================

// TestService_handleServerError는 서버 에러 처리를 검증합니다.
func TestService_handleServerError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectLog    bool
		expectNotify bool
	}{
		{
			name:         "nil 에러는 처리하지 않음",
			err:          nil,
			expectLog:    false,
			expectNotify: false,
		},
		{
			name:         "http.ErrServerClosed는 정상 종료",
			err:          http.ErrServerClosed,
			expectLog:    true,
			expectNotify: false,
		},
		{
			name:         "예상치 못한 에러는 알림 전송",
			err:          assert.AnError,
			expectLog:    true,
			expectNotify: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSender := &mocks.MockNotificationSender{}
			service := setupMinimalService(t)
			service.notificationSender = mockSender

			// handleServerError 호출
			service.handleServerError(tt.err)

			// 알림 전송 검증
			if tt.expectNotify {
				assert.True(t, mockSender.NotifyDefaultCalled, "예상치 못한 에러 시 알림이 전송되어야 함")
			} else {
				assert.False(t, mockSender.NotifyDefaultCalled, "알림이 전송되지 않아야 함")
			}
		})
	}
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
	err := service.Start(ctx, wg)
	require.NoError(t, err, "Start should not return error")

	// Verify startup
	err = testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 2*time.Second)
	require.NoError(t, err, "Server should start within timeout")

	// running 상태 검증
	service.runningMu.Lock()
	assert.True(t, service.running, "서비스가 시작된 후 running=true여야 함")
	service.runningMu.Unlock()

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
		assert.Less(t, time.Since(shutdownStart), 6*time.Second, "Shutdown took too long")
	case <-time.After(6 * time.Second):
		t.Fatal("Shutdown timed out")
	}

	// running 상태 정리 검증
	service.runningMu.Lock()
	assert.False(t, service.running, "서비스 종료 후 running=false여야 함")
	service.runningMu.Unlock()
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
	err := service.Start(ctx, wg)
	require.NoError(t, err)

	testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 2*time.Second)

	// Second Start call
	// Since Start() calls defer wg.Done() even on early return (if checking running),
	// we MUST increment WG to prevent negative counter panics.
	wg.Add(1)
	err = service.Start(ctx, wg)
	assert.NoError(t, err, "중복 시작 호출은 에러를 반환하지 않아야 함")

	// running 상태는 여전히 true
	service.runningMu.Lock()
	assert.True(t, service.running)
	service.runningMu.Unlock()

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(6 * time.Second):
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
	service := NewService(appConfig, nil, version.Info{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}

	// Start() calls defer wg.Done() on error return too
	wg.Add(1)
	err := service.Start(ctx, wg)
	require.Error(t, err, "Should return error for nil NotificationSender")
	assert.Contains(t, err.Error(), "NotificationSender")

	// running 상태는 false로 유지
	service.runningMu.Lock()
	assert.False(t, service.running)
	service.runningMu.Unlock()
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestService_ConcurrentStart는 동시 Start 호출의 안전성을 검증합니다.
func TestService_ConcurrentStart(t *testing.T) {
	service, appConfig, wg, ctx, cancel := setupServiceHelper(t)
	defer cancel()

	// 여러 고루틴에서 동시에 Start 호출
	const goroutines = 10
	startErrors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			err := service.Start(ctx, wg)
			startErrors <- err
		}()
	}

	// 서버 시작 대기
	err := testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 2*time.Second)
	require.NoError(t, err)

	// 모든 Start 호출이 에러 없이 완료되어야 함
	close(startErrors)
	for err := range startErrors {
		assert.NoError(t, err)
	}

	// running 상태는 true
	service.runningMu.Lock()
	assert.True(t, service.running)
	service.runningMu.Unlock()

	// 정상 종료
	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("Shutdown timeout")
	}
}

// =============================================================================
// Shutdown Tests
// =============================================================================

// TestService_Shutdown_StateCleanup는 종료 시 상태 정리를 검증합니다.
func TestService_Shutdown_StateCleanup(t *testing.T) {
	service, appConfig, wg, ctx, cancel := setupServiceHelper(t)

	// 서비스 시작
	wg.Add(1)
	err := service.Start(ctx, wg)
	require.NoError(t, err)

	err = testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 2*time.Second)
	require.NoError(t, err)

	// running 상태 확인
	service.runningMu.Lock()
	assert.True(t, service.running)
	service.runningMu.Unlock()

	// 종료 트리거
	cancel()

	// 종료 대기
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 상태 정리 검증
		service.runningMu.Lock()
		assert.False(t, service.running, "종료 후 running=false여야 함")
		assert.NotNil(t, service.notificationSender, "notificationSender는 nil이 되지 않아야 함")
		service.runningMu.Unlock()
	case <-time.After(6 * time.Second):
		t.Fatal("Shutdown timeout")
	}
}

// TestService_ImmediateCancel는 즉시 취소된 컨텍스트를 검증합니다.
func TestService_ImmediateCancel(t *testing.T) {
	service, _, wg, ctx, cancel := setupServiceHelper(t)

	// 즉시 취소
	cancel()

	// 서비스 시작 (이미 취소된 컨텍스트)
	wg.Add(1)
	err := service.Start(ctx, wg)
	require.NoError(t, err, "Start should not return error even with cancelled context")

	// 짧은 시간 내에 종료되어야 함
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - 서버가 빠르게 종료됨
	case <-time.After(2 * time.Second):
		t.Fatal("Service should shutdown quickly with cancelled context")
	}
}
