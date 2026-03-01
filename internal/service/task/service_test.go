package task

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"

	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/internal/service/task/provider/testutil"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Configuration & Helpers
// =============================================================================

// registerTestTask는 Provider에 테스트 전용 Task를 등록합니다.
// setupTestContext 등에서 한 번만 호출하도록 설계해야 합니다.
func registerTestTask(t *testing.T, taskID contract.TaskID, cmdID contract.TaskCommandID, allowMultiple bool, runFunc func(context.Context, contract.NotificationSender), onTaskCreated ...func(<-chan struct{})) {
	t.Helper()

	config := &provider.TaskConfig{
		Commands: []*provider.TaskCommandConfig{
			{
				ID:            cmdID,
				AllowMultiple: allowMultiple,
				NewSnapshot:   func() interface{} { return &struct{}{} },
			},
		},
		NewTask: func(p provider.NewTaskParams) (provider.Task, error) {
			task := testutil.NewStubTask(p.Request.TaskID, p.Request.CommandID, p.InstanceID)

			if runFunc != nil {
				task.RunFunc = runFunc
			}

			if len(onTaskCreated) > 0 && onTaskCreated[0] != nil {
				onTaskCreated[0](task.WaitCanceled())
			}

			// Service 테스트에서 eventLoop가 정상 종료를 감지하려면
			// Task.Run() 종료 시 taskDoneC 통신이 모사되어야 합니다.
			return &eventLoopTestTask{StubTask: task}, nil
		},
	}

	provider.RegisterForTest(taskID, config)
}

// eventLoopTestTask는 StubTask를 감싸고, Run 메서드 종료 시 Service의 cleanup을 유도합니다.
type eventLoopTestTask struct {
	*testutil.StubTask
}

// Run overrides StubTask.Run to trigger the behavior expected by service.go's handleTaskDone.
// It simply runs the original task and, upon return, nothing is explicitly needed
// because `registerAndRunTask` in `service.go` wraps this in a goroutine with `defer s.taskDoneC <- t.InstanceID()`.
func (e *eventLoopTestTask) Run(ctx context.Context, ns contract.NotificationSender) {
	e.StubTask.Run(ctx, ns)
}

// ServiceTestContext는 테스트에 필요한 공통 객체들을 묶어서 관리합니다.
type ServiceTestContext struct {
	Service        *Service
	MockSender     *notificationmocks.MockNotificationSender
	MockIDGen      *contractmocks.MockIDGenerator
	MockStorage    *contractmocks.MockTaskResultStore
	Context        context.Context
	Cancel         context.CancelFunc
	StopWG         *sync.WaitGroup
	StartCompleted bool
}

// setupTestContext는 테스트를 위한 Service 환경을 구축합니다.
// autoStart가 true이면 srvCtx.Context와 함께 service.Start()까지 완료한 상태로 반환합니다.
func setupTestContext(t *testing.T, autoStart bool) *ServiceTestContext {
	t.Helper()

	appConfig := &config.AppConfig{}

	mockIDGen := new(contractmocks.MockIDGenerator)
	// 기본 ID 반환 동작 (이후 개별 테스트에서 덮어씌울 수 있음)
	mockIDGen.On("New").Return(contract.TaskInstanceID("mocked-instance-id")).Maybe()

	mockStorage := new(contractmocks.MockTaskResultStore)
	service := NewService(appConfig, mockIDGen, mockStorage)

	mockSender := notificationmocks.NewMockNotificationSender(t)
	// 알림 발송은 기본적으로 성공(nil) 처리 (테스트 중 알림 횟수나 조건을 강하게 걸고 싶다면 개별 셋업)
	mockSender.On("Notify", mock.Anything, mock.Anything).Return(nil).Maybe()
	service.SetNotificationSender(mockSender)

	ctx, cancel := context.WithCancel(context.Background())
	stopWG := &sync.WaitGroup{}

	srvCtx := &ServiceTestContext{
		Service:     service,
		MockSender:  mockSender,
		MockIDGen:   mockIDGen,
		MockStorage: mockStorage,
		Context:     ctx,
		Cancel:      cancel,
		StopWG:      stopWG,
	}

	if autoStart {
		stopWG.Add(1)
		err := service.Start(ctx, stopWG)
		require.NoError(t, err, "서비스 시작 실패")
		srvCtx.StartCompleted = true
	}

	return srvCtx
}

