package concurrency

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Documentation Examples (GoDoc)
// =============================================================================

func ExampleKeyedMutex_Lock() {
	km := NewKeyedMutex[string]()
	var wg sync.WaitGroup

	// 상황: 여러 고루틴이 서로 다른 쇼핑몰의 상품 가격을 업데이트합니다.
	products := []string{"product-A", "product-B", "product-A"}

	for _, p := range products {
		wg.Add(1)
		go func(productID string) {
			defer wg.Done()

			// 상품 ID별로 락을 획득합니다.
			// "product-A"에 대한 작업은 순차적으로 실행되지만,
			// "product-B"는 "product-A"와 병렬로 실행될 수 있습니다.
			km.Lock(productID)
			defer km.Unlock(productID)

			// Critical Section: 가격 업데이트 로직 수행
		}(p)
	}

	wg.Wait()
	fmt.Println("All product prices updated.")

	// Output:
	// All product prices updated.
}

func ExampleKeyedMutex_TryLock() {
	km := NewKeyedMutex[string]()
	key := "hot-deal-item"

	// 첫 번째 고루틴이 락을 잡습니다.
	km.Lock(key)

	// 두 번째 고루틴이 락 획득을 시도합니다.
	if km.TryLock(key) {
		fmt.Println("Acquired lock!")
		km.Unlock(key)
	} else {
		fmt.Println("Failed to acquire lock, skipping task.")
	}

	km.Unlock(key)

	// Output:
	// Failed to acquire lock, skipping task.
}

func ExampleKeyedMutex_WithLock() {
	km := NewKeyedMutex[int]()
	key := 12345

	// WithLock 헬퍼 함수를 사용하여 Lock/Unlock을 안전하게 관리
	_ = km.WithLock(key, func() error {
		fmt.Printf("Critical section execution for key %d\n", key)
		return nil
	})

	// Output:
	// Critical section execution for key 12345
}

// =============================================================================
// Unit Tests
// =============================================================================

func TestNewKeyedMutex(t *testing.T) {
	km := NewKeyedMutex[string]()
	require.NotNil(t, km)
	assert.Equal(t, 0, km.Len())
}

func TestKeyedMutex_LockUnlock_Sequential(t *testing.T) {
	km := NewKeyedMutex[string]()
	key := "test-key"

	// 1. Lock -> Unlock
	km.Lock(key)
	assert.Equal(t, 1, km.Len(), "Lock 후에는 키가 맵에 존재해야 함")
	km.Unlock(key)
	assert.Equal(t, 0, km.Len(), "Unlock 후에는 키가 맵에서 제거되어야 함")

	// 2. Lock -> Lock (Re-entrance is not supported, effectively creates deadlock, so we don't test it here conventionally)
	// 대신 서로 다른 키에 대한 순차적 잠금 테스트
	km.Lock("key1")
	km.Lock("key2")
	assert.Equal(t, 2, km.Len())
	km.Unlock("key2")
	assert.Equal(t, 1, km.Len())
	km.Unlock("key1")
	assert.Equal(t, 0, km.Len())
}

func TestKeyedMutex_LockUnlock_Parallel(t *testing.T) {
	// Table Driven Test with Parallel Execution
	tests := []struct {
		name string
		keys []string
	}{
		{
			name: "Single Key",
			keys: []string{"key-1"},
		},
		{
			name: "Multiple Keys",
			keys: []string{"key-1", "key-2", "key-3"},
		},
		{
			name: "Duplicate Keys",
			keys: []string{"key-1", "key-1"},
		},
		{
			name: "Many Keys",
			keys: []string{"A", "B", "C", "D", "E", "A", "B", "C"},
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			km := NewKeyedMutex[string]()
			var wg sync.WaitGroup

			for _, key := range tt.keys {
				wg.Add(1)
				go func(k string) {
					defer wg.Done()
					km.Lock(k)
					// Simulate tiny work
					time.Sleep(10 * time.Microsecond)
					km.Unlock(k)
				}(key)
			}
			wg.Wait()

			// 모든 작업 완료 후 내부 상태 검증
			assert.Equal(t, 0, km.Len(), "모든 고루틴 종료 후 맵은 비워져야 합니다")
		})
	}
}

func TestKeyedMutex_TryLock(t *testing.T) {
	t.Parallel()

	km := NewKeyedMutex[string]()
	key := "try-lock-key"

	// 1. Initial TryLock (Success)
	assert.True(t, km.TryLock(key), "최초 TryLock은 성공해야 함")
	assert.Equal(t, 1, km.Len())

	// 2. TryLock Again (Fail - already locked)
	assert.False(t, km.TryLock(key), "이미 잠긴 키에 대한 TryLock은 실패해야 함")

	// 3. Unlock
	km.Unlock(key)
	assert.Equal(t, 0, km.Len())

	// 4. TryLock After Unlock (Success)
	assert.True(t, km.TryLock(key), "Unlock 후 TryLock은 성공해야 함")
	km.Unlock(key)
}

