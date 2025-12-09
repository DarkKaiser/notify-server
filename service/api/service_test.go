package api

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/pkg/common"
	"github.com/stretchr/testify/assert"
)

// MockNotificationService는 테스트용 간단한 알림 발송자입니다.
type MockNotificationService struct{}

func (m *MockNotificationService) Notify(notifierID string, title string, message string, errorOccurred bool) bool {
	return true
}

func (m *MockNotificationService) NotifyToDefault(message string) bool {
	return true
}

func (m *MockNotificationService) NotifyWithErrorToDefault(message string) bool {
	return true
}

// setupTestService는 테스트용 서비스를 설정합니다.
func setupTestService(t *testing.T, port int) (*NotifyAPIService, *config.AppConfig) {
	appConfig := &config.AppConfig{}
	appConfig.NotifyAPI.WS.ListenPort = port
	appConfig.NotifyAPI.WS.TLSServer = false

	mockService := &MockNotificationService{}
	service := NewNotifyAPIService(appConfig, mockService, common.BuildInfo{
		Version:     "1.0.0",
		BuildDate:   "2024-01-01",
		BuildNumber: "100",
	})
	return service, appConfig
}

// TestNotifyAPIService_Run은 서비스 시작을 테스트합니다.
func TestNotifyAPIService_Run(t *testing.T) {
	service, _ := setupTestService(t, 18081)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	// 서비스 시작
	go service.Start(ctx, wg)

	// 서비스 시작 대기
	time.Sleep(500 * time.Millisecond)

	// 서비스 종료
	cancel()

	// 종료 대기 (타임아웃 설정)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 정상 종료
	case <-time.After(10 * time.Second):
		t.Fatal("서비스가 제한 시간 내에 종료되지 않았습니다")
	}
}

// TestNotifyAPIService_GracefulShutdown은 우아한 종료를 테스트합니다.
func TestNotifyAPIService_GracefulShutdown(t *testing.T) {
	service, _ := setupTestService(t, 18082)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// 서비스 시작
	go service.Start(ctx, wg)

	// 서비스가 완전히 시작될 때까지 대기
	time.Sleep(500 * time.Millisecond)

	// Graceful Shutdown 시작
	shutdownStart := time.Now()
	cancel()

	// 종료 대기
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		shutdownDuration := time.Since(shutdownStart)
		// 종료가 너무 오래 걸리지 않았는지 확인 (10초 이내)
		assert.Less(t, shutdownDuration, 10*time.Second, "Graceful shutdown이 너무 오래 걸렸습니다")
	case <-time.After(15 * time.Second):
		t.Fatal("Graceful shutdown이 제한 시간 내에 완료되지 않았습니다")
	}
}

// TestNotifyAPIService_DuplicateRun은 중복 시작 방지를 테스트합니다.
func TestNotifyAPIService_DuplicateRun(t *testing.T) {
	service, _ := setupTestService(t, 18083)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}
	wg.Add(2) // 두 번 시작 시도

	// 첫 번째 시작
	go service.Start(ctx, wg)
	time.Sleep(500 * time.Millisecond)

	// 두 번째 시작 시도 (이미 실행 중이므로 즉시 반환되어야 함)
	go service.Start(ctx, wg)

	// 모든 Run 호출이 완료될 때까지 대기
	time.Sleep(500 * time.Millisecond)

	// 종료
	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 정상 종료
	case <-time.After(10 * time.Second):
		t.Fatal("서비스 종료가 제한 시간 내에 완료되지 않았습니다")
	}
}

// TestNotifyAPIService_NilNotificationService는 nil NotificationService 처리를 테스트합니다.
func TestNotifyAPIService_NilNotificationService(t *testing.T) {
	appConfig := &config.AppConfig{}
	appConfig.NotifyAPI.WS.ListenPort = 18084
	appConfig.NotifyAPI.WS.TLSServer = false

	service := NewNotifyAPIService(appConfig, nil, common.BuildInfo{
		Version:     "1.0.0",
		BuildDate:   "2024-01-01",
		BuildNumber: "100",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	// nil NotificationService로 시작 시도 - error가 반환되어야 함
	err := service.Start(ctx, wg)

	// 초기화 되지 않은 NotificationService로 인해 에러 발생 확인
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "notificationService", "에러 메시지에 notificationService가 포함되어야 합니다")
}