// Teardown은 Graceful Shutdown을 실행하고, 대기하는 헬퍼입니다. defer로 사용합니다.
func (ctx *ServiceTestContext) Teardown() {
	ctx.Cancel()
	if ctx.StartCompleted {
		ctx.StopWG.Wait()
	}
}

// =============================================================================
// Initialization & Configuration Tests
// =============================================================================

func TestNewServiceAndInitialization(t *testing.T) {
	t.Parallel()

	t.Run("성공: Service 정상 초기화", func(t *testing.T) {
		appConfig := &config.AppConfig{}
		mockIDGen := new(contractmocks.MockIDGenerator)
		mockStorage := new(contractmocks.MockTaskResultStore)

		service := NewService(appConfig, mockIDGen, mockStorage)
		require.NotNil(t, service)
		require.Equal(t, appConfig, service.appConfig)
		require.False(t, service.running)
		require.NotNil(t, service.taskSubmitC)
		require.NotNil(t, service.taskDoneC)
		require.NotNil(t, service.taskCancelC)
	})

	t.Run("패닉: IDGenerator 누락", func(t *testing.T) {
		require.PanicsWithValue(t, "IDGenerator는 필수입니다", func() {
			NewService(&config.AppConfig{}, nil, new(contractmocks.MockTaskResultStore))
		})
	})
}

func TestService_Start(t *testing.T) {
	t.Parallel()

	t.Run("에러: NotificationSender 미설정 시 실패", func(t *testing.T) {
		service := NewService(&config.AppConfig{}, new(contractmocks.MockIDGenerator), nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var wg sync.WaitGroup
		wg.Add(1)

		err := service.Start(ctx, &wg)
		require.ErrorIs(t, err, ErrNotificationSenderNotInitialized)

		// Start 실패 시 즉시 Done 함수가 호출되어야 함
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done: // 정상
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Start 실패 후 WaitGroup이 회수되지 않음")
		}
	})

	t.Run("정상: 다중 Start 무시", func(t *testing.T) {
		srvCtx := setupTestContext(t, true)
		defer srvCtx.Teardown()

		var wg sync.WaitGroup
		wg.Add(1)
		// 이미 Start 된 상태이므로 err = nil을 즉시 반환하고 wg를 Done 처리해야 함
		err := srvCtx.Service.Start(srvCtx.Context, &wg)
		require.NoError(t, err)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done: // 정상
		case <-time.After(100 * time.Millisecond):
			t.Fatal("다중 Start 시 WaitGroup이 즉시 회수되지 않음")
		}
	})
}

// =============================================================================
// Submit Core Logic Tests
// =============================================================================

