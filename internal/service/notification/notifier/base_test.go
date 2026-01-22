package notifier

import (
	"context"
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
	testID             = contract.NotifierID("test-notifier")
	testMessage        = "test-message"
	testDefaultTimeout = 5 * time.Second
)

// TestMain runs tests and checks for goroutine leaks.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// =============================================================================
// 1. Initialization & Basic State
// =============================================================================

func TestBase_Initialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		id             contract.NotifierID
		supportsHTML   bool
		bufferSize     int
		enqueueTimeout time.Duration
	}{
		{
			name:           "Standard Configuration",
			id:             "notifier-standard",
			supportsHTML:   true,
			bufferSize:     100,
			enqueueTimeout: 1 * time.Second,
		},
		{
			name:           "Unbuffered (Synchronous)",
			id:             "notifier-sync",
			supportsHTML:   false,
			bufferSize:     0,
			enqueueTimeout: 500 * time.Millisecond,
		},
		{
			name:           "Large Buffer",
			id:             "notifier-large",
			supportsHTML:   true,
			bufferSize:     10000,
			enqueueTimeout: 1 * time.Minute,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Action
			n := NewBase(tt.id, tt.supportsHTML, tt.bufferSize, tt.enqueueTimeout)

			// Assertion: Fields match
			assert.Equal(t, tt.id, n.ID())
			assert.Equal(t, tt.supportsHTML, n.SupportsHTML())
			assert.Equal(t, tt.enqueueTimeout, n.enqueueTimeout)

			// Assertion: Channel State
			require.NotNil(t, n.NotificationC())
			assert.Equal(t, tt.bufferSize, cap(n.NotificationC()))

			// Assertion: Lifecycle State
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
		// Arrange
		n := NewBase(testID, true, 1, testDefaultTimeout)
		ctx := context.Background()

		// Act
		err := n.Send(ctx, contract.NewNotification(testMessage))

		// Assert
		require.NoError(t, err)

		select {
		case req := <-n.NotificationC():
			assert.Equal(t, testMessage, req.Notification.Message)
			assert.Equal(t, ctx, req.Ctx)
		default:
			t.Fatal("Queue should contain the sent message")
		}
	})

	t.Run("Success: Unbuffered Send (Consumer Ready)", func(t *testing.T) {
		t.Parallel()
		// Arrange
		n := NewBase(testID, true, 0, testDefaultTimeout)
		notification := contract.NewNotification(testMessage)

		// Start a consumer
		go func() {
			select {
			case <-n.NotificationC():
				// Consumed
			case <-time.After(testDefaultTimeout):
				// Prevent leak if test fails
			}
		}()

		// Small delay to ensure consumer is likely ready (optional but helpful for deterministic feel)
		time.Sleep(10 * time.Millisecond)

		// Act
		err := n.Send(context.Background(), notification)

		// Assert
		require.NoError(t, err)
	})

	t.Run("Success: Nil Context Defaults to Background", func(t *testing.T) {
		t.Parallel()
		n := NewBase(testID, true, 1, testDefaultTimeout)

		err := n.Send(nil, contract.NewNotification(testMessage))
		require.NoError(t, err)

		req := <-n.NotificationC()
		assert.NotNil(t, req.Ctx, "Nil context should be replaced with default context")
	})

	t.Run("Failure: Queue Full (Timeout)", func(t *testing.T) {
		t.Parallel()
		// Unbuffered channel with no consumer -> immediately blocking
		// Very short timeout for test speed
		shortTimeout := 10 * time.Millisecond
		n := NewBase(testID, true, 0, shortTimeout)

		start := time.Now()
		err := n.Send(context.Background(), contract.NewNotification(testMessage))

		// Assert
		require.ErrorIs(t, err, ErrQueueFull)
		assert.GreaterOrEqual(t, time.Since(start), shortTimeout, "Should wait at least the timeout duration")
	})

	t.Run("Failure: Context Canceled (Already Canceled)", func(t *testing.T) {
		t.Parallel()
		n := NewBase(testID, true, 1, testDefaultTimeout)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := n.Send(ctx, contract.NewNotification(testMessage))

		// Should return standard context error
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("Failure: Context Deadline Exceeded", func(t *testing.T) {
		t.Parallel()
		// Enqueue timeout is LONG, but Context deadline is SHORT
		n := NewBase(testID, true, 0, 1*time.Minute)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := n.Send(ctx, contract.NewNotification(testMessage))

		// Should return standard context error, NOT ErrQueueFull
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.WithinDuration(t, start, time.Now(), 100*time.Millisecond)
	})

	t.Run("Failure: Notifier Closed", func(t *testing.T) {
		t.Parallel()
		n := NewBase(testID, true, 1, testDefaultTimeout)
		n.Close()

		err := n.Send(context.Background(), contract.NewNotification(testMessage))
		require.ErrorIs(t, err, ErrClosed)
	})

	t.Run("Failure: Panic Recovery", func(t *testing.T) {
		t.Parallel()
		n := NewBase(testID, true, 1, testDefaultTimeout)

		// White-box testing: manually close the channel to trigger panic on send
		close(n.notificationC)

		assert.NotPanics(t, func() {
			err := n.Send(context.Background(), contract.NewNotification("trigger panic"))
			assert.ErrorIs(t, err, ErrPanicRecovered)
		})
	})
}

// =============================================================================
// 3. Unblocking Behavior (Concurrency)
// =============================================================================

func TestBase_Unblocking(t *testing.T) {
	t.Parallel()

	// Helper to run a test that blocks
	runBlockingTest := func(t *testing.T, name string, trigger func(n *Base, cancel context.CancelFunc), expectedErr error) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := NewBase(testID, true, 0, 1*time.Minute) // Will block

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errCh := make(chan error, 1)
			go func() {
				errCh <- n.Send(ctx, contract.NewNotification("blocking"))
			}()

			// Wait for Send to block
			time.Sleep(20 * time.Millisecond)

			// Trigger the unblocking event
			start := time.Now()
			trigger(&n, cancel)

			// Verify return
			select {
			case err := <-errCh:
				require.ErrorIs(t, err, expectedErr)
				assert.WithinDuration(t, start, time.Now(), 100*time.Millisecond, "Should return immediately after trigger")
			case <-time.After(1 * time.Second):
				t.Fatal("Send did not unblock")
			}
		})
	}

	runBlockingTest(t, "Unblock via Close()", func(n *Base, _ context.CancelFunc) {
		n.Close()
	}, ErrClosed)

	runBlockingTest(t, "Unblock via Context Cancel", func(n *Base, cancel context.CancelFunc) {
		cancel()
	}, context.Canceled)
}

// =============================================================================
// 4. Lifecycle & Idempotency
// =============================================================================

func TestBase_Close_Idempotency(t *testing.T) {
	t.Parallel()
	n := NewBase(testID, true, 1, testDefaultTimeout)

	// First Close
	n.Close()
	select {
	case <-n.Done():
	default:
		t.Fatal("Done channel should be closed")
	}

	// Second Close (Should not panic)
	assert.NotPanics(t, func() {
		n.Close()
	})
}
