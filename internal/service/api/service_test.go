package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/pkg/version"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Configuration & Helpers
// =============================================================================

// setupServiceHelper는 독립적이고 고립된 테스트 환경을 구성합니다.
// 랜덤 포트 할당, Mock 의존성 주입, 리소스 자동 정리를 포함합니다.
func setupServiceHelper(t *testing.T) (*Service, *config.AppConfig, *sync.WaitGroup, context.Context, context.CancelFunc) {
	t.Helper()

	// 1. 포트 충돌 방지를 위한 동적 포트 할당
	port, err := testutil.GetFreePort()
	require.NoError(t, err, "사용 가능한 포트 확보 실패")

	// 2. 기본 앱 설정 구성 (테스트용)
	appConfig := &config.AppConfig{}
	appConfig.NotifyAPI.WS.ListenPort = port
	appConfig.NotifyAPI.WS.TLSServer = false              // 기본은 HTTP
	appConfig.NotifyAPI.CORS.AllowOrigins = []string{"*"} // 개발 모드
	appConfig.Debug = true                                // 디버그 모드

	// 3. Mock NotificationSender (스레드 안전)
	mockSender := mocks.NewMockNotificationSender()

	// 4. Service 인스턴스 생성
	buildInfo := version.Info{
		Version:     "1.0.0-test",
		BuildDate:   time.Now().Format(time.RFC3339),
		BuildNumber: "test-build",
	}

	service := NewService(appConfig, mockSender, buildInfo)

	// 5. Context 및 WaitGroup 구성
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	// 6. 리소스 정리 (t.Cleanup)
	t.Cleanup(func() {
		cancel() // 1차: Context 취소로 서비스 종료 시그널 전송

		// 2차: 고루틴 종료 대기 (타임아웃 적용)
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// 정상 종료
		case <-time.After(3 * time.Second):
			// 고루틴 누수 가능성 경고 (테스트 실패로 처리하지 않음)
			t.Logf("WARN: Service WaitGroup did not finish in time (Port: %d)", port)
		}
	})

	return service, appConfig, wg, ctx, cancel
}

// =============================================================================
// Constructor & Validation Tests
// =============================================================================

func TestNewService_Success(t *testing.T) {
	s, cfg, _, _, _ := setupServiceHelper(t)

	assert.NotNil(t, s)
	assert.Equal(t, cfg, s.appConfig)
	assert.NotNil(t, s.notificationSender)
	assert.False(t, s.running)
}

func TestNewService_Panics(t *testing.T) {
	t.Run("AppConfig 누락 시 패닉", func(t *testing.T) {
		assert.PanicsWithValue(t, constants.PanicMsgAppConfigRequired, func() {
			NewService(nil, mocks.NewMockNotificationSender(), version.Info{})
		})
	})

	t.Run("NotificationSender 누락 시 패닉", func(t *testing.T) {
		assert.PanicsWithValue(t, constants.PanicMsgNotificationSenderRequired, func() {
			NewService(&config.AppConfig{}, nil, version.Info{})
		})
	})
}

// =============================================================================
// Server Configuration Tests
// =============================================================================

func TestService_setupServer_Configuration(t *testing.T) {
	service, _, _, _, _ := setupServiceHelper(t)

	// HSTS 활성화 설정
	service.appConfig.NotifyAPI.WS.TLSServer = true

	e := service.setupServer()

	// 1. Echo 기본 설정 확인
	assert.NotNil(t, e)
	assert.True(t, e.Debug)

	// 2. HTTP Server 타임아웃 설정 전파 확인
	require.NotNil(t, e.Server)
	assert.Equal(t, constants.DefaultReadHeaderTimeout, e.Server.ReadHeaderTimeout)
	assert.Equal(t, constants.DefaultReadTimeout, e.Server.ReadTimeout)

	// 3. 보안 미들웨어 설정 추론 (TLSServer=true -> EnableHSTS=true)
	// (미들웨어 내부 상태를 직접 확인하긴 어렵지만, 서비스가 Config를 올바르게 전달했는지 간접 검증)
	// HSTS가 활성화된 config로 NewHTTPServer가 호출되었음을 가정
}

