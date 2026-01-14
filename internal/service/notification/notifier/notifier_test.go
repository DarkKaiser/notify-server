package notifier_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Constants
// =============================================================================

const (
	testNotifierBufferSize = 10
	testNotifierTimeout    = 1 * time.Second
	testNotifierMessage    = "test message"
)

// =============================================================================
// Test Helpers
// =============================================================================

// assertChannelReceivesData verifies that channel receives expected data.
func assertChannelReceivesData(t *testing.T, ch chan *notifier.NotifyRequest, expectedMsg string, expectedCtx task.TaskContext) {
	t.Helper()
	select {
	case data := <-ch:
		require.NotNil(t, data, "Received data should not be nil")
		assert.Equal(t, expectedMsg, data.Message)
		assert.Equal(t, expectedCtx, data.TaskCtx)
	case <-time.After(testNotifierTimeout):
		t.Fatal("Timeout receiving data from channel")
	}
}

// =============================================================================
// Notifier Creation Tests
// =============================================================================

// TestNewBaseNotifier verifies BaseNotifier creation.
func TestNewBaseNotifier(t *testing.T) {
	tests := []struct {
		name              string
		id                string
		supportsHTML      bool
		bufferSize        int
		expectedBufferCap int
	}{
		{"Normal buffer", "test-id", true, testNotifierBufferSize, testNotifierBufferSize},
		{"No buffer", "test-id", false, 0, 0},
		{"Large buffer", "large-id", true, 100, 100},
		{"HTML not supported", "no-html", false, 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := notifier.NewBaseNotifier(notifier.NotifierID(tt.id), tt.supportsHTML, tt.bufferSize)

			assert.Equal(t, notifier.NotifierID(tt.id), n.ID())
			assert.Equal(t, tt.supportsHTML, n.SupportsHTML())
			require.NotNil(t, n.RequestC, "Request channel should be created")
			assert.Equal(t, tt.expectedBufferCap, cap(n.RequestC))
		})
	}
}

// =============================================================================
// Notify Method Tests
// =============================================================================

// TestNotify verifies Notify method.
func TestNotify(t *testing.T) {
	tests := []struct {
		name       string
		Notifier   notifier.BaseNotifier
		message    string
		taskCtx    task.TaskContext
		expectData bool
		expectTrue bool
	}{
		{
			name:       "Success: With TaskContext",
			Notifier:   notifier.NewBaseNotifier("test", true, testNotifierBufferSize),
			message:    testNotifierMessage,
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "Success: nil TaskContext",
			Notifier:   notifier.NewBaseNotifier("test", true, testNotifierBufferSize),
			message:    testNotifierMessage,
			taskCtx:    nil,
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "Success: Empty Message",
			Notifier:   notifier.NewBaseNotifier("test", true, testNotifierBufferSize),
			message:    "",
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "Success: Long Message (10KB)",
			Notifier:   notifier.NewBaseNotifier("test", true, testNotifierBufferSize),
			message:    strings.Repeat("a", 10000),
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name: "Failure: Closed Channel (nil check)",
			Notifier: func() notifier.BaseNotifier {
				n := notifier.NewBaseNotifier("test", true, testNotifierBufferSize)
				n.Close()
				return n
			}(),
			message:    testNotifierMessage,
			taskCtx:    nil,
			expectData: false,
			expectTrue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ch chan *notifier.NotifyRequest
			if tt.Notifier.RequestC != nil {
				ch = tt.Notifier.RequestC
			}

			result := tt.Notifier.Notify(tt.taskCtx, tt.message)

			assert.Equal(t, tt.expectTrue, result)

			if tt.expectData {
				assertChannelReceivesData(t, ch, tt.message, tt.taskCtx)
			}
		})
	}
}

// =============================================================================
// Non-blocking Behavior Tests
// =============================================================================

// TestNotify_BufferFull verifies non-blocking behavior when buffer is full.
func TestNotify_BufferFull(t *testing.T) {
	n := notifier.NewBaseNotifier("test", true, 2)

	assert.True(t, n.Notify(task.NewTaskContext(), "msg1"))
	assert.True(t, n.Notify(task.NewTaskContext(), "msg2"))

	done := make(chan bool, 1)
	go func() {
		result := n.Notify(task.NewTaskContext(), "msg3")
		done <- result
	}()

	select {
	case result := <-done:
		assert.False(t, result, "Notify should return false immediately when buffer is full")
	case <-time.After(testNotifierTimeout):
		t.Fatal("Notify blocked when it should be non-blocking")
	}
}

// TestNotify_Concurrency verifies concurrency safety.
func TestNotify_Concurrency(t *testing.T) {
	n := notifier.NewBaseNotifier("test", true, 100)

	concurrency := 50
	wg := sync.WaitGroup{}
	wg.Add(concurrency)

	successCount := int32(0)
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			if n.Notify(task.NewTaskContext(), "concurrent message") {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	assert.Greater(t, successCount, int32(0), "At least some messages should succeed")
}

// =============================================================================
// Close Method Tests
// =============================================================================

// TestClose_Idempotent verifies Close is idempotent.
func TestClose_Idempotent(t *testing.T) {
	n := notifier.NewBaseNotifier("test", true, testNotifierBufferSize)

	n.Close()
	assert.False(t, n.Notify(task.NewTaskContext(), "test"), "Notify should return false after close")

	assert.NotPanics(t, func() {
		n.Close()
	}, "Second Close() should not panic")
}

// TestClose_AfterNotify verifies behaviors after Close.
func TestClose_AfterNotify(t *testing.T) {
	n := notifier.NewBaseNotifier("test", true, testNotifierBufferSize)

	require.True(t, n.Notify(task.NewTaskContext(), "msg1"))
	require.True(t, n.Notify(task.NewTaskContext(), "msg2"))

	n.Close()
	// Channel should not be nil (for Drain), but Notify should fail
	assert.NotNil(t, n.RequestC)

	result := n.Notify(task.NewTaskContext(), "msg3")
	assert.False(t, result, "Notify should return false after close")
}
