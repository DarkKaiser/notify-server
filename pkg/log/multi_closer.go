package log

import (
	"io"
)

// multiCloser 여러 Closer를 한 번에 닫습니다.
type multiCloser struct {
	closers []io.Closer

	hook *LogLevelHook
}

func (mc *multiCloser) Close() error {
	// Hook 비활성화 (파일을 닫기 전에 Hook이 더 이상 쓰지 않도록 설정)
	if mc.hook != nil {
		mc.hook.Close()
	}

	// 모든 파일 닫기 (첫 번째 에러를 기록하되, 모든 파일을 닫음)
	var firstErr error
	for _, closer := range mc.closers {
		if closer != nil {
			// 가능한 경우 Sync() 호출하여 디스크에 기록 보장
			if s, ok := closer.(interface{ Sync() error }); ok {
				_ = s.Sync() // Sync 에러는 치명적이지 않으므로 무시 (Close 에러가 더 중요)
			}

			if err := closer.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}
