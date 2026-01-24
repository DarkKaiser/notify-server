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
	testDefaultTimeout = 50 * time.Millisecond // Unit testing requires fast timeouts
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
			enqueueTimeout: 50 * time.Millisecond,
		},
		{
			name:           "Large Buffer",
			id:             "notifier-large",
			supportsHTML:   true,
			bufferSize:     1000,
			enqueueTimeout: 1 * time.Minute,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			n := NewBase(tt.id, tt.supportsHTML, tt.bufferSize, tt.enqueueTimeout)

			// Assert
			assert.Equal(t, tt.id, n.ID())
			assert.Equal(t, tt.supportsHTML, n.SupportsHTML())
			assert.Equal(t, tt.enqueueTimeout, n.enqueueTimeout)
			require.NotNil(t, n.NotificationC())
			assert.Equal(t, tt.bufferSize, cap(n.NotificationC()))

			// Verify channel states
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
// 2. Send & TrySend Logic (Table Driven)
// =============================================================================

func TestBase_SendMethods(t *testing.T) {
	t.Parallel()

	type actionFunc func(n *Base, ctx context.Context) error

	tests := []struct {
		name           string
		bufferSize     int
		enqueueTimeout time.Duration
		prefillCount   int // Number of messages to fill the buffer with before text
		action         actionFunc
		wantErr        bool
		wantErrIs      error
		wantDuration   time.Duration // Minimum duration expected (for Blocking checks)
	}{
		// ---------------------------------------------------------------------
		// Send (Blocking) Scenarios
		// ---------------------------------------------------------------------
		{
			name:           "Send: Success (Empty Buffer)",
			bufferSize:     1,
			enqueueTimeout: testDefaultTimeout,
			action: func(n *Base, ctx context.Context) error {
				return n.Send(ctx, contract.NewNotification(testMessage))
			},
			wantErr: false,
		},
		{
			name:           "Send: Success (Unbuffered with Consumer)",
			bufferSize:     0,
			enqueueTimeout: testDefaultTimeout,
			action: func(n *Base, ctx context.Context) error {
				// Start consumer first
				go func() {
					select {
					case <-n.NotificationC():
					case <-time.After(testDefaultTimeout):
					}
				}()
				return n.Send(ctx, contract.NewNotification(testMessage))
			},
			wantErr: false,
		},
		{
			name:           "Send: Failure with Timeout (Queue Full)",
			bufferSize:     0,
			enqueueTimeout: 20 * time.Millisecond,
			action: func(n *Base, ctx context.Context) error {
				return n.Send(ctx, contract.NewNotification(testMessage))
			},
			wantErr:      true,
			wantErrIs:    ErrQueueFull,
			wantDuration: 20 * time.Millisecond,
		},
		{
			name:           "Send: Failure w/ Canceled Context (Immediate)",
			bufferSize:     1,
			enqueueTimeout: testDefaultTimeout,
			action: func(n *Base, ctx context.Context) error {
				canceledCtx, cancel := context.WithCancel(ctx)
				cancel() // Cancel immediately
				return n.Send(canceledCtx, contract.NewNotification(testMessage))
			},
			wantErr:   true,
			wantErrIs: context.Canceled,
		},

		// ---------------------------------------------------------------------
		// TrySend (Non-blocking) Scenarios
		// ---------------------------------------------------------------------
		{
			name:           "TrySend: Success (Space Available)",
			bufferSize:     1,
			enqueueTimeout: testDefaultTimeout,
			action: func(n *Base, ctx context.Context) error {
				return n.TrySend(ctx, contract.NewNotification(testMessage))
			},
			wantErr: false,
		},
		{
			name:           "TrySend: Failure Immediate (Queue Full)",
			bufferSize:     0,                  // Unbuffered channel is full by default if no consumer
			enqueueTimeout: testDefaultTimeout, // Timeout shouldn't matter
			action: func(n *Base, ctx context.Context) error {
				return n.TrySend(ctx, contract.NewNotification(testMessage))
			},
			wantErr:   true,
			wantErrIs: ErrQueueFull,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			n := NewBase(testID, true, tt.bufferSize, tt.enqueueTimeout)

			// Pre-fill buffer if requested
			for i := 0; i < tt.prefillCount; i++ {
				// We use a non-blocking send directly to the channel for setup
				select {
				case n.notificationC <- &notificationRequest{}:
				default:
					t.Fatalf("Failed to pre-fill buffer, it might be full already")
				}
			}

			// Act
			start := time.Now()
			err := tt.action(&n, context.Background())
			duration := time.Since(start)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
			} else {
				require.NoError(t, err)
			}

			if tt.wantDuration > 0 {
				assert.GreaterOrEqual(t, duration, tt.wantDuration, "Operation completed too fast, expected blocking behavior")
			}
		})
	}
}

// =============================================================================
// 3. Lifecycle & Safety Tests
// =============================================================================

func TestBase_Lifecycle(t *testing.T) {
	t.Parallel()

	// 3-1. Close Idempotency
	t.Run("Close Idempotency", func(t *testing.T) {
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

		// Send after Close should fail
		err := n.Send(context.Background(), contract.NewNotification(testMessage))
		assert.ErrorIs(t, err, ErrClosed)
	})

	// 3-2. Unblocking on Close
	t.Run("Send Unblocks on Close", func(t *testing.T) {
		n := NewBase(testID, true, 0, 1*time.Minute) // Long timeout

		// Start a blocking send
		errCh := make(chan error, 1)
		readyCh := make(chan struct{}) // Synchronization channel

		go func() {
			close(readyCh) // Signal that goroutine started
			errCh <- n.Send(context.Background(), contract.NewNotification("blocking"))
		}()

		<-readyCh                         // Wait for goroutine start
		time.Sleep(10 * time.Millisecond) // Allow it to reach the select statement

		// Action: Close the notifier
		start := time.Now()
		n.Close()

		// Verify: Should unblock immediately with ErrClosed
		select {
		case err := <-errCh:
			assert.ErrorIs(t, err, ErrClosed)
			assert.WithinDuration(t, start, time.Now(), 100*time.Millisecond)
		case <-time.After(1 * time.Second):
			t.Fatal("Send did not unblock after Close()")
		}
	})
}

func TestBase_PanicRecovery(t *testing.T) {
	t.Parallel()

	n := NewBase(testID, true, 1, testDefaultTimeout)

	// White-box testing: manually close internal channel to trigger panic
	close(n.notificationC)

	assert.NotPanics(t, func() {
		err := n.Send(context.Background(), contract.NewNotification("trigger panic"))
		assert.ErrorIs(t, err, ErrPanicRecovered)
	})
}
