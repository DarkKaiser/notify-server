package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit: NewBase & Basic Methods
// =============================================================================

func TestNewBase_Validation(t *testing.T) {
	tests := []struct {
		name           string
		params         NewTaskParams
		requireScraper bool
		expectPanic    bool
		panicMsg       string
	}{
		{
			name: "Panic when Request is nil",
			params: NewTaskParams{
				Request: nil,
			},
			requireScraper: false,
			expectPanic:    true,
			panicMsg:       "NewBase: params.Request는 필수입니다",
		},
		{
			name: "Panic when RequireScraper is true but Fetcher is nil",
			params: NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID: "TEST_TASK",
				},
				Fetcher: nil,
			},
			requireScraper: true,
			expectPanic:    true,
			panicMsg:       "NewBase: 스크래핑 작업에는 Fetcher 주입이 필수입니다 (TaskID=TEST_TASK)",
		},
		{
			name: "Success with Valid Params (No Scraper)",
			params: NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    "TEST_TASK",
					CommandID: "TEST_CMD",
				},
			},
			requireScraper: false,
			expectPanic:    false,
		},
		{
			name: "Success with Valid Params (With Scraper)",
			params: NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    "TEST_TASK",
					CommandID: "TEST_CMD",
				},
				Fetcher: &mocks.MockFetcher{},
			},
			requireScraper: true,
			expectPanic:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				if tt.panicMsg != "" {
					assert.PanicsWithValue(t, tt.panicMsg, func() {
						NewBase(tt.params, tt.requireScraper)
					})
				} else {
					assert.Panics(t, func() {
						NewBase(tt.params, tt.requireScraper)
					})
				}
			} else {
				assert.NotPanics(t, func() {
					task := NewBase(tt.params, tt.requireScraper)
					assert.NotNil(t, task)
					if tt.requireScraper {
						assert.NotNil(t, task.scraper)
						assert.True(t, task.requireScraper)
					}
				})
			}
		})
	}
}

func TestBase_BasicMethods(t *testing.T) {
	taskID := contract.TaskID("TEST_TASK")
	cmdID := contract.TaskCommandID("TEST_CMD")
	instID := contract.TaskInstanceID("inst_123")
	notifier := "telegram"

	task := newBase(baseParams{
		ID:         taskID,
		CommandID:  cmdID,
		InstanceID: instID,
		NotifierID: contract.NotifierID(notifier),
		RunBy:      contract.TaskRunByUser,
		Scraper:    &dummyScraper{},
	})

	assert.Equal(t, taskID, task.ID())
	assert.Equal(t, cmdID, task.CommandID())
	assert.Equal(t, instID, task.InstanceID())
	assert.Equal(t, contract.NotifierID(notifier), task.NotifierID())
	assert.Equal(t, contract.TaskRunByUser, task.RunBy())

	// Scraper Access
	assert.NotNil(t, task.Scraper())

	// Cancel Test
	assert.False(t, task.IsCanceled())
	task.Cancel()
	assert.True(t, task.IsCanceled())

	// Elapsed Test
	task.startedAtMu.Lock()
	task.startedAt = time.Now().Add(-1 * time.Second)
	task.startedAtMu.Unlock()
	assert.GreaterOrEqual(t, task.Elapsed(), 1*time.Second)
}

// =============================================================================
// Unit: Run - Execution Flow & Edge Cases
// =============================================================================

// setupTestTask helps creating a Task with mocked dependencies for Run tests.
func setupTestTask(t *testing.T) (*Base, *contractmocks.MockTaskResultStore, *notificationmocks.MockNotificationSender) {
	tID := contract.TaskID("TEST_TASK")
	cID := contract.TaskCommandID("TEST_CMD")

	store := &contractmocks.MockTaskResultStore{}
	sender := notificationmocks.NewMockNotificationSender(t)

	// Default behavior for sender: accept Notify and SupportsHTML
	// Note: We use .Maybe() to allow tests to override or ignore these if they have specific expectations.
	sender.On("Notify", mock.Anything, mock.Anything).Return(nil).Maybe()
	sender.On("SupportsHTML", mock.Anything).Return(true).Maybe()

	task := newBase(baseParams{
		ID:        tID,
		CommandID: cID,
		Storage:   store,
		NewSnapshot: func() interface{} {
			return make(map[string]interface{})
		},
	})

	return task, store, sender
}

