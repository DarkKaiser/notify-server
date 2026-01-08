package log

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/natefinch/lumberjack.v2"
)

// =============================================================================
// Unit Tests (White-box & Logic)
// =============================================================================

// TestSetup_WhiteBox Setup이 내부적으로 lumberjack.Logger를 올바른 설정값으로 생성했는지
// Reflection을 사용하여 검증합니다 (Construction Logic Verify).
func TestSetup_WhiteBox(t *testing.T) {
	resetGlobalState()
	tempDir := t.TempDir()

	opts := Options{
		Name:       "wb-app",
		Dir:        tempDir,
		MaxSizeMB:  50,
		MaxBackups: 5,
		MaxAge:     1,
		// EnableCriticalLog, EnableVerboseLog 생략 -> Main Logger만 생성
	}

	cl, err := Setup(opts)
	require.NoError(t, err)
	defer cl.Close()

	// Internal state access (closer struct is private)
	// We cast to *closer (which is exported in the same package)
	c, ok := cl.(*closer)
	require.True(t, ok, "Returned closer must be of type *closer")

	// Verify Main Logger
	require.NotEmpty(t, c.closers)
	mainLogger, ok := c.closers[0].(*lumberjack.Logger)
	require.True(t, ok, "First closer must be lumberjack.Logger (Main)")

	assert.Equal(t, 50, mainLogger.MaxSize)
	assert.Equal(t, 5, mainLogger.MaxBackups)
	assert.Equal(t, 1, mainLogger.MaxAge)
	assert.True(t, mainLogger.LocalTime, "LocalTime should be enabled")
	assert.Equal(t, filepath.Join(tempDir, "wb-app.log"), mainLogger.Filename)
}

// TestSetup_Singleton_Error Setup 최초 호출 시 에러가 발생하면,
// 이후 호출에서도 동일한 에러를 반환해야 함을 검증합니다 (싱글톤 보장).
func TestSetup_Singleton_Error(t *testing.T) {
	resetGlobalState()

	// 1. First Call: Fail intentionally
	// Name이 없으면 Validate에서 실패함
	badOpts := Options{Dir: "somewhere"}
	_, err1 := Setup(badOpts)
	require.Error(t, err1)

	// 2. Second Call: Provide VALID options, but it should still fail
	// because Setup is sync.Once protected and already "executed" (failed).
	goodOpts := Options{Name: "valid-app"}
	_, err2 := Setup(goodOpts)

	require.Error(t, err2)
	assert.Equal(t, err1, err2, "Setup must return the cached error from the first attempt")
}

// TestSetup_Defaults 필수값이 누락되었을 때 기본값이 올바르게 적용되는지 검증합니다.
func TestSetup_Defaults(t *testing.T) {
	resetGlobalState()
	tempDir := t.TempDir()

	opts := Options{
		Name: "defaults-app",
		Dir:  tempDir,
		// Level, MaxSizeMB, MaxBackups 생략 -> 기본값 적용 기대
	}

	cl, err := Setup(opts)
	require.NoError(t, err)
	defer cl.Close()

	// 1. Log Level Default (InfoLevel)
	assert.Equal(t, logrus.InfoLevel, logrus.GetLevel(), "기본 로그 레벨은 Info여야 합니다")

	// 2. Rotation Defaults
	c, ok := cl.(*closer)
	require.True(t, ok)
	mainLogger, _ := c.closers[0].(*lumberjack.Logger)

	assert.Equal(t, defaultMaxSizeMB, mainLogger.MaxSize)
	assert.Equal(t, defaultMaxBackups, mainLogger.MaxBackups)
}

// =============================================================================
// Integration Tests (File System & Concurrency)
// =============================================================================

// TestSetup_Basic Setup 함수가 로그 파일을 올바르게 생성하는지 검증합니다.
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

	// Lazy Creation 테스트: 로그를 기록해야 파일이 생성됨
	WithFields(Fields{"foo": "bar"}).Info("Hello World")

	assert.True(t, assertLogFileExists(t, tempDir, opts.Name, "main"))
}