func TestService_Submit(t *testing.T) {
	t.Parallel()

	// 공통 테스트 Task 세팅
	registerTestTask(t, "TASK_VALID", "CMD_VALID", true, nil)

	t.Run("에러: Request Validation 실패 (nil 요청)", func(t *testing.T) {
		srvCtx := setupTestContext(t, true)
		defer srvCtx.Teardown()

		err := srvCtx.Service.Submit(srvCtx.Context, nil)
		require.ErrorIs(t, err, ErrInvalidTaskSubmitRequest)
	})

	t.Run("에러: Request 데이터 필수값 부족", func(t *testing.T) {
		srvCtx := setupTestContext(t, true)
		defer srvCtx.Teardown()

		// TaskID 비어있음
		err := srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
			CommandID:  "CMD_VALID",
			NotifierID: "user_a",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "TaskID는 필수입니다")
	})

	t.Run("에러: 미등록 Task 실행 불가 (Config Not Found)", func(t *testing.T) {
		srvCtx := setupTestContext(t, true)
		defer srvCtx.Teardown()

		err := srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
			TaskID:     "UNKNOWN_TASK",
			CommandID:  "CMD_VALID",
			NotifierID: "user_a",
			RunBy:      contract.TaskRunByUser,
		})
		require.ErrorIs(t, err, provider.ErrTaskNotSupported)

		// Submit 메서드가 비동기 알림 없이 바로 에러를 뱉는지 확인
		srvCtx.MockSender.AssertNotCalled(t, "Notify", mock.Anything, mock.Anything)
	})

	t.Run("에러: 서비스 미실행 중 Submit 차단", func(t *testing.T) {
		// AutoStart = false
		srvCtx := setupTestContext(t, false)

		err := srvCtx.Service.Submit(context.Background(), &contract.TaskSubmitRequest{
			TaskID:     "TASK_VALID",
			CommandID:  "CMD_VALID",
			NotifierID: "user_a",
			RunBy:      contract.TaskRunByUser,
		})
		require.ErrorIs(t, err, ErrServiceNotRunning)
	})

	t.Run("정상: 동기 Submit 큐 진입", func(t *testing.T) {
		srvCtx := setupTestContext(t, true)
		defer srvCtx.Teardown()

		err := srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
			TaskID:     "TASK_VALID",
			CommandID:  "CMD_VALID",
			NotifierID: "user_a",
			RunBy:      contract.TaskRunByUser,
		})
		require.NoError(t, err)
		// 이벤트 루프가 큐 사이즈를 빼내는지 여부 없이 즉시 Submit 반환.
	})

	t.Run("에러: 큐 가득 참 (Timeout 발생 시)", func(t *testing.T) {
		// 이 테스트를 위해 채널 버퍼를 인위적 꽉 채웁니다.
		// 채널 사이즈는 10이므로 10번 밀어넣은 이후에는 대기해야 합니다.
		// 이 때, 서비스 이벤트 루프가 돌아가면 채널을 비워버리므로 AutoStart = false로 둡니다.
		// (단, Submit 내부에서 s.running을 체크하므로 s.running을 수동 강제 On 시킴)

		srvCtx := setupTestContext(t, false)
		srvCtx.Service.runningMu.Lock()
		srvCtx.Service.running = true // Fake running state
		srvCtx.Service.runningMu.Unlock()

		// 10개 Push (버퍼 모두 소진)
		for i := 0; i < defaultQueueSize; i++ {
			err := srvCtx.Service.Submit(context.Background(), &contract.TaskSubmitRequest{
				TaskID:     "TASK_VALID",
				CommandID:  "CMD_VALID",
				NotifierID: "user_a",
				RunBy:      contract.TaskRunByUser,
			})
			require.NoError(t, err)
		}

		// 11번째는 블로킹될 테니 Timeout Context 사용
		timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer timeoutCancel()

		err := srvCtx.Service.Submit(timeoutCtx, &contract.TaskSubmitRequest{
			TaskID:     "TASK_VALID",
			CommandID:  "CMD_VALID",
			NotifierID: "user_a",
			RunBy:      contract.TaskRunByUser,
		})
		require.ErrorIs(t, err, context.DeadlineExceeded) // 컨텍스트 에러
	})
}

// =============================================================================
// Cancel Core Logic Tests
// =============================================================================

func TestService_Cancel(t *testing.T) {
	t.Parallel()

	// 공통 테스트 Task 세팅
	registerTestTask(t, "TASK_VALID", "CMD_VALID", true, nil)

	t.Run("에러: 서비스 미실행 중 Cancel 차단", func(t *testing.T) {
		srvCtx := setupTestContext(t, false)

		err := srvCtx.Service.Cancel("instance-123")
		require.ErrorIs(t, err, ErrServiceNotRunning)
	})

	t.Run("정상: 동기 Cancel 큐 진입", func(t *testing.T) {
		srvCtx := setupTestContext(t, true)
		defer srvCtx.Teardown()

		err := srvCtx.Service.Cancel("instance-123")
		require.NoError(t, err) // 큐에 넣는 것 자체는 ID와 무관하게 무조건 성공
	})

	t.Run("에러: 취소 큐 가득 참", func(t *testing.T) {
		srvCtx := setupTestContext(t, false)
		srvCtx.Service.runningMu.Lock()
		srvCtx.Service.running = true // Fake running state
		srvCtx.Service.runningMu.Unlock()

		for i := 0; i < defaultQueueSize; i++ {
			err := srvCtx.Service.Cancel(contract.TaskInstanceID(fmt.Sprintf("id-%d", i)))
			require.NoError(t, err)
		}

		// Queue Full 상태이므로 ErrCancelQueueFull이 즉시 반환되어야 함
		err := srvCtx.Service.Cancel("id-full")
		require.ErrorIs(t, err, ErrCancelQueueFull)
	})
}

