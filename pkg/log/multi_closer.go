package log

import (
	"errors"
	"io"
)

// multiCloser 여러 로그 파일(Main, Critical, Verbose)의 리소스 해제를 통합 관리합니다.
//
// 주요 특징:
//   - 원자적 종료 보장: 일부 파일 닫기에 실패하더라도 나머지 파일들의 Close()를 강제로 수행합니다.
//   - 안전한 종료 순서: Hook을 먼저 비활성화하여 종료 중인 파일에 대한 쓰기 시도(Panic)를 방지합니다.
type multiCloser struct {
	closers []io.Closer

	hook *LogLevelHook
}

func (mc *multiCloser) Close() error {
	// Hook 비활성화 (파일을 닫기 전에 Hook이 더 이상 쓰지 않도록 설정)
	if mc.hook != nil {
		mc.hook.Close()
	}

	// 모든 파일 닫기 (발생한 모든 에러를 수집)
	var errs error
	for _, closer := range mc.closers {
		if closer != nil {
			// 가능한 경우 Sync() 호출하여 디스크에 기록 보장
			if s, ok := closer.(interface{ Sync() error }); ok {
				_ = s.Sync() // Sync 에러는 치명적이지 않으므로 무시 (Close 에러가 더 중요)
			}

			if err := closer.Close(); err != nil {
				// Go 1.20+ errors.Join: 모든 에러를 하나로 묶어서 반환
				errs = errors.Join(errs, err)
			}
		}
	}

	return errs
}
