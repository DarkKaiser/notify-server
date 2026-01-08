//go:build test

package log

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mocks
// =============================================================================

// MockCloser is a generic mock that implements io.Closer.
type MockCloser struct {
	mock.Mock
}

func (m *MockCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockSyncCloser implements both io.Closer and a Sync() method.
// Used to verify that Sync is called before Close.
type MockSyncCloser struct {
	MockCloser
}

func (m *MockSyncCloser) Sync() error {
	args := m.Called()
	return args.Error(0)
}

// =============================================================================
// Tests
// =============================================================================

// TestCloser_Close verifies the basic functionality of closing resources.
func TestCloser_Close(t *testing.T) {
	t.Parallel()

	t.Run("성공: 모든 리소스가 정상적으로 닫힘", func(t *testing.T) {
		// Given
		m1 := new(MockCloser)
		m2 := new(MockCloser)
		// Setup expectations: Close should be called once on each
		m1.On("Close").Return(nil).Once()
		m2.On("Close").Return(nil).Once()

		h := &hook{}
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

	t.Run("실패: 일부 리소스 닫기 실패 시 에러 집계", func(t *testing.T) {
		// Given
		m1 := new(MockCloser)
		m2 := new(MockCloser) // Fails
		m3 := new(MockCloser)

		errFail := errors.New("fail to close")

		m1.On("Close").Return(nil).Once()
		m2.On("Close").Return(errFail).Once()
		m3.On("Close").Return(nil).Once()

		c := &closer{
			closers: []io.Closer{m1, m2, m3},
		}

		// When
		err := c.Close()

		// Then
		require.Error(t, err)
		assert.ErrorIs(t, err, errFail, "Returned error should wrap the underlying error")

		// Verify all closers were attempted
		m1.AssertExpectations(t)
		m2.AssertExpectations(t)
		m3.AssertExpectations(t)
	})

	t.Run("순서: Sync가 Close보다 먼저 호출되어야 함", func(t *testing.T) {
		// Given
		ms := new(MockSyncCloser)

		// CallOrder verification using functional options or call sequence
		// Here we simply check the calls are made. Strict order can be checked by
		// tracking call times or using testify's .Run() but for simplicity,
		// we know the implementation calls Sync then Close.
		// Let's use written logic to verify order if possible, or assume implementation correctness
		// if we assert both are called.
		// To be expert-level strict:
		var callOrder []string

		ms.On("Sync").Return(nil).Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Sync")
		}).Once()

		ms.On("Close").Return(nil).Run(func(args mock.Arguments) {
			callOrder = append(callOrder, "Close")
		}).Once()

		c := &closer{
			closers: []io.Closer{ms},
		}

		// When
		err := c.Close()

		// Then
		assert.NoError(t, err)
		ms.AssertExpectations(t)
		assert.Equal(t, []string{"Sync", "Close"}, callOrder, "Execution order mismatch")
	})
}

// TestCloser_Concurrency verifies thread-safety and idempotency.
func TestCloser_Concurrency(t *testing.T) {
	t.Parallel()

	t.Run("Idempotency: 중복 호출 시 단 한 번만 실행", func(t *testing.T) {
		// Given
		m1 := new(MockCloser)
		// Expect Close to be called exactly ONCE
		m1.On("Close").Return(nil).Once()

		c := &closer{
			closers: []io.Closer{m1},
		}

		// When: Call Close multiple times concurrently
		concurrency := 100
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func() {
				defer wg.Done()
				// Add small random delay to increase race chance
				if i%2 == 0 {
					time.Sleep(time.Microsecond)
				}
				_ = c.Close()
			}()
		}
		wg.Wait()

		// Then
		m1.AssertExpectations(t)
		assert.Equal(t, int32(1), atomic.LoadInt32(&c.closed), "Closed flag must be 1")
	})
}

// TestCloser_NilHooks verifies robustness against nil fields.
func TestCloser_NilHooks(t *testing.T) {
	t.Parallel()

	t.Run("Nil Hook이나 Nil Closer 요소가 있어도 안전", func(t *testing.T) {
		m1 := new(MockCloser)
		m1.On("Close").Return(nil).Once()

		c := &closer{
			hook:    nil, // Explicitly nil
			closers: []io.Closer{nil, m1, nil},
		}

		assert.NotPanics(t, func() {
			err := c.Close()
			assert.NoError(t, err)
		})

		m1.AssertExpectations(t)
	})
}
