package log

import (
	"io"

	log "github.com/sirupsen/logrus"
)

// multiCloser 여러 Closer를 한 번에 닫습니다.
type multiCloser struct {
	closers []io.Closer

	hook *LogLevelFileHook
}

func (mc *multiCloser) Close() error {
	// Hook 제거 (파일을 닫기 전에 Hook을 먼저 제거)
	if mc.hook != nil {
		// logrus의 모든 레벨에서 이 Hook 제거
		logger := log.StandardLogger()
		for _, level := range log.AllLevels {
			// 현재 레벨의 Hook 리스트에서 우리의 Hook을 제거
			hooks := logger.Hooks[level]
			newHooks := make([]log.Hook, 0, len(hooks))
			for _, h := range hooks {
				if h != mc.hook {
					newHooks = append(newHooks, h)
				}
			}
			logger.Hooks[level] = newHooks
		}
	}

	// 모든 파일 닫기 (첫 번째 에러를 기록하되, 모든 파일을 닫음)
	var firstErr error
	for _, closer := range mc.closers {
		if closer != nil {
			if err := closer.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}
