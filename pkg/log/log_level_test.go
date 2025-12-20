package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupLogLevelTest는 로그 레벨 테스트를 위한 임시 디렉토리 및 환경 정리 함수입니다.
func setupLogLevelTest(t *testing.T) (string, func()) {
	t.Helper()
	tempDir := t.TempDir()
	originalLogDirBasePath := logDirectoryBasePath
	logDirectoryBasePath = tempDir + string(os.PathSeparator)

	return tempDir, func() {
		logDirectoryBasePath = originalLogDirBasePath
		log.SetOutput(os.Stdout)
	}
}

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

		switch logType {
		case "main":
			if strings.HasSuffix(name, "."+defaultLogFileExtension) &&
				!strings.Contains(name, ".critical.") &&
				!strings.Contains(name, ".verbose.") {
				return true
			}
		case "critical":
			if strings.Contains(name, ".critical.") {
				return true
			}
		case "verbose":
			if strings.Contains(name, ".verbose.") {
				return true
			}
		}
	}
	return false
}

// =============================================================================
// Log Level Separation Tests
// =============================================================================

// TestInitFileWithOptions_ErrorLog는 Critical 로그 파일 분리를 검증합니다.
//
// 검증 항목:
//   - 메인 로그 파일 생성
//   - Critical 로그 파일 생성 (.critical. 포함)
//   - Closer 반환
func TestInitFileWithOptions_ErrorLog(t *testing.T) {
	t.Run("에러 로그 파일 생성", func(t *testing.T) {
		tempDir, teardown := setupLogLevelTest(t)
		defer teardown()

		appName := "test-app"
		closer := InitFileWithOptions(appName, 7.0, InitFileOptions{
			EnableCriticalLog: true,
		})

		defer func() {
			if closer != nil {
				closer.Close()
			}
		}()

		assert.NotNil(t, closer, "closer를 반환해야 합니다")

		// 로그 디렉토리 확인
		logDir := filepath.Join(tempDir, defaultLogDirectoryName)

		// 메인 로그 파일과 에러 로그 파일이 생성되었는지 확인
		hasMainLog := assertLogFileExists(t, logDir, appName, "main")
		hasCriticalLog := assertLogFileExists(t, logDir, appName, "critical")

		require.True(t, hasMainLog, "메인 로그 파일이 생성되어야 합니다")
		require.True(t, hasCriticalLog, "에러 로그 파일이 생성되어야 합니다")
	})
}

// TestInitFileWithOptions_BothLogs는 Critical과 Verbose 로그 파일 분리를 검증합니다.
//
// 검증 항목:
//   - 메인 로그 파일 생성
//   - Critical 로그 파일 생성 (.critical. 포함)
//   - Verbose 로그 파일 생성 (.verbose. 포함)
//   - 총 3개의 로그 파일 생성
func TestInitFileWithOptions_BothLogs(t *testing.T) {
	t.Run("에러 및 디버그 로그 파일 모두 생성", func(t *testing.T) {
		tempDir, teardown := setupLogLevelTest(t)
		defer teardown()

		appName := "test-app"
		closer := InitFileWithOptions(appName, 7.0, InitFileOptions{
			EnableCriticalLog: true,
			EnableVerboseLog:  true,
		})

		defer func() {
			if closer != nil {
				closer.Close()
			}
		}()

		assert.NotNil(t, closer, "closer를 반환해야 합니다")

		// 로그 디렉토리 확인
		logDir := filepath.Join(tempDir, defaultLogDirectoryName)
		files, err := os.ReadDir(logDir)
		require.NoError(t, err, "로그 디렉토리를 읽을 수 있어야 합니다")

		// 3개의 로그 파일이 생성되었는지 확인
		hasMainLog := assertLogFileExists(t, logDir, appName, "main")
		hasCriticalLog := assertLogFileExists(t, logDir, appName, "critical")
		hasVerboseLog := assertLogFileExists(t, logDir, appName, "verbose")

		require.True(t, hasMainLog, "메인 로그 파일이 생성되어야 합니다")
		require.True(t, hasCriticalLog, "에러 로그 파일이 생성되어야 합니다")
		require.True(t, hasVerboseLog, "디버그 로그 파일이 생성되어야 합니다")
		assert.Equal(t, 3, len(files), "총 3개의 로그 파일이 생성되어야 합니다")
	})
}

// TestInitFileWithOptions_EmptyOptions는 옵션 없이 초기화 시 메인 로그만 생성되는지 검증합니다.
//
// 검증 항목:
//   - 메인 로그 파일만 생성
//   - Critical, Verbose 로그 파일 미생성
//   - 총 1개의 로그 파일 생성
func TestInitFileWithOptions_EmptyOptions(t *testing.T) {
	t.Run("옵션 없이 초기화 시 메인 로그만 생성", func(t *testing.T) {
		tempDir, teardown := setupLogLevelTest(t)
		defer teardown()

		appName := "test-app-empty"
		closer := InitFileWithOptions(appName, 7.0, InitFileOptions{}) // Empty options

		defer func() {
			if closer != nil {
				closer.Close()
			}
		}()

		assert.NotNil(t, closer)

		// 로그 디렉토리 확인
		logDir := filepath.Join(tempDir, defaultLogDirectoryName)
		files, err := os.ReadDir(logDir)
		assert.NoError(t, err)

		// 메인 로그만 있어야 함
		var mainLogCount int
		for _, file := range files {
			if strings.HasPrefix(file.Name(), appName) && strings.HasSuffix(file.Name(), "."+defaultLogFileExtension) {
				// .critical. 이나 .verbose. 가 없어야 함
				if !strings.Contains(file.Name(), ".critical.") && !strings.Contains(file.Name(), ".verbose.") {
					mainLogCount++
				}
			}
		}
		assert.Equal(t, 1, mainLogCount, "메인 로그 파일 1개만 생성되어야 합니다")
	})
}
