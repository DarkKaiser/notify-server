package notifier

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// =============================================================================
// Constants & Helpers
// =============================================================================

const (
	testID          = contract.NotifierID("test-notifier")
	testMessage     = "test-message"
	testSafeTimeout = 5 * time.Second
)

// TestMain runs tests and checks for goroutine leaks.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// =============================================================================
// 1. Initialization & Basic Behavior
// =============================================================================

func TestBase_Initialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		id           contract.NotifierID
		supportsHTML bool
		bufferSize   int
		timeout      time.Duration
	}{
		{"Standard Config", "notifier-standard", true, 100, 1 * time.Second},
		{"Zero Buffer (Unbuffered)", "notifier-sync", false, 0, 500 * time.Millisecond},
		{"Large Config", "notifier-large", true, 10000, 1 * time.Minute},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Action
			n := NewBase(tt.id, tt.supportsHTML, tt.bufferSize, tt.timeout)

			// Assertion: Fields
			assert.Equal(t, tt.id, n.ID())
			assert.Equal(t, tt.supportsHTML, n.SupportsHTML())
			assert.Equal(t, tt.timeout, n.enqueueTimeout)

			// Assertion: Channel State
			// NotificationC should be initialized
			require.NotNil(t, n.NotificationC())
			assert.Equal(t, tt.bufferSize, cap(n.NotificationC()))

			// Done channel should be open initially
			select {
			case <-n.Done():
				t.Fatal("Done() channel should be open (not closed) initially")
			default:
				// OK
			}
		})
	}
}

// =============================================================================
// 2. Send Logic (Core Scenarios)
// =============================================================================

func TestBase_Send(t *testing.T) {
	t.Parallel()

	t.Run("Success: Buffered Send (Space Available)", func(t *testing.T) {
		t.Parallel()
		n := NewBase(testID, true, 1, testSafeTimeout)
		ctx := contract.NewTaskContext()

		// Queue has space (1), so this should not block and return nil
		err := n.Send(ctx, testMessage)
		require.NoError(t, err)

		// Verify message in queue
		select {
		case msg := <-n.NotificationC():
			assert.Equal(t, testMessage, msg.Message)
			assert.Equal(t, ctx, msg.TaskContext)
		default:
			t.Fatal("Queue should contain the sent message")
		}
	})

	t.Run("Success: Unbuffered Send (Consumer Ready)", func(t *testing.T) {
		t.Parallel()
		n := NewBase(testID, true, 0, testSafeTimeout)

		// Unbuffered channel requires a receiver
		go func() {
			select {
			case <-n.NotificationC():
				// Consumed
			case <-time.After(testSafeTimeout):
				// Prevent test hang
			}
		}()

		// Wait briefly to ensure consumer is ready (though not strictly guaranteed without sync, NewBase readiness is enough here)
		time.Sleep(10 * time.Millisecond)

		err := n.Send(contract.NewTaskContext(), testMessage)
		require.NoError(t, err)
	})

	t.Run("Success: Nil TaskContext is Allowed", func(t *testing.T) {
		t.Parallel()
		n := NewBase(testID, true, 1, testSafeTimeout)

		err := n.Send(nil, testMessage)
		require.NoError(t, err)

		msg := <-n.NotificationC()
		assert.Equal(t, testMessage, msg.Message)
		assert.Nil(t, msg.TaskContext)
	})

	t.Run("Failure: Queue Full (Timeout)", func(t *testing.T) {
		t.Parallel()
		// Buffer size 0 and no consumer -> immediately blocking
		// Small timeout to fail fast
		shortTimeout := 10 * time.Millisecond
		n := NewBase(testID, true, 0, shortTimeout)

		start := time.Now()
		err := n.Send(contract.NewTaskContext(), testMessage)

		// Should return ErrQueueFull after approx shortTimeout
		require.ErrorIs(t, err, ErrQueueFull)

		elapsed := time.Since(start)
		assert.GreaterOrEqual(t, elapsed, shortTimeout, "Should wait at least the timeout duration")
		// Allow generous margin for context switch / CI slowness
		assert.Less(t, elapsed, shortTimeout+200*time.Millisecond, "Should not wait excessively longer than timeout")
	})

	t.Run("Failure: Context Canceled (Pre-check)", func(t *testing.T) {
		t.Parallel()
		n := NewBase(testID, true, 0, testSafeTimeout)

		// Create a pre-canceled context
		ctx := contract.NewTaskContext()
		_, cancel := context.WithCancel(ctx) // This wraps standard Context
		cancel()                             // Cancel immediately

		// We need to inject this canceled context behavior into TaskContext.
		// Since contract.NewTaskContext() returns a struct implemented in `task_context.go` which embeds `context.Context`,
		// wrapping it with `context.WithCancel` works if we cast it back or if the interface allows access to Done().
		//
		// Challenge: `contract.TaskContext` is an interface. `context.WithCancel` returns `context.Context`.
		// We CANNOT simply pass the result of `WithCancel` to `Send` because parameters don't match.
		//
		// Solution: Use a test helper or struct embedding to construct a "Canceled TaskContext".
		canceledTaskCtx := &testCanceledTaskContext{
			TaskContext: ctx,
			doneCh:      make(chan struct{}),
		}
		close(canceledTaskCtx.doneCh) // Pre-closed

		err := n.Send(canceledTaskCtx, testMessage)
		require.ErrorIs(t, err, ErrContextCanceled)
	})

	t.Run("Failure: Notifier Closed", func(t *testing.T) {
		t.Parallel()
		n := NewBase(testID, true, 1, testSafeTimeout)
		n.Close()

		err := n.Send(contract.NewTaskContext(), testMessage)
		require.ErrorIs(t, err, ErrClosed)
	})

	t.Run("Failure: Panic Recovery", func(t *testing.T) {
		t.Parallel()
		n := NewBase(testID, true, 1, testSafeTimeout)

		// Cause a panic by manually closing the send channel while simulating sending
		// Go panics when sending to a closed channel.
		// Note: Accessing internal field for white-box testing
		close(n.notificationC)

		assert.NotPanics(t, func() {
			err := n.Send(contract.NewTaskContext(), "trigger panic")
			assert.ErrorIs(t, err, ErrPanicRecovered)
		})
	})
}

