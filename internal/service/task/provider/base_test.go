package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	applog "github.com/darkkaiser/notify-server/pkg/log"
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

	mockStorage := &contractmocks.MockTaskResultStore{}
	task := newBase(baseParams{
		ID:         taskID,
		CommandID:  cmdID,
		InstanceID: instID,
		NotifierID: contract.NotifierID(notifier),
		RunBy:      contract.TaskRunByUser,
		Storage:    mockStorage,
		Scraper:    &dummyScraper{},
	})

	// When & Then
	assert.Equal(t, taskID, task.ID())
	assert.Equal(t, cmdID, task.CommandID())
	assert.Equal(t, instID, task.InstanceID())
	assert.Equal(t, contract.NotifierID(notifier), task.NotifierID())
	assert.Equal(t, contract.TaskRunByUser, task.RunBy())

	// Cancel Test
	assert.False(t, task.IsCanceled())
	task.Cancel()
	assert.True(t, task.IsCanceled())

	// Elapsed Test
	task.runTime = time.Now().Add(-1 * time.Second)
	assert.GreaterOrEqual(t, task.Elapsed(), 1*time.Second)
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
		runBy                contract.TaskRunBy                                                                            // 실행 주체 (User vs Scheduler)
		setup                func(tID contract.TaskID, cID contract.TaskCommandID) (contract.TaskResultStore, ExecuteFunc) // 테스트 환경 설정
		preRunAction         func(task *Base)                                                                              // Run 실행 전 동작 (예: 취소)
		verifyNotification   func(t *testing.T, notifs []contract.Notification)                                            // Notification 상태 검증 콜백
		expectedNotifyCount  int                                                                                           // 알림 발송 횟수 (에러 알림 포함)
		expectedMessageParts []string                                                                                      // 알림 메시지에 포함되어야 할 문자열
		expectPanic          bool                                                                                          // Panic 발생 여부 (Recover 되었는지)
	}{
		{
			name:  "성공: 정상적인 실행 및 저장 (Scheduler)",
			runBy: contract.TaskRunByScheduler,
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (contract.TaskResultStore, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStore{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)
				store.On("Save", tID, cID, mock.Anything).Return(nil)

				exec := func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
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
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (contract.TaskResultStore, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStore{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)
				store.On("Save", tID, cID, mock.Anything).Return(nil)

				exec := func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
					return "사용자 요청 완료", map[string]string{"foo": "bar"}, nil
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			verifyNotification: func(t *testing.T, notifs []contract.Notification) {
				// 사용자 실행 -> 실행 중에는 취소 가능했지만, 최종 결과 알림 시점에는 취소 불가능으로 변경됨
				// handleExecutionResult에서 완료 알림은 Cancelable=false로 강제 설정함.
				if len(notifs) == 0 {
					return
				}
				lastNotif := notifs[len(notifs)-1]
				assert.False(t, lastNotif.Cancelable, "최종 결과 알림 시점에는 취소 불가능 상태여야 합니다")
			},

			expectedNotifyCount:  1,
			expectedMessageParts: []string{"사용자 요청 완료"},
		},
		{
			name: "성공: 메시지가 없으면 알림을 보내지 않음",
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (contract.TaskResultStore, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStore{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)
				// 메시지가 없어도 Snapshot이 변경되면 저장은 수행될 수 있음
				store.On("Save", tID, cID, mock.Anything).Return(nil)

				exec := func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
					return "", map[string]string{"foo": "bar"}, nil // Empty Message
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			expectedNotifyCount: 0,
		},
		{
			name: "실패: Execute 함수 미설정 (방어 코드)",
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (contract.TaskResultStore, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStore{}
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
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (contract.TaskResultStore, ExecuteFunc) {
				exec := func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
					return "ok", nil, nil
				}
				registerTestConfig(tID, cID)
				return nil, exec // Storage is nil
			},
			expectedNotifyCount:  1,
			expectedMessageParts: []string{msgStorageNotInitialized},
		},
		{
			name: "실패: 실행 전 작업 취소 (Before Run)",
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (contract.TaskResultStore, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStore{}
				// [Policy Change] Run 메서드 시작 시 IsCanceled()를 체크하여 조기 종료(Early Exit)하므로,
				// 조기 취소 시에는 Load/Save/Execute가 모두 실행되지 않아야 합니다.

				exec := func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
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
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (contract.TaskResultStore, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStore{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)

				exec := func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
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
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (contract.TaskResultStore, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStore{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)
				store.On("Save", tID, cID, mock.Anything).Return(errors.New("DB Disk Full"))

				exec := func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
					return "성공했어나 저장실패", map[string]interface{}{}, nil
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			expectedNotifyCount:  1,                                                                      // 저장 실패 시 성공 알림을 보내지 않고 에러 알림 1회만 발송
			expectedMessageParts: []string{"DB Disk Full", msgNewSnapshotSaveFailed[0:10], "성공했어나 저장실패"}, // 저장 실패 정보와 원래 성공 메시지가 모두 포함되어야 함
			verifyNotification: func(t *testing.T, notifs []contract.Notification) {
				for _, n := range notifs {
					assert.False(t, n.Cancelable)
					assert.True(t, n.ErrorOccurred)
				}
			},
		},
		{
			name: "Panic: 실행 중 런타임 패닉 발생 (Recovery)",
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (contract.TaskResultStore, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStore{}
				store.On("Load", tID, cID, mock.Anything).Return(nil)

				exec := func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
					panic("예기치 못한 닐 포인터 참조")
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			expectedNotifyCount: 1,
			// handleExecutionResult에서 "작업이 실패하였습니다" 문구와 에러 메시지를 조합하여 전송함
			expectedMessageParts: []string{msgTaskExecutionFailed, "시스템 내부 오류(Panic)", "예기치 못한 닐 포인터 참조"},
		},
		{
			name:  "에러: 이전 데이터 로드 실패 (Load Error) - 실행 중단 (Fail-Fast)",
			runBy: contract.TaskRunByUser,
			setup: func(tID contract.TaskID, cID contract.TaskCommandID) (contract.TaskResultStore, ExecuteFunc) {
				store := &contractmocks.MockTaskResultStore{}
				store.On("Load", tID, cID, mock.Anything).Return(errors.New("Corrupted Data"))
				// Load 실패 시 즉시 리턴하므로 Save는 호출되지 않아야 함
				// store.On("Save", ...).Return(...) -> Removed

				exec := func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
					return "실행되면 안됨", map[string]interface{}{}, nil
				}
				registerTestConfig(tID, cID)
				return store, exec
			},
			verifyNotification: func(t *testing.T, notifs []contract.Notification) {
				// 에러 알림 1개만 와야 함
				require.Len(t, notifs, 1)
				assert.True(t, notifs[0].ErrorOccurred, "에러가 발생했으므로 ErrorOccurred=true여야 합니다")
				assert.False(t, notifs[0].Cancelable, "에러에 의한 종료이므로 취소 불가능해야 합니다")
			},
			expectedNotifyCount:  1, // Load 에러 알림 1회
			expectedMessageParts: []string{"이전 작업결과데이터 로딩이 실패"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 테스트 격리: 유니크 ID 생성
			tID, cID := generateUniqueIDs("TASK")

			// Mock 객체 생성
			mockSender := notificationmocks.NewMockNotificationSender(t)
			// 기본적으로 모든 Notify 호출을 허용합니다. (실제 횟수 검증은 별도 수행)
			mockSender.On("Notify", mock.Anything, mock.Anything).Return(nil).Maybe()
			mockSender.On("SupportsHTML", mock.Anything).Return(true).Maybe()

			// Setup
			store, exec := tt.setup(tID, cID)

			// Task 초기화
			// 테스트 케이스별 runBy 설정 적용 (기본값: RunByScheduler)
			runBy := tt.runBy
			if runBy == contract.TaskRunByUnknown {
				runBy = contract.TaskRunByScheduler
			}
			task := newBase(baseParams{
				ID:         tID,
				CommandID:  cID,
				InstanceID: "test_inst",
				NotifierID: "test_notifier",
				RunBy:      runBy,
				Storage:    store,
				Scraper:    &dummyScraper{},
				NewSnapshot: func() interface{} {
					return make(map[string]interface{})
				},
			})
			task.SetExecute(exec)

			// Pre-Run Action
			if tt.preRunAction != nil {
				tt.preRunAction(task)
			}

			// Run
			wg := &sync.WaitGroup{}
			doneC := make(chan contract.TaskInstanceID, 1)
			wg.Add(1)

			go func() {
				defer wg.Done()
				defer func() {
					doneC <- task.InstanceID()
				}()
				task.Run(context.Background(), mockSender)
			}()

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
				if mockStore, ok := store.(*contractmocks.MockTaskResultStore); ok {
					mockStore.AssertExpectations(t)
				}
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
	defaultRegistry.RegisterForTest(tID, &TaskConfig{
		NewTask: func(p NewTaskParams) (Task, error) {
			return nil, nil
		},
		Commands: []*TaskCommandConfig{
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

// dummyScraper Scraper 인터페이스의 더미 구현체입니다. (테스트용)
type dummyScraper struct{}

func (d *dummyScraper) FetchHTML(ctx context.Context, method, rawURL string, body io.Reader, header http.Header) (*goquery.Document, error) {
	return nil, nil
}
func (d *dummyScraper) FetchHTMLDocument(ctx context.Context, rawURL string, header http.Header) (*goquery.Document, error) {
	return nil, nil
}
func (d *dummyScraper) ParseHTML(ctx context.Context, r io.Reader, rawURL string, contentType string) (*goquery.Document, error) {
	return nil, nil
}
func (d *dummyScraper) FetchJSON(ctx context.Context, method, rawURL string, body any, header http.Header, v any) error {
	return nil
}

// TestTask_PrepareExecution_SnapshotCreationFailed 스냅샷 생성 함수가 없는 경우의 처리를 테스트합니다.
func TestTask_PrepareExecution_SnapshotCreationFailed(t *testing.T) {
	task := newBase(baseParams{
		ID:          "UNKNOWN_TASK",
		CommandID:   "UNKNOWN_CMD",
		InstanceID:  "inst",
		NotifierID:  "noti",
		RunBy:       contract.TaskRunByUser,
		Scraper:     &dummyScraper{},
		NewSnapshot: func() interface{} { return nil }, // Explicitly trigger SnapshotCreationFailed
	})

	// ExecuteFunc 설정 (호출되지 않아야 함)
	task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
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
	assert.Contains(t, err.Error(), msgSnapshotCreationFailed)
}

// TestTask_FeatureFlags 기능 플래그(UseStorage, UseScraper)의 동작을 검증합니다.
func TestTask_FeatureFlags(t *testing.T) {
	mockSender := notificationmocks.NewMockNotificationSender(t)
	mockSender.On("Notify", mock.Anything, mock.Anything).Return(nil).Maybe()
	ctx := context.Background()

	t.Run("Snapshot 팩토리(NewSnapshot)가 있고 Storage가 nil일 때는 Run 실행 시 에러가 발생해야 함", func(t *testing.T) {
		task := newBase(baseParams{
			ID:          "STRICT_TASK",
			CommandID:   "CMD",
			NewSnapshot: func() interface{} { return &struct{}{} },
		})
		task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
			return "ok", &struct{}{}, nil // NewSnapshot 반환
		})

		// prepareExecution에서는 스냅샷 팩토리가 있으면 Storage nil을 체크함 (현재 구현 유지)
		_, err := task.prepareExecution(ctx, mockSender)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), msgStorageNotInitialized)
	})

	t.Run("UseStorage=false일 경우 Storage가 nil이어도 에러 없이 통과해야 함", func(t *testing.T) {
		task := newBase(baseParams{
			ID:        "NO_STORAGE_TASK",
			CommandID: "CMD",
			Scraper:   &dummyScraper{},
		})
		task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
			return "ok", nil, nil
		})

		_, err := task.prepareExecution(ctx, mockSender)
		assert.NoError(t, err)
	})

	t.Run("RequireScraper=true일 때 Scraper가 nil이면 에러가 발생해야 함", func(t *testing.T) {
		task := newBase(baseParams{
			ID:             "STRICT_SCRAPER_TASK",
			CommandID:      "CMD",
			RequireScraper: true,
			Scraper:        nil,
		})
		task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
			return "ok", nil, nil
		})

		_, err := task.prepareExecution(ctx, mockSender)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), msgScraperNotInitialized)
	})

	t.Run("RequireScraper=false일 때 Scraper가 nil이어도 통과해야 함", func(t *testing.T) {
		task := newBase(baseParams{
			ID:             "LAX_SCRAPER_TASK",
			CommandID:      "CMD",
			RequireScraper: false,
			Scraper:        nil,
		})
		task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
			return "ok", nil, nil
		})

		_, err := task.prepareExecution(ctx, mockSender)
		assert.NoError(t, err)
	})
}

