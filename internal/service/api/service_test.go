package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/pkg/version"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/mocks"

	"github.com/darkkaiser/notify-server/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Configuration & Helpers
// =============================================================================

// setupServiceHelper는 독립적이고 고립된 테스트 환경을 구성합니다.
// 랜덤 포트 할당, Mock 의존성 주입, 리소스 자동 정리를 포함합니다.
// customSender가 nil이면 기본 MockNotificationSender를 사용합니다.
func setupServiceHelper(t *testing.T, customSender contract.NotificationSender) (*Service, *config.AppConfig, *sync.WaitGroup, context.Context, context.CancelFunc) {
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

	// 3. Mock NotificationSender (스레드 안전) 또는 커스텀 Sender 사용
	var sender contract.NotificationSender
	if customSender != nil {
		sender = customSender
	} else {
		sender = mocks.NewMockNotificationSender()
	}

	// Default Health check expectation for mock sender if it's the mock
	if mSender, ok := sender.(*mocks.MockNotificationSender); ok {
		mSender.On("Health").Return(nil).Maybe()
	}

	// 4. Service 인스턴스 생성
	buildInfo := version.Info{
		Version:     "1.0.0-test",
		BuildDate:   time.Now().Format(time.RFC3339),
		BuildNumber: "test-build",
	}

	service := NewService(appConfig, sender, buildInfo)

	// 5. Context 및 WaitGroup 구성
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	// 6. 리소스 정리
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

// mockSenderWithoutHealth HealthChecker 인터페이스를 구현하지 않는 Mock Sender
type mockSenderWithoutHealth struct{}

func (m *mockSenderWithoutHealth) Notify(taskCtx contract.TaskContext, notifierID contract.NotifierID, message string) error {
	return nil
}
func (m *mockSenderWithoutHealth) NotifyWithTitle(notifierID contract.NotifierID, title string, message string, errorOccurred bool) error {
	return nil
}
func (m *mockSenderWithoutHealth) NotifyDefault(message string) error { return nil }
func (m *mockSenderWithoutHealth) NotifyDefaultWithError(message string) error {
	return nil
}
func (m *mockSenderWithoutHealth) SupportsHTML(notifierID contract.NotifierID) bool { return false }

// =============================================================================
// Constructor & Validation Tests
// =============================================================================

func TestNewService_Success(t *testing.T) {
	t.Parallel()
	s, cfg, _, _, _ := setupServiceHelper(t, nil)

	assert.NotNil(t, s)
	assert.Equal(t, cfg, s.appConfig)
	assert.NotNil(t, s.notificationSender)
	assert.False(t, s.running)
}

func TestNewService_Validation(t *testing.T) {
	t.Parallel()

	validSender := mocks.NewMockNotificationSender()
	invalidSender := &mockSenderWithoutHealth{}
	validConfig := &config.AppConfig{}
	buildInfo := version.Info{}

	tests := []struct {
		name        string
		appConfig   *config.AppConfig
		sender      contract.NotificationSender
		expectPanic string
	}{
		{
			name:        "AppConfig 누락 시 패닉",
			appConfig:   nil,
			sender:      validSender,
			expectPanic: "AppConfig는 필수입니다",
		},
		{
			name:        "NotificationSender 누락 시 패닉",
			appConfig:   validConfig,
			sender:      nil,
			expectPanic: "NotificationSender는 필수입니다",
		},
		{
			name:        "HealthChecker 미구현 Sender 사용 시 패닉",
			appConfig:   validConfig,
			sender:      invalidSender,
			expectPanic: "HealthChecker는 필수입니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.PanicsWithValue(t, tt.expectPanic, func() {
				NewService(tt.appConfig, tt.sender, buildInfo)
			})
		})
	}
}

// =============================================================================
// Server Configuration Tests
// =============================================================================

func TestService_setupServer_Configuration(t *testing.T) {
	t.Parallel()
	service, _, _, _, _ := setupServiceHelper(t, nil)

	// HSTS 활성화 설정
	service.appConfig.NotifyAPI.WS.TLSServer = true

	e := service.setupServer()

	// 1. Echo 기본 설정 확인
	assert.NotNil(t, e)
	assert.True(t, e.Debug)

	// 2. HTTP Server 타임아웃 설정 전파 확인
	require.NotNil(t, e.Server)
	assert.Equal(t, defaultReadHeaderTimeout, e.Server.ReadHeaderTimeout)
	assert.Equal(t, defaultReadTimeout, e.Server.ReadTimeout)
}

// =============================================================================
// Lifecycle Tests (Start/Stop)
// =============================================================================

