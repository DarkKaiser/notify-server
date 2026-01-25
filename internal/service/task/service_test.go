package task

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Mocks
// =============================================================================

// MockHandler는 테스트용 Handler 구현체입니다.
type MockHandler struct {
	id         contract.TaskID
	commandID  contract.TaskCommandID
	instanceID contract.TaskInstanceID
	canceled   bool
	cancelC    chan struct{}
	cancelOnce sync.Once
}

func (h *MockHandler) GetID() contract.TaskID                 { return h.id }
func (h *MockHandler) GetCommandID() contract.TaskCommandID   { return h.commandID }
func (h *MockHandler) GetInstanceID() contract.TaskInstanceID { return h.instanceID }
func (h *MockHandler) GetNotifierID() contract.NotifierID {
	return contract.NotifierID("test-notifier")
}
func (h *MockHandler) IsCanceled() bool                     { return h.canceled }
func (h *MockHandler) ElapsedTimeAfterRun() int64           { return 0 }
func (h *MockHandler) SetStorage(storage TaskResultStorage) {}

func (h *MockHandler) Run(ctx context.Context, notificationSender contract.NotificationSender, taskStopWG *sync.WaitGroup, taskDoneC chan<- contract.TaskInstanceID) {
	defer taskStopWG.Done()

	select {
	case <-ctx.Done():
	case <-h.cancelC:
	}

	taskDoneC <- h.instanceID
}

func (h *MockHandler) Cancel() {
	h.cancelOnce.Do(func() {
		h.canceled = true
		close(h.cancelC)
	})
}

func init() {
	// 정상 테스트용 Task 등록
	config := &Config{
		Commands: []*CommandConfig{
			{
				ID:            "TEST_COMMAND",
				AllowMultiple: true,
				NewSnapshot:   func() interface{} { return nil },
			},
		},
		NewTask: func(instanceID contract.TaskInstanceID, req *contract.TaskSubmitRequest, appConfig *config.AppConfig) (Handler, error) {
			return &MockHandler{
				id:         req.TaskID,
				commandID:  req.CommandID,
				instanceID: instanceID,
				cancelC:    make(chan struct{}),
			}, nil
		},
	}
	defaultRegistry.RegisterForTest("TEST_TASK", config)
}

// setupTestService는 테스트를 위한 공통 설정을 생성합니다.
//
// 반환값:
//   - Service: 설정된 서비스
//   - MockNotificationSender: Mock 알림 발송자
//   - Context: 컨텍스트
//   - CancelFunc: 취소 함수
//   - WaitGroup: 동기화용 WaitGroup
func setupTestService(t *testing.T) (*Service, *MockNotificationSender, context.Context, context.CancelFunc, *sync.WaitGroup) {
	appConfig := &config.AppConfig{}
	service := NewService(appConfig)
	mockSender := notificationmocks.NewMockNotificationSender(t)
	// Default expectations for async notifications (Task service is chatty)
	mockSender.On("Notify", mock.Anything, mock.Anything).Return(nil).Maybe()

	service.SetNotificationSender(mockSender)

	ctx, cancel := context.WithCancel(context.Background())
	serviceStopWG := &sync.WaitGroup{}
	serviceStopWG.Add(1)

	// Start를 동기적으로 호출하여 초기화가 완료될 때까지 대기
	// 별도의 고루틴(go service.Start)이나 Sleep이 필요하지 않음
	err := service.Start(ctx, serviceStopWG)
	require.NoError(t, err, "서비스 시작 실패")

	return service, mockSender, ctx, cancel, serviceStopWG
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
	service := NewService(appConfig)

	// 검증
	require.NotNil(t, service, "서비스가 생성되어야 합니다")
	require.Equal(t, appConfig, service.appConfig, "설정이 올바르게 설정되어야 합니다")
	require.False(t, service.running, "초기 상태에서는 실행 중이 아니어야 합니다")
	require.NotNil(t, service.handlers, "handlers가 초기화되어야 합니다")
	require.NotNil(t, service.taskSubmitC, "taskSubmitC 채널이 초기화되어야 합니다")
	require.NotNil(t, service.taskDoneC, "taskDoneC 채널이 초기화되어야 합니다")
	require.NotNil(t, service.taskCancelC, "taskCancelC 채널이 초기화되어야 합니다")
	require.NotNil(t, service.taskStopWG, "taskStopWG가 초기화되어야 합니다")
}

// =============================================================================
// Service Configuration Tests
// =============================================================================

// TestService_SetNotificationSender는 알림 발송자 설정을 검증합니다.
func TestService_SetNotificationSender(t *testing.T) {
	appConfig := &config.AppConfig{}
	service := NewService(appConfig)

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
	service, _, _, cancel, serviceStopWG := setupTestService(t)

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
	service, _, _, cancel, serviceStopWG := setupTestService(t)

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
	service, _, _, cancel, serviceStopWG := setupTestService(t)

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
	service, mockSender, _, cancel, serviceStopWG := setupTestService(t)

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
	service, _, _, cancel, serviceStopWG := setupTestService(t)

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
	service, _, _, cancel, serviceStopWG := setupTestService(t)

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
			for instanceID := range service.handlers {
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