// =============================================================================
// Lifecycle Tests (Start/Stop)
// =============================================================================

func TestService_Start_HTTPS_Success(t *testing.T) {
	service, appConfig, wg, ctx, _ := setupServiceHelper(t)

	// Self-Signed Cert 생성
	certFile, keyFile, cleanup := testutil.GenerateSelfSignedCert(t)
	defer cleanup()

	appConfig.NotifyAPI.WS.TLSServer = true
	appConfig.NotifyAPI.WS.TLSCertFile = certFile
	appConfig.NotifyAPI.WS.TLSKeyFile = keyFile

	wg.Add(1)
	err := service.Start(ctx, wg)
	require.NoError(t, err)

	require.NoError(t, testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 2*time.Second))

	// HTTPS 클라이언트로 요청
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 1 * time.Second,
	}

	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/health", appConfig.NotifyAPI.WS.ListenPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestService_Start_PortConflict(t *testing.T) {
	// 1. 포트 선점
	ls, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer ls.Close()
	port := ls.Addr().(*net.TCPAddr).Port

	// 2. 동일 포트로 서비스 시작 시도
	service, _, wg, ctx, _ := setupServiceHelper(t)
	service.appConfig.NotifyAPI.WS.ListenPort = port

	wg.Add(1)
	err = service.Start(ctx, wg)
	require.NoError(t, err)

	// 3. 에러 발생으로 인해 즉시 종료되어야 함
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 성공적 종료
		service.runningMu.Lock()
		assert.False(t, service.running)
		service.runningMu.Unlock()
	case <-time.After(2 * time.Second):
		t.Fatal("포트 충돌에도 불구하고 서비스가 종료되지 않았습니다")
	}

	// 4. 알림 전송 확인
	mockSender := service.notificationSender.(*mocks.MockNotificationSender)
	assert.True(t, mockSender.WasNotifyDefaultCalled(), "서버 시작 실패 시 알림이 전송되어야 함")
}

func TestService_Start_Duplicate_Idempotency(t *testing.T) {
	service, cfg, wg, ctx, _ := setupServiceHelper(t)

	wg.Add(1)
	require.NoError(t, service.Start(ctx, wg))
	require.NoError(t, testutil.WaitForServer(cfg.NotifyAPI.WS.ListenPort, 2*time.Second))

	// 두 번째 시작 시도 -> 에러 없고 무시됨
	wg.Add(1)
	err := service.Start(ctx, wg)
	assert.NoError(t, err)

	// WaitGroup 카운트가 즉시 감소했는지 확인은 어렵지만(Race),
	// 로그나 커버리지로 실행되지 않음을 보장.
	// 여기서는 에러가 없다는 것만 확인.
}

func TestService_Start_WithCanceledContext(t *testing.T) {
	service, _, wg, ctx, cancel := setupServiceHelper(t)

	cancel() // 시작 전 취소

	wg.Add(1)
	err := service.Start(ctx, wg)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("취소된 컨텍스트로 시작 시 즉시 종료되어야 함")
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestService_handleServerError(t *testing.T) {
	service, _, _, _, _ := setupServiceHelper(t)
	mockSender := service.notificationSender.(*mocks.MockNotificationSender)

	tests := []struct {
		name         string
		inputErr     error
		expectNotify bool
	}{
		{"No Error", nil, false},
		{"Server Closed", http.ErrServerClosed, false},
		{"Unexpected Error", assert.AnError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSender.Reset() // 상태 초기화
			service.handleServerError(tt.inputErr)
			assert.Equal(t, tt.expectNotify, mockSender.WasNotifyDefaultCalled())
		})
	}
}
