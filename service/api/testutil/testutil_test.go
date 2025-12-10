package testutil

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFreePort(t *testing.T) {
	port, err := GetFreePort()
	require.NoError(t, err, "GetFreePort should not return error")
	assert.Greater(t, port, 0, "Port should be greater than 0")

	// Verify port is actually free by trying to listen on it
	addr := fmt.Sprintf("localhost:%d", port)
	l, err := net.Listen("tcp", addr)
	require.NoError(t, err, "Should be able to listen on the returned port")
	l.Close()
}

func TestWaitForServer(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		port, err := GetFreePort()
		require.NoError(t, err)

		// Start a dummy server
		l, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
		require.NoError(t, err)
		defer l.Close()

		// WaitForServer should return nil immediately (or quickly)
		err = WaitForServer(port, 1*time.Second)
		assert.NoError(t, err)
	})

	t.Run("Timeout", func(t *testing.T) {
		port, err := GetFreePort()
		require.NoError(t, err)

		// Do NOT start a server

		// WaitForServer should fail after timeout
		start := time.Now()
		err = WaitForServer(port, 100*time.Millisecond)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.GreaterOrEqual(t, duration, 100*time.Millisecond)
	})
}

// TestMockNotificationService_NotifyWithTitle tests the NotifyWithTitle method.
func TestMockNotificationService_NotifyWithTitle(t *testing.T) {
	mock := &MockNotificationService{}

	t.Run("Success", func(t *testing.T) {
		mock.Reset()
		mock.ShouldFail = false
		success := mock.NotifyWithTitle("notifier1", "Title", "Message", false)

		assert.True(t, success)
		assert.True(t, mock.NotifyCalled)
		assert.Equal(t, "notifier1", mock.LastNotifierID)
		assert.Equal(t, "Title", mock.LastTitle)
		assert.Equal(t, "Message", mock.LastMessage)
		assert.False(t, mock.LastErrorOccurred)
	})

	t.Run("Failure", func(t *testing.T) {
		mock.Reset()
		mock.ShouldFail = true
		success := mock.NotifyWithTitle("notifier1", "Title", "Message", false)

		assert.False(t, success)
	})
}

func TestMockNotificationService(t *testing.T) {
	mock := &MockNotificationService{}

	t.Run("NotifyDefault", func(t *testing.T) {
		mock.Reset()
		mock.NotifyDefault("Default Message")
		assert.True(t, mock.NotifyDefaultCalled)
		assert.Equal(t, "Default Message", mock.LastMessage)
	})

	t.Run("NotifyDefaultWithError", func(t *testing.T) {
		mock.Reset()
		mock.NotifyDefaultWithError("Error Message")
		assert.True(t, mock.NotifyDefaultCalled)
		assert.Equal(t, "Error Message", mock.LastMessage)
		assert.True(t, mock.LastErrorOccurred)
	})

	t.Run("Concurrency Safety", func(t *testing.T) {
		mock.Reset()
		var wg sync.WaitGroup
		concurrency := 100

		wg.Add(concurrency)
		for i := 0; i < concurrency; i++ {
			go func() {
				defer wg.Done()
				mock.NotifyWithTitle("id", "title", "msg", false)
			}()
		}
		wg.Wait()

		assert.True(t, mock.NotifyCalled)
	})
}
