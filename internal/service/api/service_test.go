package api

import (
	"context"
	"net/http"
	"path/filepath"
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
func setupServiceHelper(t *testing.T) (*Service, *config.AppConfig, *sync.WaitGroup, context.Context, context.CancelFunc) {
	t.Helper()

	// 충돌 방지를 위한 동적 포트 할당
	port, err := testutil.GetFreePort()
	require.NoError(t, err, "사용 가능한 포트를 가져오는데 실패했습니다")

	appConfig := &config.AppConfig{}
	appConfig.NotifyAPI.WS.ListenPort = port
	appConfig.NotifyAPI.WS.TLSServer = false
	appConfig.NotifyAPI.CORS.AllowOrigins = []string{"*"}
	appConfig.Debug = true

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

	appConfig := &config.AppConfig{
		Debug: true,
	}
	appConfig.NotifyAPI.WS.ListenPort = 8080 // 기본값

	mockService := &mocks.MockNotificationSender{}
	buildInfo := version.Info{
		Version: "1.0.0",
	}

	return NewService(appConfig, mockService, buildInfo)
}

// =============================================================================
// Constructor Tests
// =============================================================================

// TestNewService는 Service 생성자가 올바르게 초기화되는지 검증합니다.
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

	// 1. Echo 인스턴스 검증
	assert.NotNil(t, e)
	assert.NotNil(t, e.Router())
	assert.True(t, e.Debug, "Config의 Debug가 true이면 Echo Debug도 true여야 함")

	// 2. 라우트 등록 검증
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
// TLS Configuration Tests
// =============================================================================

// TestNotifyAPIService_StartTLS는 TLS 설정이 활성화되었을 때 서버 동작을 검증합니다.
func TestNotifyAPIService_StartTLS(t *testing.T) {
	service, appConfig, wg, ctx, cancel := setupServiceHelper(t)
	defer cancel()

	// TLS 설정 활성화
	appConfig.NotifyAPI.WS.TLSServer = true
	// 존재하지 않거나 유효하지 않은 인증서 경로 설정
	appConfig.NotifyAPI.WS.TLSCertFile = filepath.Join("invalid", "cert.pem")
	appConfig.NotifyAPI.WS.TLSKeyFile = filepath.Join("invalid", "key.pem")

	wg.Add(1)
	err := service.Start(ctx, wg)
	require.NoError(t, err, "비동기 서버 시작은 에러를 반환하지 않아야 함")

	// 서버가 시작되고 TLS 파일 로드 실패로 인해 종료될 때까지 대기
	// (실제 TLS 인증서가 없으므로 startHTTPServer에서 에러 발생 예상)
	// 우리는 이 동작을 검하기 위해 notificationSender의 호출 여부를 확인

	// Mock Sender는 포인터 공유되므로 service 생성 시 사용된 것을 그대로 사용
	mockSender := service.notificationSender.(*mocks.MockNotificationSender)

	// 짧은 대기 (에러 처리가 비동기로 발생)
	time.Sleep(100 * time.Millisecond)

	// TLS 파일이 없으므로 startHTTPServer -> StartTLS -> 에러 발생 -> handleServerError
	// -> NotifyDefaultWithError 호출되어야 함
	assert.True(t, mockSender.NotifyDefaultCalled, "TLS 파일 로드 실패 시 알림이 전송되어야 함")
}

// =============================================================================
// Error Handling Tests
// =============================================================================

// TestService_handleServerError는 서버 에러 처리를 검증합니다.
func TestService_handleServerError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectNotify bool
	}{
		{
			name:         "nil 에러: 처리하지 않음",
			err:          nil,
			expectNotify: false,
		},
		{
			name:         "http.ErrServerClosed: 정상 종료 (알림 없음)",
			err:          http.ErrServerClosed,
			expectNotify: false,
		},
		{
			name:         "예상치 못한 에러: 알림 전송",
			err:          assert.AnError,
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

// TestNotifyAPIService_Lifecycle는 API 서비스의 시작 및 종료를 통합 검증합니다.
func TestNotifyAPIService_Lifecycle(t *testing.T) {
	service, appConfig, wg, ctx, cancel := setupServiceHelper(t)
	defer cancel()

	wg.Add(1)
	err := service.Start(ctx, wg)
	require.NoError(t, err, "Start 호출 성공해야 함")

	// 서버 시작 대기
	err = testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 2*time.Second)
	require.NoError(t, err, "서버가 타임아웃 내에 시작되어야 함")

	// 1. Running 상태 검증
	service.runningMu.Lock()
	assert.True(t, service.running, "서비스 시작 후 running=true")
	service.runningMu.Unlock()

	// 2. 종료 프로세스 시작
	shutdownStart := time.Now()
	cancel() // Context 취소로 종료 트리거

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 성공
		assert.Less(t, time.Since(shutdownStart), 6*time.Second, "Shutdown은 타임아웃(5초) 내에 완료되어야 함")
	case <-time.After(6 * time.Second):
		t.Fatal("Shutdown 타임아웃 발생 (WaitGroup mismatch 가능성)")
	}

	// 3. 종료 후 상태 검증
	service.runningMu.Lock()
	assert.False(t, service.running, "서비스 종료 후 running=false")
	service.runningMu.Unlock()
}

