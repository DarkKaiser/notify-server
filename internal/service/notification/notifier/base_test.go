package notifier

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Test Constants & Helpers
// =============================================================================

const (
	testID           = contract.NotifierID("test-notifier")
	testBufferSize   = 5
	testTimeout      = 100 * time.Millisecond
	testMessage      = "hello world"
	testDrainTimeout = 200 * time.Millisecond
)

// waitForRequestC waits for a message on the channel or times out.
func waitForRequestC(t *testing.T, ch <-chan *Request, timeout time.Duration) *Request {
	t.Helper()
	select {
	case req := <-ch:
		return req
	case <-time.After(timeout):
		t.Fatal("timeout waiting for request")
		return nil
	}
}

// =============================================================================
// 1. Constructor & Getters Tests
// =============================================================================

func TestBase_Constructor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		id           contract.NotifierID
		supportsHTML bool
		bufferSize   int
		timeout      time.Duration
	}{
		{"Standard", "notifier-1", true, 10, time.Second},
		{"NoBuffer", "notifier-2", false, 0, 500 * time.Millisecond},
		{"LargeBuffer", "notifier-3", true, 1000, time.Minute},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := NewBase(tt.id, tt.supportsHTML, tt.bufferSize, tt.timeout)

			assert.Equal(t, tt.id, n.ID())
			assert.Equal(t, tt.supportsHTML, n.SupportsHTML())
			assert.NotNil(t, n.RequestC())
			assert.Equal(t, tt.bufferSize, cap(n.RequestC()))
			assert.NotNil(t, n.Done())
			assert.Equal(t, tt.timeout, n.enqueueTimeout)
		})
	}
}

// =============================================================================
// 2. Notify Logic Tests
// =============================================================================

func TestBase_Notify_Success(t *testing.T) {
	t.Parallel()

	n := NewBase(testID, true, 1, testTimeout)
	ctx := contract.NewTaskContext()

	// When: Notify called
	ok := n.Notify(ctx, testMessage)

	// Then: Should succeed and be received
	assert.True(t, ok)
	req := waitForRequestC(t, n.RequestC(), testDrainTimeout)
	assert.Equal(t, testMessage, req.Message)
	assert.Equal(t, ctx, req.TaskContext)
}

func TestBase_Notify_BufferFull_Drop(t *testing.T) {
	t.Parallel()

	// Given: Buffer size 0 (unbuffered), short timeout
	n := NewBase(testID, true, 0, 10*time.Millisecond)

	// When: Sending without a receiver
	// Since channel is unbuffered and no one is reading, this blocks until timeout
	ok := n.Notify(contract.NewTaskContext(), "dropped message")

	// Then: Should return false (Time out -> Drop)
	assert.False(t, ok, "Notify should fail when buffer is full (or unbuffered) and timeout passes")
}

func TestBase_Notify_Backpressure_Success(t *testing.T) {
	t.Parallel()

	// Given: Unbuffered channel, but we will start a receiver after a short delay
	n := NewBase(testID, true, 0, 200*time.Millisecond)

	go func() {
		time.Sleep(50 * time.Millisecond) // Simulate slow consumer
		<-n.RequestC()
	}()

	// When: Notify called
	start := time.Now()
	ok := n.Notify(contract.NewTaskContext(), "delayed message")
	elapsed := time.Since(start)

	// Then: Should succeed because consumer picked it up within timeout (200ms)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond, "Should have waited for consumer")
}

// cancelledTaskContext wraps contract.TaskContext but overrides Done()
type cancelledTaskContext struct {
	contract.TaskContext
	done <-chan struct{}
}

func (c *cancelledTaskContext) Done() <-chan struct{} {
	return c.done
}

func TestBase_Notify_ContextCancelled(t *testing.T) {
	t.Parallel()

	// Given: Unbuffered channel, very long timeout
	n := NewBase(testID, true, 0, 10*time.Second)

	// And: A cancelled context
	// We use standard context to get a closed Done channel
	stdCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Wrap it in our custom TaskContext
	taskCtx := &cancelledTaskContext{
		TaskContext: contract.NewTaskContext(),
		done:        stdCtx.Done(),
	}

	// When: Notify called
	start := time.Now()
	ok := n.Notify(taskCtx, "skipped message")
	elapsed := time.Since(start)

	// Then: Should return false IMMEDIATELY
	assert.False(t, ok, "Notify should fail when context is cancelled")
	assert.Less(t, elapsed, 100*time.Millisecond, "Should not wait for enqueue timeout")
}

