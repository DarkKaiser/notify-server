package concurrency

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Test Helpers
// =============================================================================

// assertRefCheck는 KeyedMutex의 RefCount를 검증하는 헬퍼 함수입니다.
func assertRefCheck(t *testing.T, km *KeyedMutex, key string, expected int) {
	t.Helper()
	km.mu.Lock()
	defer km.mu.Unlock()
	entry, ok := km.locks[key]
	assert.True(t, ok, "키가 존재해야 합니다")
	if ok {
		assert.Equal(t, expected, entry.refCount, "RefCount 불일치")
	}
}

// =============================================================================
// Basic Lock/Unlock Tests
// =============================================================================

// TestKeyedMutex_LockUnlock_Scenarios_TableDriven은 다양한 Lock/Unlock 시나리오를 검증합니다.
//
// 검증 항목:
//   - 단일 키 Lock/Unlock
//   - 여러 다른 키 Lock/Unlock
//   - 동일 키 순차적 Lock/Unlock
func TestKeyedMutex_LockUnlock_Scenarios_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		parallel bool
	}{
		{
			name:     "Single Key",
			keys:     []string{"key1"},
			parallel: false,
		},
		{
			name:     "Multiple Different Keys",
			keys:     []string{"key1", "key2", "key3"},
			parallel: false,
		},
		{
			name:     "Same Key Multiple Times (Sequential)",
			keys:     []string{"key1", "key1"},
			parallel: false,
		},
		{
			name:     "Empty String Key",
			keys:     []string{""},
			parallel: false,
		},
		{
			name:     "Special Characters in Key",
			keys:     []string{"key:with:colons", "key/with/slashes", "key-with-dashes"},
			parallel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			km := NewKeyedMutex()
			for _, key := range tt.keys {
				km.Lock(key)
				// Critical Section Simulation
				km.Unlock(key)
			}
		})
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestKeyedMutex_Concurrency_Scenarios는 고동시성 환경에서 KeyedMutex의 동작을 검증합니다.
//
// 검증 항목:
//   - 단일 키에 대한 높은 동시성 (Hot Key)
//   - 여러 키에 대한 높은 동시성
//   - 모든 작업이 누락 없이 수행되는지 검증
func TestKeyedMutex_Concurrency_Scenarios(t *testing.T) {
	tests := []struct {
		name       string
		workers    int
		iterations int
		keys       []string // 각 워커가 사용할 키 (순환 사용)
	}{
		{
			name:       "High Concurrency on Single Key",
			workers:    100,
			iterations: 100,
			keys:       []string{"hot-key"},
		},
		{
			name:       "High Concurrency on Multiple Keys",
			workers:    100,
			iterations: 100,
			keys:       []string{"key1", "key2", "key3", "key4"},
		},
		{
			name:       "Moderate Concurrency on Many Keys",
			workers:    50,
			iterations: 50,
			keys:       []string{"k1", "k2", "k3", "k4", "k5", "k6", "k7", "k8", "k9", "k10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			km := NewKeyedMutex()

			// 키별 카운터 생성
			counters := make(map[string]*int32)
			for _, k := range tt.keys {
				var zero int32
				counters[k] = &zero
			}

			var wg sync.WaitGroup
			wg.Add(tt.workers)

			for i := 0; i < tt.workers; i++ {
				go func(id int) {
					defer wg.Done()
					key := tt.keys[id%len(tt.keys)] // 키 할당
					counter := counters[key]        // 해당 키의 카운터

					for j := 0; j < tt.iterations; j++ {
						km.Lock(key)
						// Critical Section
						// 여기서는 동일한 키에 대해서만 상호 배제가 보장됨
						// 따라서 키별 카운터를 사용해야 Race Condition 없이 Load->Store 검증 가능
						c := atomic.LoadInt32(counter)
						// time.Sleep(1 * time.Microsecond) // 인위적 지연 (필요시)
						atomic.StoreInt32(counter, c+1)
						km.Unlock(key)
					}
				}(i)
			}

			wg.Wait()

			// 총 실행 횟수 검증
			var total int32
			for _, c := range counters {
				total += atomic.LoadInt32(c)
			}
			expected := int32(tt.workers * tt.iterations)
			assert.Equal(t, expected, total, "모든 작업이 누락 없이 수행되어야 합니다")
		})
	}
}

// =============================================================================
// RefCount and Cleanup Tests
// =============================================================================