// TestTask_Run_PanicRecovery 패닉 발생 시 복구 및 알림 전송 로직을 검증합니다.
func TestTask_Run_PanicRecovery(t *testing.T) {
	// Given
	tID := contract.TaskID("PANIC_TASK")
	cID := contract.TaskCommandID("PANIC_CMD")
	registerTestConfig(tID, cID)

	mockSender := notificationmocks.NewMockNotificationSender(t)
	// 패닉 알림이 1회 호출되어야 함
	mockSender.On("Notify", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
		return n.ErrorOccurred && n.TaskID == tID
	})).Return(nil).Once()
	mockSender.On("SupportsHTML", mock.Anything).Return(true).Maybe()

	task := newBase(baseParams{
		ID:         tID,
		CommandID:  cID,
		InstanceID: "inst_id",
	})
	// 의도적으로 Panic을 일으키는 ExecuteFunc 설정
	task.SetExecute(func(ctx context.Context, prev any, html bool) (string, any, error) {
		panic("intentional panic")
	})

	// When
	assert.NotPanics(t, func() {
		task.Run(context.Background(), mockSender)
	})

	// Then
	mockSender.AssertExpectations(t)
}

// TestTask_Run_SecondaryPanicRecovery 패닉 복구 로직 내부에서 발생하는 2차 패닉에 대한 방어 로직을 검증합니다.
func TestTask_Run_SecondaryPanicRecovery(t *testing.T) {
	// Given
	tID := contract.TaskID("DOUBLE_PANIC_TASK")
	cID := contract.TaskCommandID("DOUBLE_PANIC_CMD")
	registerTestConfig(tID, cID)

	mockSender := notificationmocks.NewMockNotificationSender(t)
	// 알림 전송 중 2차 패닉을 유도합니다.
	mockSender.On("Notify", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		panic("secondary panic in notify")
	}).Return(nil) // 실제로는 panic 때문에 실행되지 않음
	mockSender.On("SupportsHTML", mock.Anything).Return(true).Maybe()

	task := newBase(baseParams{
		ID:         tID,
		CommandID:  cID,
		InstanceID: "inst_id",
	})
	task.SetExecute(func(ctx context.Context, prev any, html bool) (string, any, error) {
		panic("primary panic")
	})

	// When & Then
	// 고루틴 내에서 실행되지 않도록 직접 Run을 호출하여 패닉 전파 여부 확인
	assert.NotPanics(t, func() {
		task.Run(context.Background(), mockSender)
	}, "2차 패닉이 발생하더라도 최종적으로 복구되어야 합니다")
}

