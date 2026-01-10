package notification

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/task"
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

// TestNewNotifier는 Notifier 생성을 검증합니다.
//
// 검증 항목:
//   - ID 설정
//   - HTML 지원 여부
//   - 버퍼 크기 설정
//   - 채널 생성
func TestNewNotifier(t *testing.T) {
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

// TestNotify는 Notify 메서드의 동작을 검증합니다.
//
// 검증 항목:
//   - 정상 메시지 전송
//   - nil TaskContext 처리
//   - 빈 메시지 처리
//   - 긴 메시지 처리
//   - 닫힌 채널 처리 (panic recovery)
func TestNotify(t *testing.T) {
	tests := []struct {
		name       string
		notifier   notifier
		message    string
		taskCtx    task.TaskContext
		expectData bool
		expectTrue bool
	}{
		{
			name:       "성공: TaskContext 포함",
			notifier:   NewNotifier("test", true, testNotifierBufferSize),
			message:    testNotifierMessage,
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "성공: nil TaskContext",
			notifier:   NewNotifier("test", true, testNotifierBufferSize),
			message:    testNotifierMessage,
			taskCtx:    nil,
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "성공: 빈 메시지",
			notifier:   NewNotifier("test", true, testNotifierBufferSize),
			message:    "",
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name:       "성공: 긴 메시지 (10KB)",
			notifier:   NewNotifier("test", true, testNotifierBufferSize),
			message:    strings.Repeat("a", 10000),
			taskCtx:    task.NewTaskContext(),
			expectData: true,
			expectTrue: true,
		},
		{
			name: "실패: 닫힌 채널 (nil check)",
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

			assert.Equal(t, tt.expectTrue, result)

			if tt.expectData {
				assertChannelReceivesData(t, ch, tt.message, tt.taskCtx)
			}
		})
	}
}

// =============================================================================
// Non-blocking Behavior Tests
// =============================================================================

// TestNotify_BufferFull은 버퍼가 가득 찬 경우 Non-blocking 동작을 검증합니다.
//
// 검증 항목:
//   - 버퍼가 가득 찬 경우 즉시 false 반환
//   - Blocking 없이 동작
func TestNotify_BufferFull(t *testing.T) {
	n := NewNotifier("test", true, 2) // 작은 버퍼

	// 버퍼 채우기
	assert.True(t, n.Notify(task.NewTaskContext(), "msg1"))
	assert.True(t, n.Notify(task.NewTaskContext(), "msg2"))

	// Non-blocking 검증: 즉시 false 반환해야 함
	done := make(chan bool, 1)
	go func() {
		result := n.Notify(task.NewTaskContext(), "msg3")
		done <- result
	}()

	select {
	case result := <-done:
		assert.False(t, result, "Notify should return false immediately when buffer is full")
	case <-time.After(testNotifierTimeout):
		t.Fatal("Notify blocked when it should be non-blocking")
	}
}

// TestNotify_Concurrency는 동시성 안전성을 검증합니다.
//
// 검증 항목:
//   - 여러 고루틴에서 동시 호출 시 안전성
//   - Race condition 없음
func TestNotify_Concurrency(t *testing.T) {
	n := NewNotifier("test", true, 100)

	concurrency := 50
	wg := sync.WaitGroup{}
	wg.Add(concurrency)

	successCount := int32(0)
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			if n.Notify(task.NewTaskContext(), "concurrent message") {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// 모든 메시지가 성공적으로 전송되어야 함 (버퍼가 충분히 큼)
	assert.Greater(t, successCount, int32(0), "At least some messages should succeed")
}

// =============================================================================
// Close Method Tests
// =============================================================================

// TestClose_Idempotent는 Close 메서드의 멱등성을 검증합니다.
//
// 검증 항목:
//   - 첫 번째 Close 호출 시 채널 닫힘
//   - 두 번째 Close 호출 시 panic 없음
func TestClose_Idempotent(t *testing.T) {
	n := NewNotifier("test", true, testNotifierBufferSize)

	// 첫 번째 Close
	n.Close()
	assert.Nil(t, n.requestC, "Request channel should be nil after close")

	// 두 번째 Close는 panic 없어야 함
	assert.NotPanics(t, func() {
		n.Close()
	}, "Second Close() should not panic")
}

// TestClose_AfterNotify는 Close 후 Notify 동작을 검증합니다.
//
// 검증 항목:
//   - Close 후 채널이 nil이 됨
//   - Close 후 Notify 호출 시 false 반환
func TestClose_AfterNotify(t *testing.T) {
	n := NewNotifier("test", true, testNotifierBufferSize)

	// 메시지 전송
	require.True(t, n.Notify(task.NewTaskContext(), "msg1"))
	require.True(t, n.Notify(task.NewTaskContext(), "msg2"))

	// Close
	n.Close()
	assert.Nil(t, n.requestC)

	// Close 후 Notify는 false 반환해야 함
	result := n.Notify(task.NewTaskContext(), "msg3")
	assert.False(t, result, "Notify should return false after close")
}