// =============================================================================
// Event Loop and Asynchronous Behavior Tests
// =============================================================================

func TestService_EventLoop_HandleTaskSubmit_And_Done(t *testing.T) {
	t.Parallel()

	var srvCtx *ServiceTestContext

	// Task 내부 로직에서 Done()을 호출하도록 셋업
	taskRunWait := make(chan struct{})
	var stubTaskWaitCanceled <-chan struct{}

	registerTestTask(t, "TASK_RUN_WAIT", "CMD_WAIT", true, func(ctx context.Context, sender contract.NotificationSender) {
		close(taskRunWait)
		select {
		case <-ctx.Done():
		case <-stubTaskWaitCanceled:
		}
	}, func(cancelC <-chan struct{}) {
		stubTaskWaitCanceled = cancelC
	})

	srvCtx = setupTestContext(t, true)
	defer srvCtx.Teardown()

	// Intercept the cancelC for test wait
	srvCtx.MockIDGen.ExpectedCalls = nil // Reset
	srvCtx.MockIDGen.On("New").Return(contract.TaskInstanceID("running-instance")).Once()

	// 1. Submit
	err := srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
		TaskID:     "TASK_RUN_WAIT",
		CommandID:  "CMD_WAIT",
		NotifierID: "tester",
		RunBy:      contract.TaskRunByUser,
	})
	require.NoError(t, err)

	// 2. Task 실행 시작 대기
	select {
	case <-taskRunWait:
	case <-time.After(1 * time.Second):
	case <-time.After(3 * time.Second):
		t.Fatal("Task Run()이 호출되지 않음")
	}

	// 3. Cancel
	err = srvCtx.Service.Cancel("running-instance")
	require.NoError(t, err)

	// Cancel 요청이 EventLoop를 거쳐 실제 Task를 종료(WaitCanceled 닫힘)시키는지 확인
	select {
	case <-stubTaskWaitCanceled:
	case <-time.After(3 * time.Second):
		t.Fatal("Task가 정상적으로 Cancel 처리되지 않음 (WaitCanceled 이벤트 수신 실패)")
	}
}

func TestService_EventLoop_HandleTaskCancel(t *testing.T) {
	t.Parallel()

	srvCtx := setupTestContext(t, true)
	defer srvCtx.Teardown()

	// 기존 Mock(Notify)을 제거하고 이 테스트에서 모든 Notify를 잡아서 확인합니다.
	srvCtx.MockSender.ExpectedCalls = nil

	// 존재하지 않는 알림 취소 시: Notification 발송 검증
	// Cancel 이라는 내부 행위는 사용자 알림(NotifySender)으로 종결되므로
	// Notify Mock을 이용해 EventLoop가 올바로 처리했는지 동기화함.
	notifyCalled := make(chan struct{}, 1)
	srvCtx.MockSender.On("Notify", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		n, ok := args.Get(1).(contract.Notification)
		if ok && n.ErrorOccurred == true && n.Cancelable == false && strings.Contains(n.Message, "unknown-id") {
			select {
			case notifyCalled <- struct{}{}:
			default:
			}
		}
	}).Return(nil).Maybe()

	err := srvCtx.Service.Cancel("unknown-id")
	require.NoError(t, err) // 요청 자체는 큐에 들어감

	// 이벤트 루프가 Cancel 큐에서 꺼내어 HandleTaskCancel 실행
	select {
	case <-notifyCalled: // 확인
	case <-time.After(1 * time.Second):
		t.Fatal("존재하지 않는 ID Cancel 처리 후 알림이 발송되지 않음")
	}

	srvCtx.MockSender.AssertExpectations(t)
}

