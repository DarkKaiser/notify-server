package log

import (
	"io"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

// LogLevelHook 로그 레벨에 따라 다른 파일에 로그를 기록하는 Hook입니다.
// logrus의 Hook 인터페이스를 구현합니다.
type LogLevelHook struct {
	criticalWriter io.Writer     // ERROR, FATAL, PANIC 레벨 로그를 기록할 Writer
	verboseWriter  io.Writer     // DEBUG, TRACE 레벨 로그를 기록할 Writer
	formatter      log.Formatter // 로그 포매터

	closed int32 // 원자적 접근을 위한 플래그 (0: open, 1: closed)
}

// Levels 이 Hook이 처리할 로그 레벨을 반환합니다.
func (hook *LogLevelHook) Levels() []log.Level {
	return log.AllLevels
}

// Fire 로그 엔트리를 레벨에 따라 적절한 파일에 기록합니다.
func (hook *LogLevelHook) Fire(entry *log.Entry) error {
	// 닫힌 경우 무시 (Race Condition 방지)
	if atomic.LoadInt32(&hook.closed) == 1 {
		return nil
	}

	var writer io.Writer

	switch entry.Level {
	case log.ErrorLevel, log.FatalLevel, log.PanicLevel:
		writer = hook.criticalWriter
		if writer == nil {
			return nil
		}
	case log.DebugLevel, log.TraceLevel:
		writer = hook.verboseWriter
		if writer == nil {
			return nil
		}
	default:
		return nil // Info, Warn은 메인 파일에만 기록
	}

	// 로그 포맷팅 및 기록
	msg, err := hook.formatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = writer.Write(msg)
	return err
}

// Close Hook을 비활성화합니다. 이후 Fire 호출은 무시됩니다.
func (hook *LogLevelHook) Close() {
	atomic.StoreInt32(&hook.closed, 1)
}
