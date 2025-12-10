package task

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	// 테스트용 설정
	appConfig := &config.AppConfig{}

	// 서비스 생성
	service := NewService(appConfig)

	// 검증
	require.NotNil(t, service, "서비스가 생성되어야 합니다")
	require.Equal(t, appConfig, service.appConfig, "설정이 올바르게 설정되어야 합니다")
	require.False(t, service.running, "초기 상태에서는 실행 중이 아니어야 합니다")
	require.NotNil(t, service.taskHandlers, "taskHandlers가 초기화되어야 합니다")
	require.NotNil(t, service.taskRunC, "taskRunC 채널이 초기화되어야 합니다")
	require.NotNil(t, service.taskDoneC, "taskDoneC 채널이 초기화되어야 합니다")
	require.NotNil(t, service.taskCancelC, "taskCancelC 채널이 초기화되어야 합니다")
	require.NotNil(t, service.taskStopWaiter, "taskStopWaiter가 초기화되어야 합니다")
}

func TestTaskService_SetNotificationSender(t *testing.T) {
	appConfig := &config.AppConfig{}
	service := NewService(appConfig)

	mockSender := NewMockNotificationSender()

	// 알림 발송자 설정
	service.SetNotificationSender(mockSender)

	// 검증
	require.Equal(t, mockSender, service.notificationSender, "알림 발송자가 올바르게 설정되어야 합니다")
}

func TestTaskService_TaskRun_Success(t *testing.T) {
	appConfig := &config.AppConfig{}
	service := NewService(appConfig)
	mockSender := NewMockNotificationSender()
	service.SetNotificationSender(mockSender)

	// 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Start(ctx, serviceStopWaiter)

	// 서비스가 시작될 때까지 대기
	time.Sleep(100 * time.Millisecond)

	// Task 실행 요청
	err := service.Run(&RunRequest{
		TaskID:        "TEST_TASK",
		TaskCommandID: "TEST_COMMAND",
		NotifierID:    "test-notifier",
		NotifyOnStart: false,
		RunBy:         RunByUser,
	})

	// 검증
	require.NoError(t, err, "Task 실행 요청이 성공해야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWaiter.Wait()
}

func TestTaskService_TaskRunWithContext_Success(t *testing.T) {
	appConfig := &config.AppConfig{}
	service := NewService(appConfig)
	mockSender := NewMockNotificationSender()
	service.SetNotificationSender(mockSender)

	// 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Start(ctx, serviceStopWaiter)

	// 서비스가 시작될 때까지 대기
	time.Sleep(100 * time.Millisecond)

	// Task Context 생성
	taskCtx := NewTaskContext().With("test_key", "test_value")

	// Task 실행 요청
	err := service.Run(&RunRequest{
		TaskID:        "TEST_TASK",
		TaskCommandID: "TEST_COMMAND",
		NotifierID:    "test-notifier",
		NotifyOnStart: false,
		RunBy:         RunByUser,
		TaskContext:   taskCtx,
	})

	// 검증
	require.NoError(t, err, "Task 실행 요청이 성공해야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWaiter.Wait()
}

func TestTaskService_TaskCancel_Success(t *testing.T) {
	appConfig := &config.AppConfig{}
	service := NewService(appConfig)
	mockSender := NewMockNotificationSender()
	service.SetNotificationSender(mockSender)

	// 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Start(ctx, serviceStopWaiter)

	// 서비스가 시작될 때까지 대기
	time.Sleep(100 * time.Millisecond)

	// Task 취소 요청
	instanceID := InstanceID("test_instance_123")
	err := service.Cancel(instanceID)

	// 검증
	require.NoError(t, err, "Task 취소 요청이 성공해야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWaiter.Wait()
}

func TestTaskService_TaskRun_UnsupportedTask(t *testing.T) {
	appConfig := &config.AppConfig{}
	service := NewService(appConfig)
	mockSender := NewMockNotificationSender()
	service.SetNotificationSender(mockSender)

	// 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Start(ctx, serviceStopWaiter)

	// 서비스가 시작될 때까지 대기
	time.Sleep(100 * time.Millisecond)

	// 지원되지 않는 Task 실행 요청
	err := service.Run(&RunRequest{
		TaskID:        "UNSUPPORTED_TASK",
		TaskCommandID: "UNSUPPORTED_COMMAND",
		NotifierID:    "test-notifier",
		NotifyOnStart: false,
		RunBy:         RunByUser,
	})

	// 검증
	require.NoError(t, err, "Task 실행 요청 자체는 성공해야 합니다")

	// 알림이 발송되었는지 확인
	time.Sleep(100 * time.Millisecond)
	callCount := mockSender.GetNotifyWithTaskContextCallCount()
	require.Greater(t, callCount, 0, "에러 알림이 발송되어야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWaiter.Wait()
}

func TestTaskService_Concurrency(t *testing.T) {
	appConfig := &config.AppConfig{}
	service := NewService(appConfig)
	mockSender := NewMockNotificationSender()
	service.SetNotificationSender(mockSender)

	// 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Start(ctx, serviceStopWaiter)

	// 서비스가 시작될 때까지 대기
	time.Sleep(100 * time.Millisecond)

	// 동시에 여러 Task 실행 요청
	const numGoroutines = 10
	const numRequestsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numRequestsPerGoroutine; j++ {
				// Naver Shopping Task 실행 (AllowMultipleInstances=true)
				service.Run(&RunRequest{
					TaskID:        "TEST_TASK",
					TaskCommandID: "TEST_COMMAND",
					NotifierID:    "test-notifier",
					NotifyOnStart: false,
					RunBy:         RunByUser,
				})
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// 모든 요청이 처리될 때까지 잠시 대기
	time.Sleep(500 * time.Millisecond)

	// 실행 중인 핸들러 개수 확인
	require.True(t, service.running, "서비스가 계속 실행 중이어야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWaiter.Wait()
}

func TestTaskService_CancelConcurrency(t *testing.T) {
	appConfig := &config.AppConfig{}
	service := NewService(appConfig)
	mockSender := NewMockNotificationSender()
	service.SetNotificationSender(mockSender)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Start(ctx, serviceStopWaiter)

	time.Sleep(100 * time.Millisecond)

	// Task 실행 후 즉시 취소 반복
	const numIterations = 100
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for i := 0; i < numIterations; i++ {
			// Task 실행
			service.Run(&RunRequest{
				TaskID:        "TEST_TASK",
				TaskCommandID: "TEST_COMMAND",
				NotifierID:    "test-notifier",
				NotifyOnStart: false,
				RunBy:         RunByUser,
			})

			// 실행된 Task를 찾아서 취소 시도
			service.runningMu.Lock()
			for instanceID := range service.taskHandlers {
				go service.Cancel(instanceID)
			}
			service.runningMu.Unlock()

			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	require.True(t, service.running, "동시 실행/취소 반복 후에도 서비스가 실행 중이어야 합니다")

	cancel()
	serviceStopWaiter.Wait()
}
