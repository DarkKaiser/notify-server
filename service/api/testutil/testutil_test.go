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

func TestMockNotificationService(t *testing.T) {
	mock := &MockNotificationService{}

	t.Run("Notify", func(t *testing.T) {
		mock.Reset()
		success := mock.Notify("notifier1", "Title", "Message", false)
		assert.True(t, success)
		assert.True(t, mock.NotifyCalled)
		assert.Equal(t, "notifier1", mock.LastNotifierID)
		assert.Equal(t, "Title", mock.LastTitle)
		assert.Equal(t, "Message", mock.LastMessage)
		assert.False(t, mock.LastErrorOccurred)
	})

	t.Run("Notify Fail", func(t *testing.T) {
		mock.Reset()
		mock.ShouldFail = true
		success := mock.Notify("notifier1", "Title", "Message", false)
		assert.False(t, success)
	})

	t.Run("NotifyToDefault", func(t *testing.T) {
		mock.Reset()
		mock.NotifyToDefault("Default Message")
		assert.True(t, mock.NotifyToDefaultCalled)
		assert.Equal(t, "Default Message", mock.LastMessage)
	})

	t.Run("NotifyWithErrorToDefault", func(t *testing.T) {
		mock.Reset()
		mock.NotifyWithErrorToDefault("Error Message")
		assert.True(t, mock.NotifyToDefaultCalled)
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
				mock.Notify("id", "title", "msg", false)
			}()
		}
		wg.Wait()

		assert.True(t, mock.NotifyCalled)
	})
}