// TestNewBaseFromParams_NilRequest p.Request가 nil일 때 패닉이 발생하는지 검증합니다.
func TestNewBaseFromParams_NilRequest(t *testing.T) {
	assert.Panics(t, func() {
		NewBase(NewTaskParams{
			Request: nil,
		}, false)
	})
}

// TestTask_Run_NilNotificationSender notificationSender가 nil일 때 안전하게 종료되는지 검증합니다.
func TestTask_Run_NilNotificationSender(t *testing.T) {
	task := newBase(baseParams{
		ID:        "TEST",
		CommandID: "CMD",
	})

	assert.NotPanics(t, func() {
		task.Run(context.Background(), nil)
	})
}

// -----------------------------------------------------------------------------
// Logging Tests (Integrated from base_log_test.go)
// -----------------------------------------------------------------------------

func TestTask_Log(t *testing.T) {
	// 로거 훅 설정 (로그 캡처)
	hook := NewMemoryHook()
	applog.StandardLogger().AddHook(hook)
	defer func() {
		hook.Reset()
	}()

	// Given
	task := newBase(baseParams{
		ID:         "TEST_TASK",
		CommandID:  "TEST_CMD",
		InstanceID: "TEST_INST",
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByScheduler,
		NewSnapshot: func() interface{} {
			return struct{}{}
		},
	})

	tests := []struct {
		name      string
		component string
		level     applog.Level
		message   string
		fields    applog.Fields
		err       error
		validate  func(t *testing.T, entry *applog.Entry)
	}{
		{
			name:      "기본 로깅 (필드 없음, 에러 없음)",
			component: "test.component",
			level:     applog.InfoLevel,
			message:   "info message",
			fields:    nil,
			err:       nil,
			validate: func(t *testing.T, entry *applog.Entry) {
				assert.Equal(t, applog.InfoLevel, entry.Level)
				assert.Equal(t, "info message", entry.Message)
				assert.Equal(t, "test.component", entry.Data["component"])
				assert.Equal(t, contract.TaskID("TEST_TASK"), entry.Data["task_id"])
				assert.Equal(t, contract.TaskCommandID("TEST_CMD"), entry.Data["command_id"])
			},
		},
		{
			name:      "추가 필드 포함",
			component: "test.component",
			level:     applog.WarnLevel,
			message:   "warn message",
			fields:    applog.Fields{"custom_field": "value"},
			err:       nil,
			validate: func(t *testing.T, entry *applog.Entry) {
				assert.Equal(t, applog.WarnLevel, entry.Level)
				assert.Equal(t, "warn message", entry.Message)
				assert.Equal(t, "value", entry.Data["custom_field"])
			},
		},
		{
			name:      "에러 포함",
			component: "test.component",
			level:     applog.ErrorLevel,
			message:   "error message",
			fields:    nil,
			err:       errors.New("test error"),
			validate: func(t *testing.T, entry *applog.Entry) {
				assert.Equal(t, applog.ErrorLevel, entry.Level)
				assert.Equal(t, "error message", entry.Message)
				assert.Equal(t, "test error", entry.Data["error"].(error).Error())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook.Reset()
			task.LogWithContext(tt.component, tt.level, tt.message, tt.fields, tt.err)

			requireEntry(t, hook)
			tt.validate(t, hook.LastEntry())
		})
	}
}

func requireEntry(t *testing.T, hook *MemoryHook) {
	if len(hook.Entries) == 0 {
		t.Fatal("로그가 기록되지 않았습니다")
	}
}

// MemoryHook 테스트용 로그 훅 구현체
type MemoryHook struct {
	Entries []*applog.Entry
}

func NewMemoryHook() *MemoryHook {
	return &MemoryHook{}
}

func (h *MemoryHook) Levels() []applog.Level {
	return applog.AllLevels
}

func (h *MemoryHook) Fire(entry *applog.Entry) error {
	h.Entries = append(h.Entries, entry)
	return nil
}

func (h *MemoryHook) Reset() {
	h.Entries = make([]*applog.Entry, 0)
}

func (h *MemoryHook) LastEntry() *applog.Entry {
	if len(h.Entries) == 0 {
		return nil
	}
	return h.Entries[len(h.Entries)-1]
}
