package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTask_BasicMethods Task의 기본 Getter/Setter 및 상태 메서드를 검증합니다.
func TestTask_BasicMethods(t *testing.T) {
	// Given
	taskID := contract.TaskID("TEST_TASK")
	cmdID := contract.TaskCommandID("TEST_CMD")
	instID := contract.TaskInstanceID("inst_123")
	notifier := "telegram"

	task := NewBase(taskID, cmdID, instID, contract.NotifierID(notifier), contract.TaskRunByUser)
	mockStorage := &contractmocks.MockTaskResultStorage{}
	task.SetStorage(mockStorage)

	// When & Then
	assert.Equal(t, taskID, task.GetID())
	assert.Equal(t, cmdID, task.GetCommandID())
	assert.Equal(t, instID, task.GetInstanceID())
	assert.Equal(t, contract.NotifierID(notifier), task.GetNotifierID())
	assert.Equal(t, contract.TaskRunByUser, task.GetRunBy())

	// Cancel Test
	assert.False(t, task.IsCanceled())
	task.Cancel()
	assert.True(t, task.IsCanceled())

	// RunBy Update Test
	task.SetRunBy(contract.TaskRunByScheduler)
	assert.Equal(t, contract.TaskRunByScheduler, task.GetRunBy())

	// ElapsedTime Test
	task.runTime = time.Now().Add(-1 * time.Second)
	assert.GreaterOrEqual(t, task.ElapsedTimeAfterRun(), int64(1))
}