func TestService_Start_HTTPS_Success(t *testing.T) {
	t.Parallel()
	service, appConfig, wg, ctx, _ := setupServiceHelper(t, nil)

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

func TestService_Start_HTTP_Success(t *testing.T) {
	t.Parallel()
	service, appConfig, wg, ctx, _ := setupServiceHelper(t, nil)

	// TLS 비활성화 (이미 기본값이지만 명시적 설정)
	appConfig.NotifyAPI.WS.TLSServer = false

	wg.Add(1)
	err := service.Start(ctx, wg)
	require.NoError(t, err)

	require.NoError(t, testutil.WaitForServer(appConfig.NotifyAPI.WS.ListenPort, 2*time.Second))

	// HTTP 클라이언트로 요청
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", appConfig.NotifyAPI.WS.ListenPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestService_Start_Failure_NilSender(t *testing.T) {
	t.Parallel()
	// 생성자 회피를 위한 수동 생성 (Service는 같은 패키지이므로 internal 필드 접근 가능)
	service := &Service{
		appConfig:          &config.AppConfig{},
		notificationSender: nil, // 강제 nil 설정 (Start 시 체크되어야 함)
	}

	ctx := context.Background()
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// When
	err := service.Start(ctx, wg)

	// Then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NotificationSender 객체가 초기화되지 않았습니다")
}

func TestService_Start_Failure_PortConflict(t *testing.T) {
	t.Parallel()

	// 1. 포트 선점
	ls, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer ls.Close()
	port := ls.Addr().(*net.TCPAddr).Port

	// 2. 동일 포트로 서비스 시작 시도
	service, _, wg, ctx, _ := setupServiceHelper(t, nil)
	service.appConfig.NotifyAPI.WS.ListenPort = port

	// Expectation for NotifyDefaultWithError
	service.notificationSender.(*mocks.MockNotificationSender).On("NotifyDefaultWithError", mock.Anything).Return(nil)

	wg.Add(1)
	err = service.Start(ctx, wg)
	require.NoError(t, err) // Start 자체는 에러가 없음 (비동기 처리)

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
	mockSender.AssertCalled(t, "NotifyDefaultWithError", mock.Anything)
}

func TestService_Start_Duplicate_Idempotency(t *testing.T) {
	t.Parallel()
	service, cfg, wg, ctx, _ := setupServiceHelper(t, nil)

	wg.Add(1)
	require.NoError(t, service.Start(ctx, wg))
	require.NoError(t, testutil.WaitForServer(cfg.NotifyAPI.WS.ListenPort, 2*time.Second))

	// 두 번째 시작 시도 -> 에러 없고 무시됨
	wg.Add(1)
	err := service.Start(ctx, wg)
	assert.NoError(t, err)
}

func TestService_Start_WithCanceledContext(t *testing.T) {
	t.Parallel()
	service, _, wg, ctx, cancel := setupServiceHelper(t, nil)

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

	// 실행 상태가 false여야 함
	service.runningMu.Lock()
	assert.False(t, service.running)
	service.runningMu.Unlock()
}

func TestService_Start_Concurrency(t *testing.T) {
	t.Parallel()
	service, _, wg, ctx, _ := setupServiceHelper(t, nil)

	// 동시에 10번 Start 호출
	const concurrency = 10
	var startWg sync.WaitGroup
	startWg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer startWg.Done()

			// WG Add/Done 밸런스를 외부에서 맞추기 어려우므로, Start 내부에서 Done이 호출될 것을 고려하여
			// 테스트용 임시 WG를 사용할 수도 있으나, 여기서는 service_test.go의 특성상
			// 메인 WG에 대해 Add를 매번 하고, Start가 실패(중복)하면 Done을 즉시 호출하므로
			// 메인 WG 로직을 따름.
			wg.Add(1)
			_ = service.Start(ctx, wg)
		}()
	}
	startWg.Wait()

	// 서비스가 실행 중인지 확인
	service.runningMu.Lock()
	assert.True(t, service.running)
	service.runningMu.Unlock()
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestService_handleServerError(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 각각 독립된 Mock 필요
			service, _, _, _, _ := setupServiceHelper(t, nil)
			mockSender := service.notificationSender.(*mocks.MockNotificationSender)

			if tt.expectNotify {
				mockSender.On("NotifyDefaultWithError", mock.MatchedBy(func(msg string) bool {
					return strings.Contains(msg, "API 서비스 > http 서버를 구성하는 중에 치명적인 오류가 발생하였습니다.") &&
						(tt.inputErr == nil || strings.Contains(msg, tt.inputErr.Error()))
				})).Return(nil)
			}

			service.handleServerError(tt.inputErr)

			// 1. 알림 전송 여부 확인
			if tt.expectNotify {
				mockSender.AssertCalled(t, "NotifyDefaultWithError", mock.Anything)
			} else {
				mockSender.AssertNotCalled(t, "NotifyDefaultWithError", mock.Anything)
			}
		})
	}
}
