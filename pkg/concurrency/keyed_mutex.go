// Package concurrency 키(Key) 기반의 세분화된 락킹(Fine-grained Locking) 등 고효율 동시성 제어 유틸리티를 제공합니다.
package concurrency

import (
	"sync"
)

// KeyedMutex 키별로 독립적인 Mutex를 제공하는 구조체입니다.
// 서로 다른 키에 대한 작업은 병렬로 처리될 수 있습니다.
// Reference Counting을 사용하여 사용되지 않는 Mutex를 메모리에서 정리합니다.
type KeyedMutex[T comparable] struct {
	mu    sync.Mutex
	locks map[T]*entry
	pool  sync.Pool
}

type entry struct {
	mu       sync.Mutex
	refCount int
}

// NewKeyedMutex 새로운 KeyedMutex 인스턴스를 생성합니다.
func NewKeyedMutex[T comparable]() *KeyedMutex[T] {
	return &KeyedMutex[T]{
		locks: make(map[T]*entry),
		pool: sync.Pool{
			New: func() interface{} {
				return &entry{}
			},
		},
	}
}

// Len 현재 활성화된(락이 잡혀있거나 대기 중인) 키의 개수를 반환합니다.
func (km *KeyedMutex[T]) Len() int {
	km.mu.Lock()
	defer km.mu.Unlock()
	return len(km.locks)
}

// Lock 지정된 키에 대한 락을 획득합니다.
func (km *KeyedMutex[T]) Lock(key T) {
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
func (km *KeyedMutex[T]) TryLock(key T) bool {
	km.mu.Lock()
	e, ok := km.locks[key]
	if !ok {
		// 키가 없으면 새로 생성 (무조건 성공)
		e = km.pool.Get().(*entry)
		e.refCount = 1
		// 중요: 전역 락(km.mu)을 해제하기 전에 개별 락(e.mu)을 먼저 선점해야 합니다.
		// 만약 순서가 바뀌면 km.mu Unlock 직후 다른 고루틴이 해당 키에 대해 Lock을 걸어버릴 수 있으며,
		// 이 경우 TryLock 호출자가 e.mu.Lock()에서 블로킹되어 "즉시 반환"이라는 TryLock의 계약을 위반하게 됩니다.
		e.mu.Lock()
		km.locks[key] = e
		km.mu.Unlock()

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
func (km *KeyedMutex[T]) Unlock(key T) {
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

// WithLock 지정된 키에 대해 Lock을 획득하고 에러를 반환할 수 있는 함수(action)를 실행한 뒤 자동으로 Unlock합니다.
// action 실행 중 에러가 발생하더라도 Lock은 안전하게 해제됩니다.
func (km *KeyedMutex[T]) WithLock(key T, action func() error) error {
	km.Lock(key)
	defer km.Unlock(key)
	return action()
}