// TestNotifyAPIService_DuplicateStart는 중복 시작 호출 시 동작을 검증합니다.
func TestNotifyAPIService_DuplicateStart(t *testing.T) {
	service, appConfig, wg, ctx, cancel := setupServiceHelper(t)
	defer cancel()

	// 첫 번째 Start
	wg.Add(1)
	err := service.Start(ctx, wg)
	require.NoError(t, err)

	testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 2*time.Second)

	// 두 번째 Start
	// Start 내부에서 이미 실행 중이면 defer wg.Done()을 호출하므로 WG를 증가시켜야 함
	wg.Add(1)
	err = service.Start(ctx, wg)
	assert.NoError(t, err, "중복 시작은 에러를 반환하지 않고 무시해야 함")

	// running 상태 유지 확인
	service.runningMu.Lock()
	assert.True(t, service.running)
	service.runningMu.Unlock()

	// 종료
	cancel()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("Shutdown 타임아웃")
	}
}

// TestNotifyAPIService_NilDependencies는 필수 의존성이 없을 때의 동작을 검증합니다.
func TestNotifyAPIService_NilDependencies(t *testing.T) {
	appConfig := &config.AppConfig{}
	// NotificationSender가 nil인 상태
	service := NewService(appConfig, nil, version.Info{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}

	wg.Add(1)
	err := service.Start(ctx, wg)

	// 검증
	require.Error(t, err, "NotificationSender가 nil이면 에러를 반환해야 함")
	assert.Contains(t, err.Error(), "NotificationSender", "에러 메시지에 필드명이 포함되어야 함")

	// running 상태는 false
	service.runningMu.Lock()
	assert.False(t, service.running)
	service.runningMu.Unlock()
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestService_ConcurrentStart는 동시에 여러 Start 호출이 발생해도 안전한지 검증합니다.
func TestService_ConcurrentStart(t *testing.T) {
	service, appConfig, wg, ctx, cancel := setupServiceHelper(t)
	defer cancel()

	const goroutines = 10
	startErrors := make(chan error, goroutines)
	startWg := &sync.WaitGroup{}

	// 동시에 10개의 Start 호출
	for i := 0; i < goroutines; i++ {
		// 각 고루틴마다 서비스의 wg.Add를 호출해야 함 (Start 내부에서 defer wg.Done 호출하므로)
		wg.Add(1)

		startWg.Add(1)
		go func() {
			defer startWg.Done()
			err := service.Start(ctx, wg)
			startErrors <- err
		}()
	}

	// 서버 시작 대기
	err := testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 5*time.Second)
	require.NoError(t, err)

	startWg.Wait()
	close(startErrors)

	// 모든 호출이 에러 없이 반환되어야 함 (첫 번째는 시작, 나머지는 무시)
	for err := range startErrors {
		assert.NoError(t, err)
	}

	cancel()

	// 종료 대기
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second): // 타임아웃 조금 더 여유있게
		t.Fatal("Shutdown 타임아웃 - Race condition 가능성")
	}
}