func TestService_RejectIfAlreadyRunning(t *testing.T) {
	t.Parallel()

	var srvCtx *ServiceTestContext

	// 싱글톤 Task: 작업이 도중에 끝나지 않게 무한 대기
	taskStartC := make(chan struct{})
	var singletonWaitCanceled <-chan struct{}

	registerTestTask(t, "SINGLETON", "CMD_S", false, func(ctx context.Context, sender contract.NotificationSender) {
		close(taskStartC)
		select {
		case <-ctx.Done():
		case <-singletonWaitCanceled:
		}
	}, func(cancelC <-chan struct{}) {
		singletonWaitCanceled = cancelC
	})

	srvCtx = setupTestContext(t, true)
	defer srvCtx.Teardown()

	srvCtx.MockIDGen.ExpectedCalls = nil
	srvCtx.MockIDGen.On("New").Return(contract.TaskInstanceID("inst-1")).Twice()

	// 기존 Mock(Notify)을 제거하고 이 테스트에서 모든 Notify를 잡아서 확인합니다.
	srvCtx.MockSender.ExpectedCalls = nil
	notifyCalled := make(chan struct{}, 1) // 버퍼를 두어 여러 번 호출되어도 패닉(블록)을 방지
	srvCtx.MockSender.On("Notify", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		n, ok := args.Get(1).(contract.Notification)
		if ok && n.Cancelable == true && n.InstanceID == "inst-1" {
			select {
			case notifyCalled <- struct{}{}:
			default:
			}
		}
	}).Return(nil).Maybe()

	// 1차 실행 (성공)
	err := srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
		TaskID:     "SINGLETON",
		CommandID:  "CMD_S",
		NotifierID: "tester",
		RunBy:      contract.TaskRunByUser,
	})
	require.NoError(t, err)

	// Task Start 대기
	select {
	case <-taskStartC:
	case <-time.After(3 * time.Second):
		t.Fatal("Task가 시작되지 않음")
	}

	// 2차 실행 시도 (AllowMultiple=false이므로 반려되고 사전 등록된 Notify 발송)
	err = srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
		TaskID:     "SINGLETON",
		CommandID:  "CMD_S",
		NotifierID: "tester",
		RunBy:      contract.TaskRunByUser,
	})
	require.NoError(t, err) // 큐 삽입은 성공

	select {
	case <-notifyCalled: // 성공
	case <-time.After(3 * time.Second):
		t.Fatal("중복 실행에 대한 반려 알림이 오지 않음")
	}

	// Clean up Task
	err = srvCtx.Service.Cancel("inst-1")
	require.NoError(t, err)

	// Cancel 요청이 전달되어 Task 종료가 이루어졌는지 동기화
	select {
	case <-singletonWaitCanceled:
	case <-time.After(3 * time.Second):
		t.Fatal("Task가 스무스하게 종료되지 않음")
	}

	srvCtx.MockSender.AssertExpectations(t)
}

// =============================================================================
// Shutdown and Resource Cleanup Tests
// =============================================================================

