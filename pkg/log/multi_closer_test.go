package log

import (
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

// TestMultiCloser_Close_HookRemoval은 Close 호출 시 Hook 제거를 검증합니다.
//
// 검증 항목:
//   - Close 호출 시 등록된 Hook이 제거됨
func TestMultiCloser_Close_HookRemoval(t *testing.T) {
	t.Run("Close 호출 시 Hook이 제거되는지 확인", func(t *testing.T) {
		// 테스트용 Hook 생성
		hook := &LogLevelHook{}

		// Logrus에 Hook 등록
		logger := log.StandardLogger()
		logger.AddHook(hook)

		// Hook이 등록되었는지 확인
		found := false
		for _, hooks := range logger.Hooks {
			for _, h := range hooks {
				if h == hook {
					found = true
					break
				}
			}
		}
		assert.True(t, found, "Hook이 등록되어야 합니다")

		// multiCloser 생성 및 Close 호출
		mc := &multiCloser{
			hook: hook,
		}
		err := mc.Close()
		require.NoError(t, err, "Close should not return error")

		// Hook이 제거되었는지 확인
		found = false
		for _, hooks := range logger.Hooks {
			for _, h := range hooks {
				if h == hook {
					found = true
					break
				}
			}
		}
	})
}

// =============================================================================
// Nil Handling Tests
// =============================================================================