// TestTask_Run Task 실행의 전체수명주기(Lifecycle)와 다양한 시나리오를 검증합니다.
func TestTask_Run(t *testing.T) {
	// 테스트 시작 전 레지스트리 초기화 (격리 보장)
	ClearForTest()
	defer ClearForTest()
	generateUniqueIDs := func(prefix string) (contract.TaskID, contract.TaskCommandID) {
		ts := time.Now().UnixNano()
		return contract.TaskID(fmt.Sprintf("%s_%d", prefix, ts)), contract.TaskCommandID(fmt.Sprintf("%s_%d", prefix, ts))
	}

	tests := []struct {
		name                 string
		runBy                contract.TaskRunBy                                                                                        // 실행 주체 (User vs Scheduler)
		setup                func(tID contract.TaskID, cID contract.TaskCommandID) (*contractmocks.MockTaskResultStorage, ExecuteFunc) // 테스트 환경 설정
		preRunAction         func(task *Base)                                                                                          // Run 실행 전 동작 (예: 취소)
		verifyNotification   func(t *testing.T, notifs []contract.Notification)                                                        // Notification 상태 검증 콜백
		expectedNotifyCount  int                                                                                                       // 알림 발송 횟수 (에러 알림 포함)
		expectedMessageParts []string                                                                                                  // 알림 메시지에 포함되어야 할 문자열
		expectPanic          bool                                                                                                      // Panic 발생 여부 (Recover 되었는지)
	}{
		{
			name:  "성공: 정상적인 실행 및 저장 (Scheduler)",
			runBy: contract.TaskRunByScheduler,
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (*contractmocks.MockTaskResultStorage, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStorage{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)
				store.On("Save", tID, cID, mock.Anything).Return(nil)

				exec := func(prev interface{}, html bool) (string, interface{}, error) {
					return "성공 메시지", map[string]string{"foo": "bar"}, nil
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			verifyNotification: func(t *testing.T, notifs []contract.Notification) {
				// 스케줄러 실행 -> 취소 불가
				assert.NotEmpty(t, notifs)
				for _, n := range notifs {
					assert.False(t, n.Cancelable, "스케줄러 실행 작업은 취소 불가능해야 합니다")
				}
			},
			expectedNotifyCount:  1,
			expectedMessageParts: []string{"성공 메시지"},
		},
		{
			name:  "성공: 정상적인 실행 및 저장 (User) - 취소 가능성 검증",
			runBy: contract.TaskRunByUser,
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (*contractmocks.MockTaskResultStorage, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStorage{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)
				store.On("Save", tID, cID, mock.Anything).Return(nil)

				exec := func(prev interface{}, html bool) (string, interface{}, error) {
					return "사용자 요청 완료", map[string]string{"foo": "bar"}, nil
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			verifyNotification: func(t *testing.T, notifs []contract.Notification) {
				// 사용자 실행 -> 실행 중에는 취소 가능했지만, 최종 결과 알림 시점에는 취소 불가능으로 변경됨
				// handleExecutionResult에서 완료 알림은 Cancelable=false로 강제 설정함.
				assert.NotEmpty(t, notifs)
				lastNotif := notifs[len(notifs)-1]
				assert.False(t, lastNotif.Cancelable, "최종 결과 알림 시점에는 취소 불가능 상태여야 합니다")
			},

			expectedNotifyCount:  1,
			expectedMessageParts: []string{"사용자 요청 완료"},
		},
		{
			name: "성공: 메시지가 없으면 알림을 보내지 않음",
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (*contractmocks.MockTaskResultStorage, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStorage{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)
				// 메시지가 없어도 Snapshot이 변경되면 저장은 수행될 수 있음
				store.On("Save", tID, cID, mock.Anything).Return(nil)

				exec := func(prev interface{}, html bool) (string, interface{}, error) {
					return "", map[string]string{"foo": "bar"}, nil // Empty Message
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			expectedNotifyCount: 0,
		},
		{
			name: "실패: Execute 함수 미설정 (방어 코드)",
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (*contractmocks.MockTaskResultStorage, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStorage{}
				// ExecuteFunc가 nil이므로 Load/Save 호출되지 않음
				registerTestConfig(tID, cID)
				return store, nil // ExecuteFunc is nil
			},
			expectedNotifyCount:  1,
			expectedMessageParts: []string{msgExecuteFuncNotInitialized},
			verifyNotification: func(t *testing.T, notifs []contract.Notification) {
				// 에러 알림 -> ErrorOccurred=true, Cancelable=false (notifyError 구현상)
				for _, n := range notifs {
					assert.False(t, n.Cancelable)
					assert.True(t, n.ErrorOccurred)
				}
			},
		},
		{
			name: "실패: Storage 미설정 (방어 코드)",
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (*contractmocks.MockTaskResultStorage, ExecuteFunc) {
				exec := func(prev interface{}, html bool) (string, interface{}, error) { return "ok", nil, nil }
				registerTestConfig(tID, cID)
				return nil, exec // Storage is nil
			},
			expectedNotifyCount:  1,
			expectedMessageParts: []string{msgStorageNotInitialized},
		},
		{
			name: "실패: 실행 전 작업 취소 (Before Run)",
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (*contractmocks.MockTaskResultStorage, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStorage{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)
				// Run 전에 취소되면 Execute 이후 로직(Save, Notify)은 실행되지 않아야 함
				// 하지만 prepareExecution(Load)까지는 실행됨

				exec := func(prev interface{}, html bool) (string, interface{}, error) {
					return "실행되면 안됨", nil, nil
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			preRunAction: func(task *Base) {
				task.Cancel()
			},
			expectedNotifyCount: 0,
		},
		{
			name: "에러: 비즈니스 로직 실행 실패",
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (*contractmocks.MockTaskResultStorage, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStorage{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)

				exec := func(prev interface{}, html bool) (string, interface{}, error) {
					return "", nil, errors.New("API 호출 실패")
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			expectedNotifyCount:  1,
			expectedMessageParts: []string{"API 호출 실패", msgTaskExecutionFailed},
			verifyNotification: func(t *testing.T, notifs []contract.Notification) {
				// 실패 알림(notifyError) -> Cancelable=false
				for _, n := range notifs {
					assert.False(t, n.Cancelable)
				}
			},
		},
		{
			name: "에러: 결과 저장 실패 (Save Error)",
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (*contractmocks.MockTaskResultStorage, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStorage{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)
				store.On("Save", tID, cID, mock.Anything).Return(errors.New("DB Disk Full"))

				exec := func(prev interface{}, html bool) (string, interface{}, error) {
					return "성공했으나 저장실패", map[string]interface{}{}, nil
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			expectedNotifyCount:  2, // 1. 정상 메시지, 2. 저장 실패 에러 메시지
			expectedMessageParts: []string{"성공했으나 저장실패", "DB Disk Full"},
			verifyNotification: func(t *testing.T, notifs []contract.Notification) {
				// 두 번의 알림 모두 완료 후 시점이므로 (하나는 성공 후 저장실패)
				// 1. notify(성공) -> RunBy=Scheduler(Default) -> False? Test setup uses default.
				// Wait, setup function doesn't set RunBy. Default NewBase uses provided arg in loop.
				// Loop sets RunBy based on test case. Here it is Default (Scheduler).
				// So Cancelable=False.
				for _, n := range notifs {
					assert.False(t, n.Cancelable)
				}
			},
		},
		{
			name: "Panic: 실행 중 런타임 패닉 발생 (Recovery)",
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (*contractmocks.MockTaskResultStorage, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStorage{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)

				exec := func(prev interface{}, html bool) (string, interface{}, error) {
					panic("예기치 못한 닐 포인터 참조")
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			expectedNotifyCount: 1,
			// handleExecutionResult에서 "작업이 실패하였습니다" 문구와 에러 메시지를 조합하여 전송함
			expectedMessageParts: []string{msgTaskExecutionFailed, "Task 실행 도중 Panic 발생", "예기치 못한 닐 포인터 참조"},
		},
		{
			name:  "경고: 이전 데이터 로드 실패 (Load Error) - 실행은 계속됨 (User Run)",
			runBy: contract.TaskRunByUser,
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (*contractmocks.MockTaskResultStorage, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStorage{}
				store.On("Load", tID, cID, mock.Anything).Return(errors.New("Corrupted Data"))
				// Load 실패해도 Execute는 실행되어야 함
				store.On("Save", tID, cID, mock.Anything).Return(nil)

				exec := func(prev interface{}, html bool) (string, interface{}, error) {
					return "복구 후 실행", map[string]interface{}{}, nil
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			verifyNotification: func(t *testing.T, notifs []contract.Notification) {
				// 첫 번째 알림(Load Error Warn) -> notify() 호출 -> RunBy=User -> Cancelable=True
				require.Len(t, notifs, 2)
				assert.True(t, notifs[0].Cancelable, "진행 중 발생한 경고 알림은 취소 가능해야 합니다 (User Run)")
				// 두 번째 알림(완료) -> handleExecutionResult -> Cancelable=False
				assert.False(t, notifs[1].Cancelable, "완료 알림은 취소 불가능해야 합니다")
			},
			expectedNotifyCount:  2, // 1. Load 에러 알림(Warn), 2. 실행 결과 알림
			expectedMessageParts: []string{"이전 작업결과데이터 로딩이 실패", "복구 후 실행"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 테스트 격리: 유니크 ID 생성
			tID, cID := generateUniqueIDs("TASK")

			// Mock 객체 생성
			mockSender := notificationmocks.NewMockNotificationSender(t)

			// Setup
			store, exec := tt.setup(tID, cID)

			// Task 초기화
			// 테스트 케이스별 runBy 설정 적용 (기본값: RunByScheduler)
			runBy := tt.runBy
			if runBy == contract.TaskRunByUnknown {
				runBy = contract.TaskRunByScheduler
			}
			task := NewBase(tID, cID, "test_inst", "test_notifier", runBy)
			if store != nil {
				task.SetStorage(store)
			}
			task.SetExecute(exec)

			// Pre-Run Action
			if tt.preRunAction != nil {
				tt.preRunAction(task)
			}

			// Run
			wg := &sync.WaitGroup{}
			doneC := make(chan contract.TaskInstanceID, 1)
			wg.Add(1)

			// Expectation Setup
			// Notify might be called multiple times. We allow any calls for now and verify later,
			// or we can set specific expectations if `tt.expectedNotifyCount` is known.
			// But for simplicity/flexibility, we allow calls and verify count later.
			mockSender.On("Notify", mock.Anything, mock.Anything).Return(nil).Maybe()

			mockSender.On("SupportsHTML", mock.Anything).Return(true).Maybe()

			go task.Run(context.Background(), mockSender, wg, doneC)

			// Wait for completion
			waitTimeout(t, wg, 2*time.Second)

			// Validate
			// Validate
			// Count Notify calls (Notify, NotifyWithTitle, NotifyDefault, NotifyDefaultWithError)
			// Actually `GetNotifyCallCount` logic was just `NotifyCalls` (Notify method).
			// Let's verify "Notify" primarily?
			// The test seems to focus on `Notify`.
			// Update: Some tests might trigger NotifyDefault/WithError?
			// Let's check `collectAllMessages`.

			// Verify notification call count
			actualNotifyCount := 0
			for _, call := range mockSender.Calls {
				if call.Method == "Notify" {
					actualNotifyCount++
				}
			}
			assert.Equal(t, tt.expectedNotifyCount, actualNotifyCount, "알림 발송 횟수가 일치해야 합니다")

			if len(tt.expectedMessageParts) > 0 {
				allMsg := collectAllMessages(mockSender)
				for _, part := range tt.expectedMessageParts {
					assert.Contains(t, allMsg, part, "메시지에 예상된 문구가 포함되어야 합니다")
				}
			}

			if tt.verifyNotification != nil {
				// Extract notifications from calls
				var notifs []contract.Notification
				for _, call := range mockSender.Calls {
					if call.Method == "Notify" {
						if n, ok := call.Arguments.Get(1).(contract.Notification); ok {
							notifs = append(notifs, n)
						}
					}
				}
				tt.verifyNotification(t, notifs)
			}

			if store != nil {
				store.AssertExpectations(t)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// registerTestConfig 테스트용 설정을 레지스트리에 등록합니다.
func registerTestConfig(tID contract.TaskID, cID contract.TaskCommandID) {
	// Register 대신 RegisterForTest를 사용하여 중복 시 덮어쓰기 허용
	// 또는 테스트마다 매번 ClearRegistry를 호출해야 하지만, 병렬 실행 등을 고려하여 덮어쓰기가 유리함
	defaultRegistry.RegisterForTest(tID, &Config{
		NewTask: func(contract.TaskInstanceID, *contract.TaskSubmitRequest, *config.AppConfig) (Task, error) {
			return nil, nil
		},
		Commands: []*CommandConfig{
			{
				ID: cID,
				NewSnapshot: func() interface{} {
					return make(map[string]interface{})
				},
			},
		},
	})
}

// waitTimeout WaitGroup이 지정된 시간 내에 완료되기를 기다립니다.
func waitTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return // Completed normally
	case <-time.After(timeout):
		t.Fatal("테스트 타임아웃: Task가 제시간에 종료되지 않았습니다")
	}
}

// collectAllMessages MockSender에 전송된 모든 메시지를 하나의 문자열로 합칩니다.
func collectAllMessages(sender *notificationmocks.MockNotificationSender) string {
	var sb string
	for _, call := range sender.Calls {
		// Method check and argument extraction
		if call.Method == "Notify" {
			if n, ok := call.Arguments.Get(1).(contract.Notification); ok {
				sb += n.Message + "\n"
			}
		}
	}
	return sb
}

// TestConfigNotFound Config가 없는 경우의 처리를 테스트합니다.
func TestTask_PrepareExecution_ConfigNotFound(t *testing.T) {
	task := NewBase("UNKNOWN_TASK", "UNKNOWN_CMD", "inst", "noti", contract.TaskRunByUser)

	// ExecuteFunc 설정 (호출되지 않아야 함)
	task.SetExecute(func(prev interface{}, html bool) (string, interface{}, error) {
		return "", nil, nil
	})

	mockSender := notificationmocks.NewMockNotificationSender(t)
	mockSender.On("Notify", mock.Anything, mock.Anything).Return(nil).Maybe()
	ctx := context.Background()

	// Direct call to prepareExecution to check internal error
	_, err := task.prepareExecution(ctx, mockSender)

	require.Error(t, err)
	assert.IsType(t, &apperrors.AppError{}, err) // AppError 타입 확인
	// Snapshot 생성 실패 메시지 확인
	assert.Equal(t, msgSnapshotCreationFailed, err.(*apperrors.AppError).Message())
}
