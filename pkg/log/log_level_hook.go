package log

import (
	"io"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
)

// LogLevelHook Logrus의 Hook 인터페이스를 구현하여 '로그 레벨 기반 분산 로깅'을 수행합니다.
//
// 주요 역할:
//   - 로그 라우팅: 로그 레벨(Info, Error 등)에 따라 적절한 대상(Main, Critical, Verbose)으로 출력을 분기합니다.
//   - 노이즈 필터링: 메인 로그에 불필요한 디버그 정보가 섞이지 않도록 선별적으로 기록합니다.
type LogLevelHook struct {
	mainWriter     io.Writer // INFO, WARN, ERROR, FATAL, PANIC (Debug/Trace 제외)
	criticalWriter io.Writer // ERROR, FATAL, PANIC
	verboseWriter  io.Writer // DEBUG, TRACE

	formatter log.Formatter // 로그 포매터

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

	// 로그 포맷팅 (한 번만 수행하여 재사용)
	msg, err := hook.formatter.Format(entry)
	if err != nil {
		return err
	}

	var firstErr error

	// 1. Critical Writer (Error+):
	//    - 심각한 오류(Error, Fatal, Panic)를 별도 파일에 격리 보관합니다.
	//    - 쓰기에 실패하더라도 Main 로그 기록을 시도하기 위해 에러를 즉시 반환하지 않고 보관합니다.
	if entry.Level <= log.ErrorLevel {
		if hook.criticalWriter != nil {
			if _, err := hook.criticalWriter.Write(msg); err != nil {
				firstErr = err // 에러 보관 후 계속 진행
			}
		}
	}

	// 2. Verbose Writer (Debug/Trace):
	//    - 개발 및 디버깅을 위한 상세 로그를 처리합니다.
	//    - 중요: 이 단계에서 처리를 마치고 함수를 종료하여, 대량의 디버그 로그가 Main Writer로 유입되는 것을 원천 차단합니다.
	if entry.Level >= log.DebugLevel {
		if hook.verboseWriter != nil {
			if _, err := hook.verboseWriter.Write(msg); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		// 상세 로그(Debug/Trace)는 메인 로그에 남기지 않습니다.
		// 따라서 Main Writer로 넘어가지 않고 여기서 함수를 종료합니다.
		return firstErr
	}

	// 3. Main Writer (Info+):
	//    - 일반적인 운영 로그(Info, Warn)와 문맥 파악을 위한 에러 로그를 기록합니다.
	//    - 앞선 Critical Writer에서 파일 시스템 장애 등이 발생했더라도, 이곳의 기록 시도는 보장됩니다.
	if hook.mainWriter != nil {
		if _, err := hook.mainWriter.Write(msg); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// Close Hook을 비활성화합니다. 이후 Fire 호출은 무시됩니다.
func (hook *LogLevelHook) Close() error {
	atomic.StoreInt32(&hook.closed, 1)
	return nil
}
