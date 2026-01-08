package log

import (
	"errors"
	"io"
	"sync/atomic"
)

// closer 여러 로그 파일(Main, Critical, Verbose)의 리소스 해제를 통합 관리합니다.
//
// 주요 특징:
//   - 원자적 종료 보장: 일부 파일 닫기에 실패하더라도 나머지 파일들의 Close()를 강제로 수행합니다.
//   - 안전한 종료 순서: Hook을 먼저 비활성화하여 종료 중인 파일에 대한 쓰기 시도(Panic)를 방지합니다.
//   - Idempotency 보장: Close()를 여러 번 호출해도 안전하며, 두 번째 이후 호출은 즉시 nil을 반환합니다.
type closer struct {
	closers []io.Closer

	hook *hook

	// closed 중복 Close() 호출을 방지하기 위한 원자적 플래그 (0: open, 1: closed)
	closed int32
}

func (c *closer) Close() error {
	// Idempotency 보장: 이미 닫힌 경우 즉시 반환
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil // 이미 닫힘
	}

	// Hook 우선 비활성화: 파일 리소스를 닫기 전에 로그 유입을 먼저 차단합니다.
	// 이는 닫힌 파일에 쓰기를 시도하는 경쟁 상태(Race Condition)와 패닉을 방지하기 위함입니다.
	if c.hook != nil {
		c.hook.Close()
	}

	// 리소스 해제: 관리 중인 모든 로그 파일을 닫습니다.
	// 일부 파일 닫기에 실패하더라도 중단하지 않고 모든 리소스 해제를 시도합니다.
	var errs error
	for _, closer := range c.closers {
		if closer != nil {
			// OS 버퍼 플러시: 파일 닫기 전 Sync()를 호출하여 메모리에 잔류하는 로그가 디스크에 안전하게 기록되도록 합니다.
			if s, ok := closer.(interface{ Sync() error }); ok {
				_ = s.Sync() // Sync 에러는 치명적이지 않으므로 무시 (Close 에러가 더 중요)
			}

			if err := closer.Close(); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}

	return errs
}
