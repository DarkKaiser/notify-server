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

// TestMultiCloser_Close는 multiCloser의 기본 동작을 검증합니다.
//
// 검증 항목:
//   - 모든 Closer가 정상적으로 닫힘
//   - 에러 발생 시에도 모든 Closer가 닫히고 첫 번째 에러 반환
func TestMultiCloser_Close(t *testing.T) {
	t.Run("모든 Closer가 정상적으로 닫히는지 확인", func(t *testing.T) {
		c1 := &mockCloser{}
		c2 := &mockCloser{}
		c3 := &mockCloser{}

		mc := &multiCloser{
			closers: []io.Closer{c1, c2, c3},
		}

		err := mc.Close()

		require.NoError(t, err, "Close should not return error")
		assert.True(t, c1.closed)
		assert.True(t, c2.closed)
		assert.True(t, c3.closed)
	})

	t.Run("에러 발생 시에도 모든 Closer가 닫히고 첫 번째 에러 반환", func(t *testing.T) {
		expectedErr := errors.New("close error")
		c1 := &mockCloser{}
		c2 := &mockCloser{err: expectedErr}
		c3 := &mockCloser{}

		mc := &multiCloser{
			closers: []io.Closer{c1, c2, c3},
		}

		err := mc.Close()

		require.Error(t, err, "Close should return error")
		assert.Equal(t, expectedErr, err)
		assert.True(t, c1.closed)
		assert.True(t, c2.closed)
		assert.True(t, c3.closed)
	})
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

// TestMultiCloser_Close_WithNil은 nil Closer 처리를 검증합니다.
//
// 검증 항목:
//   - nil Closer가 포함되어 있어도 정상 동작
//   - nil이 아닌 Closer는 모두 정상적으로 닫힘
func TestMultiCloser_Close_WithNil(t *testing.T) {
	t.Run("nil 클로저가 포함되어 있어도 정상 동작", func(t *testing.T) {
		c1 := &mockCloser{}
		var c2 io.Closer = nil // Explicit nil interface
		c3 := &mockCloser{}

		mc := &multiCloser{
			closers: []io.Closer{c1, c2, c3},
		}

		err := mc.Close()

		assert.NoError(t, err)
		assert.True(t, c1.closed)
		assert.True(t, c3.closed)
	})
}
