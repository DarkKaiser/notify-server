package task

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"

	"github.com/darkkaiser/notify-server/internal/service/task/idgen"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/internal/service/task/provider/testutil"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Mocks
// =============================================================================

func registerServiceTestTask() {
	// 정상 테스트용 Task 등록
	config := &provider.TaskConfig{
		Commands: []*provider.TaskCommandConfig{
			{
				ID:            "TEST_COMMAND",
				AllowMultiple: true,
				NewSnapshot:   func() interface{} { return &struct{}{} },
			},
		},
		NewTask: func(p provider.NewTaskParams) (provider.Task, error) {
			return testutil.NewStubTask(p.Request.TaskID, p.Request.CommandID, p.InstanceID), nil
		},
	}
	provider.RegisterForTest("TEST_TASK", config)
}

// setupTestService는 테스트를 위한 공통 설정을 생성합니다.
//
// 반환값:
//   - Service: 설정된 서비스
//   - MockNotificationSender: Mock 알림 발송자
//   - MockIDGenerator: Mock ID 생성자
//   - Context: 컨텍스트
//   - CancelFunc: 취소 함수
//   - WaitGroup: 동기화용 WaitGroup
func setupTestService(t *testing.T) (*Service, *notificationmocks.MockNotificationSender, *contractmocks.MockIDGenerator, context.Context, context.CancelFunc, *sync.WaitGroup) {
	registerServiceTestTask()
	appConfig := &config.AppConfig{}

	mockIDGen := new(contractmocks.MockIDGenerator)
	// 기본적으로 임의의 ID를 반환하도록 설정 (테스트 편의성)
	mockIDGen.On("New").Return(contract.TaskInstanceID("mjz7373-test-instance")).Maybe()

	mockStorage := new(contractmocks.MockTaskResultStore)
	service := NewService(appConfig, mockIDGen, mockStorage)
	mockSender := notificationmocks.NewMockNotificationSender(t)
	// Default expectations for async notifications (Task service is chatty)
	mockSender.On("Notify", mock.Anything, mock.Anything).Return(nil).Maybe()

	service.SetNotificationSender(mockSender)

	ctx, cancel := context.WithCancel(context.Background())
	serviceStopWG := &sync.WaitGroup{}
	serviceStopWG.Add(1)

	// Start를 동기적으로 호출하여 초기화가 완료될 때까지 대기
	err := service.Start(ctx, serviceStopWG)
	require.NoError(t, err, "서비스 시작 실패")

	return service, mockSender, mockIDGen, ctx, cancel, serviceStopWG
}

// =============================================================================
// Service Creation Tests
// =============================================================================

// TestNewService는 서비스 생성을 검증합니다.
//
// 검증 항목:
//   - 서비스 생성
//   - 초기 상태 확인
//   - 채널 초기화
func TestNewService(t *testing.T) {
	// 테스트용 설정
	appConfig := &config.AppConfig{}

	// 서비스 생성
	mockIDGen := new(contractmocks.MockIDGenerator)
	mockStorage := new(contractmocks.MockTaskResultStore)
	service := NewService(appConfig, mockIDGen, mockStorage)

	// 검증
	require.NotNil(t, service, "서비스가 생성되어야 합니다")
	require.Equal(t, appConfig, service.appConfig, "설정이 올바르게 설정되어야 합니다")
	require.False(t, service.running, "초기 상태에서는 실행 중이 아니어야 합니다")
	require.NotNil(t, service.tasks, "handlers가 초기화되어야 합니다")
	require.NotNil(t, service.taskSubmitC, "taskSubmitC 채널이 초기화되어야 합니다")
	require.NotNil(t, service.taskDoneC, "taskDoneC 채널이 초기화되어야 합니다")
	require.NotNil(t, service.taskCancelC, "taskCancelC 채널이 초기화되어야 합니다")

}

// =============================================================================
// Service Configuration Tests
// =============================================================================

// TestService_SetNotificationSender는 알림 발송자 설정을 검증합니다.
func TestService_SetNotificationSender(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockIDGen := new(contractmocks.MockIDGenerator)
	mockStorage := new(contractmocks.MockTaskResultStore)
	service := NewService(appConfig, mockIDGen, mockStorage)

	mockSender := notificationmocks.NewMockNotificationSender(t)

	// 알림 발송자 설정
	service.SetNotificationSender(mockSender)

	// 검증
	require.Equal(t, mockSender, service.notificationSender, "알림 발송자가 올바르게 설정되어야 합니다")
}

