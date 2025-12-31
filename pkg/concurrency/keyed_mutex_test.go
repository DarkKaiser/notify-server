package concurrency

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
			// fmt.Printf("Updating price for %s\n", productID)
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

// ExampleKeyedMutex_TryLock_success KeyedMutex.TryLock 메서드의 성공 케이스 예제입니다.
func ExampleKeyedMutex_TryLock_success() {
	km := NewKeyedMutex[string]()
	key := "resource_key"

	// 락 획득 시도 (성공)
	if km.TryLock(key) {
		fmt.Println("First lock acquired")

		// 중첩된 락 시도 (실패 - 이미 다른 곳에서 소유 중이라고 가정)
		// 주의: 동일 고루틴 내에서의 재진입(Reentrancy)은 지원하지 않으므로 실패합니다.
		if km.TryLock(key) {
			fmt.Println("Second lock acquired") // 실행되지 않음
		} else {
			fmt.Println("Second lock failed")
		}

		km.Unlock(key)
		fmt.Println("First lock released")
	}

	// Output:
	// First lock acquired
	// Second lock failed
	// First lock released
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
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // 개별 케이스 병렬 실행

			km := NewKeyedMutex[string]()
			var wg sync.WaitGroup

			for _, key := range tt.keys {
				wg.Add(1)
				go func(k string) {
					defer wg.Done()
					km.Lock(k)
					// Simulate work
					time.Sleep(time.Millisecond)
					km.Unlock(k)
				}(key)
			}
			wg.Wait()

			// 모든 작업 완료 후 내부 상태 검증 (Leak Check)
			assert.Equal(t, 0, km.Len(), "모든 작업 완료 후에는 맵이 비워져야 합니다")
		})
	}
}

func TestKeyedMutex_TryLock_Behavior(t *testing.T) {
	t.Parallel()

	km := NewKeyedMutex[string]()
	key := "try-lock-key"

	// 1. Initial Lock
	assert.True(t, km.TryLock(key), "최초 TryLock은 성공해야 합니다")
	assert.Equal(t, 1, km.Len())

	// 2. TryLock Fail (Already Locked)
	assert.False(t, km.TryLock(key), "이미 잠긴 키에 대한 TryLock은 실패해야 합니다")

	// 3. Unlock and Retry
	km.Unlock(key)
	assert.Equal(t, 0, km.Len())

	assert.True(t, km.TryLock(key), "Unlock 후 TryLock은 다시 성공해야 합니다")
	km.Unlock(key)
}

// TestKeyedMutex_MutualExclusion_StrictLocking은 비원자적 자원(map)을 보호함으로써
// 상호 배제가 실제로 작동하는지 엄격하게 검증합니다.
// 만약 Lock이 제대로 동작하지 않으면 'concurrent map writes' 패닉이 발생하거나 데이터가 깨집니다.
func TestKeyedMutex_MutualExclusion_Randomized(t *testing.T) {
	t.Parallel()

	km := NewKeyedMutex[string]()
	unsafeMap := make(map[string]int) // Thread-unsafe resource
	const (
		numGoroutines = 100
		numIncrements = 1000
		key           = "shared-resource"
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIncrements; j++ {
				km.Lock(key)
				// Critical Section
				// Lock이 없다면 여기서 Race Condition 발생 (Go Race Detector가 감지)
				unsafeMap["counter"]++
				km.Unlock(key)
			}
		}()
	}

	wg.Wait()

	// 검증
	assert.Equal(t, numGoroutines*numIncrements, unsafeMap["counter"], "카운터 값이 정확해야 합니다 (Race Condition 없음)")
	assert.Equal(t, 0, km.Len(), "리소스 정리 확인")
}

// TestKeyedMutex_IndependentLocking은 서로 다른 키에 대한 작업이
// 서로를 차단하지 않는지(독립성) 검증합니다.
func TestKeyedMutex_IndependentLocking(t *testing.T) {
	t.Parallel()

	km := NewKeyedMutex[string]()
	key1 := "slow-key"
	key2 := "fast-key"

	// Key1을 잡고 오래 대기
	km.Lock(key1)
	defer km.Unlock(key1)

	done := make(chan bool)

	go func() {
		// Key2는 Key1의 잠금 여부와 상관없이 즉시 획득 가능해야 함
		km.Lock(key2)
		km.Unlock(key2)
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("서로 다른 키에 대한 락 획득이 차단되었습니다 (독립성 위반)")
	}
}

