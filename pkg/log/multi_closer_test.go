package log

import (
	"bytes"
	"errors"
	"io"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers
// =============================================================================

// mockCloser는 테스트용 Closer 구현입니다.
type mockCloser struct {
	closed bool
	err    error
}

func (m *mockCloser) Close() error {
	m.closed = true
	return m.err
}

// =============================================================================
// Multi Closer Basic Tests
// =============================================================================

// TestMultiCloser_Close는 multiCloser의 동작을 검증합니다.
func TestMultiCloser_Close(t *testing.T) {
	errMock := errors.New("close error")

	tests := []struct {
		name          string
		closers       []io.Closer
		expectError   error
		expectedState []bool // 각 closer의 closed 상태 (순서대로)
	}{
		{
			name: "All closers close successfully",
			closers: []io.Closer{
				&mockCloser{},
				&mockCloser{},
				&mockCloser{},
			},
			expectError:   nil,
			expectedState: []bool{true, true, true},
		},
		{
			name: "Error in middle closer - Continues to close others, returns first error",
			closers: []io.Closer{
				&mockCloser{},
				&mockCloser{err: errMock},
				&mockCloser{},
			},
			expectError:   errMock,
			expectedState: []bool{true, true, true},
		},
		{
			name: "Nil closer in list - Should skip safely",
			closers: []io.Closer{
				&mockCloser{},
				nil,
				&mockCloser{},
			},
			expectError:   nil,
			expectedState: []bool{true, false, true}, // nil doesn't have state
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &multiCloser{
				closers: tt.closers,
			}

			err := mc.Close()

			if tt.expectError != nil {
				assert.ErrorIs(t, err, tt.expectError)
			} else {
				assert.NoError(t, err)
			}

			// Verify states
			for i, closer := range tt.closers {
				if mc, ok := closer.(*mockCloser); ok {
					assert.Equal(t, tt.expectedState[i], mc.closed, "Closer %d closed state mismatch", i)
				}
			}
		})
	}
}

// =============================================================================
// Hook Removal Tests
// =============================================================================

// TestMultiCloser_Close_HookRemoval은 Close 호출 시 Hook 비활성화를 검증합니다.
//
// 검증 항목:
//   - Close 호출 시 Hook의 closed 플래그가 설정됨 (Logical Disable)
func TestMultiCloser_Close_HookDisable(t *testing.T) {
	t.Run("Close 호출 시 Hook이 비활성화되는지 확인", func(t *testing.T) {
		// 테스트용 Hook 생성
		hook := &LogLevelHook{}

		// Logrus에 Hook 등록 (여기서는 등록 여부보다는 Disable 여부가 중요)
		log.AddHook(hook)

		// multiCloser 생성 및 Close 호출
		mc := &multiCloser{
			hook: hook,
		}
		err := mc.Close()
		require.NoError(t, err, "Close should not return error")

		// Hook이 비활성화(closed) 되었는지 내부 상태 확인 불가 (private field)
		// 대신 Fire를 호출했을 때 더 이상 동작하지 않거나 에러가 없는지 간접 확인이 필요하지만,
		// 여기서는 리팩토링된 동작(Atomic Flag 설정)을 신뢰하고
		// Fire 호출 시 아무런 사이드 이펙트가 없는지 확인합니다.

		// 테스트용 Writer 설정
		buf := &bytes.Buffer{}
		hook.verboseWriter = buf
		hook.formatter = &log.TextFormatter{DisableTimestamp: true}

		// Fire 호출 (이미 Close 되었으므로 기록되면 안 됨)
		err = hook.Fire(&log.Entry{
			Level:   log.DebugLevel,
			Message: "Should not be logged",
		})
		assert.NoError(t, err)
		assert.Equal(t, 0, buf.Len(), "Close된 Hook은 로그를 기록하지 않아야 합니다")
	})
}

// =============================================================================
// Sync Tests
// =============================================================================

// mockSyncCloser는 Sync를 지원하는 Mock Closer입니다.
type mockSyncCloser struct {
	mockCloser
	synced bool
}

func (m *mockSyncCloser) Sync() error {
	m.synced = true
	return nil
}

// TestMultiCloser_Sync는 Close 시 Sync 호출 여부를 검증합니다.
//
// 검증 항목:
//   - Sync 메서드를 가진 Closer에 대해 Sync()가 호출되는지
//   - Sync 메서드가 없는 Closer는 문제없이 Close 되는지
func TestMultiCloser_Sync(t *testing.T) {
	t.Run("Sync 지원 Closer는 Sync가 호출되어야 함", func(t *testing.T) {
		syncer := &mockSyncCloser{}
		normal := &mockCloser{}

		mc := &multiCloser{
			closers: []io.Closer{syncer, normal},
		}

		err := mc.Close()
		require.NoError(t, err)

		assert.True(t, syncer.synced, "Sync() should be called")
		assert.True(t, syncer.closed, "Close() should be called after Sync()")
		assert.True(t, normal.closed, "Normal closer should be closed")
	})
}