func TestService_GracefulShutdown(t *testing.T) {
	t.Parallel()

	// 10초 대기(종료되지 않는 방해꾼) Task를 만들더라도
	// Context가 취소(Shutdown)됨에 따라 올바르게 CleanUp 되는지 검증

	slowTaskStarted := make(chan struct{})
	var slowTaskWaitCanceled <-chan struct{}
	registerTestTask(t, "SLOW_TASK", "SLOW_CMD", true, func(ctx context.Context, sender contract.NotificationSender) {
		close(slowTaskStarted)
		// shutdown 시 handleStop에서 모든 Task에 Cancel()을 호출하므로, 이를 감지합니다.
		<-slowTaskWaitCanceled
	}, func(cancelC <-chan struct{}) {
		slowTaskWaitCanceled = cancelC
	})

	srvCtx := setupTestContext(t, true)

	err := srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
		TaskID:     "SLOW_TASK",
		CommandID:  "SLOW_CMD",
		NotifierID: "u1",
		RunBy:      contract.TaskRunByUser,
	})
	require.NoError(t, err)

	// Task가 실행되어 대기 상태(<-ctx.Done() 진입)가 될 때까지 기다림
	<-slowTaskStarted

	// Teardown 시작 (Graceful Shutdown)
	shutdownComplete := make(chan struct{})
	go func() {
		srvCtx.Teardown() // cancel() -> handleStop()
		close(shutdownComplete)
	}()

	select {
	case <-shutdownComplete:
		// 정상
	case <-time.After(5 * time.Second): // 30초 대기 중단 로직보다 짧게 잡아도 정상적으로 Shutdown 되어야 함
		t.Fatal("Graceful Shutdown 타임아웃. Task 취소 전파가 실패했거나 StopWG에서 데드락 발생")
	}

	// 리소스 해제 검증
	srvCtx.Service.runningMu.Lock()
	defer srvCtx.Service.runningMu.Unlock()
	require.Nil(t, srvCtx.Service.tasks, "태스크 맵이 nil로 초기화되어 메모리가 해제되어야 함")
	require.False(t, srvCtx.Service.running, "실행 상태는 false여야 함")
}

// =============================================================================
// handleTaskDone Tests
// =============================================================================

func TestService_HandleTaskDone(t *testing.T) {
	t.Parallel()

	t.Run("정상: Task 완료 후 tasks 맵에서 제거됨", func(t *testing.T) {
		// Task를 실행하고, 완료 후 tasks 맵에서 제거되는지 확인합니다.
		taskDone := make(chan struct{})

		registerTestTask(t, "DONE_TASK", "DONE_CMD", true, func(ctx context.Context, sender contract.NotificationSender) {
			// 즉시 종료하여 handleTaskDone 호출 유도
			close(taskDone)
		})

		srvCtx := setupTestContext(t, true)
		defer srvCtx.Teardown()

		srvCtx.MockIDGen.ExpectedCalls = nil
		srvCtx.MockIDGen.On("New").Return(contract.TaskInstanceID("done-instance")).Once()

		err := srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
			TaskID:     "DONE_TASK",
			CommandID:  "DONE_CMD",
			NotifierID: "tester",
			RunBy:      contract.TaskRunByUser,
		})
		require.NoError(t, err)

		// Task Run() 완료 대기
		select {
		case <-taskDone:
		case <-time.After(3 * time.Second):
			t.Fatal("Task Run()이 완료되지 않음")
		}

		// handleTaskDone이 이벤트 루프에서 처리されるため 잠시 대기
		time.Sleep(100 * time.Millisecond)

		// tasks 맵에서 제거되었는지 확인
		_, exists := srvCtx.Service.tasks["done-instance"]
		require.False(t, exists, "Task 완료 후 tasks 맵에서 제거되어야 함")
	})

	t.Run("비정상: 등록되지 않은 InstanceID Done 수신(경고만 출력)", func(t *testing.T) {
		srvCtx := setupTestContext(t, true)
		defer srvCtx.Teardown()

		// 이벤트 루프를 통해 직접 taskDoneC에 알 수 없는 ID 전송
		select {
		case srvCtx.Service.taskDoneC <- contract.TaskInstanceID("unknown-instance"):
		case <-time.After(1 * time.Second):
			t.Fatal("taskDoneC에 전송 실패")
		}

		// 경고 로그만 남기며 패닉/에러 없이 처리되는지 확인하기 위해 잠시 대기
		// (정상적으로 처리되면 테스트 통과)
		time.Sleep(100 * time.Millisecond)
	})
}

// =============================================================================
// registerAndRunTask Edge Case Tests
// =============================================================================

