package concurrency

import (
	"sync"
)

// KeyedMutex 키별로 독립적인 Mutex를 제공하는 구조체입니다.
// 서로 다른 키에 대한 작업은 병렬로 처리될 수 있습니다.
// Reference Counting을 사용하여 사용되지 않는 Mutex를 메모리에서 정리합니다.
type KeyedMutex struct {
	mu    sync.Mutex
	locks map[string]*entry
	pool  sync.Pool
}

type entry struct {
	mu       sync.Mutex
	refCount int
}

// NewKeyedMutex 새로운 KeyedMutex 인스턴스를 생성합니다.
func NewKeyedMutex() *KeyedMutex {
	return &KeyedMutex{
		locks: make(map[string]*entry),
		pool: sync.Pool{
			New: func() interface{} {
				return &entry{}
			},
		},
	}
}

// Len 현재 활성화된(락이 잡혀있거나 대기 중인) 키의 개수를 반환합니다.
func (km *KeyedMutex) Len() int {
	km.mu.Lock()
	defer km.mu.Unlock()
	return len(km.locks)
}

// Lock 지정된 키에 대한 락을 획득합니다.
func (km *KeyedMutex) Lock(key string) {
	km.mu.Lock()
	e, ok := km.locks[key]
	if !ok {
		e = km.pool.Get().(*entry)
		e.refCount = 1
		km.locks[key] = e
	} else {
		e.refCount++
	}
	km.mu.Unlock()

	e.mu.Lock()
}

// TryLock 지정된 키에 대한 락을 시도합니다.
// 락을 획득하면 true를, 이미 다른 고루틴이 락을 소유하고 있으면 대기하지 않고 false를 반환합니다.
//
// 성공(true) 시: 반드시 Unlock을 호출하여 락을 해제해야 합니다.
// 실패(false) 시: 아무런 작업도 수행하지 않으며, Unlock을 호출해서는 안 됩니다.
func (km *KeyedMutex) TryLock(key string) bool {
	km.mu.Lock()
	e, ok := km.locks[key]
	if !ok {
		// 키가 없으면 새로 생성 (무조건 성공)
		e = km.pool.Get().(*entry)
		e.refCount = 1
		km.locks[key] = e
		km.mu.Unlock()

		// 새 뮤텍스이므로 Lock은 무조건 성공하지만, 일관성을 위해 Lock 호출
		e.mu.Lock()
		return true
	}

	// 키가 있으면 TryLock 시도
	// 주의: TryLock은 mu가 잠겨있지 않을 때만 성공함
	if e.mu.TryLock() {
		// 락 획득 성공 시 참조 카운트 증가
		e.refCount++
		km.mu.Unlock()
		return true
	}

	// 락 획득 실패 (이미 사용 중)
	km.mu.Unlock()
	return false
}

// Unlock 지정된 키에 대한 락을 해제합니다.
// 주의: 반드시 Lock을 호출한 후에 호출해야 합니다.
// 락이 걸려있지 않은 키에 대해 Unlock을 호출하면 런타임 패닉이 발생합니다.
func (km *KeyedMutex) Unlock(key string) {
	km.mu.Lock()
	defer km.mu.Unlock()

	e, ok := km.locks[key]
	if !ok {
		panic("잠기지 않은 KeyedMutex의 잠금 해제 시도")
	}

	// 1. 개별 키에 대한 락을 해제합니다.
	e.mu.Unlock()

	// 2. 참조 카운트 감소 및 정리
	e.refCount--
	if e.refCount <= 0 {
		delete(km.locks, key)
		km.pool.Put(e)
	}
}
