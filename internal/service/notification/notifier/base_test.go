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
// Constants & Helpers
// =============================================================================

const (
	testID           = contract.NotifierID("test-notifier")
	testMessage      = "test-message"
	testShortTimeout = 10 * time.Millisecond
	testSafeTimeout  = 5 * time.Second
)

// =============================================================================
// 1. Initialization & Accessors
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
		{"Standard Config", "notifier-normal", true, 100, 1 * time.Second},
		{"Zero Buffer", "notifier-sync", false, 0, 500 * time.Millisecond},
		{"Large Config", "notifier-large", true, 10000, 1 * time.Minute},
	}

	for _, tt := range tests {
		tt := tt // capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			n := NewBase(tt.id, tt.supportsHTML, tt.bufferSize, tt.timeout)

			// Field verification
			assert.Equal(t, tt.id, n.ID())
			assert.Equal(t, tt.supportsHTML, n.SupportsHTML())
			assert.Equal(t, tt.timeout, n.enqueueTimeout)

			// Channel initialization verification
			require.NotNil(t, n.NotificationC())
			assert.Equal(t, tt.bufferSize, cap(n.NotificationC()))

			// Done channel should be open
			select {
			case <-n.Done():
				t.Fatal("Done() channel should be open initially")
			default:
				// OK
			}
		})
	}
}

// =============================================================================
// 2. Send Logic (Core Behavior)
// =============================================================================

func TestBase_Send(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		bufferSize  int
		timeout     time.Duration
		prepare     func(n *Base) (contract.TaskContext, context.CancelFunc)
		action      func(n *Base, ctx contract.TaskContext) error
		expectError error
		verify      func(t *testing.T, n *Base)
	}

	tests := []testCase{
		{
			name:       "Success: Buffered Send (Space Available)",
			bufferSize: 1,
			timeout:    testSafeTimeout,
			prepare: func(n *Base) (contract.TaskContext, context.CancelFunc) {
				return contract.NewTaskContext(), func() {}
			},
			action: func(n *Base, ctx contract.TaskContext) error {
				return n.Send(ctx, testMessage)
			},
			expectError: nil,
			verify: func(t *testing.T, n *Base) {
				select {
				case noti := <-n.NotificationC():
					assert.Equal(t, testMessage, noti.Message)
					assert.NotNil(t, noti.TaskContext)
				default:
					t.Fatal("Message should be in the queue")
				}
			},
		},
		{
			name:       "Success: Unbuffered Send (Consumer Ready)",
			bufferSize: 0,
			timeout:    testSafeTimeout,
			prepare: func(n *Base) (contract.TaskContext, context.CancelFunc) {
				// Start a consumer to read the message immediately
				go func() {
					<-n.NotificationC()
				}()
				return contract.NewTaskContext(), func() {}
			},
			action: func(n *Base, ctx contract.TaskContext) error {
				return n.Send(ctx, testMessage)
			},
			expectError: nil,
			verify:      nil,
		},
		{
			name:       "Success: Nil TaskContext",
			bufferSize: 1,
			timeout:    testSafeTimeout,
			prepare: func(n *Base) (contract.TaskContext, context.CancelFunc) {
				return nil, func() {}
			},
			action: func(n *Base, ctx contract.TaskContext) error {
				// Pass nil explicitly
				return n.Send(nil, testMessage)
			},
			expectError: nil,
			verify: func(t *testing.T, n *Base) {
				select {
				case noti := <-n.NotificationC():
					assert.Equal(t, testMessage, noti.Message)
					assert.Nil(t, noti.TaskContext)
				default:
					t.Fatal("Message should be in the queue even with nil context")
				}
			},
		},
		{
			name:       "Failure: Queue Full (Timeout)",
			bufferSize: 0,
			timeout:    10 * time.Millisecond, // very short timeout
			prepare: func(n *Base) (contract.TaskContext, context.CancelFunc) {
				// No consumer -> simulates full queue
				return contract.NewTaskContext(), func() {}
			},
			action: func(n *Base, ctx contract.TaskContext) error {
				return n.Send(ctx, testMessage)
			},
			expectError: ErrQueueFull,
			verify:      nil,
		},
		{
			name:       "Failure: Closed Notifier",
			bufferSize: 10,
			timeout:    testSafeTimeout,
			prepare: func(n *Base) (contract.TaskContext, context.CancelFunc) {
				n.Close() // Close before sending
				return contract.NewTaskContext(), func() {}
			},
			action: func(n *Base, ctx contract.TaskContext) error {
				return n.Send(ctx, testMessage)
			},
			expectError: ErrClosed,
			verify:      nil,
		},
		{
			name:       "Failure: Caller Context Canceled",
			bufferSize: 0,
			timeout:    testSafeTimeout,
			prepare: func(n *Base) (contract.TaskContext, context.CancelFunc) {
				// Create a cancelable context
				ctx := contract.NewTaskContext()
				// We need to simulate the underlying context cancellation.
				// Since contract.TaskContext interface doesn't expose Cancel(),
				// we pass a custom context implementation OR use internal knowledge.
				//
				// Better approach: Test integration with standard context cancellation.
				// However, `Base.Send` takes `contract.TaskContext`.
				//
				// Let's use a mock or custom impl for this specific test
				// to avoid depending on specific `TaskContext` impl details.
				wrappableCtx, cancel := context.WithCancel(context.Background())
				// Cancel immediately
				cancel()

				return &mockCanceledContext{
					TaskContext: ctx,
					done:        wrappableCtx.Done(),
				}, func() {}
			},
			action: func(n *Base, ctx contract.TaskContext) error {
				return n.Send(ctx, testMessage)
			},
			expectError: ErrContextCanceled,
			verify:      nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			n := NewBase(testID, true, tt.bufferSize, tt.timeout)

			ctx, cleanup := tt.prepare(&n)
			defer cleanup()

			err := tt.action(&n, ctx)

			if tt.expectError != nil {
				assert.ErrorIs(t, err, tt.expectError)
			} else {
				assert.NoError(t, err)
			}

			if tt.verify != nil {
				tt.verify(t, &n)
			}
		})
	}
}