func TestService_RegisterAndRunTask_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("에러: Task 생성 실패 시 에러 알림 발송", func(t *testing.T) {
		// NewTask가 (nil, error)를 반환하는 Task 등록
		registerTestTask(t, "FAIL_CREATE_TASK", "FAIL_CREATE_CMD", true, nil)

		// RegisterForTest로 NewTask가 nil을 반환하도록 덮어씌우기
		provider.RegisterForTest("FAIL_CREATE_TASK_NIL", &provider.TaskConfig{
			Commands: []*provider.TaskCommandConfig{
				{
					ID:            "FAIL_CMD",
					AllowMultiple: true,
					NewSnapshot:   func() interface{} { return &struct{}{} },
				},
			},
			NewTask: func(p provider.NewTaskParams) (provider.Task, error) {
				return nil, fmt.Errorf("task 생성 실패: 테스트 오류")
			},
		})

		notifyCalled := make(chan struct{}, 1)
		srvCtx := setupTestContext(t, true)
		defer srvCtx.Teardown()

		srvCtx.MockSender.ExpectedCalls = nil
		srvCtx.MockSender.On("Notify", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			n, ok := args.Get(1).(contract.Notification)
			if ok && n.ErrorOccurred {
				select {
				case notifyCalled <- struct{}{}:
				default:
				}
			}
		}).Return(nil).Maybe()

		err := srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
			TaskID:     "FAIL_CREATE_TASK_NIL",
			CommandID:  "FAIL_CMD",
			NotifierID: "tester",
			RunBy:      contract.TaskRunByUser,
		})
		require.NoError(t, err)

		select {
		case <-notifyCalled:
			// 에러 알림 발송 확인
		case <-time.After(3 * time.Second):
			t.Fatal("Task 생성 실패 시 에러 알림이 발송되지 않음")
		}
	})

	t.Run("정상: NotifyOnStart=true 시 시작 알림 발송", func(t *testing.T) {
		startNotifyCalled := make(chan struct{}, 1)
		var notifyStartWaitCanceled <-chan struct{}

		registerTestTask(t, "NOTIFY_START_TASK", "NOTIFY_CMD", true, func(ctx context.Context, sender contract.NotificationSender) {
			<-notifyStartWaitCanceled
		}, func(cancelC <-chan struct{}) {
			notifyStartWaitCanceled = cancelC
		})

		srvCtx := setupTestContext(t, true)
		defer srvCtx.Teardown()

		srvCtx.MockIDGen.ExpectedCalls = nil
		srvCtx.MockIDGen.On("New").Return(contract.TaskInstanceID("notify-start-inst")).Once()

		srvCtx.MockSender.ExpectedCalls = nil
		srvCtx.MockSender.On("Notify", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			n, ok := args.Get(1).(contract.Notification)
			// 시작 알림은 Cancelable=true, ErrorOccurred=false이며 "진행중" 메시지 포함
			if ok && !n.ErrorOccurred && n.Cancelable && n.InstanceID == "notify-start-inst" {
				select {
				case startNotifyCalled <- struct{}{}:
				default:
				}
			}
		}).Return(nil).Maybe()

		err := srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
			TaskID:        "NOTIFY_START_TASK",
			CommandID:     "NOTIFY_CMD",
			NotifierID:    "tester",
			RunBy:         contract.TaskRunByUser,
			NotifyOnStart: true,
		})
		require.NoError(t, err)

		select {
		case <-startNotifyCalled:
			// 시작 알림 정상 발송
		case <-time.After(3 * time.Second):
			t.Fatal("NotifyOnStart=true 시 시작 알림이 발송되지 않음")
		}
	})

	t.Run("에러: InstanceID 충돌 시 에러 알림 발송", func(t *testing.T) {
		// 동일한 InstanceID를 반환하는 MockIDGenerator → 두 번째 Submit 시 충돌
		conflictNotifyCalled := make(chan struct{}, 1)
		taskStarted := make(chan struct{})
		var conflictWaitCanceled <-chan struct{}

		registerTestTask(t, "CONFLICT_TASK", "CONFLICT_CMD", true, func(ctx context.Context, sender contract.NotificationSender) {
			close(taskStarted)
			<-conflictWaitCanceled
		}, func(cancelC <-chan struct{}) {
			conflictWaitCanceled = cancelC
		})

		srvCtx := setupTestContext(t, true)
		defer srvCtx.Teardown()

		// 모든 호출에 동일한 ID 반환
		srvCtx.MockIDGen.ExpectedCalls = nil
		srvCtx.MockIDGen.On("New").Return(contract.TaskInstanceID("dup-instance"))

		srvCtx.MockSender.ExpectedCalls = nil
		srvCtx.MockSender.On("Notify", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			n, ok := args.Get(1).(contract.Notification)
			if ok && n.ErrorOccurred {
				select {
				case conflictNotifyCalled <- struct{}{}:
				default:
				}
			}
		}).Return(nil).Maybe()

		// 1차: 정상 실행 (tasks에 dup-instance 등록)
		err := srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
			TaskID:     "CONFLICT_TASK",
			CommandID:  "CONFLICT_CMD",
			NotifierID: "tester",
			RunBy:      contract.TaskRunByUser,
		})
		require.NoError(t, err)

		// Task가 started 확인
		select {
		case <-taskStarted:
		case <-time.After(3 * time.Second):
			t.Fatal("Task가 시작되지 않음")
		}

		// 2차: 동일 ID → 충돌 발생 → 에러 알림
		err = srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
			TaskID:     "CONFLICT_TASK",
			CommandID:  "CONFLICT_CMD",
			NotifierID: "tester",
			RunBy:      contract.TaskRunByUser,
		})
		require.NoError(t, err)

		select {
		case <-conflictNotifyCalled:
			// ID 충돌로 인한 에러 알림 확인
		case <-time.After(3 * time.Second):
			t.Fatal("InstanceID 충돌 시 에러 알림이 발송되지 않음")
		}
	})
}

