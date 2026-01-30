//go:build test

package log

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Delegation Verification (StandardLogger & Global Config)
// =============================================================================

func TestStandardLogger(t *testing.T) {
	t.Parallel()
	// pkg/log.StandardLogger() must return the underlying logrus.StandardLogger()
	assert.Same(t, logrus.StandardLogger(), StandardLogger())
}

func TestSetOutput(t *testing.T) {
	// Global state modification - run serially
	resetForTest()
	defer resetForTest()

	var buf bytes.Buffer
	SetOutput(&buf)

	Info("test output")
	assert.Contains(t, buf.String(), "test output")
}

func TestSetFormatter(t *testing.T) {
	resetForTest()
	defer resetForTest()

	SetFormatter(&logrus.JSONFormatter{})
	var buf bytes.Buffer
	SetOutput(&buf)

	Info("json test")
	assert.Contains(t, buf.String(), `"msg":"json test"`)
	assert.Contains(t, buf.String(), `"level":"info"`)
}

func TestSetLevel(t *testing.T) {
	resetForTest()
	defer resetForTest()

	SetLevel(ErrorLevel)
	assert.Equal(t, ErrorLevel, logrus.GetLevel())

	SetLevel(DebugLevel)
	assert.Equal(t, DebugLevel, logrus.GetLevel())
}

// =============================================================================
// Helper Functions Verification (Entry Constructors)
// =============================================================================

func TestContextHelpers_Construction(t *testing.T) {
	t.Parallel()

	t.Run("WithField", func(t *testing.T) {
		entry := WithField("key", "val")
		assert.Equal(t, "val", entry.Data["key"])
	})

	t.Run("WithFields", func(t *testing.T) {
		entry := WithFields(Fields{"foo": "bar", "baz": 123})
		assert.Equal(t, "bar", entry.Data["foo"])
		assert.Equal(t, 123, entry.Data["baz"])
	})

	t.Run("WithError", func(t *testing.T) {
		err := errors.New("oops")
		entry := WithError(err)
		assert.Equal(t, err, entry.Data[logrus.ErrorKey])
	})

	t.Run("WithTime", func(t *testing.T) {
		now := time.Now().Add(-1 * time.Hour)
		entry := WithTime(now)
		assert.Equal(t, now, entry.Time)
	})

	t.Run("WithComponent", func(t *testing.T) {
		entry := WithComponent("my-component")
		assert.Equal(t, "my-component", entry.Data["component"])
	})

	t.Run("WithComponentAndFields", func(t *testing.T) {
		entry := WithComponentAndFields("my-component", Fields{
			"extra": "data",
		})
		assert.Equal(t, "my-component", entry.Data["component"])
		assert.Equal(t, "data", entry.Data["extra"])
	})

	t.Run("WithContext", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), contextKey{}, "val")
		entry := WithContext(ctx)
		assert.Equal(t, ctx, entry.Context)
	})
}

// =============================================================================
// Global Logging Behavior (Integration-like)
// =============================================================================

func TestGlobalLogging_Levels(t *testing.T) {
	// Captures output via Hook to verify that global function calls (Info, Warn...)
	// actually trigger valid log entries.
	resetForTest()
	defer resetForTest()

	hook := &testHook{} // minimal hook to capture entries
	logrus.AddHook(hook)
	logrus.SetOutput(io.Discard) // prevent stdout noise
	logrus.SetLevel(logrus.TraceLevel)

	tests := []struct {
		name     string
		logFunc  func(args ...interface{})
		expected logrus.Level
		message  string
	}{
		{"Trace", Trace, logrus.TraceLevel, "trace msg"},
		{"Debug", Debug, logrus.DebugLevel, "debug msg"},
		{"Info", Info, logrus.InfoLevel, "info msg"},
		{"Warn", Warn, logrus.WarnLevel, "warn msg"},
		{"Error", Error, logrus.ErrorLevel, "error msg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook.reset()
			tt.logFunc(tt.message)
			require.Len(t, hook.entries, 1)
			assert.Equal(t, tt.expected, hook.entries[0].Level)
			assert.Equal(t, tt.message, hook.entries[0].Message)
		})
	}
}