func TestBase_Notify_Closed(t *testing.T) {
	t.Parallel()

	n := NewBase(testID, true, 10, testTimeout)
	n.Close()

	// When: Notify on closed notifier
	ok := n.Notify(contract.NewTaskContext(), "ignore me")

	// Then: Should return false immediately
	assert.False(t, ok)
}

// =============================================================================
// 3. Close & Lifecycle Tests
// =============================================================================

func TestBase_Close_SignalBroadcast(t *testing.T) {
	t.Parallel()

	n := NewBase(testID, true, 10, testTimeout)
	doneCh := n.Done()

	// Pre-check: Done channel should be open
	select {
	case <-doneCh:
		t.Fatal("Done channel should be open initially")
	default:
	}

	// When: Close called
	n.Close()

	// Then: Done channel should be closed
	select {
	case <-doneCh:
		// Success: channel is closed
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed after Close()")
	}
}

func TestBase_Close_Idempotency(t *testing.T) {
	t.Parallel()

	n := NewBase(testID, true, 10, testTimeout)

	// Calling Close multiple times should not panic
	assert.NotPanics(t, func() {
		n.Close()
		n.Close()
		n.Close()
	})

	assert.True(t, n.closed)
}

func TestBase_Close_DuringNotify(t *testing.T) {
	t.Parallel()

	// Given: Unbuffered channel, long timeout
	n := NewBase(testID, true, 0, 2*time.Second)

	// Block Notify in a goroutine
	errCh := make(chan bool)
	go func() {
		// This will block because no receiver
		ok := n.Notify(contract.NewTaskContext(), "will be cancelled")
		errCh <- ok
	}()

	time.Sleep(50 * time.Millisecond) // Ensure Notify is blocked

	// When: Close called while Notify is waiting
	start := time.Now()
	n.Close()
	duration := time.Since(start)

	// Then: Close should return immediately
	assert.Less(t, duration, 100*time.Millisecond, "Close should not block")

	// And: Notify should unblock and return false
	select {
	case result := <-errCh:
		assert.False(t, result, "Notify should fail when Notifier is closed while waiting")
	case <-time.After(1 * time.Second):
		t.Fatal("Notify failed to return after Close")
	}
}

// =============================================================================
// 4. Concurrency & Race Tests
// =============================================================================

func TestBase_Concurrency_Notify(t *testing.T) {
	t.Parallel()

	concurrency := 50
	bufferSize := 50
	n := NewBase(testID, true, bufferSize, time.Second)

	var wg sync.WaitGroup
	wg.Add(concurrency)

	// Concurrent Producers
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			n.Notify(contract.NewTaskContext(), "msg")
		}()
	}

	// Wait for all to finish
	wg.Wait()

	// Verify all messages are in the channel
	assert.Equal(t, concurrency, len(n.RequestC()))
}

func TestBase_Concurrency_CloseVsNotify(t *testing.T) {
	t.Parallel()

	// This test stresses the race between Notify and Close
	// Run with -race flag to detect race conditions
	n := NewBase(testID, true, 0, 10*time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(2)

	// Routine 1: Hammer Notify
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			n.Notify(contract.NewTaskContext(), "race")
		}
	}()

	// Routine 2: Close asynchronously
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		n.Close()
	}()

	wg.Wait()
	// Pass if no panic and race detector is happy
}

// =============================================================================
// 5. Recover Scenarios
// =============================================================================

func TestBase_PanicRecovery(t *testing.T) {
	t.Parallel()

	// Given: A notifier
	n := NewBase(testID, true, 10, time.Second)

	// Simulate catastrophic state: User manually closes requestC (Simulating logic bug)
	// Note: We use the internal field directly to force this state
	close(n.requestC)

	// When: Notify called on a channel that will cause panic on send
	// Then: It should recover and return false
	assert.NotPanics(t, func() {
		ok := n.Notify(contract.NewTaskContext(), "boom")
		assert.False(t, ok, "Should return false on recovered panic")
	})
}