// TestSetup_Concurrency 멀티 고루틴 환경에서 Setup이 안전하게 한 번만 실행되는지 검증합니다.
func TestSetup_Concurrency(t *testing.T) {
	resetGlobalState()
	tempDir := t.TempDir()

	concurrency := 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	results := make([]error, concurrency)
	closers := make([]interface{}, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			// 약간의 딜레이를 주어 경합 유도
			if idx%2 == 0 {
				time.Sleep(1 * time.Millisecond)
			}
			c, err := Setup(Options{
				Name: "concurrent-app",
				Dir:  tempDir,
			})
			results[idx] = err
			closers[idx] = c
		}(i)
	}

	wg.Wait()

	// 모든 고루틴이 에러 없이 성공하고, 동일한 Closer 인스턴스를 반환해야 함
	var firstCloser interface{}
	for i := 0; i < concurrency; i++ {
		require.NoError(t, results[i])
		if firstCloser == nil {
			firstCloser = closers[i]
		} else {
			assert.Same(t, firstCloser, closers[i], "모든 Setup 호출은 동일한 인스턴스를 반환해야 합니다")
		}
	}

	// Cleanup
	if c, ok := firstCloser.(*closer); ok {
		c.Close()
	}
}

// TestSetup_ReportCaller 호출자 정보 기록 및 경로 프리픽스 단축 기능을 검증합니다.
func TestSetup_ReportCaller(t *testing.T) {
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	_, currentFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(currentFile))) // .../notify-server

	opts := Options{
		Name:             "caller-app",
		Dir:              tempDir,
		ReportCaller:     true,
		CallerPathPrefix: projectRoot, // 프로젝트 루트 경로를 Prefix로 설정
	}

	closer, err := Setup(opts)
	require.NoError(t, err)
	defer closer.Close()

	Info("Test Message")

	content := readLogFile(t, tempDir, "caller-app.log")

	// 검증:
	// 1. 함수명 포함
	// 2. 파일 경로가 단축되어야 함 (Prefix 제거 확인)
	// 주의: `go test` 실행 방식(패키지 vs 파일 목록)에 따라 패키지명이 달라질 수 있음.
	// 예: `pkg/log.Test...` vs `command-line-arguments.Test...`
	// 따라서, "TestSetup_ReportCaller"가 포함되어 있고, Prefix로 설정한 프로젝트 루트 경로가 제거되었는지 확인하는 것으로 충분함.
	assert.Contains(t, content, "TestSetup_ReportCaller", "함수명이 포함되어야 합니다")
	assert.NotContains(t, content, projectRoot, "프로젝트 루트 경로(Prefix)는 제거되어야 합니다")
}

// TestLogLevelSeparation 로깅 레벨에 따라 올바르게 파일이 분리되는지 검증합니다.
func TestLogLevelSeparation(t *testing.T) {
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	opts := Options{
		Name:              "test-app-levels",
		Dir:               tempDir,
		EnableCriticalLog: true,
		EnableVerboseLog:  true,
		Level:             TraceLevel,
	}

	closer, err := Setup(opts)
	require.NoError(t, err)
	defer closer.Close()

	WithFields(Fields{"level": "debug"}).Debug("Debug Message")
	WithFields(Fields{"level": "info"}).Info("Info Message")
	WithFields(Fields{"level": "error"}).Error("Error Message")

	// Main: Info, Error (No Debug)
	mainContent := readLogFile(t, tempDir, "test-app-levels.log")
	assert.Contains(t, mainContent, "Info Message")
	assert.Contains(t, mainContent, "Error Message")
	assert.NotContains(t, mainContent, "Debug Message")

	// Critical: Error Only
	critContent := readLogFile(t, tempDir, "test-app-levels.critical.log")
	assert.Contains(t, critContent, "Error Message")
	assert.NotContains(t, critContent, "Info Message")

	// Verbose: Debug Only (setup.go/hook.go 정책)
	verbContent := readLogFile(t, tempDir, "test-app-levels.verbose.log")
	assert.Contains(t, verbContent, "Debug Message")
}

