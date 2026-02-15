package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
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
	t.Run("Panic when Request is nil", func(t *testing.T) {
		assert.PanicsWithValue(t, "NewBase: params.Request는 필수입니다", func() {
			NewBase(NewTaskParams{
				Request: nil,
			}, false)
		})
	})

	t.Run("Panic when RequireScraper is true but Fetcher is nil", func(t *testing.T) {
		assert.Panics(t, func() {
			NewBase(NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID: "TEST_TASK",
				},
				Fetcher: nil,
			}, true)
		})
	})

	t.Run("Success with valid params and RequireScraper=true", func(t *testing.T) {
		fetcher := &mocks.MockFetcher{}
		task := NewBase(NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID:    "TEST_TASK",
				CommandID: "TEST_CMD",
			},
			Fetcher: fetcher,
		}, true)

		assert.NotNil(t, task)
		assert.NotNil(t, task.scraper)
		assert.True(t, task.requireScraper)
	})
}

func TestTask_BasicMethods(t *testing.T) {
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

	// Registry reset required for RegisterForTest
	ClearForTest()

	store := &contractmocks.MockTaskResultStore{}
	sender := notificationmocks.NewMockNotificationSender(t)
	// Default behavior for sender: accept Notify and SupportsHTML
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

func TestRun_Success(t *testing.T) {
	task, store, sender := setupTestTask(t)

	// Mock Expectation
	store.On("Load", task.ID(), task.CommandID(), mock.Anything).Return(nil)
	store.On("Save", task.ID(), task.CommandID(), mock.Anything).Return(nil)

	// Execute Logic
	task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
		return "Success Message", map[string]string{"data": "new"}, nil
	})

	// Run
	task.Run(context.Background(), sender)

	// Verify
	store.AssertExpectations(t)
	// Verify Notification contains success message
	calls := sender.Calls
	assert.NotEmpty(t, calls)
	lastCall := calls[len(calls)-1]
	assert.Equal(t, "Notify", lastCall.Method)
	notif := lastCall.Arguments.Get(1).(contract.Notification)
	assert.Equal(t, "Success Message", notif.Message)
	assert.False(t, notif.ErrorOccurred)
}

func TestRun_ExecutionError(t *testing.T) {
	task, store, sender := setupTestTask(t)

	store.On("Load", task.ID(), task.CommandID(), mock.Anything).Return(nil)
	// Save should NOT be called on execution error

	task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
		return "", nil, errors.New("Business Logic Failed")
	})

	task.Run(context.Background(), sender)

	store.AssertExpectations(t)
	store.AssertNotCalled(t, "Save", mock.Anything, mock.Anything, mock.Anything)

	// Verify Error Notification
	calls := sender.Calls
	assert.NotEmpty(t, calls)
	notif := calls[len(calls)-1].Arguments.Get(1).(contract.Notification)
	assert.True(t, notif.ErrorOccurred)
	assert.Contains(t, notif.Message, "Business Logic Failed")
	assert.Contains(t, notif.Message, notifyTaskExecutionFailed)
}

func TestRun_SnapshotSaveFailure(t *testing.T) {
	task, store, sender := setupTestTask(t)

	store.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("Disk Full"))

	task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
		return "Success but Save Fail", map[string]string{}, nil
	})

	task.Run(context.Background(), sender)

	// Verify Notification
	calls := sender.Calls
	assert.NotEmpty(t, calls)
	notif := calls[len(calls)-1].Arguments.Get(1).(contract.Notification)
	assert.True(t, notif.ErrorOccurred) // Save failed -> Error Notification
	assert.Contains(t, notif.Message, "Disk Full")
	assert.Contains(t, notif.Message, "Success but Save Fail") // Should contain original success msg
}

func TestRun_DependencyValidation(t *testing.T) {
	t.Run("ExecuteFunc Missing", func(t *testing.T) {
		task, _, sender := setupTestTask(t)
		task.execute = nil // Force nil

		task.Run(context.Background(), sender)

		// Check internal error notification
		calls := sender.Calls
		require.NotEmpty(t, calls)
		notif := calls[len(calls)-1].Arguments.Get(1).(contract.Notification)
		assert.Contains(t, notif.Message, errMsgExecuteFuncNotInitialized)
	})

	t.Run("Scraper Missing when required", func(t *testing.T) {
		task, _, sender := setupTestTask(t)
		task.requireScraper = true
		task.scraper = nil

		// Execute func must be present to pass first check
		task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
			return "", nil, nil
		})

		task.Run(context.Background(), sender)

		calls := sender.Calls
		require.NotEmpty(t, calls)
		notif := calls[len(calls)-1].Arguments.Get(1).(contract.Notification)
		assert.Contains(t, notif.Message, errMsgScraperNotInitialized)
	})
}

func TestRun_SnapshotLoading(t *testing.T) {
	t.Run("ErrTaskResultNotFound (First Run) - Should be ignored", func(t *testing.T) {
		task, store, sender := setupTestTask(t)

		store.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(contract.ErrTaskResultNotFound)
		store.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
			return "OK", map[string]string{"result": "new"}, nil
		})

		task.Run(context.Background(), sender)

		store.AssertExpectations(t)
		// Should succeed (no error notification)
		calls := sender.Calls
		notif := calls[len(calls)-1].Arguments.Get(1).(contract.Notification)
		assert.False(t, notif.ErrorOccurred)
	})

	t.Run("Other Load Error - Should fail", func(t *testing.T) {
		task, store, sender := setupTestTask(t)

		store.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("Corrupt DB"))

		task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
			return "OK", nil, nil
		})

		task.Run(context.Background(), sender)

		store.AssertExpectations(t)
		// Should fail
		calls := sender.Calls
		notif := calls[len(calls)-1].Arguments.Get(1).(contract.Notification)
		assert.True(t, notif.ErrorOccurred)
		assert.Contains(t, notif.Message, "이전 작업 결과 데이터를 불러오는 과정에서 오류가 발생하였습니다")
	})
}

func TestRun_Cancellation(t *testing.T) {
	t.Run("Cancel before Run", func(t *testing.T) {
		task, store, sender := setupTestTask(t)
		task.Cancel()

		task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
			return "Should No Run", nil, nil
		})

		task.Run(context.Background(), sender)

		// Store should not be touched
		store.AssertNotCalled(t, "Load", mock.Anything, mock.Anything, mock.Anything)
		// No notifications
		assert.Empty(t, sender.Calls)
		// Note check setupTestTask's sender mock if it catches info logs? No, NotificationSender only gets Notify calls.
		// Base logs to applog, so checking sender.Calls is correct for verification of NO user notification.
	})
}

func TestRun_PanicRecovery(t *testing.T) {
	task, store, sender := setupTestTask(t)
	store.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	task.SetExecute(func(ctx context.Context, prev interface{}, html bool) (string, interface{}, error) {
		panic("Boom!")
	})

	assert.NotPanics(t, func() {
		task.Run(context.Background(), sender)
	})

	// Verify panic notification
	calls := sender.Calls
	require.NotEmpty(t, calls)
	notif := calls[len(calls)-1].Arguments.Get(1).(contract.Notification)
	assert.True(t, notif.ErrorOccurred)
	assert.Contains(t, notif.Message, "Boom!")
	assert.Contains(t, notif.Message, "Panic")
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
		assert.Equal(t, contract.TaskID("TEST_TASK"), entry.Data["task_id"])
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