func TestRun_scenarios(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*Base, *contractmocks.MockTaskResultStore, *notificationmocks.MockNotificationSender)
		executeFunc   ExecuteFunc
		preRunAction  func(*Base)
		expectedError bool     // Expect error notification sent to user
		expectedMsg   []string // Substrings expected in notification message
		unexpectedMsg []string // Substrings NOT expected in notification
		expectSave    bool     // Expect Save to be called
		expectLoad    bool     // Expect Load to be called
	}{
		{
			name: "Success: Normal Execution",
			setupMocks: func(b *Base, s *contractmocks.MockTaskResultStore, n *notificationmocks.MockNotificationSender) {
				s.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				s.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			executeFunc: func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
				return "Task Completed Successfully", map[string]string{"result": "ok"}, nil
			},
			expectedError: false,
			expectedMsg:   []string{"Task Completed Successfully"},
			expectSave:    true,
			expectLoad:    true,
		},
		{
			name: "Failure: ExecuteFunc Returns Error",
			setupMocks: func(b *Base, s *contractmocks.MockTaskResultStore, n *notificationmocks.MockNotificationSender) {
				s.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			executeFunc: func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
				return "", nil, errors.New("Parsing Error")
			},
			expectedError: true,
			expectedMsg:   []string{"Parsing Error", notifyTaskExecutionFailed},
			expectSave:    false,
			expectLoad:    true,
		},
		{
			name: "Failure: Snapshot Save Error",
			setupMocks: func(b *Base, s *contractmocks.MockTaskResultStore, n *notificationmocks.MockNotificationSender) {
				s.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				s.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("Disk Full"))
			},
			executeFunc: func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
				return "Result Ready", map[string]string{"data": "val"}, nil
			},
			expectedError: true,
			expectedMsg:   []string{"Disk Full", "Result Ready", "작업 실행은 성공하였으나"},
			expectSave:    true,
			expectLoad:    true,
		},
		{
			name: "Failure: Pre-Execution Load Error (Not First Run)",
			setupMocks: func(b *Base, s *contractmocks.MockTaskResultStore, n *notificationmocks.MockNotificationSender) {
				s.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("Corrupt Data"))
			},
			executeFunc: func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
				return "Should Not Run", nil, nil
			},
			expectedError: true,
			expectedMsg:   []string{"Corrupt Data", "불러오는 과정에서 오류가 발생"},
			expectSave:    false,
			expectLoad:    true,
		},
		{
			name: "Success: First Run (ErrTaskResultNotFound)",
			setupMocks: func(b *Base, s *contractmocks.MockTaskResultStore, n *notificationmocks.MockNotificationSender) {
				s.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(contract.ErrTaskResultNotFound)
				s.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			executeFunc: func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
				return "First Run Success", map[string]interface{}{}, nil
			},
			expectedError: false,
			expectedMsg:   []string{"First Run Success"},
			expectSave:    true,
			expectLoad:    true,
		},
		{
			name: "Failure: Missing Dependency (ExecuteFunc)",
			setupMocks: func(b *Base, s *contractmocks.MockTaskResultStore, n *notificationmocks.MockNotificationSender) {
				b.execute = nil // Force nil
			},
			executeFunc:   nil, // already nil
			expectedError: true,
			expectedMsg:   []string{errMsgExecuteFuncNotInitialized},
			expectSave:    false,
			expectLoad:    false,
		},
		{
			name: "Panic Recovery: System Panic",
			setupMocks: func(b *Base, s *contractmocks.MockTaskResultStore, n *notificationmocks.MockNotificationSender) {
				s.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			executeFunc: func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
				panic("Unexpected Kernel Panic")
			},
			expectedError: true,
			expectedMsg:   []string{"시스템 내부 오류(Panic)", "Unexpected Kernel Panic"},
			expectSave:    false,
			expectLoad:    true,
		},
		{
			name: "Cancellation: Mid-Execution (Check Logic)",
			setupMocks: func(b *Base, s *contractmocks.MockTaskResultStore, n *notificationmocks.MockNotificationSender) {
				s.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
				// Set execute here to access 'b'
				b.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
					b.Cancel()
					return "Should Not Notify", nil, nil
				})
			},
			executeFunc:   nil,
			expectedError: false,
			// Since Base.Run checks canceled status after execute, it should return early
			// WITHOUT calling finalizeExecution, effectively sending NO notification.
			expectSave: false,
			expectLoad: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, store, sender := setupTestTask(t)

			// Clear default expectations to ensure our capturing mock wins
			sender.ExpectedCalls = nil
			sender.On("SupportsHTML", mock.Anything).Return(true).Maybe()

			// Capture notifications to assert on them later
			var capturedNotifications []contract.Notification
			// Using .Run to capture arguments. We need to match arguments generally.
			sender.On("Notify", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				capturedNotifications = append(capturedNotifications, args.Get(1).(contract.Notification))
			}).Return(nil).Maybe()

			if tt.setupMocks != nil {
				tt.setupMocks(task, store, sender)
			}
			if tt.executeFunc != nil {
				task.SetExecute(tt.executeFunc)
			}
			if tt.preRunAction != nil {
				tt.preRunAction(task)
			}

			// Execution
			if strings.Contains(tt.name, "Panic") {
				assert.NotPanics(t, func() {
					task.Run(context.Background(), sender)
				})
			} else {
				task.Run(context.Background(), sender)
			}

			// Verification: Mock Assertions
			// We only assert expectations on store because sender expectations are captured manually or via Maybe
			store.AssertExpectations(t)

			// Verification: Notifications
			if tt.expectedError || len(tt.expectedMsg) > 0 {
				require.NotEmpty(t, capturedNotifications, "Expected notification but got none")
				notif := capturedNotifications[len(capturedNotifications)-1]

				assert.Equal(t, tt.expectedError, notif.ErrorOccurred, "ErrorOccurred mismatch")

				for _, msg := range tt.expectedMsg {
					assert.Contains(t, notif.Message, msg, "Notification message missing expected content")
				}
				for _, msg := range tt.unexpectedMsg {
					assert.NotContains(t, notif.Message, msg, "Notification message contains unexpected content")
				}
			} else {
				if tt.name == "Cancellation: Mid-Execution (Check Logic)" {
					// Expect NO notifications because we canceled mid-execution
					assert.Empty(t, capturedNotifications, "Expected no notifications for mid-execution cancellation")
				}
			}

			// Verification: Check if Save/Load were called as expected
			if !tt.expectSave {
				store.AssertNotCalled(t, "Save", mock.Anything, mock.Anything, mock.Anything)
			}
			if !tt.expectLoad {
				store.AssertNotCalled(t, "Load", mock.Anything, mock.Anything, mock.Anything)
			}
		})
	}
}