// mockCanceledContext allows injecting a done channel
type mockCanceledContext struct {
	contract.TaskContext
	done <-chan struct{}
}

func (m *mockCanceledContext) Done() <-chan struct{} {
	return m.done
}

// =============================================================================
// 3. Lifecycle (Close) Behavior
// =============================================================================

func TestBase_Close_Idempotency(t *testing.T) {
	t.Parallel()

	n := NewBase(testID, true, 10, testSafeTimeout)

	// First Close
	n.Close()
	select {
	case <-n.Done():
		// OK
	default:
		t.Fatal("Done() should be closed after Close()")
	}

	// Second Close (Should not panic)
	assert.NotPanics(t, func() {
		n.Close()
	})

	// Should remain closed
	err := n.Send(nil, "msg")
	assert.ErrorIs(t, err, ErrClosed)
}

func TestBase_Close_UnblocksWaitingSend(t *testing.T) {
	t.Parallel()

	// Unbuffered channel with long timeout -> Send will block
	n := NewBase(testID, true, 0, 1*time.Minute)

	errCh := make(chan error)
	readyCh := make(chan struct{})

	go func() {
		close(readyCh) // Signal that goroutine started
		errCh <- n.Send(contract.NewTaskContext(), "blocked")
	}()

	<-readyCh
	// Yield to allow Send to block
	time.Sleep(50 * time.Millisecond)

	// Action: call Close
	start := time.Now()
	n.Close()

	// Verify
	select {
	case err := <-errCh:
		// Logic: Send should return ErrClosed immediately when Close() is called
		assert.ErrorIs(t, err, ErrClosed)
		assert.WithinDuration(t, start, time.Now(), 100*time.Millisecond, "Send should unblock immediately")
	case <-time.After(1 * time.Second):
		t.Fatal("Send did not return after Close()")
	}
}

// =============================================================================
// 4. Panic Recovery (White-box Test)
// =============================================================================

func TestBase_PanicRecovery(t *testing.T) {
	t.Parallel()

	n := NewBase(testID, true, 10, testSafeTimeout)

	// Simulate a catastrophic state (nil channel) that would cause panic on send
	// We access unexported field 'notificationC' for white-box testing.
	// NOTE: Depending on implementation, sending to nil channel blocks forever, not panic.
	// EXCEPT if we close it? Sending to closed channel panics.

	// Let's rely on internal implementation: Close channel manually to cause panic on send?
	// Go spec: send to closed channel causes panic.
	close(n.notificationC)

	assert.NotPanics(t, func() {
		err := n.Send(contract.NewTaskContext(), "will panic internal")
		assert.ErrorIs(t, err, ErrPanicRecovered)
	}, "Send should catch panic and return error")
}

// =============================================================================
// 5. Concurrency Stress Test
// =============================================================================

func TestBase_Concurrency_Safe(t *testing.T) {
	t.Parallel()

	// High concurrency to detect race conditions with -race flag
	concurrency := 100
	loops := 50
	n := NewBase(testID, true, concurrency, testSafeTimeout)

	var wg sync.WaitGroup
	wg.Add(concurrency)

	// Launch multiple senders
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for k := 0; k < loops; k++ {
				// Ignores error (queue might be full or closed, just checking for data races)
				_ = n.Send(nil, testMessage)
			}
		}()
	}

	// Simultaneously read from channel to clear buffer
	consumerStop := make(chan struct{})
	go func() {
		for {
			select {
			case <-n.NotificationC():
				// consume
			case <-consumerStop:
				return
			}
		}
	}()

	// Randomly Close later
	go func() {
		time.Sleep(10 * time.Millisecond)
		n.Close()
	}()

	// Wait for all senders to finish
	wg.Wait()

	// Stop consumer
	close(consumerStop)

	// Close notifier asynchronously if not already closed
	// (In some race scenarios, it might have been closed by the random closer)
	// But here we just want to ensure it's eventually closed for the check.
	// Actually, the random closer is already running. We just need to wait for it.

	// Better approach: Wait for Done() with timeout (simulate Eventually)
	select {
	case <-n.Done():
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Notifier should be closed by now (timeout waiting for Done)")
	}
}