// TestKeyedMutex_RefCountCleanup_Deterministic는 RefCount 기반 메모리 정리를 검증합니다.
//
// 검증 항목:
//   - RefCount가 올바르게 증가/감소하는지
//   - 모든 고루틴이 완료된 후 맵이 비워지는지
func TestKeyedMutex_RefCountCleanup_Deterministic(t *testing.T) {
	km := NewKeyedMutex()
	key := "cleanup-key"

	// 1. 메인: 락 획득
	km.Lock(key)
	assertRefCheck(t, km, key, 1)

	// 2. 서브: 락 획득 시도 (별도 고루틴)
	done := make(chan bool)
	go func() {
		km.Lock(key)   // 메인이 Unlock 할 때까지 여기서 대기
		km.Unlock(key) // 획득 즉시 해제
		done <- true
	}()

	// 3. 서브 고루틴이 락 대기 상태에 들어갈 때까지 대기 (Polling)
	// time.Sleep 대신 조건이 만족될 때까지 검사
	assert.Eventually(t, func() bool {
		km.mu.Lock()
		defer km.mu.Unlock()
		if e, ok := km.locks[key]; ok {
			return e.refCount == 2
		}
		return false
	}, 1*time.Second, 10*time.Millisecond, "서브 고루틴이 진입하여 RefCount가 2가 되어야 합니다")

	// 4. 메인: 락 해제 (이제 서브가 진행됨)
	km.Unlock(key)

	// 5. 서브 완료 대기
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("서브 고루틴이 제시간에 완료되지 않았습니다")
	}

	// 6. 최종 상태 검증 (맵이 비워져야 함)
	km.mu.Lock()
	_, ok := km.locks[key]
	lenLocks := len(km.locks)
	km.mu.Unlock()

	assert.False(t, ok, "키가 제거되어야 합니다")
	assert.Equal(t, 0, lenLocks, "맵이 완전히 비워져야 합니다")
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestKeyedMutex_EdgeCases는 엣지 케이스를 검증합니다.
//
// 검증 항목:
//   - Unlock without Lock (안전하게 처리되는지)
//   - 매우 긴 키 이름
//   - Unicode 키 이름
func TestKeyedMutex_EdgeCases(t *testing.T) {
	t.Run("Unlock without Lock", func(t *testing.T) {
		km := NewKeyedMutex()
		// Unlock을 Lock 없이 호출 (패닉이 발생하지 않아야 함)
		assert.NotPanics(t, func() {
			km.Unlock("non-existent-key")
		}, "Lock 없이 Unlock을 호출해도 패닉이 발생하지 않아야 합니다")
	})

	t.Run("Very Long Key Name", func(t *testing.T) {
		km := NewKeyedMutex()
		longKey := string(make([]byte, 10000))
		for i := range longKey {
			longKey = longKey[:i] + "a"
		}

		km.Lock(longKey)
		km.Unlock(longKey)

		// 맵이 비워졌는지 확인
		km.mu.Lock()
		lenLocks := len(km.locks)
		km.mu.Unlock()
		assert.Equal(t, 0, lenLocks, "긴 키도 정상적으로 정리되어야 합니다")
	})

	t.Run("Unicode Key Name", func(t *testing.T) {
		km := NewKeyedMutex()
		unicodeKey := "키-🔒-テスト-测试"

		km.Lock(unicodeKey)
		km.Unlock(unicodeKey)

		// 맵이 비워졌는지 확인
		km.mu.Lock()
		lenLocks := len(km.locks)
		km.mu.Unlock()
		assert.Equal(t, 0, lenLocks, "Unicode 키도 정상적으로 정리되어야 합니다")
	})

	t.Run("Rapid Lock/Unlock Cycles", func(t *testing.T) {
		km := NewKeyedMutex()
		key := "rapid-key"

		for i := 0; i < 1000; i++ {
			km.Lock(key)
			km.Unlock(key)
		}

		// 맵이 비워졌는지 확인
		km.mu.Lock()
		lenLocks := len(km.locks)
		km.mu.Unlock()
		assert.Equal(t, 0, lenLocks, "빠른 Lock/Unlock 사이클 후에도 정리되어야 합니다")
	})
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// BenchmarkKeyedMutex_SingleKey는 단일 키에 대한 Lock/Unlock 성능을 측정합니다.
func BenchmarkKeyedMutex_SingleKey(b *testing.B) {
	km := NewKeyedMutex()
	key := "bench-key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		km.Lock(key)
		km.Unlock(key)
	}
}

// BenchmarkKeyedMutex_MultipleKeys는 여러 키에 대한 Lock/Unlock 성능을 측정합니다.
func BenchmarkKeyedMutex_MultipleKeys(b *testing.B) {
	km := NewKeyedMutex()
	keys := []string{"key1", "key2", "key3", "key4", "key5"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		km.Lock(key)
		km.Unlock(key)
	}
}

// BenchmarkKeyedMutex_Parallel는 병렬 환경에서의 성능을 측정합니다.
func BenchmarkKeyedMutex_Parallel(b *testing.B) {
	km := NewKeyedMutex()
	keys := []string{"key1", "key2", "key3", "key4"}

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
