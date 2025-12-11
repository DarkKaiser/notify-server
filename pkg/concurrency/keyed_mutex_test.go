package concurrency

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestKeyedMutex_LockUnlock_Scenarios_TableDriven 테이블 주도 테스트
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

// TestKeyedMutex_Concurrency_Scenarios 동시성 시나리오 테스트
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

// TestKeyedMutex_RefCountCleanup_Deterministic 결정론적 RefCount 테스트
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

// assertRefCheck RefCount 검증 헬퍼
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
