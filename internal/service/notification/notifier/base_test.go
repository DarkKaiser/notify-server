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

			// Assert: Public Accessors
			assert.Equal(t, tt.id, n.ID())
			assert.Equal(t, tt.supportsHTML, n.SupportsHTML())
			assert.Equal(t, tt.enqueueTimeout, n.enqueueTimeout)
			require.NotNil(t, n.NotificationC())
			assert.Equal(t, tt.bufferSize, cap(n.NotificationC()))

			// 포인터 필드들이 올바르게 초기화되었는지 확인하여 Nil Pointer Dereference 방지
			assert.NotNil(t, n.done, "Done channel must be initialized")

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
// 2. Send (Blocking) Logic
// =============================================================================

func TestBase_Send(t *testing.T) {
	t.Parallel()

	t.Run("Success_EmptyBuffer", func(t *testing.T) {
		n := NewBase(testID, true, 1, testDefaultTimeout)
		err := n.Send(context.Background(), contract.NewNotification(testMessage))
		assert.NoError(t, err)
	})

	t.Run("Context Propagation", func(t *testing.T) {
		// 전달한 Context가 Consumer에게 그대로 전달되는지 확인
		n := NewBase(testID, true, 1, testDefaultTimeout)

		key := "req-id"
		val := "1234"
		ctx := context.WithValue(context.Background(), key, val)

		err := n.Send(ctx, contract.NewNotification(testMessage))
		require.NoError(t, err)

		select {
		case req := <-n.NotificationC():
			assert.Equal(t, val, req.Ctx.Value(key), "Context value should be propagated to consumer")
		case <-time.After(testDefaultTimeout):
			t.Fatal("Message not received")
		}
	})

	t.Run("Failure_BufferFull_Timeout", func(t *testing.T) {
		n := NewBase(testID, true, 0, 10*time.Millisecond) // Short timeout

		// Unbuffered channel with no consumer -> Send will block then timeout
		start := time.Now()
		err := n.Send(context.Background(), contract.NewNotification(testMessage))
		duration := time.Since(start)

		assert.ErrorIs(t, err, ErrQueueFull)
		assert.GreaterOrEqual(t, duration, 10*time.Millisecond, "Should block for at least timeout duration")
	})

	t.Run("Failure_ContextAlreadyCancelled", func(t *testing.T) {
		n := NewBase(testID, true, 1, testDefaultTimeout)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := n.Send(ctx, contract.NewNotification(testMessage))
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("Failure_ContextCancelled_WhileBlocking", func(t *testing.T) {
		n := NewBase(testID, true, 0, 1*time.Second) // Long timeout
		ctx, cancel := context.WithCancel(context.Background())

		// Start cancel timer
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		err := n.Send(ctx, contract.NewNotification(testMessage))
		duration := time.Since(start)

		assert.ErrorIs(t, err, context.Canceled)
		assert.Less(t, duration, 100*time.Millisecond, "Should return immediately after cancellation")
	})

	t.Run("Success_PrepareSend_ContextTODO", func(t *testing.T) {
		// Context가 TODO일 때도 정상 동작해야 함
		n := NewBase(testID, true, 1, testDefaultTimeout)

		err := n.Send(context.TODO(), contract.NewNotification(testMessage))
		assert.NoError(t, err)
	})
}

// =============================================================================
// 3. TrySend (Non-Blocking) Logic
// =============================================================================

func TestBase_TrySend(t *testing.T) {
	t.Parallel()

	t.Run("Success_SpaceAvailable", func(t *testing.T) {
		n := NewBase(testID, true, 1, testDefaultTimeout)
		err := n.TrySend(context.Background(), contract.NewNotification(testMessage))
		assert.NoError(t, err)
	})

	t.Run("Failure_BufferFull_Immediate", func(t *testing.T) {
		n := NewBase(testID, true, 0, 1*time.Second) // Timeout shouldn't matter

		start := time.Now()
		err := n.TrySend(context.Background(), contract.NewNotification(testMessage))
		duration := time.Since(start)

		assert.ErrorIs(t, err, ErrQueueFull)
		// OS 스케줄링 등을 고려하여 넉넉하게 잡지만, 타임아웃(1초)보다는 훨씬 빨라야 함
		assert.Less(t, duration, 100*time.Millisecond, "TrySend should return immediately")
	})
}

// =============================================================================
// 4. Lifecycle & Safety Tests
// =============================================================================

func TestBase_Lifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Close_Idempotency", func(t *testing.T) {
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

	t.Run("Close_Unblocks_PendingSend", func(t *testing.T) {
		n := NewBase(testID, true, 0, 1*time.Minute)

		errCh := make(chan error, 1)

		// Start a blocking send
		go func() {
			errCh <- n.Send(context.Background(), contract.NewNotification("blocking"))
		}()

		// Give it time to block
		time.Sleep(20 * time.Millisecond)

		start := time.Now()
		n.Close()

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
	// Send/TrySend 메서드 내부의 recover() 로직이 제대로 동작하는지 검증합니다.
	// 강제로 Panic을 유발하기 위해 내부 상태를 비정상적으로 조작합니다.

	t.Run("Send_Recover_From_Panic", func(t *testing.T) {
		n := NewBase(testID, true, 1, testDefaultTimeout)

		// 강제 Panic 유발: notificationC 닫기
		close(n.notificationC)

		var err error
		assert.NotPanics(t, func() {
			err = n.Send(context.Background(), contract.NewNotification(testMessage))
		})
		assert.ErrorIs(t, err, ErrPanicRecovered)
	})

	t.Run("TrySend_Recover_From_Panic", func(t *testing.T) {
		n := NewBase(testID, true, 1, testDefaultTimeout)

		// 강제 Panic 유발: notificationC 닫기
		close(n.notificationC)

		var err error
		assert.NotPanics(t, func() {
			err = n.TrySend(context.Background(), contract.NewNotification(testMessage))
		})
		assert.ErrorIs(t, err, ErrPanicRecovered)
	})
}

// =============================================================================
// 5. Concurrency & Race Condition Tests
// =============================================================================

func TestBase_WaitForPendingSends_Integration(t *testing.T) {
	// Send가 Close에 의해 중단되는 시나리오에서,
	// WaitForPendingSends가 Race 없이 정상적으로 대기를 수행하는지 검증

	n := NewBase("test-wait-group", true, 0, 1*time.Second)

	sendErrCh := make(chan error)
	blockEntered := make(chan struct{})

	// 1. Start a BLOCKING Sender
	go func() {
		close(blockEntered) // Signal that we are about to call Send
		// This will block because buffer=0 and no receiver
		sendErrCh <- n.Send(context.Background(), contract.NewNotification("msg"))
	}()

	<-blockEntered
	// Ensure Send has acquired lock/entered select
	time.Sleep(20 * time.Millisecond)

	// 2. Trigger Shutdown
	// Calling Close acquires the Lock, ensuring synchronization
	n.Close()

	// 3. Wait for pending sends
	done := make(chan struct{})
	go func() {
		n.WaitForPendingSends()
		close(done)
	}()

	// 4. Verification
	// The Send() should have returned ErrClosed
	select {
	case err := <-sendErrCh:
		// Send는 Close에 의해 깨어남 -> ErrClosed 반환
		assert.ErrorIs(t, err, ErrClosed)
	case <-time.After(1 * time.Second):
		t.Fatal("Sender stuck")
	}

	// The WaitForPendingSends should complete shortly after Send returns
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("WaitForPendingSends stuck")
	}
}

func TestBase_Concurrent_Send_Close(t *testing.T) {
	// Hammer Test: Massive concurrent Send/TrySend vs Close
	n := NewBase("hammer-test", true, 1000, 1*time.Second)

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Consumers: To prevent deadlock if buffer fills up before Close
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case <-n.NotificationC():
			case <-n.Done():
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	senderCount := 100
	for i := 0; i < senderCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 50; j++ {
				if j%2 == 0 {
					_ = n.Send(context.Background(), contract.NewNotification("msg"))
				} else {
					_ = n.TrySend(context.Background(), contract.NewNotification("msg"))
				}
			}
		}()
	}

	close(start) // GO!
	time.Sleep(5 * time.Millisecond)
	n.Close()

	// Wait for all senders
	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Deadlock detected")
	}
}
