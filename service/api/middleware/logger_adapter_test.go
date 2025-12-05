package middleware

import (
	"bytes"
	"testing"

	"github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLogger_Level(t *testing.T) {
	l := logrus.New()
	logger := Logger{l}

	l.SetLevel(logrus.DebugLevel)
	assert.Equal(t, log.DEBUG, logger.Level())

	l.SetLevel(logrus.InfoLevel)
	assert.Equal(t, log.INFO, logger.Level())

	l.SetLevel(logrus.WarnLevel)
	assert.Equal(t, log.WARN, logger.Level())

	l.SetLevel(logrus.ErrorLevel)
	assert.Equal(t, log.ERROR, logger.Level())
}

func TestLogger_SetLevel(t *testing.T) {
	l := logrus.New()
	logger := Logger{l}

	logger.SetLevel(log.DEBUG)
	assert.Equal(t, logrus.DebugLevel, l.Level)

	logger.SetLevel(log.INFO)
	assert.Equal(t, logrus.InfoLevel, l.Level)

	logger.SetLevel(log.WARN)
	assert.Equal(t, logrus.WarnLevel, l.Level)

	logger.SetLevel(log.ERROR)
	assert.Equal(t, logrus.ErrorLevel, l.Level)
}

func TestLogger_Output(t *testing.T) {
	l := logrus.New()
	logger := Logger{l}
	var buf bytes.Buffer
	logger.SetOutput(&buf)

	assert.Equal(t, &buf, logger.Output())
}

func TestLogger_Methods(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	logger := Logger{l}

	// Test Print
	logger.Print("test print")
	assert.Contains(t, buf.String(), "test print")
	buf.Reset()

	// Test Info
	logger.Info("test info")
	assert.Contains(t, buf.String(), "test info")
	assert.Contains(t, buf.String(), "info")
	buf.Reset()

	// Test Warn
	logger.Warn("test warn")
	assert.Contains(t, buf.String(), "test warn")
	assert.Contains(t, buf.String(), "warning")
	buf.Reset()

	// Test Error
	logger.Error("test error")
	assert.Contains(t, buf.String(), "test error")
	assert.Contains(t, buf.String(), "error")
	buf.Reset()

	// Test Debug (Level must be debug)
	logger.SetLevel(log.DEBUG)
	logger.Debug("test debug")
	assert.Contains(t, buf.String(), "test debug")
	assert.Contains(t, buf.String(), "debug")
	buf.Reset()
}

func TestLogger_JSONMethods(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	logger := Logger{l}

	j := log.JSON{"key": "value"}

	// Test Infoj
	logger.Infoj(j)
	assert.Contains(t, buf.String(), "value")
	buf.Reset()

	// Test Warnj
	logger.Warnj(j)
	assert.Contains(t, buf.String(), "value")
	buf.Reset()

	// Test Errorj
	logger.Errorj(j)
	assert.Contains(t, buf.String(), "value")
	buf.Reset()
}