func TestKeyedMutex_WithLock(t *testing.T) {
	t.Parallel()

	t.Run("Success Case", func(t *testing.T) {
		km := NewKeyedMutex[string]()
		key := "success-key"
		called := false

		err := km.WithLock(key, func() error {
			called = true
			return nil
		})

		assert.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, 0, km.Len())
	})

	t.Run("Error Case", func(t *testing.T) {
		km := NewKeyedMutex[string]()
		key := "error-key"
		expectedErr := fmt.Errorf("some error")

		err := km.WithLock(key, func() error {
			return expectedErr
		})

		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, 0, km.Len())
	})

	t.Run("Panic Recovery", func(t *testing.T) {
		// WithLock은 현재 패닉 복구를 명시적으로 처리하지 않지만(Lock 상태 유지 위험),
		// defer Unlock이 호출되므로 패닉 발생 시에도 Unlock은 수행되어야 합니다.
		km := NewKeyedMutex[string]()
		key := "panic-key"

		assert.Panics(t, func() {
			_ = km.WithLock(key, func() error {
				panic("oops")
			})
		})

		// 패닉 이후에도 락은 해제되어야 함 (defer 덕분에)
		assert.Equal(t, 0, km.Len(), "패닉 발생 시에도 Unlock은 호출되어야 함")
	})
}

// TestKeyedMutex_MapLeak_Prevention 은 Unlock 시 맵이 비워지면
// 내부 맵이 재생성(Reset)되어 메모리 누수가 방지되는지 검증합니다.
func TestKeyedMutex_MapLeak_Prevention(t *testing.T) {
	t.Parallel()

	// 이 테스트는 화이트박스 테스트 성격이 강하므로,
	// 공개 API(Len, Address check 등)를 통해 간접적으로 검증하거나
	// 리플렉션 없이 동작을 추론해야 합니다.
	// Go에서 맵의 주소를 직접 비교하기는 어려우므로,
	// 여기서는 기능적으로 "대량의 키를 쓰고 지웠을 때 동작에 문제가 없는지"와
	// "비워진 후 재사용이 가능한지"를 봅니다.
	// 실제 메모리 해제 여부는 프로파일링 영역이지만, 로직상 len==0일 때 make가 호출되면 됩니다.

	km := NewKeyedMutex[int]()
	const iterations = 1000

	// 1. 대량의 락 생성 및 해제 반복
	for i := 0; i < iterations; i++ {
		km.Lock(i)
		km.Unlock(i)
	}

	assert.Equal(t, 0, km.Len())

	// 2. 다시 락 사용 (맵이 재생성되었어도 정상 동작해야 함)
	km.Lock(9999)
	assert.Equal(t, 1, km.Len())
	km.Unlock(9999)
	assert.Equal(t, 0, km.Len())
}

// TestKeyedMutex_MutualExclusion_Randomized 는 실제 데이터(map) 보호 여부를 통해
// 상호 배제(Mutual Exclusion)를 엄격히 검증합니다.
func TestKeyedMutex_MutualExclusion_Randomized(t *testing.T) {
	t.Parallel()

	km := NewKeyedMutex[string]()
	// 동시 접근 시 패닉이 발생하는 map을 공유 자원으로 사용
	sharedMap := make(map[string]int)
	key := "shared-resource"

	const (
		numGoroutines = 50
		numOps        = 100
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				km.Lock(key)
				// Critical Section: 동기화되지 않으면 여기서 'concurrent map writes' 패닉 발생
				sharedMap["counter"]++
				km.Unlock(key)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, numGoroutines*numOps, sharedMap["counter"], "카운터 값이 정확해야 함 (Race Condition 없음)")
	assert.Equal(t, 0, km.Len())
}

func TestKeyedMutex_IndependentLocking(t *testing.T) {
	t.Parallel()

	km := NewKeyedMutex[string]()
	key1 := "slow-key"
	key2 := "fast-key"

	// Key1 Lock
	km.Lock(key1)
	defer km.Unlock(key1)

	done := make(chan bool)

	go func() {
		// Key2는 Key1과 독립적이므로 즉시 획득 가능해야 함
		km.Lock(key2)
		km.Unlock(key2)
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Independency Violation: Key2 lock was blocked by Key1")
	}
}

func TestKeyedMutex_Panic_UnlockNotLocked(t *testing.T) {
	t.Parallel()

	km := NewKeyedMutex[string]()
	assert.Panics(t, func() {
		km.Unlock("never-locked")
	}, "Lock되지 않은 키를 Unlock 시 패닉 발생해야 함")
}

// Benchmarks

func BenchmarkKeyedMutex_LockUnlock_NoContention(b *testing.B) {
	km := NewKeyedMutex[string]()
	key := "bench-key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		km.Lock(key)
		km.Unlock(key)
	}
}

func BenchmarkKeyedMutex_LockUnlock_HighContention(b *testing.B) {
	km := NewKeyedMutex[string]()
	key := "hot-key"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			km.Lock(key)
			km.Unlock(key)
		}
	})
}
