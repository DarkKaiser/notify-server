package api

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/g"
	"github.com/stretchr/testify/assert"
)

// MockNotificationSender는 테스트용 간단한 알림 발송자입니다.
type MockNotificationSender struct{}

func (m *MockNotificationSender) Notify(notifierID string, title string, message string, errorOccurred bool) bool {
	return true
}

func (m *MockNotificationSender) NotifyToDefault(message string) bool {
	return true
}

func (m *MockNotificationSender) NotifyWithErrorToDefault(message string) bool {
	return true
}

// TestNotifyAPIService_Run은 서비스 시작을 테스트합니다.
func TestNotifyAPIService_Run(t *testing.T) {
	appConfig := &g.AppConfig{}
	appConfig.NotifyAPI.WS.ListenPort = 18080 // 테스트용 포트
	appConfig.NotifyAPI.WS.TLSServer = false

	mockSender := &MockNotificationSender{}
	service := NewNotifyAPIService(appConfig, mockSender, "1.0.0", "2024-01-01", "100")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	// 서비스 시작
	go service.Run(ctx, wg)

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
	appConfig := &g.AppConfig{}
	appConfig.NotifyAPI.WS.ListenPort = 18081 // 다른 포트 사용
	appConfig.NotifyAPI.WS.TLSServer = false

	mockSender := &MockNotificationSender{}
	service := NewNotifyAPIService(appConfig, mockSender, "1.0.0", "2024-01-01", "100")

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// 서비스 시작
	go service.Run(ctx, wg)

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
	appConfig := &g.AppConfig{}
	appConfig.NotifyAPI.WS.ListenPort = 18082
	appConfig.NotifyAPI.WS.TLSServer = false

	mockSender := &MockNotificationSender{}
	service := NewNotifyAPIService(appConfig, mockSender, "1.0.0", "2024-01-01", "100")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}
	wg.Add(2) // 두 번 시작 시도

	// 첫 번째 시작
	go service.Run(ctx, wg)
	time.Sleep(500 * time.Millisecond)

	// 두 번째 시작 시도 (이미 실행 중이므로 즉시 반환되어야 함)
	go service.Run(ctx, wg)

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

// TestNotifyAPIService_NilNotificationSender는 nil NotificationSender 처리를 테스트합니다.
func TestNotifyAPIService_NilNotificationSender(t *testing.T) {
	appConfig := &g.AppConfig{}
	appConfig.NotifyAPI.WS.ListenPort = 18083
	appConfig.NotifyAPI.WS.TLSServer = false

	service := NewNotifyAPIService(appConfig, nil, "1.0.0", "2024-01-01", "100")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}
	wg.Add(1)

	// nil NotificationSender로 시작 시도 - error가 반환되어야 함
	err := service.Run(ctx, wg)

	assert.Error(t, err, "nil NotificationSender로 시작 시 error가 반환되어야 합니다")
	assert.Contains(t, err.Error(), "NotificationSender", "에러 메시지에 NotificationSender가 포함되어야 합니다")
}
