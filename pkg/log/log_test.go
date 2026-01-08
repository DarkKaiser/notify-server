//go:build test

package log

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// API Unit Tests (log.go Facade)
// =============================================================================

// TestNew verifies that New() returns a fresh Logger instance.
func TestNew(t *testing.T) {
	t.Parallel()

	logger := New()
	assert.NotNil(t, logger)
	assert.IsType(t, &Logger{}, logger)
	// Default level is Info
	assert.Equal(t, InfoLevel, logger.Level)
}

// TestStandardLogger verifies access to the global singleton logger.
func TestStandardLogger(t *testing.T) {
	t.Parallel()

	std := StandardLogger()
	assert.NotNil(t, std)
	assert.IsType(t, &Logger{}, std)
}

// TestSetOutput verifies that SetOutput affects the standard logger.
func TestSetOutput(t *testing.T) {
	// Not parallel because it modifies global state
	resetForTest()
	defer resetForTest()

	var buf bytes.Buffer
	SetOutput(&buf)

	// Log something using the standard logger (via helper or direct)
	StandardLogger().Info("test output")

	assert.Contains(t, buf.String(), "test output")
}

// TestSetFormatter verifies that SetFormatter affects the standard logger.
func TestSetFormatter(t *testing.T) {
	// Not parallel because it modifies global state
	resetForTest()
	defer resetForTest()

	// Use JSON formatter
	SetFormatter(&JSONFormatter{})

	var buf bytes.Buffer
	SetOutput(&buf)

	StandardLogger().Info("test json")

	assert.Contains(t, buf.String(), "test json")
	assert.Contains(t, buf.String(), "{") // Should look like JSON
	assert.Contains(t, buf.String(), "}")
}

// TestContextHelpers verifies the convenience functions for adding context.
func TestContextHelpers(t *testing.T) {
	t.Parallel()

	// 1. WithFields
	t.Run("WithFields", func(t *testing.T) {
		fields := Fields{"key": "value", "id": 123}
		entry := WithFields(fields)

		assert.NotNil(t, entry)
		assert.Equal(t, "value", entry.Data["key"])
		assert.Equal(t, 123, entry.Data["id"])
	})

	// 2. WithComponent
	t.Run("WithComponent", func(t *testing.T) {
		entry := WithComponent("auth-service")

		assert.NotNil(t, entry)
		// Assuming "component" is the key used in WithComponent implementation
		assert.Equal(t, "auth-service", entry.Data["component"])
	})

	// 3. WithComponentAndFields
	t.Run("WithComponentAndFields", func(t *testing.T) {
		fields := Fields{"action": "login"}
		entry := WithComponentAndFields("user-service", fields)

		assert.NotNil(t, entry)
		assert.Equal(t, "user-service", entry.Data["component"])
		assert.Equal(t, "login", entry.Data["action"])
	})
}

// =============================================================================
// Contract Tests (Entry Immutability & Behavior)
// =============================================================================
// Although Entry is an alias, we verify expected behavior for stability.

func TestEntry_Immutability(t *testing.T) {
	t.Parallel()

	base := New().WithField("base", "origin")

	// 1. Derive child
	child := base.WithField("child", "modified")

	// 2. Verify independence
	assert.NotSame(t, base, child, "WithField should return a new Entry instance")
	assert.Equal(t, "origin", base.Data["base"])
	assert.Nil(t, base.Data["child"], "Base entry should not be modified")

	assert.Equal(t, "origin", child.Data["base"])
	assert.Equal(t, "modified", child.Data["child"])
}

func TestEntry_WithError(t *testing.T) {
	t.Parallel()

	base := New().WithField("foo", "bar")
	err := errors.New("something wrong")

	entry := base.WithError(err)

	assert.Equal(t, err, entry.Data["error"]) // logrus uses "error" key by default
	assert.Equal(t, "bar", entry.Data["foo"])
}

func TestEntry_Concurrency_Safe(t *testing.T) {
	t.Parallel()

	base := WithFields(Fields{"static": "value"})
	var wg sync.WaitGroup

	// Concurrently derive new entries
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = base.WithField("id", id)
		}(i)
	}
	wg.Wait()

	// Base should remain untouched
	assert.Equal(t, "value", base.Data["static"])
	assert.Len(t, base.Data, 1)
}

// TestCallerReporting verifies that helper functions don't mess up caller reporting.
// This is somewhat an integration test but relevant to proper usage of helpers.
func TestCallerReporting_Helpers(t *testing.T) {
	// Setup environment
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	opts := Options{
		Name:             "caller-helper-test",
		Dir:              tempDir,
		ReportCaller:     true,
		EnableConsoleLog: false,
	}
	closer, err := Setup(opts)
	require.NoError(t, err)
	defer closer.Close()

	// Act: Log using helper
	WithComponent("test-helper").Info("Helper Call")

	// Verify
	files, _ := os.ReadDir(tempDir)
	logFile := filepath.Join(tempDir, files[0].Name()) // Assume main log is generated
	content, _ := os.ReadFile(logFile)

	// Check if the log file contains this function name "TestCallerReporting_Helpers"
	// and NOT "log.go" or "WithComponent" as the caller
	assert.Contains(t, string(content), "TestCallerReporting_Helpers", "Caller should be the test function, not the helper")
}
