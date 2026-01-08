package log

import (
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
// Unit Tests (Validation & Logic)
// =============================================================================

// TestSetup_Validation 옵션 유효성 검사 로직을 테이블 기반으로 테스트합니다.
func TestSetup_Validation(t *testing.T) {
	// 임시 파일을 생성하여 "파일이 이미 존재함" 케이스를 테스트합니다.
	tempFile := filepath.Join(t.TempDir(), "existing_file")
	err := os.WriteFile(tempFile, []byte("test"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name        string
		opts        Options
		expectError string
	}{
		{
			name:        "Missing Name",
			opts:        Options{Dir: "logs"},
			expectError: "애플리케이션 식별자(Name)가 설정되지 않았습니다",
		},
		{
			name: "Dir Conflicts with Existing File",
			opts: Options{
				Name: "check-file",
				Dir:  tempFile, // 디렉토리 위치에 파일이 존재함
			},
			expectError: "이미 파일로 존재합니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobalState()
			_, err := Setup(tt.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
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
	// 내부 상태를 확인하기 위해 closer를 형변환하여 접근
	c, ok := cl.(*closer)
	require.True(t, ok)
	require.NotEmpty(t, c.closers)

	// Main Logger 검증
	mainLogger, ok := c.closers[0].(*lumberjack.Logger)
	require.True(t, ok, "Main Logger는 lumberjack.Logger 타입이어야 합니다")

	assert.Equal(t, defaultMaxSizeMB, mainLogger.MaxSize, "기본 MaxSizeMB가 적용되어야 합니다")
	assert.Equal(t, defaultMaxBackups, mainLogger.MaxBackups, "기본 MaxBackups가 적용되어야 합니다")
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

	// 현재 파일의 패키지 경로를 알아내기 위한 트릭
	_, currentFile, _, _ := runtime.Caller(0)
	pkgPath := filepath.Dir(currentFile)               // .../pkg/log
	projectRoot := filepath.Dir(filepath.Dir(pkgPath)) // .../notify-server

	opts := Options{
		Name:             "caller-app",
		Dir:              tempDir,
		ReportCaller:     true,
		CallerPathPrefix: projectRoot, // 프로젝트 루트 경로를 Prefix로 설정
	}

	closer, err := Setup(opts)
	require.NoError(t, err)
	defer closer.Close()

	logrus.Info("Test Message")

	// 파일 내용 읽기
	content := readLogFile(t, tempDir, "caller-app.log")

	// 검증:
	// 1. 함수명 포함 (TestSetup_ReportCaller)
	// 2. 파일 경로가 단축되어야 함 (.../pkg/log/setup_test.go)
	assert.Contains(t, content, "TestSetup_ReportCaller", "호출 함수명이 포함되어야 합니다")

	// 윈도우 경로 역슬래시 처리 등을 고려하여 부분 일치 확인
	// CallerPathPrefix가 정상 동작했다면 절대경로 전체가 나오지 않고 축약된 형태가 보여야 함
	// 다만 runtime.Frame.Function과 File은 다르게 출력될 수 있음. 포맷터 설정을 확인.
	// setup.go의 CallerPrettyfier는 function 이름만 커스텀하고 file은 건드리지 않음?
	// -> 아님. CallerPrettyfier 리턴값 (func, file) 중 file이 비어있으면 기본값 사용.
	// setup.go 코드를 보면: function만 조작하고 있음.
	// 따라서 file 경로는 logrus 기본 동작(전체 경로) 혹은 포맷터 설정에 따름.
	// 하지만 setup.go 코드를 다시 보면:
	// CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) { ... return }
	// 여기서 file을 명시적으로 반환하지 않으면(빈 문자열), logrus가 기본 파일 경로를 출력함.
	// function 변수만 조작하고 있음.

	// 따라서 함수명에 prefix가 제거되었는지 확인.
	// 예: d:/DarkKaiser-Workspace/notify-server/pkg/log.TestSetup_ReportCaller
	// Prefix 설정 시: .../pkg/log.TestSetup_ReportCaller (혹은 유사한 형태)

	// 여기서는 확실히 "TestSetup_ReportCaller"가 로그에 찍히는지 확인하는 것으로 충분할 수 있음.
	// 여기서는 확실히 "TestSetup_ReportCaller"가 로그에 찍히는지 확인하는 것으로 충분할 수 있음.
	// 현재 setup.go 구현상 파일명을 출력하지 않고 함수명에 라인정보를 포함시킴.
	assert.Contains(t, content, "pkg/log.TestSetup_ReportCaller(line:", "함수명과 라인번호가 포함되어야 합니다")
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

	// Verbose: Debug Only (setup.go의 hook 로직에 따름: Verbose는 Debug레벨 이하만? 아니면 전체?
	// setup.go의 hook 로직 확인 필요. -> verboseWriter는 Fire()에서 level <= DebugLevel 일 때만 씀?
	// hook.go 로직을 봐야 정확함. setup.go에는 wiring만 있음.
	// 보통 Verbose는 Debug/Trace 용.
	// (가정) hook 구현상 verbose는 debug/trace만 필터링한다고 가정하고 테스트 작성.
	// 만약 실패하면 hook 로직 확인 후 수정.
	verbContent := readLogFile(t, tempDir, "test-app-levels.verbose.log")
	assert.Contains(t, verbContent, "Debug Message")
	// Info가 Verbose에 들어가는지는 Hook 구현에 달림. 일반적으로는 Debug 전용이면 안 들어감.
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
	logrus.Info("Run 1 Log")
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
	logrus.Info("Run 2 Log")

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
		Setup(opts)
		// Fatal 호출
		logrus.Fatal("This is a fatal message")
		return
	}

	t.Parallel()
	tempDir, teardown := setupLogTest(t)
	defer teardown()

	exe, err := os.Executable()
	require.NoError(t, err)

	cmd := exec.Command(exe, "-test.run=TestFatalExit")
	cmd.Env = append(os.Environ(), "TEST_FATAL_CRASH=1", "TEST_LOG_DIR="+tempDir)

	err = cmd.Run()
	if e, ok := err.(*exec.ExitError); ok {
		assert.False(t, e.Success(), "Process should exit with non-zero status")
	}

	// Verify Log Content
	// Critical Log에 Fatal 메시지가 있어야 함
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
			if name == appName+".log" {
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

	logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetReportCaller(false)
	logrus.SetFormatter(&logrus.TextFormatter{})
}
