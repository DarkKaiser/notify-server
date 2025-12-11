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
}

type entry struct {
	mu       sync.Mutex
	refCount int
}

// NewKeyedMutex 새로운 KeyedMutex 인스턴스를 생성합니다.
func NewKeyedMutex() *KeyedMutex {
	return &KeyedMutex{
		locks: make(map[string]*entry),
	}
}

// Lock 지정된 키에 대한 락을 획득합니다.
func (km *KeyedMutex) Lock(key string) {
	km.mu.Lock()
	e, ok := km.locks[key]
	if !ok {
		e = &entry{refCount: 1}
		km.locks[key] = e
	} else {
		e.refCount++
	}
	km.mu.Unlock()

	e.mu.Lock()
}

// Unlock 지정된 키에 대한 락을 해제합니다.
// 주의: 반드시 Lock을 호출한 후에 호출해야 합니다.
func (km *KeyedMutex) Unlock(key string) {
	km.mu.Lock()
	e, ok := km.locks[key]
	if !ok {
		km.mu.Unlock()
		// 락을 걸지 않고 언락을 시도한 경우 (프로그래머 실수)
		// 패닉을 발생시키거나 무시할 수 있습니다. 여기서는 안전하게 리턴합니다.
		return
	}

	e.refCount--
	if e.refCount <= 0 {
		delete(km.locks, key)
	}
	km.mu.Unlock()

	e.mu.Unlock()
}
