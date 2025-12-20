package log

import (
	"os"
	"path/filepath"
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

// setupLogTest는 테스트를 위한 임시 디렉토리 및 환경 정리 함수입니다.
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

// createTestFile은 테스트용 파일을 생성하고 수정 시간을 설정합니다.
func createTestFile(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err, "Should create test file")
	f.Close()
	err = os.Chtimes(path, modTime, modTime)
	require.NoError(t, err, "Should set file modification time")
}

// =============================================================================
// Log Initialization Tests
// =============================================================================

// TestInit_DebugMode는 Debug 모드에서 로그 초기화를 검증합니다.
//
// 검증 항목:
//   - 로그 디렉토리 생성
//   - 로그 파일 생성
//   - Closer 반환
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

// TestInit_ProductionMode는 Production 모드에서 로그 초기화를 검증합니다.
//
// 검증 항목:
//   - 로그 디렉토리 생성
//   - 앱 이름이 포함된 로그 파일 생성
//   - Closer 반환
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

// =============================================================================
// Log File Cleanup Tests
// =============================================================================

// TestCleanOutOfLogFiles는 만료된 로그 파일 정리를 검증합니다.
//
// 검증 항목:
//   - 만료된 로그 파일 삭제
//   - 최근 로그 파일 유지
//   - 다른 앱의 로그 파일 유지
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

// =============================================================================
// Constants Tests
// =============================================================================

// TestConstants는 로그 관련 상수 값을 검증합니다.
//
// 검증 항목:
//   - 로그 파일 확장자
//   - 로그 디렉토리 이름
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

// =============================================================================
// Caller Path Prefix Tests
// =============================================================================

// TestSetCallerPathPrefix는 호출자 경로 prefix 설정을 검증합니다.
//
// 검증 항목:
//   - 다양한 prefix 값 설정
//   - 빈 문자열 설정
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

// =============================================================================
// Component Field Tests
// =============================================================================

// TestWithComponentFields는 component 필드 추가 함수를 검증합니다.
//
// 검증 항목:
//   - Component만 추가
//   - Component와 추가 필드 함께 추가
//   - 빈 Component 처리
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