func TestGlobalLogging_Formats(t *testing.T) {
	resetForTest()
	defer resetForTest()

	hook := &testHook{}
	logrus.AddHook(hook)
	logrus.SetOutput(io.Discard)
	logrus.SetLevel(logrus.TraceLevel)

	tests := []struct {
		name     string
		logFunc  func(format string, args ...interface{})
		expected logrus.Level
		format   string
		args     []interface{}
		message  string
	}{
		{"Tracef", Tracef, logrus.TraceLevel, "trace %d", []interface{}{1}, "trace 1"},
		{"Debugf", Debugf, logrus.DebugLevel, "debug %d", []interface{}{2}, "debug 2"},
		{"Infof", Infof, logrus.InfoLevel, "info %d", []interface{}{3}, "info 3"},
		{"Warnf", Warnf, logrus.WarnLevel, "warn %d", []interface{}{4}, "warn 4"},
		{"Errorf", Errorf, logrus.ErrorLevel, "error %d", []interface{}{5}, "error 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook.reset()
			tt.logFunc(tt.format, tt.args...)
			require.Len(t, hook.entries, 1)
			assert.Equal(t, tt.expected, hook.entries[0].Level)
			assert.Equal(t, tt.message, hook.entries[0].Message)
		})
	}
}

func TestGlobals_Panic(t *testing.T) {
	resetForTest()
	defer resetForTest()
	logrus.SetOutput(io.Discard)

	t.Run("Panic", func(t *testing.T) {
		assert.Panics(t, func() {
			Panic("panic msg")
		})
	})

	t.Run("Panicf", func(t *testing.T) {
		assert.Panics(t, func() {
			Panicf("panic code %d", 500)
		})
	})
}

// TestGlobals_Fatal uses a subprocess to verify os.Exit(1)
func TestGlobals_Fatal(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		logrus.SetOutput(io.Discard)
		Fatal("fatal msg")
		return
	}
	if os.Getenv("BE_CRASHER") == "2" {
		logrus.SetOutput(io.Discard)
		Fatalf("fatal code %d", 1)
		return
	}

	t.Run("Fatal", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=TestGlobals_Fatal")
		cmd.Env = append(os.Environ(), "BE_CRASHER=1")
		err := cmd.Run()
		if e, ok := err.(*exec.ExitError); ok && !e.Success() {
			return // Expected exit status 1
		}
		t.Fatalf("process ran with err %v, want exit status 1", err)
	})

	t.Run("Fatalf", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=TestGlobals_Fatal")
		cmd.Env = append(os.Environ(), "BE_CRASHER=2")
		err := cmd.Run()
		if e, ok := err.(*exec.ExitError); ok && !e.Success() {
			return // Expected exit status 1
		}
		t.Fatalf("process ran with err %v, want exit status 1", err)
	})
}

func TestCallerReporting_Integration(t *testing.T) {
	// Ensures that wrapper functions don't obscure caller information when ReportCaller is true.
	resetForTest()
	tempDir := t.TempDir()

	opts := Options{
		Name:             "caller-test",
		Dir:              tempDir,
		ReportCaller:     true,
		EnableConsoleLog: false,
	}

	closer, err := Setup(opts)
	require.NoError(t, err)
	defer closer.Close()

	// Call via wrapper
	Info("Wrapper Call")

	// Verify file content
	content, err := os.ReadFile(filepath.Join(tempDir, "caller-test.log"))
	require.NoError(t, err)

	assert.Contains(t, string(content), "TestCallerReporting_Integration", "Should report the test function as caller")
}

func TestConcurrentLogging(t *testing.T) {
	resetForTest()
	defer resetForTest()
	logrus.SetOutput(io.Discard)

	var wg sync.WaitGroup
	count := 100
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func(i int) {
			defer wg.Done()
			Info(fmt.Sprintf("log %d", i))
			WithField("idx", i).Debug("debug log")
		}(i)
	}
	wg.Wait()
}

// =============================================================================
// Helpers
// =============================================================================

type testHook struct {
	entries []*logrus.Entry
	mu      sync.Mutex
}

func (h *testHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *testHook) Fire(e *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Copy entry to avoid mutation issues if reused (though logrus usually creates new)
	// We just store reference for simple verify

	// Create a partial copy to safely store data
	newEntry := &logrus.Entry{
		Level:   e.Level,
		Message: e.Message,
		Data:    make(logrus.Fields),
		Time:    e.Time,
	}
	for k, v := range e.Data {
		newEntry.Data[k] = v
	}

	h.entries = append(h.entries, newEntry)
	return nil
}

func (h *testHook) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = nil
}

// contextKey is for testing WithContext
type contextKey struct{}
