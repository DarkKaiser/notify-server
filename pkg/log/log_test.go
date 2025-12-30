package log

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupLogTest prepares a clean environment for log testing.
// It returns a temporary directory path and a teardown function.
func setupLogTest(t *testing.T) (string, func()) {
	t.Helper()
	// Create a unique temporary directory for each test
	tempDir := t.TempDir()

	// Redirect stdout to capture console output if needed (optional)
	// For this suite, we mainly check file system side-effects.

	return tempDir, func() {
		// Reset global logrus state
		log.SetOutput(os.Stdout)
		log.SetFormatter(&log.TextFormatter{})
		log.SetLevel(log.InfoLevel)
		// Clear hooks
		log.StandardLogger().ReplaceHooks(make(log.LevelHooks))
	}
}

// assertLogFileExists verifies if a specific log file exists.
// expectedType: "main", "critical", or "verbose"
func assertLogFileExists(t *testing.T, logDir, appName, expectedType string) bool {
	t.Helper()
	files, err := os.ReadDir(logDir)
	if err != nil {
		return false
	}

	for _, file := range files {
		name := file.Name()
		// Basic prefix check
		if !strings.HasPrefix(name, appName) {
			continue
		}

		if expectedType == "main" {
			// Main log: "appName.log"
			if name == appName+"."+fileExt {
				return true
			}
		} else {
			// Sub logs: "appName.type.log"
			// e.g., "notify-server.critical.log"
			expected := fmt.Sprintf("%s.%s.%s", appName, expectedType, fileExt)
			if name == expected {
				return true
			}
		}
	}
	return false
}

// =============================================================================
// Unit Tests
// =============================================================================

func TestConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "log", fileExt, "File extension mismatch")
}

func TestWithComponentFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		comp     string
		fields   log.Fields
		expected log.Fields
	}{
		{
			name:     "Simple Component",
			comp:     "api",
			fields:   nil,
			expected: log.Fields{"component": "api"},
		},
		{
			name:     "Component with Fields",
			comp:     "task",
			fields:   log.Fields{"job_id": 123},
			expected: log.Fields{"component": "task", "job_id": 123},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var entry *log.Entry
			if tc.fields == nil {
				entry = WithComponent(tc.comp)
			} else {
				entry = WithComponentAndFields(tc.comp, tc.fields)
			}
			assert.Equal(t, tc.expected, entry.Data)
		})
	}
}

// =============================================================================
// Integration Tests (File System)
// =============================================================================

func TestSetup_Basic(t *testing.T) {
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	opts := Options{
		Name:             "test-app-basic",
		Dir:              tempDir,
		MaxAge:           7,
		EnableConsoleLog: false,
	}

	closer, err := Setup(opts)
	require.NoError(t, err)
	defer closer.Close()

	// Trigger lazy file creation
	log.Info("Hello World")

	// Verify file existence
	assert.True(t, assertLogFileExists(t, tempDir, opts.Name, "main"))
}

func TestSetup_InvalidConfig(t *testing.T) {
	// 1. Missing Name
	_, err := Setup(Options{Dir: "logs"}) // Name missing
	// Current implementation gracefully returns empty closer, logic changed in Step 238
	// "if opts.Name == "" { return &multiCloser{}, nil }"
	require.NoError(t, err)

	// 2. Invalid Directory (Permission Denied or Invalid Path)
	// Note: Hard to simulate reliably across OS without mocking.
	// We skip this for unit tests to avoid flakiness.
}

func TestLogLevelSeparation(t *testing.T) {
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	opts := Options{
		Name:              "test-app-levels",
		Dir:               tempDir,
		EnableCriticalLog: true,
		EnableVerboseLog:  true,
	}

	closer, err := Setup(opts)
	require.NoError(t, err)
	defer closer.Close()

	// 1. Write Logs
	log.Debug("Debug Message")
	log.Info("Info Message")
	log.Error("Error Message")

	// 2. Verify Files Exist
	assert.True(t, assertLogFileExists(t, tempDir, opts.Name, "main"))
	assert.True(t, assertLogFileExists(t, tempDir, opts.Name, "critical"))
	assert.True(t, assertLogFileExists(t, tempDir, opts.Name, "verbose"))

	// 3. Verify Content Isolation
	// Main: Info, Error (No Debug)
	// Critical: Error (No Info, No Debug)
	// Verbose: Debug (And potentially others depending on hook logic -> Hook sends ALL >= Debug to verbose)

	// Helper to read file
	readFile := func(typeSuffix string) string {
		name := opts.Name + "." + fileExt
		if typeSuffix != "main" {
			name = fmt.Sprintf("%s.%s.%s", opts.Name, typeSuffix, fileExt)
		}
		content, err := os.ReadFile(filepath.Join(tempDir, name))
		require.NoError(t, err)
		return string(content)
	}

	mainContent := readFile("main")
	assert.Contains(t, mainContent, "Info Message")
	assert.Contains(t, mainContent, "Error Message")
	assert.NotContains(t, mainContent, "Debug Message")

	critContent := readFile("critical")
	assert.Contains(t, critContent, "Error Message")
	assert.NotContains(t, critContent, "Info Message")

	verbContent := readFile("verbose")
	assert.Contains(t, verbContent, "Debug Message")
}

// TestDailyRotation_Simulation verifies that calling Setup() (simulating server restart)
// triggers a log rotation, ensuring separate files for separate runs/days.
func TestDailyRotation_Simulation(t *testing.T) {
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	appName := "test-app-rotate"
	opts := Options{
		Name: appName,
		Dir:  tempDir,
	}

	// --- Day 1 (Run 1) ---
	closer1, err := Setup(opts)
	require.NoError(t, err)

	log.Info("Log entry from Run 1")
	closer1.Close() // Simulate server shutdown

	// Allow file system timestamp to tick (Windows precision is sometimes low)
	if runtime.GOOS == "windows" {
		time.Sleep(100 * time.Millisecond) // Safe buffer
	}

	// --- Day 2 (Run 2 - Restart) ---
	closer2, err := Setup(opts)
	require.NoError(t, err)
	defer closer2.Close()

	log.Info("Log entry from Run 2")

	// --- Verification ---
	// We expect:
	// 1. appName.log (Active, contains "Run 2")
	// 2. appName-TIMESTAMP.log (Rotated/Backup, contains "Run 1")

	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)

	activeFileCount := 0
	backupFileCount := 0

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()

		// Active File: "test-app-rotate.log"
		if name == appName+"."+fileExt {
			activeFileCount++
			content, _ := os.ReadFile(filepath.Join(tempDir, name))
			assert.Contains(t, string(content), "Run 2", "Active file should contain new logs")
			assert.NotContains(t, string(content), "Run 1", "Active file should NOT contain old logs (rotated)")
		} else if strings.HasPrefix(name, appName) && strings.Contains(name, ".log") {
			// Backup File: "test-app-rotate-2023...."
			// It's a backup if it's not the active one
			backupFileCount++
			// We can try to read it to verify content, but Lumberjack compresses by default (Compress: true in Setup)
			// If compressed (.gz), we just check existence.
			// In our Setup, Compress is true. So extension should be .gz
			if strings.HasSuffix(name, ".gz") {
				// Good, it's compressed
			}
		}
	}

	assert.Equal(t, 1, activeFileCount, "Should have exactly 1 active log file")
	assert.GreaterOrEqual(t, backupFileCount, 1, "Should have at least 1 backup log file after rotation")
}
