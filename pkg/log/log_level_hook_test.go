package log

import (
	"bytes"
	"errors"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Test Helpers
// =============================================================================

// errorFormatter는 포맷팅 에러를 시뮬레이션하는 테스트용 Formatter입니다.
type errorFormatter struct{}

func (f *errorFormatter) Format(entry *log.Entry) ([]byte, error) {
	return nil, errors.New("format error")
}

// channelWriter는 동시성 테스트를 위한 안전한 Writer입니다.
type channelWriter struct {
	ch chan []byte
}

func (w *channelWriter) Write(p []byte) (n int, err error) {
	// Send to channel or just discard for concurrency testing
	// Sending might block if channel is full, so just discard for this specific test
	// strictly checking for panic/race
	return len(p), nil
}

// =============================================================================
// Hook Interface Tests
// =============================================================================

// TestLogLevelFileHook_Levels는 Hook이 처리할 로그 레벨을 검증합니다.
//
// 검증 항목:
//   - 모든 로그 레벨 반환 (AllLevels)
func TestLogLevelFileHook_Levels(t *testing.T) {
	hook := &LogLevelHook{}
	assert.Equal(t, log.AllLevels, hook.Levels())
}

// =============================================================================
// Log Level Routing Tests
// =============================================================================

// TestLogLevelFileHook_Fire는 로그 레벨에 따른 라우팅을 검증합니다.
//
// 검증 항목:
//   - Critical 레벨 (Error, Fatal, Panic)은 criticalWriter에 기록
//   - Verbose 레벨 (Debug, Trace)은 verboseWriter에 기록
//   - 기타 레벨 (Info, Warn)은 무시
func TestLogLevelFileHook_Fire(t *testing.T) {
	formatter := &log.TextFormatter{DisableTimestamp: true}

	t.Run("Critical 레벨 로그는 criticalWriter에 기록", func(t *testing.T) {
		criticalBuf := &bytes.Buffer{}
		verboseBuf := &bytes.Buffer{}

		hook := &LogLevelHook{
			criticalWriter: criticalBuf,
			verboseWriter:  verboseBuf,
			formatter:      formatter,
		}

		entry := &log.Entry{
			Level:   log.ErrorLevel,
			Message: "error message",
		}

		err := hook.Fire(entry)
		assert.NoError(t, err)

		assert.Contains(t, criticalBuf.String(), "level=error")
		assert.Contains(t, criticalBuf.String(), "msg=\"error message\"")
		assert.Empty(t, verboseBuf.String())
	})

	t.Run("Verbose 레벨 로그는 verboseWriter에 기록", func(t *testing.T) {
		criticalBuf := &bytes.Buffer{}
		verboseBuf := &bytes.Buffer{}

		hook := &LogLevelHook{
			criticalWriter: criticalBuf,
			verboseWriter:  verboseBuf,
			formatter:      formatter,
		}

		entry := &log.Entry{
			Level:   log.DebugLevel,
			Message: "debug message",
		}

		err := hook.Fire(entry)
		assert.NoError(t, err)

		assert.Contains(t, verboseBuf.String(), "level=debug")
		assert.Contains(t, verboseBuf.String(), "msg=\"debug message\"")
		assert.Empty(t, criticalBuf.String())
	})

	t.Run("Info 레벨 로그는 무시 (Writer에 기록하지 않음)", func(t *testing.T) {
		criticalBuf := &bytes.Buffer{}
		verboseBuf := &bytes.Buffer{}

		hook := &LogLevelHook{
			criticalWriter: criticalBuf,
			verboseWriter:  verboseBuf,
			formatter:      formatter,
		}

		entry := &log.Entry{
			Level:   log.InfoLevel,
			Message: "info message",
		}

		err := hook.Fire(entry)
		assert.NoError(t, err)

		assert.Empty(t, criticalBuf.String())
		assert.Empty(t, verboseBuf.String())
	})
}

// =============================================================================
// Error Handling Tests
// =============================================================================

// TestLogLevelFileHook_Fire_NilWriter는 nil Writer 처리를 검증합니다.
//
// 검증 항목:
//   - Writer가 nil일 때 에러 없이 무시
func TestLogLevelFileHook_Fire_NilWriter(t *testing.T) {
	formatter := &log.TextFormatter{DisableTimestamp: true}

	t.Run("Writer가 nil일 때 에러 없이 무시", func(t *testing.T) {
		hook := &LogLevelHook{
			criticalWriter: nil, // nil writer
			verboseWriter:  nil, // nil writer
			formatter:      formatter,
		}

		entry := &log.Entry{
			Level:   log.ErrorLevel,
			Message: "error message",
		}

		err := hook.Fire(entry)
		assert.NoError(t, err)
	})
}

// TestLogLevelFileHook_Fire_FormatError는 포맷팅 에러 처리를 검증합니다.
//
// 검증 항목:
//   - 포맷팅 에러 발생 시 에러 반환
func TestLogLevelFileHook_Fire_FormatError(t *testing.T) {
	t.Run("포맷팅 에러 발생 시 에러 반환", func(t *testing.T) {
		hook := &LogLevelHook{
			criticalWriter: &bytes.Buffer{},
			formatter:      &errorFormatter{},
		}

		entry := &log.Entry{
			Level:   log.ErrorLevel,
			Message: "test",
		}

		err := hook.Fire(entry)
		assert.Error(t, err)
		assert.Equal(t, "format error", err.Error())
	})
}

// =============================================================================
// Concurrency Tests
// =============================================================================

// TestLogLevelFileHook_ConcurrentWrite는 동시성 환경에서의 안전성을 검증합니다.
//
// 검증 항목:
//   - 여러 고루틴에서 동시에 Fire 호출 시 Data Race 없음
//   - Panic 발생하지 않음
func TestLogLevelFileHook_ConcurrentWrite(t *testing.T) {
	formatter := &log.TextFormatter{DisableTimestamp: true}

	// Use our safe writer mock
	safeWriter := &channelWriter{
		ch: make(chan []byte, 10000),
	}

	hook := &LogLevelHook{
		criticalWriter: safeWriter,
		verboseWriter:  safeWriter,
		formatter:      formatter,
	}

	t.Run("동시성 쓰기 테스트 - Data Race 확인용", func(t *testing.T) {
		concurrency := 10
		iterations := 100
		done := make(chan bool)

		for i := 0; i < concurrency; i++ {
			go func() {
				for j := 0; j < iterations; j++ {
					// We ignore errors here as we are testing for race conditions/panics
					_ = hook.Fire(&log.Entry{
						Level:   log.ErrorLevel,
						Message: "concurrent error",
					})
					_ = hook.Fire(&log.Entry{
						Level:   log.DebugLevel,
						Message: "concurrent debug",
					})
				}
				done <- true
			}()
		}

		for i := 0; i < concurrency; i++ {
			<-done
		}

		// If we reached here without panic/race (when run with -race), pass.
		assert.True(t, true)
	})
}
