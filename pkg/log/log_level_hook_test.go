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
func TestLogLevelFileHook_Fire(t *testing.T) {
	formatter := &log.TextFormatter{DisableTimestamp: true}

	tests := []struct {
		name              string
		level             log.Level
		message           string
		wantMainEntry     bool // Info+ (No Debug/Trace)
		wantCriticalEntry bool // Error+
		wantVerboseEntry  bool // Debug/Trace
	}{
		{
			name:              "Critical Log (Error)",
			level:             log.ErrorLevel,
			message:           "error msg",
			wantMainEntry:     true, // Main also gets Error for context
			wantCriticalEntry: true,
			wantVerboseEntry:  false,
		},
		{
			name:              "Critical Log (Fatal)",
			level:             log.FatalLevel,
			message:           "fatal msg",
			wantMainEntry:     true,
			wantCriticalEntry: true,
			wantVerboseEntry:  false,
		},
		{
			name:              "Verbose Log (Debug)",
			level:             log.DebugLevel,
			message:           "debug msg",
			wantMainEntry:     false, // Filtered out
			wantCriticalEntry: false,
			wantVerboseEntry:  true,
		},
		{
			name:              "Verbose Log (Trace)",
			level:             log.TraceLevel,
			message:           "trace msg",
			wantMainEntry:     false, // Filtered out
			wantCriticalEntry: false,
			wantVerboseEntry:  true,
		},
		{
			name:              "Main Log (Info)",
			level:             log.InfoLevel,
			message:           "info msg",
			wantMainEntry:     true,
			wantCriticalEntry: false,
			wantVerboseEntry:  false,
		},
		{
			name:              "Main Log (Warn)",
			level:             log.WarnLevel,
			message:           "warn msg",
			wantMainEntry:     true,
			wantCriticalEntry: false,
			wantVerboseEntry:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mainBuf := &bytes.Buffer{}
			criticalBuf := &bytes.Buffer{}
			verboseBuf := &bytes.Buffer{}

			hook := &LogLevelHook{
				mainWriter:     mainBuf,
				criticalWriter: criticalBuf,
				verboseWriter:  verboseBuf,
				formatter:      formatter,
			}

			entry := &log.Entry{
				Level:   tt.level,
				Message: tt.message,
			}

			err := hook.Fire(entry)
			assert.NoError(t, err)

			if tt.wantMainEntry {
				assert.Contains(t, mainBuf.String(), tt.message, "Should contain message in MAIN writer")
			} else {
				assert.NotContains(t, mainBuf.String(), tt.message, "Should NOT contain message in MAIN writer (Filtered)")
			}

			if tt.wantCriticalEntry {
				assert.Contains(t, criticalBuf.String(), tt.message, "Should contain message in critical writer")
			} else {
				assert.NotContains(t, criticalBuf.String(), tt.message, "Should NOT contain message in critical writer")
			}

			if tt.wantVerboseEntry {
				assert.Contains(t, verboseBuf.String(), tt.message, "Should contain message in verbose writer")
			} else {
				assert.NotContains(t, verboseBuf.String(), tt.message, "Should NOT contain message in verbose writer")
			}
		})
	}
}

// TestLogLevelFileHook_Fire_BestEffort_Write는 부분 실패 시 지속성을 검증합니다.
//
// 검증 항목:
//   - Critical Writer 실패 시에도 Main Writer에는 정상 기록되어야 함 (Cascading Failure 방지)
//   - 에러가 반환되어야 함
func TestLogLevelFileHook_Fire_BestEffort_Write(t *testing.T) {
	formatter := &log.TextFormatter{DisableTimestamp: true}

	// 모의 에러 Writer
	errWriter := &errorWriter{err: errors.New("disk full")}
	mainBuf := &bytes.Buffer{}

	hook := &LogLevelHook{
		mainWriter:     mainBuf,   // 정상 작동해야 함
		criticalWriter: errWriter, // 실패하도록 설정
		verboseWriter:  nil,
		formatter:      formatter,
	}

	entry := &log.Entry{
		Level:   log.ErrorLevel,
		Message: "critical error",
	}

	// 실행
	err := hook.Fire(entry)

	// 검증 1: 에러는 반환되어야 함 (로그 실패 알림)
	assert.Error(t, err)
	assert.Equal(t, "disk full", err.Error())

	// 검증 2: ★핵심★ Critical 실패와 무관하게 Main 로그는 기록되어야 함
	assert.Contains(t, mainBuf.String(), "critical error", "Critical Writer ERROR should NOT prevent Main Writer from writing")
}

// errorWriter는 항상 에러를 반환하는 테스트용 Writer입니다.
type errorWriter struct {
	err error
}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	return 0, w.err
}

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

// =============================================================================
// State Management Tests
// =============================================================================

// TestLogLevelFileHook_Close_AtomicState는 Close 상태의 원자적 관리를 검증합니다.
//
// 검증 항목:
//   - Close 호출 후 Fire가 무시되는지 (Safe Shutdown)
//   - 이미 닫힌 파일에 대한 쓰기 시도를 방지하는지
func TestLogLevelFileHook_Close_AtomicState(t *testing.T) {
	formatter := &log.TextFormatter{DisableTimestamp: true}
	buf := &bytes.Buffer{}

	hook := &LogLevelHook{
		verboseWriter: buf,
		formatter:     formatter,
	}

	// 1. 정상 상태: 로그 기록됨
	err := hook.Fire(&log.Entry{
		Level:   log.DebugLevel,
		Message: "Before Close",
	})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Before Close")

	// 2. Close 호출
	err = hook.Close()
	assert.NoError(t, err)

	buf.Reset()

	// 3. 닫힌 후 상태: 로그 기록 안 됨 (No Error, Just Ignore)
	err = hook.Fire(&log.Entry{
		Level:   log.DebugLevel,
		Message: "After Close",
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, buf.Len(), "Close된 Hook은 로그를 기록하지 않아야 합니다")
}
