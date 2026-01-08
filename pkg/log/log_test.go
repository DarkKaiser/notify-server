//go:build test

package log

import (
	"bytes"
	"errors"
	"io"
	"os"
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

	// WithContext is not wrapped in log.go but often used with WithTime/WithField.
	// We check standard logrus behavior compatibility if we add it or just ensure Entry usage is compatible.
	// log.go doesn't export WithContext wrapper currently.
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

	// Call all convenience wrappers
	Trace("trace msg")
	Debug("debug msg")
	Info("info msg")
	Warn("warn msg")
	Error("error msg")

	// Verify entries captured
	require.Len(t, hook.entries, 5)
	assert.Equal(t, logrus.TraceLevel, hook.entries[0].Level)
	assert.Equal(t, "trace msg", hook.entries[0].Message)

	assert.Equal(t, logrus.DebugLevel, hook.entries[1].Level)
	assert.Equal(t, "debug msg", hook.entries[1].Message)

	assert.Equal(t, logrus.InfoLevel, hook.entries[2].Level)
	assert.Equal(t, "info msg", hook.entries[2].Message)

	assert.Equal(t, logrus.WarnLevel, hook.entries[3].Level)
	assert.Equal(t, "warn msg", hook.entries[3].Message)

	assert.Equal(t, logrus.ErrorLevel, hook.entries[4].Level)
	assert.Equal(t, "error msg", hook.entries[4].Message)
}

func TestGlobalLogging_Formats(t *testing.T) {
	resetForTest()
	defer resetForTest()

	hook := &testHook{}
	logrus.AddHook(hook)
	logrus.SetOutput(io.Discard)
	logrus.SetLevel(logrus.TraceLevel)

	Tracef("trace %d", 1)
	Debugf("debug %d", 2)
	Infof("info %d", 3)
	Warnf("warn %d", 4)
	Errorf("error %d", 5)

	require.Len(t, hook.entries, 5)
	assert.Equal(t, "trace 1", hook.entries[0].Message)
	assert.Equal(t, "debug 2", hook.entries[1].Message)
	assert.Equal(t, "info 3", hook.entries[2].Message)
	assert.Equal(t, "warn 4", hook.entries[3].Message)
	assert.Equal(t, "error 5", hook.entries[4].Message)
}

func TestCallerReporting_Integration(t *testing.T) {
	// Ensures that wrapper functions don't obscure caller information when ReportCaller is true.
	// This is critical for function wrappers.
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

	// The log should point to THIS function (TestCallerReporting_Integration),
	// NOT the wrapper function in log.go.
	// If wrappers are not marked as helper or logrus depth isn't handled (logrus handles it via expensive stack walk finding first non-logrus),
	// wrapper in another package might be issue?
	// logrus generally attempts to skip its own frames. Since pkg/log wrappers calls logrus.*,
	// logrus sees pkg/log as the caller.
	// However, standard logrus wrappers usually work because user calls pkg/log.Info -> logrus.Info.
	// Setup sets CallerPrettyfier.

	// In Setup, we configure CallerPrettyfier.
	// Logrus ReportCaller logic: finds first stack frame outside of logrus package.
	// Since our wrappers are in `github.com/darkkaiser/notify-server/pkg/log`,
	// and we are calling from `.../pkg/log/log_test.go` (same package!),
	// distincting wrapping vs calling is tricky if they are in same package.
	// BUT, usually `pkg/log` is imported by `main` or `internal/service`.
	// Testing inside `pkg/log` (same package) might report `log_test.go` correctly.

	// Let's check expectation.
	assert.Contains(t, string(content), "TestCallerReporting_Integration", "Should report the test function as caller")
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
	// But entry.Data map is mutable.
	h.entries = append(h.entries, e)
	return nil
}

// contextKey is for testing WithContext (if valid)
type contextKey struct{}
