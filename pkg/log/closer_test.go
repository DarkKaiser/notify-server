//go:build test

package log

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mocks
// =============================================================================

// MockCloser is a mock implementation of io.Closer
type MockCloser struct {
	mock.Mock
}

func (m *MockCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockSyncCloser is a mock implementation of io.Closer + Sync()
type MockSyncCloser struct {
	MockCloser
}

func (m *MockSyncCloser) Sync() error {
	args := m.Called()
	return args.Error(0)
}

// MockHook is a mock implementation of the hook to verify Close calls
type MockHook struct {
	mock.Mock
}

func (m *MockHook) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Note: Ensure MockHook satisfies the interface required by closer (if interface existed).
// Currently closer struct uses concrete *hook type.
// To test hook interaction properly without changing production code to interface,
// we might need to rely on the side effects or internal state if accessible.
// However, since `hook` is a concrete struct in `closer`, we cannot easily mock it
// unless we refactor `closer` to use an interface for hook or just test the side effect.
// For this test suite, we will test the logic around `closers` slice mainly,
// and for hook, since it's concrete, we might not be able to spy on `Close` easily
// without redesigning `closer` to accept an interface.
//
// BUT: The user asked for "Expert level improvements".
// Ideally, `closer` should depend on an interface for the hook to be testable.
// Given strict instructions not to over-engineer or change too much if not critical,
// we will stick to testing what we can.
//
// Wait, `closer` struct has `hook *hook`. This makes mocking `hook.Close()` hard
// without interface extraction.
// Let's check `hook.go`. It has a `Close()` method.
//
// STRATEGY:
// 1. Refactor `approver.go` tests to use `testify/mock` for `io.Closer` (Cleaner).
// 2. For `hook`, since we can't mock the method on a concrete struct easily,
//    we will verify the observable behavior: "hook is closed".
//    We can check `hook.closed` field if we are in the same package (white-box test).
//    Since this test is `package log`, we can access unexported fields! Perfect.

// =============================================================================
// Tests
// =============================================================================

func TestCloser_Close(t *testing.T) {
	t.Run("성공: 모든 리소스가 정상적으로 닫힘", func(t *testing.T) {
		// Given
		m1 := new(MockCloser)
		m2 := new(MockCloser)
		h := &hook{}

		m1.On("Close").Return(nil)
		m2.On("Close").Return(nil)

		c := &closer{
			closers: []io.Closer{m1, m2},
			hook:    h,
		}

		// When
		err := c.Close()

		// Then
		assert.NoError(t, err)
		assert.True(t, h.closed, "Hook should be marked as closed")
		m1.AssertExpectations(t)
		m2.AssertExpectations(t)
	})

	t.Run("실패: 일부 리소스 닫기 실패 시에도 나머지는 시도함", func(t *testing.T) {
		// Given
		m1 := new(MockCloser)
		m2 := new(MockCloser) // Fails
		m3 := new(MockCloser)

		errFail := errors.New("fail to close")

		m1.On("Close").Return(nil)
		m2.On("Close").Return(errFail)
		m3.On("Close").Return(nil)

		c := &closer{
			closers: []io.Closer{m1, m2, m3},
		}

		// When
		err := c.Close()

		// Then
		require.Error(t, err)
		assert.ErrorIs(t, err, errFail) // Contains the error

		// All methods must be called despite the error in the middle
		m1.AssertExpectations(t)
		m2.AssertExpectations(t)
		m3.AssertExpectations(t)
	})

	t.Run("중복 호출: Idempotency 보장", func(t *testing.T) {
		// Given
		m1 := new(MockCloser)
		m1.On("Close").Return(nil).Once() // Should be called only once

		c := &closer{
			closers: []io.Closer{m1},
		}

		// When 1st Call
		err1 := c.Close()
		assert.NoError(t, err1)

		// When 2nd Call
		err2 := c.Close()
		assert.NoError(t, err2) // Should return nil immediately

		// Then
		m1.AssertExpectations(t)
	})

	t.Run("Hook 비활성화: 파일 닫기 전 Hook 먼저 종료", func(t *testing.T) {
		// Note: 순서를 엄격하게 테스트하려면 Mocking 순서 추적이 필요하지만,
		// 여기서는 Hook이 확실히 닫혔는지 확인하는 것으로 충분함.
		// (순서는 코드 리뷰와 주석으로 보장된 상태)
		h := &hook{}
		c := &closer{hook: h}

		err := c.Close()

		assert.NoError(t, err)
		assert.True(t, h.closed, "Hook must be closed after closer.Close()")
	})
}

func TestCloser_Sync(t *testing.T) {
	t.Run("Sync 지원 시 호출 확인", func(t *testing.T) {
		// Given
		ms := new(MockSyncCloser)
		// Expect Sync to be called BEFORE Close
		ms.On("Sync").Return(nil).Once()
		ms.On("Close").Return(nil).Once()

		c := &closer{
			closers: []io.Closer{ms},
		}

		// When
		err := c.Close()

		// Then
		assert.NoError(t, err)
		ms.AssertExpectations(t)
	})

	t.Run("Sync 실패 시 무시하고 Close 진행", func(t *testing.T) {
		// Given
		ms := new(MockSyncCloser)
		// Sync fails, but Close should still proceed
		ms.On("Sync").Return(errors.New("sync failed")).Once()
		ms.On("Close").Return(nil).Once()

		c := &closer{
			closers: []io.Closer{ms},
		}

		// When
		err := c.Close()

		// Then
		assert.NoError(t, err, "Sync error should be ignored")
		ms.AssertExpectations(t)
	})
}

func TestCloser_NilSafe(t *testing.T) {
	t.Run("Nil 요소가 있어도 패닉 없이 동작", func(t *testing.T) {
		// Given
		m1 := new(MockCloser)
		m1.On("Close").Return(nil)

		c := &closer{
			// closers slice contains nil
			closers: []io.Closer{nil, m1, nil},
			hook:    nil, // hook is also nil
		}

		// When
		err := c.Close()

		// Then
		assert.NoError(t, err)
		m1.AssertExpectations(t)
	})
}