func TestRun_ContextDetachment(t *testing.T) {
	// Verify that Notify is called with a detached context.
	// We simulate this by cancelling the parent context passed to Run,
	// and verifying that Notify receives a context that is NOT cancelled.

	task, store, sender := setupTestTask(t)
	store.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// We use a channel to synchronize the test
	executionStarted := make(chan struct{})

	task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
		close(executionStarted)
		return "Success", nil, nil
	})

	var capturedCtx context.Context
	var wg sync.WaitGroup
	wg.Add(1)

	// Explicitly mock Notify to capture the context
	// We overwrite the default expectation from setupTestTask
	sender.ExpectedCalls = nil // Clear existing calls
	sender.On("SupportsHTML", mock.Anything).Return(true).Maybe()
	sender.On("Notify", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedCtx = args.Get(0).(context.Context)
		wg.Done()
	}).Return(nil)

	// Create a parent context we can cancel
	parentCtx, parentCancel := context.WithCancel(context.Background())

	// Run task in separate goroutine
	go func() {
		task.Run(parentCtx, sender)
	}()

	// Wait for execution to start
	<-executionStarted

	// Checkpoint: Execution logic started.
	// Now, we cancel the parent context.
	// NOTE: If we cancel here, `base.Run` calls `finalizeExecution`.
	// `finalizeExecution` creates `notifyCtx` using `context.WithoutCancel(ctx)`.
	parentCancel()

	// Wait for Notify to be called
	wg.Wait()

	require.NotNil(t, capturedCtx, "Notify should have been called")

	// Verification
	// 1. The context passed to Notify should NOT be done, even though parentCtx is cancelled.
	select {
	case <-capturedCtx.Done():
		t.Fatal("Notify context was cancelled unexpectedly (should be detached from parent)")
	default:
		// Context is alive, which is correct
	}

	// 2. The context passed to Notify should have a deadline (timeout)
	deadline, ok := capturedCtx.Deadline()
	assert.True(t, ok, "Notify context should have a deadline")
	assert.WithinDuration(t, time.Now().Add(60*time.Second), deadline, 5*time.Second, "Deadline should be roughly 60s from now")
}