// TestKeyedMutex_PanicSafety_UnlockWithoutLock
// Lock하지 않은 키를 Unlock할 때 패닉이 발생하는지 확인합니다.
func TestKeyedMutex_PanicSafety_UnlockWithoutLock(t *testing.T) {
	t.Parallel()

	km := NewKeyedMutex[string]()
	assert.Panics(t, func() {
		km.Unlock("never-locked")
	}, "Lock되지 않은 키를 Unlock하면 패닉이 발생해야 합니다")
}

// TestKeyedMutex_Generics_IntKey 정수형 키가 정상 작동하는지 확인합니다.
func TestKeyedMutex_Generics_IntKey(t *testing.T) {
	t.Parallel()

	km := NewKeyedMutex[int]()
	key := 12345
	unsafeCounter := 0

	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			_ = km.WithLock(key, func() error {
				// Critical Section
				unsafeCounter++
				return nil
			})
		}()
	}

	wg.Wait()
	assert.Equal(t, 10, unsafeCounter)
	assert.Equal(t, 0, km.Len())
}

// TestKeyedMutex_WithLock_Correctness WithLock 내부에서 Lock이 걸려있는지 확인합니다.
func TestKeyedMutex_WithLock_Correctness(t *testing.T) {
	t.Parallel()
	km := NewKeyedMutex[string]()
	key := "test-withlock"

	executed := false
	err := km.WithLock(key, func() error {
		executed = true
		// Lock이 걸려있는지 확인 (TryLock 실패)
		if km.TryLock(key) {
			t.Error("WithLock 내부에서는 Lock이 걸려있어야 합니다 (TryLock 실패 예상)")
			km.Unlock(key) // 테스트 복구
		}
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
	// WithLock 종료 후에는 Lock이 해제되어 있어야 함 (Reference count 0 - Len 0)
	assert.Equal(t, 0, km.Len())
}

func TestKeyedMutex_WithLock_ErrorHandling(t *testing.T) {
	t.Parallel()
	km := NewKeyedMutex[string]()
	key := "test-withlock-error"
	expectedErr := fmt.Errorf("simulated error")

	// 1. Success Case
	err := km.WithLock(key, func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, km.Len())

	// 2. Error Case
	err = km.WithLock(key, func() error {
		// Lock이 걸려있는지 확인
		if km.TryLock(key) {
			return fmt.Errorf("lock should be acquired")
		}
		return expectedErr
	})
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 0, km.Len())
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkKeyedMutex_LockUnlock_SingleKey(b *testing.B) {
	km := NewKeyedMutex[string]()
	key := "bench-key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		km.Lock(key)
		km.Unlock(key)
	}
}

func BenchmarkKeyedMutex_LockUnlock_Parallel_Disjoint(b *testing.B) {
	// 서로 다른 키를 사용하여 경합이 없는 상태에서의 오버헤드 측정
	km := NewKeyedMutex[string]()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// 고루틴마다 고유한 키 사용
		key := fmt.Sprintf("key-%d", rand.Int63())
		for pb.Next() {
			km.Lock(key)
			km.Unlock(key)
		}
	})
}

func BenchmarkKeyedMutex_LockUnlock_Parallel_HighContention(b *testing.B) {
	// 소수의 키에 대해 높은 경합 발생
	km := NewKeyedMutex[string]()
	keys := []string{"key-A", "key-B", "key-C", "key-D"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := keys[i%len(keys)]
			km.Lock(key)
			km.Unlock(key)
			i++
		}
	})
}

func BenchmarkKeyedMutex_Allocation(b *testing.B) {
	// 메모리 할당 효율성 측정 (sync.Pool 효과)
	km := NewKeyedMutex[string]()
	key := "alloc-key"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		km.Lock(key)
		km.Unlock(key)
	}
}
