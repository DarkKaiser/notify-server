package notification

import (
	"strings"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Constants
// =============================================================================

const (
	testNotifierBufferSize = 10
	testNotifierTimeout    = 100 * time.Millisecond
	testNotifierMessage    = "test message"
)

// =============================================================================
// Test Helpers
// =============================================================================

// assertChannelReceivesData는 채널에서 데이터를 수신하고 검증합니다.
func assertChannelReceivesData(t *testing.T, ch chan *notifyRequest, expectedMsg string, expectedCtx task.TaskContext) {
	t.Helper()
	select {
	case data := <-ch:
		require.NotNil(t, data, "Received data should not be nil")
		assert.Equal(t, expectedMsg, data.message)
		assert.Equal(t, expectedCtx, data.taskCtx)
	case <-time.After(testNotifierTimeout):
		t.Fatal("Timeout receiving data from channel")
	}
}

// =============================================================================
// Notifier Creation Tests
// =============================================================================

// TestNotifier_NewNotifier는 Notifier 생성을 검증합니다.
//
// 검증 항목:
//   - ID 설정
//   - HTML 지원 여부
//   - 버퍼 크기 설정
//   - 채널 생성
func TestNotifier_NewNotifier(t *testing.T) {
	tests := []struct {
		name              string
		id                string
		supportsHTML      bool
		bufferSize        int
		expectedBufferCap int
	}{
		{"Normal buffer", "test-id", true, testNotifierBufferSize, testNotifierBufferSize},
		{"No buffer", "test-id", false, 0, 0},
		{"Large buffer", "large-id", true, 100, 100},
		{"HTML not supported", "no-html", false, 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNotifier(NotifierID(tt.id), tt.supportsHTML, tt.bufferSize)

			assert.Equal(t, NotifierID(tt.id), n.ID())
			assert.Equal(t, tt.supportsHTML, n.SupportsHTML())
			require.NotNil(t, n.requestC, "Request channel should be created")
			assert.Equal(t, tt.expectedBufferCap, cap(n.requestC))
		})
	}
}

// =============================================================================
// Notify Method Tests
// =============================================================================

// TestNotifier_Notify_Table은 Notify 메서드의 동작을 검증합니다.
//
// 검증 항목:
//   - 정상 메시지 전송
//   - nil TaskContext 처리
//   - 닫힌 채널 처리 (panic recovery)
//   - 빈 메시지 처리
//   - 긴 메시지 처리
func TestNotifier_Notify_Table(t *testing.T) {
	tests := []struct {
		name       string
		notifier   notifier
		message    string
		taskCtx    task.TaskContext
		expectData bool
		expectTrue bool
	}{
		{
			name:       "Success with context",
			notifier:   NewNotifier("test", true, testNotifierBufferSize),
			message:    testNotifierMessage,
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "Nil TaskContext",
			notifier:   NewNotifier("test", true, testNotifierBufferSize),
			message:    testNotifierMessage,
			taskCtx:    nil,
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "Empty message",
			notifier:   NewNotifier("test", true, testNotifierBufferSize),
			message:    "",
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "Long message",
			notifier:   NewNotifier("test", true, testNotifierBufferSize),
			message:    strings.Repeat("a", 1000),
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name: "Closed channel (panic recovery)",
			notifier: func() notifier {
				n := NewNotifier("test", true, testNotifierBufferSize)
				n.Close()
				return n
			}(),
			message:    testNotifierMessage,
			taskCtx:    nil,
			expectData: false,
			expectTrue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ch chan *notifyRequest
			if tt.notifier.requestC != nil {
				ch = tt.notifier.requestC
			}

			result := tt.notifier.Notify(tt.taskCtx, tt.message)

			if tt.expectTrue {
				assert.True(t, result, "Notify should return true")
			} else {
				assert.False(t, result, "Notify should return false")
			}

			if tt.expectData {
				assertChannelReceivesData(t, ch, tt.message, tt.taskCtx)
			}
		})
	}
}

// =============================================================================
// Buffer Full Tests
// =============================================================================

// TestNotifier_Notify_BufferFull은 버퍼가 가득 찬 경우를 검증합니다.
//
// 검증 항목:
//   - 버퍼가 가득 찬 경우 Notify 동작
//   - 타임아웃 처리
func TestNotifier_Notify_BufferFull(t *testing.T) {
	n := NewNotifier("test", true, 2) // Small buffer

	// Fill the buffer
	assert.True(t, n.Notify(task.NewTaskContext(), "msg1"))
	assert.True(t, n.Notify(task.NewTaskContext(), "msg2"))

	// This should timeout and return false (or block depending on implementation)
	// Since Notify uses select with default, it should return false immediately
	done := make(chan bool)
	go func() {
		result := n.Notify(task.NewTaskContext(), "msg3")
		done <- result
	}()

	select {
	case result := <-done:
		// If Notify has a timeout or returns false when buffer is full
		assert.False(t, result, "Notify should return false when buffer is full")
	case <-time.After(testNotifierTimeout):
		// If Notify blocks, this is also acceptable behavior
		t.Log("Notify blocked as expected when buffer is full")
	}
}

// =============================================================================
// Close Method Tests
// =============================================================================

// TestNotifier_Close_Idempotent는 Close 메서드의 멱등성을 검증합니다.
//
// 검증 항목:
//   - 첫 번째 Close 호출 시 채널 닫힘
//   - 두 번째 Close 호출 시 panic 없음
func TestNotifier_Close_Idempotent(t *testing.T) {
	n := NewNotifier("test", true, testNotifierBufferSize)

	// First close
	n.Close()
	assert.Nil(t, n.requestC, "Request channel should be nil after close")

	// Second close should not panic
	assert.NotPanics(t, func() {
		n.Close()
	}, "Second Close() should not panic")
}

// TestNotifier_Close_DrainChannel은 Close 시 채널 정리를 검증합니다.
//
// 검증 항목:
//   - Close 후 채널이 nil이 됨
//   - Close 후 Notify 호출 시 false 반환
func TestNotifier_Close_DrainChannel(t *testing.T) {
	n := NewNotifier("test", true, testNotifierBufferSize)

	// Send some messages
	require.True(t, n.Notify(task.NewTaskContext(), "msg1"))
	require.True(t, n.Notify(task.NewTaskContext(), "msg2"))

	// Close
	n.Close()
	assert.Nil(t, n.requestC)

	// Notify after close should return false
	result := n.Notify(task.NewTaskContext(), "msg3")
	assert.False(t, result, "Notify should return false after close")
}
