package notifier_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/notification/types"
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
			n := notifier.NewBaseNotifier(types.NotifierID(tt.id), tt.supportsHTML, tt.bufferSize, testNotifierTimeout)

			assert.Equal(t, types.NotifierID(tt.id), n.ID())
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
			Notifier:   notifier.NewBaseNotifier("test", true, testNotifierBufferSize, testNotifierTimeout),
			message:    testNotifierMessage,
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "Success: nil TaskContext",
			Notifier:   notifier.NewBaseNotifier("test", true, testNotifierBufferSize, testNotifierTimeout),
			message:    testNotifierMessage,
			taskCtx:    nil,
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "Success: Empty Message",
			Notifier:   notifier.NewBaseNotifier("test", true, testNotifierBufferSize, testNotifierTimeout),
			message:    "",
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "Success: Long Message (10KB)",
			Notifier:   notifier.NewBaseNotifier("test", true, testNotifierBufferSize, testNotifierTimeout),
			message:    strings.Repeat("a", 10000),
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name: "Failure: Closed Channel (nil check)",
			Notifier: func() notifier.BaseNotifier {
				n := notifier.NewBaseNotifier("test", true, testNotifierBufferSize, testNotifierTimeout)
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

// TestNotify_Backpressure verifies blocking behavior (Backpressure) when buffer is full.
func TestNotify_Backpressure(t *testing.T) {
	// Small timeout for test
	n := notifier.NewBaseNotifier("test", true, 1, 100*time.Millisecond)

	// Fill buffer
	require.True(t, n.Notify(task.NewTaskContext(), "msg1"))

	done := make(chan bool)
	go func() {
		// This should block until msg1 is drained
		result := n.Notify(task.NewTaskContext(), "msg2")
		done <- result
	}()

	select {
	case <-done:
		t.Fatal("Notify should block when buffer is full")
	case <-time.After(50 * time.Millisecond):
		// Expected: still blocking
	}

	// Drain one item
	select {
	case <-n.RequestC:
		// drained
	default:
		t.Fatal("Buffer should have an item")
	}

	// Now Notify should succeed
	select {
	case result := <-done:
		assert.True(t, result, "Notify should succeed after draining")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Notify did not return after draining")
	}
}

// TestNotify_Timeout verifies that Notify returns false after timeout if buffer remains full.
func TestNotify_Timeout(t *testing.T) {
	n := notifier.NewBaseNotifier("test", true, 1, 50*time.Millisecond)

	require.True(t, n.Notify(task.NewTaskContext(), "msg1"))

	start := time.Now()
	result := n.Notify(task.NewTaskContext(), "msg2")
	elapsed := time.Since(start)

	assert.False(t, result, "Notify should return false after timeout")
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond, "Should block for at least timeout duration")
}

// TestNotify_Concurrency verifies concurrency safety.
func TestNotify_Concurrency(t *testing.T) {
	n := notifier.NewBaseNotifier("test", true, 100, testNotifierTimeout)

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
	n := notifier.NewBaseNotifier("test", true, testNotifierBufferSize, testNotifierTimeout)

	n.Close()
	assert.False(t, n.Notify(task.NewTaskContext(), "test"), "Notify should return false after close")

	assert.NotPanics(t, func() {
		n.Close()
	}, "Second Close() should not panic")
}

// TestClose_AfterNotify verifies behaviors after Close.
func TestClose_AfterNotify(t *testing.T) {
	n := notifier.NewBaseNotifier("test", true, testNotifierBufferSize, testNotifierTimeout)

	require.True(t, n.Notify(task.NewTaskContext(), "msg1"))
	require.True(t, n.Notify(task.NewTaskContext(), "msg2"))

	n.Close()
	// Channel should not be nil (for Drain), but Notify should fail
	assert.NotNil(t, n.RequestC)

	result := n.Notify(task.NewTaskContext(), "msg3")
	assert.False(t, result, "Notify should return false after close")
}

// TestNotify_CloseDuringBlock verifies that Close() can be called and returns immediately
// even if Notify() is blocked waiting for buffer space.
func TestNotify_CloseDuringBlock(t *testing.T) {
	// Create a notifier with buffer size 1 and long timeout
	n := notifier.NewBaseNotifier("test-blocking", true, 1, 5*time.Second)

	// Fill the buffer
	require.True(t, n.Notify(task.NewTaskContext(), "msg1"), "First message should fill buffer")

	// WaitGroup to synchronize start of blocking call
	var wgStart sync.WaitGroup
	wgStart.Add(1)

	// Channel to signal that Notify finished
	notifyDone := make(chan bool)

	// 1. Start a goroutine that will block on Notify
	go func() {
		wgStart.Done()
		// This should block because buffer is full
		result := n.Notify(task.NewTaskContext(), "msg2")
		notifyDone <- result
	}()

	wgStart.Wait()
	// Give a small amount of time for the goroutine to actually enter the select block
	time.Sleep(50 * time.Millisecond)

	// 2. Call Close() - This should NOT block waiting for Notify timeout
	startClose := time.Now()
	n.Close()
	closeDuration := time.Since(startClose)

	// Close should happen almost instantly, definitely much faster than the 5s timeout
	assert.Less(t, closeDuration, 1*time.Second, "Close should prevent blocking waiting for timeout")

	// 3. Verify Notify's result
	select {
	case result := <-notifyDone:
		// Notify should fail (via panic recover) when channel is closed during send
		assert.False(t, result, "Notify should fail (via panic recover) when channel is closed during send")
	case <-time.After(1 * time.Second):
		t.Fatal("Notify did not return after Close")
	}
}

func TestBaseNotifier_Notify_PanicRecovery(t *testing.T) {
	// Setup
	n := notifier.NewBaseNotifier("test_notifier", true, 10, time.Second)

	// Simulate a scenario that causes panic:
	// Sending to a closed channel causes panic.
	// We manually close the channel without setting the 'closed' flag via Close().
	close(n.RequestC)

	// Test
	// This should NOT panic, but return false
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Notify panicked: %v", r)
		}
	}()

	succeeded := n.Notify(nil, "test message")

	// Verification
	assert.False(t, succeeded, "Notify should return false on panic recovery")
}
