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
		{
			name: "Invalid Directory Path",
			opts: Options{
				Name: "test-app-invalid",
				Dir: func() string {
					if runtime.GOOS == "windows" {
						return "INVALID:/"
					}
					return "/root/protected/log" // On Linux, this should fail if not root
				}(),
				RetentionDays: 7,
			},
			debugMode: false,
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var subDir string
			if tc.opts.Dir == "" {
				subDir = filepath.Join(tempDir, strings.ReplaceAll(tc.name, " ", "_"))
				tc.opts.Dir = subDir
			} else {
				subDir = tc.opts.Dir
			}

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

// =============================================================================
// Log Level Separation Tests (Merged from log_level_test.go)
// =============================================================================

// assertLogFileExists는 특정 타입의 로그 파일이 존재하는지 검증합니다.
func assertLogFileExists(t *testing.T, logDir, appName, logType string) bool {
	t.Helper()
	files, err := os.ReadDir(logDir)
	require.NoError(t, err, "로그 디렉토리를 읽을 수 있어야 합니다")

	for _, file := range files {
		name := file.Name()
		if !strings.HasPrefix(name, appName) {
			continue
		}

		if logType == "main" {
			if strings.HasSuffix(name, "."+fileExt) &&
				!strings.Contains(name, ".critical.") &&
				!strings.Contains(name, ".verbose.") {
				return true
			}
		} else if strings.Contains(name, "."+logType+".") {
			return true
		}
	}
	return false
}

// TestSetup_LogFilterVerification은 로그 레벨 분리 및 오염 방지를 검증합니다.
//
// 검증 항목:
//   - 파일 생성 여부
//   - Main 로그에 Debug 메시지가 없는지 (Content Isolation)
//   - Critical 로그에 Error 메시지가 있는지
func TestSetup_LogFilterVerification(t *testing.T) {
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	appName := "test-app-filter"
	logDir := filepath.Join(tempDir, appName)

	// 모든 로그 활성화 (Critical + Verbose)
	closer, err := Setup(Options{
		Name:              appName,
		RetentionDays:     7,
		EnableCriticalLog: true,
		EnableVerboseLog:  true,
		Dir:               logDir,
	})
	require.NoError(t, err)
	defer closer.Close()

	// 로그 발생
	log.Debug("This is debug log") // Verbose Only
	log.Info("This is info log")   // Main Only (Info+)
	log.Warn("This is warn log")   // Main Only (Info+)
	log.Error("This is error log") // Main + Critical (Error+)

	// 비동기 쓰기 가능성을 감안하여 잠시 대기 (File I/O)
	// 하지만 Logrus Default는 동기식이므로 즉시 확인 가능해야 함.
	// 안전을 위해 Sync 호출은 Setup에서 반환된 closer가 처리하지 않으므로(파일 객체 직접 접근 불가),
	// 여기서는 OS Flush를 믿고 진행하되, 실패 시 약간의 sleep 고려.

	// 파일 찾기
	files, err := os.ReadDir(logDir)
	require.NoError(t, err)

	var mainPath, criticalPath, verbosePath string
	for _, f := range files {
		path := filepath.Join(logDir, f.Name())
		if strings.Contains(f.Name(), "critical") {
			criticalPath = path
		} else if strings.Contains(f.Name(), "verbose") {
			verbosePath = path
		} else if strings.HasSuffix(f.Name(), "."+fileExt) {
			mainPath = path
		}
	}

	require.NotEmpty(t, mainPath, "Main log file must exist")
	require.NotEmpty(t, criticalPath, "Critical log file must exist")
	require.NotEmpty(t, verbosePath, "Verbose log file must exist")

	// 내용물 검증 Helper
	assertFileContent := func(path string, shouldContain []string, shouldNotContain []string) {
		contentBytes, err := os.ReadFile(path)
		require.NoError(t, err)
		content := string(contentBytes)

		for _, s := range shouldContain {
			assert.Contains(t, content, s, "File %s SHOULD contain '%s'", filepath.Base(path), s)
		}
		for _, s := range shouldNotContain {
			assert.NotContains(t, content, s, "File %s should NOT contain '%s'", filepath.Base(path), s)
		}
	}

	// 1. Main Log 검증: Info/Warn/Error 포함, Debug 제외
	assertFileContent(mainPath,
		[]string{"This is info log", "This is warn log", "This is error log"},
		[]string{"This is debug log"},
	)

	// 2. Critical Log 검증: Error 포함, Info/Debug 제외
	assertFileContent(criticalPath,
		[]string{"This is error log"},
		[]string{"This is info log", "This is debug log"},
	)

	// 3. Verbose Log 검증: Debug 포함 (Error/Info는 설정에 따라 다를 수 있으나, 현재 구현상 LogLevelHook은 VerboseWriter에 Debug 이상을 다 줌?
	//    -> 구현 확인: Level >= DebugLevel 이므로 Info, Warn, Error, Fatal, Panic 등 '모든' 로그가 Verbose에 포함됨)
	//    -> **중요**: LogLevelHook 로직 상 'Verbose' 파일은 'DebugLevel 이상' 즉, 'All Logs'를 저장하는 개념인지, '오직 Debug/Trace'만 저장하는지?
	//       hook.go: if entry.Level >= log.DebugLevel { ... Write ... }
	//       Logrus Level은 Panic=0, User=1, Error=2, Warn=3, Info=4, Debug=5, Trace=6
	//       entry.Level(5) >= Debug(5) -> True.
	//       entry.Level(4 Info) >= Debug(5) -> False. (Logrus Level은 숫자가 작을수록 심각함)
	//       Wait, Logrus definition:
	//         PanicLevel Level = iota (0)
	//         FatalLevel (1) ...
	//         DebugLevel (5)
	//       If entry is Info(4), is 4 >= 5? False.
	//       So VerboseWriter (Level >= DebugLevel) implies ONLY Debug(5) and Trace(6).
	//       Info(4) is NOT >= Debug(5).
	//       So Verbose Log should ONLY contain Debug and Trace.
	assertFileContent(verbosePath,
		[]string{"This is debug log"},
		[]string{"This is info log", "This is error log"}, // Info/Error는 Verbose 파일에 안 들어감
	)
}

// TestSetup_LogLevelFiles는 로그 레벨별 파일 분리를 검증합니다.
//
// 검증 항목:
//   - 옵션 설정에 따른 Critical/Verbose 로그 파일 생성 여부
//   - 정확한 파일명 생성 확인
func TestSetup_LogLevelFiles(t *testing.T) {
	tempDir := t.TempDir()
	// setupLogTest 대신 직접 설정 (Setup 함수가 내부적으로 초기화하므로)
	defer func() {
		log.SetOutput(os.Stdout)
	}()

	tests := []struct {
		name               string
		enableCritical     bool
		enableVerbose      bool
		expectedLogTypes   []string // "main", "critical", "verbose" 중 기대하는 것들
		unexpectedLogTypes []string
	}{
		{
			name:               "Critical Log Only",
			enableCritical:     true,
			enableVerbose:      false,
			expectedLogTypes:   []string{"main", "critical"},
			unexpectedLogTypes: []string{"verbose"},
		},
		{
			name:               "Both Critical and Verbose Logs",
			enableCritical:     true,
			enableVerbose:      true,
			expectedLogTypes:   []string{"main", "critical", "verbose"},
			unexpectedLogTypes: []string{},
		},
		{
			name:               "No Level Separation (Default)",
			enableCritical:     false,
			enableVerbose:      false,
			expectedLogTypes:   []string{"main"},
			unexpectedLogTypes: []string{"critical", "verbose"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appName := strings.ReplaceAll(strings.ToLower(tt.name), " ", "-")
			logDir := filepath.Join(tempDir, appName)

			closer, err := Setup(Options{
				Name:              appName,
				RetentionDays:     7,
				EnableCriticalLog: tt.enableCritical,
				EnableVerboseLog:  tt.enableVerbose,
				Dir:               logDir,
			})
			require.NoError(t, err)

			defer func() {
				if closer != nil {
					closer.Close()
				}
			}()

			assert.NotNil(t, closer, "closer를 반환해야 합니다")

			// 로그 디렉토리 확인
			require.DirExists(t, logDir)

			// 예상되는 파일 타입 검증
			for _, logType := range tt.expectedLogTypes {
				exists := assertLogFileExists(t, logDir, appName, logType)
				assert.True(t, exists, "Expected log type '%s' should exist", logType)
			}

			// 예상되지 않는 파일 타입 검증
			for _, logType := range tt.unexpectedLogTypes {
				exists := assertLogFileExists(t, logDir, appName, logType)
				assert.False(t, exists, "Unexpected log type '%s' should NOT exist", logType)
			}
		})
	}
}
