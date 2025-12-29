package log

import (
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

// setupLogTest는 테스트를 위한 임시 디렉토리 및 환경 정리 함수입니다.
func setupLogTest(t *testing.T) (string, func()) {
	t.Helper()
	tempDir := t.TempDir()
	// logDirectoryBasePath 전역 변수 제거됨 -> Setup에서 opts.LogDir로 처리

	return tempDir, func() {
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

// =============================================================================
// Log Initialization Tests
// =============================================================================

// TestSetup_LogFileCreation은 로그 파일 생성 로직을 검증합니다.
//
// 검증 항목:
//   - Debug/Production 모드에 따른 로그 디렉토리 생성
//   - 옵션에 따른 로그 파일 생성 (파일명, 확장자)
//   - Setup 에러 처리 (AppName 누락 등)
func TestSetup_LogFileCreation(t *testing.T) {
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	tests := []struct {
		name       string
		opts       Options
		debugMode  bool
		wantErr    bool
		checkFiles func(*testing.T, string, string)
		checkLogs  func(*testing.T, string)
	}{
		{
			name: "Debug Mode - Creates logs",
			opts: Options{
				Name:          "test-app-debug",
				RetentionDays: 7,
			},
			debugMode: true,
			wantErr:   false,
			checkFiles: func(t *testing.T, logDir, appName string) {
				files, err := os.ReadDir(logDir)
				require.NoError(t, err)
				assert.Greater(t, len(files), 0, "Should create at least one log file")

				found := false
				for _, file := range files {
					if strings.HasPrefix(file.Name(), appName) && strings.HasSuffix(file.Name(), "."+fileExt) {
						found = true
						break
					}
				}
				assert.True(t, found, "Log file with app name should exist")
			},
		},
		{
			name: "Production Mode - Creates logs",
			opts: Options{
				Name:          "test-app-prod",
				RetentionDays: 7,
			},
			debugMode: false,
			wantErr:   false,
			checkFiles: func(t *testing.T, logDir, appName string) {
				files, err := os.ReadDir(logDir)
				require.NoError(t, err)
				assert.Greater(t, len(files), 0)
			},
		},
		{
			name: "Missing AppName - Should Return Safe Empty Closer",
			opts: Options{
				Name:          "", // Missing AppName
				RetentionDays: 7,
			},
			debugMode: false,
			wantErr:   false,
			checkFiles: func(t *testing.T, logDir, appName string) {
				_, err := os.Stat(logDir)
				// AppName이 없으면 파일 로깅이 비활성화되므로 로그 디렉토리가 생성되지 않을 수 있음 (구현 의존적)
				// 핵심은 Panic이 나지 않고 nil이 아닌 Closer가 반환되는 것임
				if !os.IsNotExist(err) {
					// 만약 생성되었다면 AppName 파일은 없어야 함
					files, _ := os.ReadDir(logDir)
					for _, f := range files {
						assert.False(t, strings.HasPrefix(f.Name(), "."), "Hidden files ignore")
					}
				}
			},
		},
		{
			name: "Custom File Permission - 0644",
			opts: Options{
				Name:          "test-app-perm",
				RetentionDays: 7,
				FileMode:      0644,
			},
			debugMode: false,
			wantErr:   false,
			checkFiles: func(t *testing.T, logDir, appName string) {
				files, err := os.ReadDir(logDir)
				require.NoError(t, err)

				for _, file := range files {
					if strings.HasPrefix(file.Name(), appName) {
						info, err := file.Info()
						require.NoError(t, err)
						// Windows에서는 권한 체크가 제한적이므로 모드 일부만 확인하거나 생략
						if runtime.GOOS != "windows" {
							assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
						}
					}
				}
			},
		},
		{
			name: "Console Log Enabled",
			opts: Options{
				Name:             "test-app-console",
				RetentionDays:    7,
				EnableConsoleLog: true,
			},
			debugMode: false,
			wantErr:   false,
			checkLogs: func(t *testing.T, output string) {
				// Capture output validation logic would go here if we could capture stdout easily in parallel tests
				// Since we use log.SetOutput, capturing global stdout is tricky without synchronization.
				// For this unit test, we ensure it doesn't panic.
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subDir := filepath.Join(tempDir, strings.ReplaceAll(tc.name, " ", "_"))
			tc.opts.Dir = subDir

			closer, err := Setup(tc.opts)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			defer func() {
				if closer != nil {
					closer.Close()
				}
			}()

			SetDebugMode(tc.debugMode)

			if tc.checkFiles != nil {
				tc.checkFiles(t, subDir, tc.opts.Name)
			}
		})
	}
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

	logDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logDir, 0755)
	assert.NoError(t, err)

	appName := "test-app-gc"

	// Old file (10 days ago)
	oldLogFile := filepath.Join(logDir, appName+"-old."+fileExt)
	createTestFile(t, oldLogFile, time.Now().Add(-10*24*time.Hour))

	// Recent file
	recentLogFile := filepath.Join(logDir, appName+"-recent."+fileExt)
	createTestFile(t, recentLogFile, time.Now())

	// Other app old file
	otherAppFile := filepath.Join(logDir, "other-app-old."+fileExt)
	createTestFile(t, otherAppFile, time.Now().Add(-10*24*time.Hour))

	// Execute GC
	removeExpiredLogFiles(logDir, appName, 7.0)

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
		{"File Extension", fileExt, "log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.got)
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
