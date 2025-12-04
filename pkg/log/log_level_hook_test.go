package log

import (
	"bytes"
	"errors"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLogLevelFileHook_Levels(t *testing.T) {
	hook := &LogLevelHook{}
	assert.Equal(t, log.AllLevels, hook.Levels())
}

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

type errorFormatter struct{}

func (f *errorFormatter) Format(entry *log.Entry) ([]byte, error) {
	return nil, errors.New("format error")
}

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
