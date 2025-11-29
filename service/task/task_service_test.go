package task

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/g"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	// 테스트용 설정
	config := &g.AppConfig{}

	// 서비스 생성
	service := NewService(config)

	// 검증
	require.NotNil(t, service, "서비스가 생성되어야 합니다")
	require.Equal(t, config, service.config, "설정이 올바르게 설정되어야 합니다")
	require.False(t, service.running, "초기 상태에서는 실행 중이 아니어야 합니다")
	require.NotNil(t, service.taskHandlers, "taskHandlers가 초기화되어야 합니다")
	require.NotNil(t, service.taskRunC, "taskRunC 채널이 초기화되어야 합니다")
	require.NotNil(t, service.taskDoneC, "taskDoneC 채널이 초기화되어야 합니다")
	require.NotNil(t, service.taskCancelC, "taskCancelC 채널이 초기화되어야 합니다")
	require.NotNil(t, service.taskStopWaiter, "taskStopWaiter가 초기화되어야 합니다")
}

func TestTaskService_SetTaskNotificationSender(t *testing.T) {
	config := &g.AppConfig{}
	service := NewService(config)

	mockSender := NewMockTaskNotificationSender()

	// 알림 발송자 설정
	service.SetTaskNotificationSender(mockSender)

	// 검증
	require.Equal(t, mockSender, service.taskNotificationSender, "알림 발송자가 올바르게 설정되어야 합니다")
}

func TestTaskService_TaskRun_Success(t *testing.T) {
	config := &g.AppConfig{}
	service := NewService(config)
	mockSender := NewMockTaskNotificationSender()
	service.SetTaskNotificationSender(mockSender)

	// 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Run(ctx, serviceStopWaiter)

	// 서비스가 시작될 때까지 대기
	time.Sleep(100 * time.Millisecond)

	// Task 실행 요청
	taskID := TidNaverShopping
	commandID := TcidNaverShoppingWatchPriceAny
	notifierID := "test-notifier"

	succeeded := service.TaskRun(taskID, commandID, notifierID, false, TaskRunByUser)

	// 검증
	require.True(t, succeeded, "Task 실행 요청이 성공해야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWaiter.Wait()
}

func TestTaskService_TaskRunWithContext_Success(t *testing.T) {
	config := &g.AppConfig{}
	service := NewService(config)
	mockSender := NewMockTaskNotificationSender()
	service.SetTaskNotificationSender(mockSender)

	// 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Run(ctx, serviceStopWaiter)

	// 서비스가 시작될 때까지 대기
	time.Sleep(100 * time.Millisecond)

	// Task Context 생성
	taskCtx := NewContext().With("test_key", "test_value")

	// Task 실행 요청
	taskID := TidNaverShopping
	commandID := TcidNaverShoppingWatchPriceAny
	notifierID := "test-notifier"

	succeeded := service.TaskRunWithContext(taskID, commandID, taskCtx, notifierID, false, TaskRunByUser)

	// 검증
	require.True(t, succeeded, "Task 실행 요청이 성공해야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWaiter.Wait()
}

func TestTaskService_TaskCancel_Success(t *testing.T) {
	config := &g.AppConfig{}
	service := NewService(config)
	mockSender := NewMockTaskNotificationSender()
	service.SetTaskNotificationSender(mockSender)

	// 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Run(ctx, serviceStopWaiter)

	// 서비스가 시작될 때까지 대기
	time.Sleep(100 * time.Millisecond)

	// Task 취소 요청
	instanceID := TaskInstanceID("test_instance_123")
	succeeded := service.TaskCancel(instanceID)

	// 검증
	require.True(t, succeeded, "Task 취소 요청이 성공해야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWaiter.Wait()
}

func TestTaskService_TaskRun_UnsupportedTask(t *testing.T) {
	config := &g.AppConfig{}
	service := NewService(config)
	mockSender := NewMockTaskNotificationSender()
	service.SetTaskNotificationSender(mockSender)

	// 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Run(ctx, serviceStopWaiter)

	// 서비스가 시작될 때까지 대기
	time.Sleep(100 * time.Millisecond)

	// 지원되지 않는 Task 실행 요청
	taskID := TaskID("UNSUPPORTED_TASK")
	commandID := TaskCommandID("UNSUPPORTED_COMMAND")
	notifierID := "test-notifier"

	succeeded := service.TaskRun(taskID, commandID, notifierID, false, TaskRunByUser)

	// 검증
	require.True(t, succeeded, "Task 실행 요청 자체는 성공해야 합니다")

	// 알림이 발송되었는지 확인
	time.Sleep(100 * time.Millisecond)
	callCount := mockSender.GetNotifyWithTaskContextCallCount()
	require.Greater(t, callCount, 0, "에러 알림이 발송되어야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWaiter.Wait()
}

func TestTaskService_Concurrency(t *testing.T) {
	config := &g.AppConfig{}
	service := NewService(config)
	mockSender := NewMockTaskNotificationSender()
	service.SetTaskNotificationSender(mockSender)

	// 서비스 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Run(ctx, serviceStopWaiter)

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
				service.TaskRun(TidNaverShopping, TcidNaverShoppingWatchPriceAny, "test-notifier", false, TaskRunByUser)
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// 모든 요청이 처리될 때까지 잠시 대기
	time.Sleep(500 * time.Millisecond)

	// 실행 중인 핸들러 개수 확인 (정확한 개수는 타이밍에 따라 다를 수 있으므로 에러가 없는지만 확인)
	// 하지만 적어도 서비스가 죽지 않고 살아있어야 함
	require.True(t, service.running, "서비스가 계속 실행 중이어야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWaiter.Wait()
}

func TestTaskService_CancelConcurrency(t *testing.T) {
	config := &g.AppConfig{}
	service := NewService(config)
	mockSender := NewMockTaskNotificationSender()
	service.SetTaskNotificationSender(mockSender)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serviceStopWaiter := &sync.WaitGroup{}
	serviceStopWaiter.Add(1)
	go service.Run(ctx, serviceStopWaiter)

	time.Sleep(100 * time.Millisecond)

	// Task 실행 후 즉시 취소 반복
	const numIterations = 100
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for i := 0; i < numIterations; i++ {
			// Task 실행 (오래 걸리는 작업 시뮬레이션은 어렵지만, 등록은 됨)
			service.TaskRun(TidNaverShopping, TcidNaverShoppingWatchPriceAny, "test-notifier", false, TaskRunByUser)

			// 실행된 Task를 찾아서 취소 시도 (ID를 알기 어려우므로 모든 핸들러 취소 시도)
			service.runningMu.Lock()
			for instanceID := range service.taskHandlers {
				go service.TaskCancel(instanceID)
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