// =============================================================================
// 3. Lifecycle & Shutdown Logic
// =============================================================================

func TestBase_Close_Behavior(t *testing.T) {
	t.Parallel()

	n := NewBase(testID, true, 1, testSafeTimeout)

	// 1. Initial State
	select {
	case <-n.Done():
		t.Fatal("Done channel should be open initially")
	default:
	}

	// 2. Perform Close
	n.Close()

	// 3. Check Done Channel (Should be closed)
	select {
	case <-n.Done():
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed immediately after Close()")
	}

	// 4. Idempotency (Calling Close again should be safe)
	assert.NotPanics(t, func() {
		n.Close()
	})

	// 5. Check Internal Channel State (Design Decision: notificationC is NOT closed)
	// We verify we can still read from it if it had items (though it's empty now).
	// More importantly, we verify we generally don't panic or misbehave.
	// Since we can't easily check "is closed" without risk, we rely on behavior.
	// But `Send` should reject new items.
	err := n.Send(nil, "new msg")
	assert.ErrorIs(t, err, ErrClosed)
}

func TestBase_Close_UnblocksBlockingSend(t *testing.T) {
	t.Parallel()
	n := NewBase(testID, true, 0, 1*time.Minute) // Unbuffered, blocks forever without receiver

	// Channel to signal send started
	sendErrCh := make(chan error, 1)

	// Start blocking Send
	go func() {
		sendErrCh <- n.Send(contract.NewTaskContext(), "blocking")
	}()

	// Ensure Send is definitely blocking or about to block
	time.Sleep(50 * time.Millisecond)

	// Action: Output Close triggers cancellation of Send
	start := time.Now()
	n.Close()

	// Verify
	select {
	case err := <-sendErrCh:
		// Expect ErrClosed because n.done was closed
		require.ErrorIs(t, err, ErrClosed)
		assert.WithinDuration(t, start, time.Now(), 100*time.Millisecond, "Close should unblock Send immediately")
	case <-time.After(1 * time.Second):
		t.Fatal("Close did not unblock Send goroutine")
	}
}

// =============================================================================
// 4. Concurrency & Race Conditions
// =============================================================================

func TestBase_Concurrency_Stress(t *testing.T) {
	t.Parallel()

	workerCount := 10
	msgCount := 10
	n := NewBase(testID, true, workerCount*msgCount, testSafeTimeout)

	var wg sync.WaitGroup
	wg.Add(workerCount)

	// Multiple writers
	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < msgCount; j++ {
				// We don't check error here, focusing on race detection
				_ = n.Send(nil, testMessage)
			}
		}()
	}

	// Single Reader (Consumer)
	consumedCount := 0
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		for {
			select {
			case <-n.NotificationC():
				consumedCount++
			case <-n.Done():
				// Drain remaining
				for {
					select {
					case <-n.NotificationC():
						consumedCount++
					default:
						// Empty
						return
					}
				}
			}
		}
	}()

	wg.Wait()
	n.Close()
	<-doneCh

	// Since we closed AFTER wg.Wait, all messages *should* have been accepted.
	// However, if logic allows drops or timeouts, count might differ.
	// In this buffered setup with SafeTimeout, drops are unlikely unless logic fails.
	// But primarily this test is for RACE DETECTION.
}

// =============================================================================
// Helpers / Mocks
// =============================================================================

// testCanceledTaskContext wraps a TaskContext to simulate cancellation
type testCanceledTaskContext struct {
	contract.TaskContext
	doneCh chan struct{}
}

func (m *testCanceledTaskContext) Done() <-chan struct{} {
	return m.doneCh
}
