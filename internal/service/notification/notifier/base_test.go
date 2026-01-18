package notifier

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Constants & Helpers
// =============================================================================

const (
	testID           = contract.NotifierID("test-notifier")
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
		tt := tt
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
// 2. Send Logic Tests (Table-Driven)
// =============================================================================

func TestBase_Send(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		bufferSize  int
		timeout     time.Duration
		setup       func(*Base) (contract.TaskContext, func()) // returns context and teardown/trigger
		assertError func(*testing.T, error)
		assertState func(*testing.T, *Base)
	}

	tests := []testCase{
		{
			name:       "Success_Buffered_Immediate",
			bufferSize: 1,
			timeout:    time.Second,
			setup: func(n *Base) (contract.TaskContext, func()) {
				return contract.NewTaskContext(), func() {}
			},
			assertError: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
			assertState: func(t *testing.T, n *Base) {
				req := waitForRequestC(t, n.RequestC(), testDrainTimeout)
				require.NotNil(t, req, "Request should be in the channel")
				assert.Equal(t, testMessage, req.Message)
			},
		},
		{
			name:       "Success_Unbuffered_WithConsumer",
			bufferSize: 0,
			timeout:    time.Second,
			setup: func(n *Base) (contract.TaskContext, func()) {
				// Start consumer to unblock Send
				go func() {
					time.Sleep(10 * time.Millisecond)
					<-n.RequestC()
				}()
				return contract.NewTaskContext(), func() {}
			},
			assertError: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
			assertState: nil,
		},
		{
			name:       "Failure_QueueFull_Timeout",
			bufferSize: 0,
			timeout:    10 * time.Millisecond,
			setup: func(n *Base) (contract.TaskContext, func()) {
				// No consumer -> triggers timeout
				return contract.NewTaskContext(), func() {}
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrQueueFull)
			},
			assertState: nil,
		},
		{
			name:       "Failure_ContextCanceled",
			bufferSize: 0,
			timeout:    time.Second,
			setup: func(n *Base) (contract.TaskContext, func()) {
				// Context cancelled immediately
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				// Wrap manually to simulate contract.TaskContext behavior if needed,
				// but here we just need Done() to be closed.
				// Since contract.TaskContext interface has Done(), we can make a verified mock or just use a real one.
				// However, contract.TaskContext doesn't expose cancel directly unless we use the implementation details.
				// The previous test used a wrapper. Let's use a cleaner wrapper.
				return &wrapperTaskContext{TaskContext: contract.NewTaskContext(), ctx: ctx}, func() {}
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrContextCanceled)
			},
			assertState: nil,
		},
		{
			name:       "Failure_ClosedNotifier",
			bufferSize: 10,
			timeout:    time.Second,
			setup: func(n *Base) (contract.TaskContext, func()) {
				n.Close()
				return contract.NewTaskContext(), func() {}
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrClosed)
			},
			assertState: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := NewBase(testID, true, tt.bufferSize, tt.timeout)

			ctx, cleanup := tt.setup(&n)
			defer cleanup()

			err := n.Send(ctx, testMessage)

			if tt.assertError != nil {
				tt.assertError(t, err)
			}
			if tt.assertState != nil {
				tt.assertState(t, &n)
			}
		})
	}
}

// wrapperTaskContext is a helper to inject a cancelable context into TaskContext
type wrapperTaskContext struct {
	contract.TaskContext
	ctx context.Context
}

func (w *wrapperTaskContext) Done() <-chan struct{} {
	return w.ctx.Done()
}

// =============================================================================
// 3. Close & Lifecycle Tests
// =============================================================================

func TestBase_Close_Lifecycle(t *testing.T) {
	t.Parallel()

	n := NewBase(testID, true, 10, time.Second)
	doneCh := n.Done()

	// 1. Initial State
	select {
	case <-doneCh:
		t.Fatal("Done channel should be open initially")
	default:
	}

	// 2. Close
	n.Close()
	assert.True(t, n.closed)

	// 3. Verify Signal Broadcast
	select {
	case <-doneCh:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed after Close()")
	}

	// 4. Idempotency (Should not panic)
	assert.NotPanics(t, func() {
		n.Close()
	})
}

func TestBase_Close_UnblocksSend(t *testing.T) {
	t.Parallel()

	n := NewBase(testID, true, 0, 2*time.Second) // Unbuffered, Long timeout

	errCh := make(chan error)
	go func() {
		errCh <- n.Send(contract.NewTaskContext(), "blocked")
	}()

	// Ensure Send is blocked
	time.Sleep(50 * time.Millisecond)

	// Close should unblock Send
	start := time.Now()
	n.Close()

	select {
	case err := <-errCh:
		assert.ErrorIs(t, err, ErrClosed)
		assert.Less(t, time.Since(start), 100*time.Millisecond, "Send should return immediately after Close")
	case <-time.After(time.Second):
		t.Fatal("Send did not return after Close")
	}
}

// =============================================================================
// 4. Panic Recovery Tests
// =============================================================================

func TestBase_PanicRecovery(t *testing.T) {
	t.Parallel()

	n := NewBase(testID, true, 10, time.Second)

	// Force panic by closing the internal channel manually (simulating catastrophic bug)
	// This is white-box testing accessing unexported field, which is acceptable for internal tests
	close(n.requestC)

	assert.NotPanics(t, func() {
		err := n.Send(contract.NewTaskContext(), "trigger panic")
		// Verify exact error type
		assert.ErrorIs(t, err, ErrPanicRecovered)
	})
}

// =============================================================================
// 5. Concurrency Stress Tests
// =============================================================================

func TestBase_Concurrency_Send(t *testing.T) {
	t.Parallel()

	count := 100
	n := NewBase(testID, true, count, time.Second)

	var wg sync.WaitGroup
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			err := n.Send(contract.NewTaskContext(), "msg")
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
	assert.Equal(t, count, len(n.RequestC()))
}

func TestBase_Concurrency_CloseVsSend(t *testing.T) {
	t.Parallel()

	// This test aims to crash the system by racing Close and Send
	n := NewBase(testID, true, 0, 10*time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			// Ignore errors, we just want to ensure no panics
			_ = n.Send(contract.NewTaskContext(), "race")
		}
	}()

	go func() {
		defer wg.Done()
		time.Sleep(2 * time.Millisecond)
		n.Close()
	}()

	wg.Wait()
}
