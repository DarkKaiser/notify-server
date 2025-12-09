package notification

import (
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
)

func TestNotifier_NewNotifier(t *testing.T) {
	tests := []struct {
		name              string
		id                string
		supportsHTML      bool
		bufferSize        int
		expectedBufferCap int
	}{
		{"Normal", "test-id", true, 10, 10},
		{"No Buffer", "test-id", false, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNotifier(NotifierID(tt.id), tt.supportsHTML, tt.bufferSize)
			assert.Equal(t, NotifierID(tt.id), n.ID())
			assert.Equal(t, tt.supportsHTML, n.SupportsHTMLMessage())
			if tt.bufferSize > 0 {
				assert.NotNil(t, n.requestC)
				assert.Equal(t, tt.expectedBufferCap, cap(n.requestC))
			} else {
				assert.NotNil(t, n.requestC)
				assert.Equal(t, 0, cap(n.requestC))
			}
		})
	}
}

func TestNotifier_Notify_Table(t *testing.T) {
	tests := []struct {
		name        string
		notifier    notifier
		message     string
		taskCtx     task.TaskContext
		expectPanic bool // Notify handles panic gracefully, returning false
		expectData  bool
	}{
		{
			name:        "Success",
			notifier:    NewNotifier("test", true, 10),
			message:     "msg",
			taskCtx:     task.NewTaskContext(),
			expectPanic: false,
			expectData:  true,
		},
		{
			name:        "Nil TaskContext",
			notifier:    NewNotifier("test", true, 10),
			message:     "msg",
			taskCtx:     nil,
			expectPanic: false,
			expectData:  true,
		},
		{
			name: "Closed Channel (Panic Recovery)",
			notifier: func() notifier {
				n := NewNotifier("test", true, 10)
				n.Close()
				return n
			}(),
			message:     "msg",
			taskCtx:     nil,
			expectPanic: false, // Internal recover
			expectData:  false, // Should return false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result bool
			if tt.expectPanic {
				// If Notify didn't have recover, we would assert panic here.
				// But Notify has recover, so we check return value.
			}

			// We need to keep a reference to channel before calling Notify if we expect data
			// and if Notify is blocking? No, Notify is buffered here (except 0 buffer case).
			var ch chan *notifyRequest
			if tt.notifier.requestC != nil {
				ch = tt.notifier.requestC
			}

			result = tt.notifier.Notify(tt.message, tt.taskCtx)

			if tt.expectData {
				assert.True(t, result)
				select {
				case data := <-ch:
					assert.Equal(t, tt.message, data.message)
					assert.Equal(t, tt.taskCtx, data.taskCtx)
				case <-time.After(100 * time.Millisecond):
					t.Fatal("Timeout receiving data")
				}
			} else {
				assert.False(t, result)
			}
		})
	}
}

func TestNotifier_Close_Idempotent(t *testing.T) {
	n := NewNotifier("test", true, 10)
	n.Close()
	assert.Nil(t, n.requestC)
	assert.NotPanics(t, func() {
		n.Close() // Second call
	})
}
