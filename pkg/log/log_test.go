package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// setupLogTest 테스트를 위한 임시 디렉토리 및 환경 정리 함수
func setupLogTest(t *testing.T) (string, func()) {
	t.Helper()
	tempDir := t.TempDir()
	originalLogDirParentPath := logDirectoryBasePath
	logDirectoryBasePath = tempDir + string(os.PathSeparator)

	return tempDir, func() {
		logDirectoryBasePath = originalLogDirParentPath
		log.SetOutput(os.Stdout)
	}
}

func TestInit_DebugMode(t *testing.T) {
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	appName := "test-app-debug"
	closer := InitFile(appName, 7.0)
	defer func() {
		if closer != nil {
			closer.Close()
		}
	}()

	SetDebugMode(true)

	assert.NotNil(t, closer, "Should return closer")

	// Verify log directory creation
	logDir := filepath.Join(tempDir, defaultLogDirectoryName)
	_, err := os.Stat(logDir)
	assert.NoError(t, err, "Log directory should exist")

	// Verify log file creation
	files, err := os.ReadDir(logDir)
	assert.NoError(t, err)
	assert.Greater(t, len(files), 0, "Should create at least one log file")
}

func TestInit_ProductionMode(t *testing.T) {
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	appName := "test-app-prod"
	closer := InitFile(appName, 7.0)
	defer func() {
		if closer != nil {
			closer.Close()
		}
	}()

	SetDebugMode(false)

	assert.NotNil(t, closer)

	logDir := filepath.Join(tempDir, defaultLogDirectoryName)
	_, err := os.Stat(logDir)
	assert.NoError(t, err)

	files, err := os.ReadDir(logDir)
	assert.NoError(t, err)
	assert.Greater(t, len(files), 0)

	found := false
	for _, file := range files {
		if strings.HasPrefix(file.Name(), appName) && strings.HasSuffix(file.Name(), "."+defaultLogFileExtension) {
			found = true
			break
		}
	}
	assert.True(t, found, "Log file with app name should exist")
}

func TestCleanOutOfLogFiles(t *testing.T) {
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	logDir := filepath.Join(tempDir, defaultLogDirectoryName)
	err := os.MkdirAll(logDir, 0755)
	assert.NoError(t, err)

	appName := "test-app-gc"

	// Old file (10 days ago)
	oldLogFile := filepath.Join(logDir, appName+"-old."+defaultLogFileExtension)
	createTestFile(t, oldLogFile, time.Now().Add(-10*24*time.Hour))

	// Recent file
	recentLogFile := filepath.Join(logDir, appName+"-recent."+defaultLogFileExtension)
	createTestFile(t, recentLogFile, time.Now())

	// Other app old file
	otherAppFile := filepath.Join(logDir, "other-app-old."+defaultLogFileExtension)
	createTestFile(t, otherAppFile, time.Now().Add(-10*24*time.Hour))

	// Execute GC
	removeExpiredLogFiles(appName, 7.0)

	// Verifications
	assert.NoFileExists(t, oldLogFile, "Expired file should be deleted")
	assert.FileExists(t, recentLogFile, "Recent file should exist")
	assert.FileExists(t, otherAppFile, "Other app file should exist")
}

func createTestFile(t *testing.T, path string, modTime time.Time) {
	f, err := os.Create(path)
	assert.NoError(t, err)
	f.Close()
	err = os.Chtimes(path, modTime, modTime)
	assert.NoError(t, err)
}

func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"File Extension", defaultLogFileExtension, "log"},
		{"Directory Name", defaultLogDirectoryName, "logs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.got)
		})
	}
}

func TestSetCallerPathPrefix(t *testing.T) {
	originalPrefix := callerFunctionPathPrefix
	defer func() { callerFunctionPathPrefix = originalPrefix }()

	tests := []struct {
		input string
	}{
		{"github.com/my/project"},
		{""},
		{"custom/path"},
	}

	for _, tt := range tests {
		t.Run("Set "+tt.input, func(t *testing.T) {
			SetCallerPathPrefix(tt.input)
			assert.Equal(t, tt.input, callerFunctionPathPrefix)
		})
	}
}

func TestWithComponentFields(t *testing.T) {
	tests := []struct {
		name           string
		component      string
		fields         log.Fields
		expectedFields map[string]interface{}
	}{
		{
			name:      "Only Component",
			component: "test-c",
			fields:    nil,
			expectedFields: map[string]interface{}{
				"component": "test-c",
			},
		},
		{
			name:      "Component and Fields",
			component: "test-c-f",
			fields: log.Fields{
				"key1": "val1",
				"key2": 100,
			},
			expectedFields: map[string]interface{}{
				"component": "test-c-f",
				"key1":      "val1",
				"key2":      100,
			},
		},
		{
			name:      "Empty Component",
			component: "",
			fields: log.Fields{
				"k": "v",
			},
			expectedFields: map[string]interface{}{
				"component": "",
				"k":         "v",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entry *log.Entry
			if tt.fields == nil {
				entry = WithComponent(tt.component)
			} else {
				entry = WithComponentAndFields(tt.component, tt.fields)
			}

			assert.NotNil(t, entry)
			for k, v := range tt.expectedFields {
				assert.Equal(t, v, entry.Data[k])
			}
		})
	}
}