// =============================================================================
// Task Execution Tests
// =============================================================================

// TestService_TaskRun_Success는 Task 정상 실행을 검증합니다.
func TestService_TaskRun_Success(t *testing.T) {
	service, _, _, _, cancel, serviceStopWG := setupTestService(t)

	// Task 실행 요청
	err := service.Submit(context.Background(), &contract.TaskSubmitRequest{
		TaskID:        "TEST_TASK",
		CommandID:     "TEST_COMMAND",
		NotifierID:    contract.NotifierID("test-notifier"),
		NotifyOnStart: false,
		RunBy:         contract.TaskRunByUser,
	})

	// 검증
	require.NoError(t, err, "Task 실행 요청이 성공해야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWG.Wait()
}

// TestService_TaskRunWithContext_Success는 Task Context와 함께 Task 실행을 검증합니다.
func TestService_TaskRunWithContext_Success(t *testing.T) {
	service, _, _, _, cancel, serviceStopWG := setupTestService(t)

	// Task Context 생성
	// taskCtx := contract.NewTaskContext().With("test_key", "test_value")
	taskCtx := context.WithValue(context.Background(), "test_key", "test_value")

	// Task 실행 요청
	err := service.Submit(taskCtx, &contract.TaskSubmitRequest{
		TaskID:        "TEST_TASK",
		CommandID:     "TEST_COMMAND",
		NotifierID:    contract.NotifierID("test-notifier"),
		NotifyOnStart: false,
		RunBy:         contract.TaskRunByUser,
	})

	// 검증
	require.NoError(t, err, "Task 실행 요청이 성공해야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWG.Wait()
}

// TestService_TaskCancel_Success는 Task 취소를 검증합니다.
func TestService_TaskCancel_Success(t *testing.T) {
	service, _, _, _, cancel, serviceStopWG := setupTestService(t)

	// Task 취소 요청
	instanceID := contract.TaskInstanceID("test_instance_123")
	err := service.Cancel(instanceID)

	// 검증
	require.NoError(t, err, "Task 취소 요청이 성공해야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWG.Wait()
}

// TestService_TaskRun_UnsupportedTask는 지원하지 않는 Task 처리를 검증합니다.
func TestService_TaskRun_UnsupportedTask(t *testing.T) {
	service, mockSender, _, _, cancel, serviceStopWG := setupTestService(t)

	// 지원되지 않는 Task 실행 요청
	err := service.Submit(context.Background(), &contract.TaskSubmitRequest{
		TaskID:        "UNSUPPORTED_TASK",
		CommandID:     "UNSUPPORTED_COMMAND",
		NotifierID:    contract.NotifierID("test-notifier"),
		NotifyOnStart: false,
		RunBy:         contract.TaskRunByUser,
	})

	// 검증
	require.Error(t, err, "지원하지 않는 Task는 즉시 에러를 반환해야 합니다")
	require.Contains(t, err.Error(), "지원하지 않는", "에러 메시지에 원인이 포함되어야 합니다")

	// 큐에 들어가지 않았으므로 비동기 알림은 발송되지 않아야 합니다 (단, Fail Fast로 인해 호출자가 직접 처리 가능)
	time.Sleep(100 * time.Millisecond) // 알림 발송 대기 (비동기 확인용)
	callCount := len(mockSender.Calls)
	require.Equal(t, 0, callCount, "큐에 들어가지 않았으므로 비동기 알림은 없어야 합니다")

	// 서비스 중지
	cancel()
	serviceStopWG.Wait()
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestService_Concurrency는 동시성 처리를 검증합니다.
func TestService_Concurrency(t *testing.T) {
	// 동시성 테스트에서는 고유한 ID 생성이 필수적이므로 Mock 대신 실제 Generator를 사용하거나
	// 매번 다른 값을 반환하는 Stub을 사용해야 합니다. 여기서는 실제 Generator를 사용합니다.
	registerServiceTestTask()
	appConfig := &config.AppConfig{}

	// 실제 ID 생성기 사용
	service := NewService(appConfig, idgen.New(), new(contractmocks.MockTaskResultStore))

	mockSender := notificationmocks.NewMockNotificationSender(t)
	// 비동기 알림 허용
	mockSender.On("Notify", mock.Anything, mock.Anything).Return(nil).Maybe()
	service.SetNotificationSender(mockSender)

	ctx, cancel := context.WithCancel(context.Background())
	serviceStopWG := &sync.WaitGroup{}
	serviceStopWG.Add(1)

	err := service.Start(ctx, serviceStopWG)
	require.NoError(t, err)

	// 동시에 여러 Task 실행 요청
	const numGoroutines = 10
	const numRequestsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numRequestsPerGoroutine; j++ {
				// Naver Shopping Task 실행 (AllowMultiple=true)
				service.Submit(context.Background(), &contract.TaskSubmitRequest{
					TaskID:        "TEST_TASK",
					CommandID:     "TEST_COMMAND",
					NotifierID:    contract.NotifierID("test-notifier"),
					NotifyOnStart: false,
					RunBy:         contract.TaskRunByUser,
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
	serviceStopWG.Wait()
}

// TestService_CancelConcurrency는 동시 취소 처리를 검증합니다.
func TestService_CancelConcurrency(t *testing.T) {
	// 동시성 테스트를 위해 실제 Generator 사용
	registerServiceTestTask()
	appConfig := &config.AppConfig{}
	service := NewService(appConfig, idgen.New(), new(contractmocks.MockTaskResultStore))

	mockSender := notificationmocks.NewMockNotificationSender(t)
	mockSender.On("Notify", mock.Anything, mock.Anything).Return(nil).Maybe()
	service.SetNotificationSender(mockSender)

	ctx, cancel := context.WithCancel(context.Background())
	serviceStopWG := &sync.WaitGroup{}
	serviceStopWG.Add(1)

	err := service.Start(ctx, serviceStopWG)
	require.NoError(t, err)

	// Task 실행 후 즉시 취소 반복
	const numIterations = 100
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for i := 0; i < numIterations; i++ {
			// Task 실행
			service.Submit(context.Background(), &contract.TaskSubmitRequest{
				TaskID:        "TEST_TASK",
				CommandID:     "TEST_COMMAND",
				NotifierID:    contract.NotifierID("test-notifier"),
				NotifyOnStart: false,
				RunBy:         contract.TaskRunByUser,
			})

			// 실행된 Task를 찾아서 취소 시도
			service.runningMu.Lock()
			for instanceID := range service.tasks {
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
	serviceStopWG.Wait()
}

// TestService_Submit_Timeout tests that Submit returns a timeout error when the queue is full
// and the context deadline is exceeded.
func TestService_Submit_Timeout(t *testing.T) {
	// 1. Setup Service with a slow task initializer to simulate a blocked consumer
	registerServiceTestTask() // Register basic tasks

	// Register a slow task
	provider.RegisterForTest("SLOW_TASK", &provider.TaskConfig{
		Commands: []*provider.TaskCommandConfig{
			{ID: "SLOW_CMD", AllowMultiple: true},
		},
		NewTask: func(p provider.NewTaskParams) (provider.Task, error) {
			// Simulate slow initialization to block the consumer (run0 loop)
			time.Sleep(100 * time.Millisecond)
			return testutil.NewStubTask(p.Request.TaskID, p.Request.CommandID, p.InstanceID), nil
		},
	})

	appConfig := &config.AppConfig{}
	stubIDGen := &testutil.StubIDGenerator{}

	service := NewService(appConfig, stubIDGen, new(contractmocks.MockTaskResultStore))
	mockSender := notificationmocks.NewMockNotificationSender(t)
	// mockSender doesn't need expectations as we won't trigger notifications in this short test
	service.SetNotificationSender(mockSender)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var serviceStopWG sync.WaitGroup
	serviceStopWG.Add(1)

	err := service.Start(ctx, &serviceStopWG)
	require.NoError(t, err)
	defer func() {
		cancel()
		serviceStopWG.Wait()
	}()

	// 2. Fill the buffer (size 10)
	// We send 12 requests.
	// Consumer takes 100ms per request.
	// Filling 10 requests takes negligible time (pushes to channel).
	// 11th request will block until consumer picks up 1st (starts processing).
	// But consumer is blocked on "Processing 1st" for 100ms.
	// So 11th request blocks for ~100ms.

	// Send 11 requests to fill buffer and engage consumer
	for i := 0; i < 11; i++ {
		go service.Submit(context.Background(), &contract.TaskSubmitRequest{
			TaskID:     "SLOW_TASK",
			CommandID:  "SLOW_CMD",
			NotifierID: contract.NotifierID("n"),
			RunBy:      contract.TaskRunByUser,
		})
	}

	// Give a moment for the channel to fill up
	time.Sleep(10 * time.Millisecond)

	// 3. Send the 12th request with a short timeout
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer timeoutCancel()

	start := time.Now()
	err = service.Submit(timeoutCtx, &contract.TaskSubmitRequest{
		TaskID:     "SLOW_TASK",
		CommandID:  "SLOW_CMD",
		NotifierID: contract.NotifierID("n"),
		RunBy:      contract.TaskRunByUser,
	})
	elapsed := time.Since(start)

	// 4. Verify
	require.Error(t, err, "Submit should coincide with timeout")
	require.ErrorIs(t, err, context.DeadlineExceeded, "Error should be DeadlineExceeded")

	// Ensure it didn't block forever (should be around 20ms)
	// We allow some buffer for test execution overhead
	require.Less(t, elapsed, 200*time.Millisecond, "Submit blocked longer than expected")
}

// =============================================================================
// Lifecycle and Edge Case Tests
// =============================================================================

// TestService_Start_MissingDependencies는 의존성이 누락된 상태에서 Start 호출 시 패닉/에러가 발생하는지 검증합니다.
func TestService_Start_MissingDependencies(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockIDGen := new(contractmocks.MockIDGenerator)
	mockStorage := new(contractmocks.MockTaskResultStore)
	service := NewService(appConfig, mockIDGen, mockStorage)

	// NotificationSender를 주입하지 않고 Start 시도
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serviceStopWG := &sync.WaitGroup{}
	serviceStopWG.Add(1)

	err := service.Start(ctx, serviceStopWG)
	require.Error(t, err, "NotificationSender가 없으면 Start는 실패해야 합니다")
	require.Contains(t, err.Error(), "NotificationSender", "원인을 나타내는 메시지가 포함되어야 합니다")

	// Start 실패 시 WaitGroup.Done()을 호출하는지 직접 확인하기 위해 대기
	// 타임아웃 방지를 위해 별도 고루틴에서 대기
	wgDone := make(chan struct{})
	go func() {
		serviceStopWG.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		// 성공
	case <-time.After(1 * time.Second):
		t.Fatal("Start 실패 시 serviceStopWG.Done()이 호출되지 않았습니다")
	}
}

// TestService_Submit_WhenStopped는 서비스가 실행 중이 아닐 때 Submit 요청이 거부되는지 검증합니다.
func TestService_Submit_WhenStopped(t *testing.T) {
	service, _, _, _, _, _ := setupTestService(t) // 일단 시작하지만 내부에서 중지 중인지 시뮬레이션

	// 인위적으로 running 플래그를 false로 변경
	service.runningMu.Lock()
	service.running = false
	service.runningMu.Unlock()

	err := service.Submit(context.Background(), &contract.TaskSubmitRequest{
		TaskID:     "TEST_TASK",
		CommandID:  "TEST_COMMAND",
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByUser,
	})

	require.Error(t, err, "서비스가 중지 중일 때는 Submit이 거부되어야 합니다")
	require.Contains(t, err.Error(), "실행 중이지 않아", "거부 사유가 명확해야 합니다")
}

// TestService_Cancel_WhenStopped는 서비스가 중지 중일 때 Cancel 요청이 무시(에러 반환 없이 early return)되는지 검증합니다.
func TestService_Cancel_WhenStopped(t *testing.T) {
	service, _, _, _, _, _ := setupTestService(t)

	service.runningMu.Lock()
	service.running = false
	service.runningMu.Unlock()

	err := service.Cancel(contract.TaskInstanceID("some_id"))
	require.Error(t, err, "서비스 중지 중 Cancel은 방어 로직에 의해 에러를 반환해야 합니다")
	require.Contains(t, err.Error(), "실행 중이지 않아")
}

// TestService_RejectIfAlreadyRunning는 AllowMultiple=false인 Task의 중복 실행 방지 로직을 검증합니다.
func TestService_RejectIfAlreadyRunning(t *testing.T) {
	appConfig := &config.AppConfig{}

	mockIDGen := new(contractmocks.MockIDGenerator)
	mockIDGen.On("New").Return(contract.TaskInstanceID("singleton-instance-id")).Maybe()
	mockStorage := new(contractmocks.MockTaskResultStore)
	service := NewService(appConfig, mockIDGen, mockStorage)

	mockSender := notificationmocks.NewMockNotificationSender(t)
	service.SetNotificationSender(mockSender)

	ctx, cancel := context.WithCancel(context.Background())
	serviceStopWG := &sync.WaitGroup{}
	serviceStopWG.Add(1)

	err := service.Start(ctx, serviceStopWG)
	require.NoError(t, err)

	defer func() {
		cancel()
		serviceStopWG.Wait()
	}()

	// 테스트용 싱글톤 Task 등록 (AllowMultiple=false)
	provider.RegisterForTest("SINGLETON_TASK", &provider.TaskConfig{
		Commands: []*provider.TaskCommandConfig{
			{ID: "SINGLETON_CMD", AllowMultiple: false},
		},
		NewTask: func(p provider.NewTaskParams) (provider.Task, error) {
			// 작업이 끝나지 않도록 영원히 블록되는 StubTask 생성
			task := testutil.NewStubTask(p.Request.TaskID, p.Request.CommandID, p.InstanceID)
			task.RunFunc = func(ctx context.Context, ns contract.NotificationSender) {
				<-ctx.Done() // 취소될 때까지 대기
			}
			return task, nil
		},
	})

	// 1차 요청 (성공해야 함)
	err = service.Submit(context.Background(), &contract.TaskSubmitRequest{
		TaskID:     "SINGLETON_TASK",
		CommandID:  "SINGLETON_CMD",
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByUser,
	})
	require.NoError(t, err, "첫 번째 요청은 성공해야 합니다")

	// Task가 실행되어 `s.tasks` 맵에 등록될 시간을 잠깐 줍니다
	time.Sleep(100 * time.Millisecond)

	// 두 번째 요청에 의한 중복 반려(비동기) 알림 전송을 감지하기 위한 채널
	notifyCalled := make(chan struct{})

	mockSender.On("Notify", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
		return n.ErrorOccurred == false && n.Cancelable == true
	})).Run(func(args mock.Arguments) {
		close(notifyCalled)
	}).Return(nil).Once()

	// 2차 요청 (중복 거부되어 알림이 발송되어야 함)
	err2 := service.Submit(context.Background(), &contract.TaskSubmitRequest{
		TaskID:     "SINGLETON_TASK",
		CommandID:  "SINGLETON_CMD",
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByUser,
	})
	require.NoError(t, err2, "Submit 자체는 성공하지만 비동기 큐에서 거부됩니다")

	// 비동기 알림 전송 대기
	select {
	case <-notifyCalled:
		// 성공
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for AlreadyRunning notification")
	}

	mockSender.AssertExpectations(t)
}

// =============================================================================
// Error Handling and Edge Case Tests
// =============================================================================

// TestService_IDCollisionMaxRetries는 ID 생성 충돌이 한계치(MaxRetries)를 초과할 때의 에러 처리와 알림을 검증합니다.
func TestService_IDCollisionMaxRetries(t *testing.T) {
	appConfig := &config.AppConfig{}

	// 항상 동일한 ID만 반환하여 고의로 무한 충돌을 유발하는 Mock Generator
	mockIDGen := new(contractmocks.MockIDGenerator)
	mockIDGen.On("New").Return(contract.TaskInstanceID("always-colliding-id"))

	mockStorage := new(contractmocks.MockTaskResultStore)
	service := NewService(appConfig, mockIDGen, mockStorage)

	// 비동기 통지 호출을 감지하기 위한 채널
	notifyCalled := make(chan struct{})

	mockSender := notificationmocks.NewMockNotificationSender(t)
	// ID 생성 완전 실패 시 발송되는 에러 알림을 기대함
	mockSender.On("Notify", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
		return n.ErrorOccurred == true && n.Message == "시스템 오류로 작업 실행에 실패했습니다. (ID 충돌)"
	})).Run(func(args mock.Arguments) {
		close(notifyCalled)
	}).Return(nil).Once()
	service.SetNotificationSender(mockSender)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	err := service.Start(ctx, &wg)
	require.NoError(t, err)

	// 테스트용 Task를 등록해야 FindConfig 검증을 통과하여 ID 발급 로직까지 진행됩니다.
	provider.RegisterForTest("TEST_TASK", &provider.TaskConfig{
		Commands: []*provider.TaskCommandConfig{
			{ID: "TEST_COMMAND", AllowMultiple: true},
		},
		NewTask: func(p provider.NewTaskParams) (provider.Task, error) {
			return testutil.NewStubTask(p.Request.TaskID, p.Request.CommandID, p.InstanceID), nil
		},
	})

	// 이미 동일한 ID로 Task 1개를 미리 등록해 둡니다.
	service.runningMu.Lock()
	service.tasks["always-colliding-id"] = testutil.NewStubTask("T", "C", "always-colliding-id")
	service.runningMu.Unlock()

	// 두 번째 Task를 제출합니다. 이것은 ID 생성기에서 계속 같은 ID를 받아오므로 maxRetries(3)번 모두 실패해야 합니다.
	err = service.Submit(context.Background(), &contract.TaskSubmitRequest{
		TaskID:     "TEST_TASK",
		CommandID:  "TEST_COMMAND",
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByUser,
	})
	require.NoError(t, err) // Submit 자체는 큐에 등록될 뿐 에러를 즉시 반환하지 않습니다.

	// 비동기 처리가 끝나 에러 알림이 발송될 때까지 대기
	select {
	case <-notifyCalled:
		// 성공
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for ID collision error notification")
	}

	// mockSender의 Notify가 호출되었는지 검증
	mockSender.AssertExpectations(t)
}

// TestService_PanicInSubmit는 Submit 메서드 호출 중 내부 로직에서 발생한 패닉이 복구(Recover)되는지 검증합니다.
func TestService_PanicInSubmit(t *testing.T) {
	service, _, _, _, _, _ := setupTestService(t)

	// 억지로 panic을 유도합니다. (예: req 파라미터가 nil일 때)
	// *Service.Submit() 내부에서 panic을 처리하는 defer가 동작하는지 확인합니다.
	err := service.Submit(context.Background(), nil)

	require.Error(t, err, "nil req에 대한 방어 로직이 에러를 뱉는지 확인")
	require.Contains(t, err.Error(), "요청을 처리할 수 없습니다")
}

// TestService_HandleTaskEvent_UnknownID는 존재하지 않는 Task ID에 대한 Done/Cancel 이벤트가 안전하게 무시되는지 검증합니다.
func TestService_HandleTaskEvent_UnknownID(t *testing.T) {
	appConfig := &config.AppConfig{}
	mockIDGen := new(contractmocks.MockIDGenerator)
	mockStorage := new(contractmocks.MockTaskResultStore)
	service := NewService(appConfig, mockIDGen, mockStorage)

	mockSender := notificationmocks.NewMockNotificationSender(t)
	service.SetNotificationSender(mockSender)

	ctx, cancel := context.WithCancel(context.Background())
	serviceStopWG := &sync.WaitGroup{}
	serviceStopWG.Add(1)

	// 비동기 통지 호출을 감지하기 위한 채널
	notifyCalled := make(chan struct{})

	// 없는 ID에 대해 취소를 요청하면 에러 메시지 알림이 가야 합니다.
	mockSender.On("Notify", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
		return n.ErrorOccurred == true && n.Cancelable == false
	})).Run(func(args mock.Arguments) {
		close(notifyCalled)
	}).Return(nil).Once()

	err := service.Start(ctx, serviceStopWG)
	require.NoError(t, err)

	defer func() {
		cancel()
		serviceStopWG.Wait()
	}()

	err = service.Cancel("unknown-instance-id")
	require.NoError(t, err, "존재하지 않는 ID에 대한 Cancel 요청 자체는 에러가 발생하지 않아야 합니다")

	// 비동기 알림 전송이 완료될 때까지 안전하게 대기 (최대 1초)
	select {
	case <-notifyCalled:
		// 성공
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for mockSender.Notify to be called")
	}

	mockSender.AssertExpectations(t)
}