func TestBase_Concurrency(t *testing.T) {
	task, store, sender := setupTestTask(t)
	store.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// We expect NO Save because we are cancelling and racing

	// SetExecute with delay to allow cancellation race
	task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
		select {
		case <-ctx.Done():
			return "", nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return "Success", nil, nil
		}
	})

	// Run heavy concurrency test
	iterations := 10
	for i := 0; i < iterations; i++ {
		var wg sync.WaitGroup
		wg.Add(2)

		// Goroutine 1: Run
		go func() {
			defer wg.Done()
			task.Run(context.Background(), sender)
		}()

		// Goroutine 2: Cancel randomly
		go func() {
			defer wg.Done()
			time.Sleep(time.Duration(i*5) * time.Millisecond) // Vary timing
			task.Cancel()
		}()

		wg.Wait()

		// Reset canceled state for next iteration is NOT possible because Base is stateful and one-shot?
		// Base is designed to be reusable? In current code, `canceled` flag stays true.
		// So `task.Run` on 2nd iteration will return immediately.
		// We need to re-create task for correct race testing if we want "Fresh Run vs Cancel".
		// But here we just want to ensure NO DATA RACING panics.
	}

	// Ensure final state
	assert.True(t, task.IsCanceled())
}

func TestBase_Defensive_PrepareExecution(t *testing.T) {
	// Test defensive checks in prepareExecution that might not be reachable via NewBase
	// by using newBase directly to inject invalid states.

	t.Run("RequireScraper is true but Scraper is nil", func(t *testing.T) {
		task := newBase(baseParams{
			ID:             "TEST_TASK",
			CommandID:      "TEST_CMD",
			RequireScraper: true,
			Scraper:        nil, // Invalid State
		})

		// Set execute to pass the first check
		task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
			return "OK", nil, nil
		})

		sender := notificationmocks.NewMockNotificationSender(t)
		// Expect Error Notification
		sender.On("Notify", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
			return n.ErrorOccurred && strings.Contains(n.Message, errMsgScraperNotInitialized)
		})).Return(nil)

		// Run
		task.Run(context.Background(), sender)

		// Verify
		sender.AssertExpectations(t)
	})

	t.Run("Snapshot Creation Failed (Factory returns nil)", func(t *testing.T) {
		store := &contractmocks.MockTaskResultStore{}
		task := newBase(baseParams{
			ID:        "TEST_TASK",
			CommandID: "TEST_CMD",
			Storage:   store,
			NewSnapshot: func() interface{} {
				return nil // Factory Error
			},
			// Execute must be set to check snapshot creation
			// (prepareExecution checks execute != nil first)
		})
		task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
			return "OK", nil, nil
		})

		sender := notificationmocks.NewMockNotificationSender(t)
		sender.On("Notify", mock.Anything, mock.MatchedBy(func(n contract.Notification) bool {
			return n.ErrorOccurred && strings.Contains(n.Message, errMsgSnapshotCreationFailed)
		})).Return(nil)

		task.Run(context.Background(), sender)
		sender.AssertExpectations(t)
	})
}

// =============================================================================
// Unit: Logging
// =============================================================================

func TestTask_Log(t *testing.T) {
	// 로거 훅 설정 (로그 캡처)
	hook := NewMemoryHook()
	applog.StandardLogger().AddHook(hook)
	defer func() {
		hook.Reset()
	}()

	task := newBase(baseParams{
		ID:        "TEST_TASK",
		CommandID: "TEST_CMD",
	})

	t.Run("Info Log", func(t *testing.T) {
		hook.Reset()
		task.Log("test.comp", applog.InfoLevel, "Info Msg", nil, nil)
		requireEntry(t, hook)
		entry := hook.LastEntry()
		assert.Equal(t, applog.InfoLevel, entry.Level)
		assert.Equal(t, "Info Msg", entry.Message)
		// Base logger fields are present
		assert.Equal(t, contract.TaskID("TEST_TASK"), entry.Data["task_id"])
		// Custom fields are present
		assert.Equal(t, "test.comp", entry.Data["component"])
	})

	t.Run("Error Log", func(t *testing.T) {
		hook.Reset()
		task.Log("test.comp", applog.ErrorLevel, "Error Msg", errors.New("fail"), nil)
		requireEntry(t, hook)
		entry := hook.LastEntry()
		assert.Equal(t, applog.ErrorLevel, entry.Level)
		assert.Equal(t, "fail", entry.Data["error"].(error).Error())
	})
}

// =============================================================================
// Helpers & Mocks
// =============================================================================

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

func requireEntry(t *testing.T, hook *MemoryHook) {
	if len(hook.Entries) == 0 {
		t.Fatal("로그가 기록되지 않았습니다")
	}
}
