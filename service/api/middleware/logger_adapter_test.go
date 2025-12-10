package middleware

import (
	"bytes"
	"testing"

	"github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLoggerAdapter_Level_Table(t *testing.T) {
	tests := []struct {
		name          string
		logrusLevel   logrus.Level
		expectedLevel log.Lvl
	}{
		{"Debug Level", logrus.DebugLevel, log.DEBUG},
		{"Info Level", logrus.InfoLevel, log.INFO},
		{"Warn Level", logrus.WarnLevel, log.WARN},
		{"Error Level", logrus.ErrorLevel, log.ERROR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := logrus.New()
			logger := Logger{l}
			l.SetLevel(tt.logrusLevel)
			assert.Equal(t, tt.expectedLevel, logger.Level())
		})
	}
}

func TestLoggerAdapter_SetLevel_Table(t *testing.T) {
	tests := []struct {
		name          string
		inputLevel    log.Lvl
		expectedLevel logrus.Level
	}{
		{"Set Debug", log.DEBUG, logrus.DebugLevel},
		{"Set Info", log.INFO, logrus.InfoLevel},
		{"Set Warn", log.WARN, logrus.WarnLevel},
		{"Set Error", log.ERROR, logrus.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := logrus.New()
			logger := Logger{l}
			logger.SetLevel(tt.inputLevel)
			assert.Equal(t, tt.expectedLevel, l.Level)
		})
	}
}

func TestLoggerAdapter_Methods_Table(t *testing.T) {
	tests := []struct {
		name      string
		action    func(*Logger)
		expectLog []string
		level     log.Lvl
	}{
		{
			name:      "Print",
			action:    func(l *Logger) { l.Print("test print") },
			expectLog: []string{"test print"},
			level:     log.INFO,
		},
		{
			name:      "Info",
			action:    func(l *Logger) { l.Info("test info") },
			expectLog: []string{"test info", "info"},
			level:     log.INFO,
		},
		{
			name:      "Warn",
			action:    func(l *Logger) { l.Warn("test warn") },
			expectLog: []string{"test warn", "warning"},
			level:     log.WARN,
		},
		{
			name:      "Error",
			action:    func(l *Logger) { l.Error("test error") },
			expectLog: []string{"test error", "error"},
			level:     log.ERROR,
		},
		{
			name:      "Debug",
			action:    func(l *Logger) { l.Debug("test debug") },
			expectLog: []string{"test debug", "debug"},
			level:     log.DEBUG,
		},
		{
			name:      "Infof",
			action:    func(l *Logger) { l.Infof("info %s", "formatted") },
			expectLog: []string{"info formatted", "info"},
			level:     log.INFO,
		},
		{
			name:      "Warnf",
			action:    func(l *Logger) { l.Warnf("warn %d", 123) },
			expectLog: []string{"warn 123", "warning"},
			level:     log.WARN,
		},
		{
			name:      "Errorf",
			action:    func(l *Logger) { l.Errorf("error %v", true) },
			expectLog: []string{"error true", "error"},
			level:     log.ERROR,
		},
		{
			name:      "Debugf",
			action:    func(l *Logger) { l.Debugf("debug %s", "msg") },
			expectLog: []string{"debug msg", "debug"},
			level:     log.DEBUG,
		},
		{
			name:      "Infoj",
			action:    func(l *Logger) { l.Infoj(log.JSON{"key": "value"}) },
			expectLog: []string{"value", "info"},
			level:     log.INFO,
		},
		{
			name:      "Warnj",
			action:    func(l *Logger) { l.Warnj(log.JSON{"key": "value"}) },
			expectLog: []string{"value", "warning"},
			level:     log.WARN,
		},
		{
			name:      "Errorj",
			action:    func(l *Logger) { l.Errorj(log.JSON{"key": "value"}) },
			expectLog: []string{"value", "error"},
			level:     log.ERROR,
		},
		{
			name:      "Debugj",
			action:    func(l *Logger) { l.Debugj(log.JSON{"key": "value"}) },
			expectLog: []string{"value", "debug"},
			level:     log.DEBUG,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := logrus.New()
			l.SetOutput(&buf)
			l.SetFormatter(&logrus.JSONFormatter{}) // Use JSON formatter for easier check

			logger := &Logger{l}
			logger.SetLevel(tt.level)

			tt.action(logger)

			logOutput := buf.String()
			for _, expect := range tt.expectLog {
				assert.Contains(t, logOutput, expect)
			}
		})
	}
}

func TestLoggerAdapter_Output(t *testing.T) {
	l := logrus.New()
	logger := Logger{l}
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	assert.Equal(t, &buf, logger.Output())
}

func TestLoggerAdapter_Prefix(t *testing.T) {
	// Logger adapter implementation of SetPrefix might be no-op or specific.
	// Current implementation doesn't seem to use prefix but let's check basic interface compliance
	l := logrus.New()
	logger := Logger{l}
	logger.SetPrefix("test")
	// Implementation intentionally ignores prefix
	assert.Equal(t, "", logger.Prefix())
}

func TestLoggerAdapter_SetHeader(t *testing.T) {
	// Header is likely ignored by logrus adapter but needs to be callable
	l := logrus.New()
	logger := Logger{l}
	logger.SetHeader("header")
	// No assertion really possible if it's a no-op, just compliance
}
