package notifier

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBaseNotifier_Notify_PanicRecovery(t *testing.T) {
	// Setup
	n := NewBaseNotifier("test_notifier", true, 10, time.Second)

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