// TestRestart_AppendsLog 재시작(Setup 재호출) 시 로그 파일이 초기화되지 않고 이어지는지 검증합니다.
func TestRestart_AppendsLog(t *testing.T) {
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	appName := "test-app-restart"
	opts := Options{Name: appName, Dir: tempDir}

	// Run 1
	closer1, err := Setup(opts)
	require.NoError(t, err)
	Info("Run 1 Log")
	closer1.Close()

	// Windows 파일 락 해제 대기
	if runtime.GOOS == "windows" {
		time.Sleep(50 * time.Millisecond)
	}
	resetGlobalState()

	// Run 2
	closer2, err := Setup(opts)
	require.NoError(t, err)
	defer closer2.Close()
	Info("Run 2 Log")

	// Verify
	content := readLogFile(t, tempDir, appName+".log")
	assert.Contains(t, content, "Run 1 Log")
	assert.Contains(t, content, "Run 2 Log")
}

// TestFatalExit verifies that logs are flushed to disk before exiting on Fatal.
func TestFatalExit(t *testing.T) {
	// 서브 프로세스에서 실행하여 Fatal 종료를 감지
	if os.Getenv("TEST_FATAL_CRASH") == "1" {
		tempDir := os.Getenv("TEST_LOG_DIR")
		opts := Options{
			Name:              "fatal-app",
			Dir:               tempDir,
			EnableCriticalLog: true,
		}
		if _, err := Setup(opts); err != nil {
			panic(err)
		}
		// Fatal 호출
		Fatal("This is a fatal message")
		return
	}

	t.Parallel()
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	exe, err := os.Executable()
	require.NoError(t, err)

	cmd := exec.Command(exe, "-test.run=TestFatalExit")
	cmd.Env = append(os.Environ(), "TEST_FATAL_CRASH=1", "TEST_LOG_DIR="+tempDir)

	// Run and expect failure (exit status 1)
	err = cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		assert.False(t, exitErr.Success(), "Process should exit with non-zero status")
	} else {
		// Logrus Fatal might call os.Exit(1) which cmd.Run returns as error.
		// If no error, it means it didn't exit?? Fatal MUST exit.
		// If it's not ExitError, it might be something else, but we expect ExitError.
		// However, assert.Error is better here.
	}

	// Verify Log Content
	files, _ := os.ReadDir(tempDir)
	found := false
	for _, f := range files {
		if strings.Contains(f.Name(), "fatal-app.critical") {
			content, _ := os.ReadFile(filepath.Join(tempDir, f.Name()))
			if strings.Contains(string(content), "This is a fatal message") {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "Fatal log message should be persisted in critical log file")
}

// =============================================================================
// Helpers
// =============================================================================

func setupLogTest(t *testing.T) (string, func()) {
	t.Helper()
	resetGlobalState()
	tempDir := t.TempDir()
	return tempDir, func() {
		resetGlobalState()
	}
}

func readLogFile(t *testing.T, dir, filename string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(t, err)
	return string(content)
}

func assertLogFileExists(t *testing.T, logDir, appName, expectedType string) bool {
	t.Helper()
	files, err := os.ReadDir(logDir)
	if err != nil {
		return false
	}
	for _, file := range files {
		name := file.Name()
		if !strings.HasPrefix(name, appName) {
			continue
		}
		if expectedType == "main" {
			if name == appName+"."+fileExt { // "app.log"
				return true
			}
		} else {
			// e.g. "app.critical.log"
			if strings.Contains(name, "."+expectedType+".") {
				return true
			}
		}
	}
	return false
}

func resetGlobalState() {
	setupOnce = sync.Once{}
	globalCloser = nil
	globalSetupErr = nil

	// Logrus Reset
	logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetReportCaller(false)
	logrus.SetFormatter(&logrus.TextFormatter{})
}