// =============================================================================
// EventLoop Panic Recovery Tests
// =============================================================================

func TestService_EventLoop_PanicRecovery(t *testing.T) {
	t.Parallel()

	// 이벤트 루프의 패닉 복구 동작을 검증합니다.
	// 이벤트 루프 내부(handleTaskCancel)에서 패닉을 유발하기 위해
	// tasks 맵에 nil Task를 직접 주입 후 Cancel을 호출합니다.
	// nil Task에 대해 task.Cancel()을 호출하면 패닉이 발생하여 recover()가 작동해야 합니다.

	normalDone := make(chan struct{})
	registerTestTask(t, "NORMAL_AFTER_PANIC2", "NORMAL_CMD2", true, func(ctx context.Context, sender contract.NotificationSender) {
		close(normalDone)
	})

	srvCtx := setupTestContext(t, true)
	defer srvCtx.Teardown()

	// tasks 맵에 nil Task를 직접 삽입하여 handleTaskCancel에서 패닉 유발
	// (nil Task에 대해 Cancel() 호출 시 nil pointer dereference 발생)
	panicInstanceID := contract.TaskInstanceID("panic-nil-task")
	srvCtx.Service.tasks[panicInstanceID] = nil

	// Cancel 요청 → 이벤트 루프에서 handleTaskCancel 실행 → nil.Cancel() 패닉 발생
	err := srvCtx.Service.Cancel(panicInstanceID)
	require.NoError(t, err)

	// 패닉 복구 후 이벤트 루프가 살아있는지 확인하기 위해 정상 Task 제출
	time.Sleep(100 * time.Millisecond)

	err = srvCtx.Service.Submit(srvCtx.Context, &contract.TaskSubmitRequest{
		TaskID:     "NORMAL_AFTER_PANIC2",
		CommandID:  "NORMAL_CMD2",
		NotifierID: "tester",
		RunBy:      contract.TaskRunByUser,
	})
	require.NoError(t, err)

	// 정상 Task가 이벤트 루프에 의해 처리되었는지 확인
	select {
	case <-normalDone:
		// 패닉 이후에도 이벤트 루프가 정상 동작 확인
	case <-time.After(3 * time.Second):
		t.Fatal("패닉 복구 이후 이벤트 루프가 정상 동작하지 않음")
	}

	// Teardown 시 handleStop 패닉 방지를 위해 의도적으로 삽입한 nil 값을 제거합니다.
	srvCtx.Service.runningMu.Lock()
	delete(srvCtx.Service.tasks, panicInstanceID)
	srvCtx.Service.runningMu.Unlock()
}
