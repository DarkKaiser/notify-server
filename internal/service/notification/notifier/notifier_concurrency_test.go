package notifier_test

import (
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		// Depending on race timing and implementation details, Notify might return True (if it wrote before close)
		// or False (panic recover or closed channel check).
		// However, with our specific change, if it was in the select, closing the channel might panic or proceed.
		// Our BaseNotifier has a recover block that returns false on panic.
		// Sending to a closed channel causes panic.
		// So we expect result to be false (due to panic recover) OR false (due to select case if we handled close there, but we just let it panic-recover).
		assert.False(t, result, "Notify should fail (via panic recover) when channel is closed during send")
	case <-time.After(1 * time.Second):
		t.Fatal("Notify did not return after Close")
	}
}
