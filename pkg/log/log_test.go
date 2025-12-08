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

func TestInit_DebugMode(t *testing.T) {
	t.Run("디버그 모드에서도 로그 파일 생성", func(t *testing.T) {
		// 테스트용 임시 디렉토리 사용
		tempDir := t.TempDir()
		originalLogDirParentPath := logDirectoryBasePath
		logDirectoryBasePath = tempDir + string(os.PathSeparator)
		defer func() {
			logDirectoryBasePath = originalLogDirParentPath
		}()

		appName := "test-app"
		closer := InitFile(appName, 7.0)
		SetDebugMode(true)

		// 테스트 종료 시 로거를 표준 출력으로 복원
		defer func() {
			if closer != nil {
				closer.Close()
				log.SetOutput(os.Stdout)
			}
		}()

		assert.NotNil(t, closer, "디버그 모드에서도 closer를 반환해야 합니다")

		// 로그 디렉토리가 생성되었는지 확인
		logDir := filepath.Join(tempDir, defaultLogDirectoryName)
		_, err := os.Stat(logDir)
		assert.NoError(t, err, "로그 디렉토리가 생성되어야 합니다")

		// 로그 파일이 생성되었는지 확인
		files, err := os.ReadDir(logDir)
		assert.NoError(t, err, "로그 디렉토리를 읽을 수 있어야 합니다")
		assert.Greater(t, len(files), 0, "최소 1개의 로그 파일이 생성되어야 합니다")
	})
}

func TestInit_ProductionMode(t *testing.T) {
	t.Run("프로덕션 모드에서 로그 파일 생성", func(t *testing.T) {
		// 테스트용 임시 디렉토리 사용
		tempDir := t.TempDir()
		originalLogDirParentPath := logDirectoryBasePath
		logDirectoryBasePath = tempDir + string(os.PathSeparator)
		defer func() {
			logDirectoryBasePath = originalLogDirParentPath
		}()

		appName := "test-app"
		closer := InitFile(appName, 7.0)
		SetDebugMode(false)

		// 테스트 종료 시 로거를 표준 출력으로 복원하여 다른 테스트에 영향을 주지 않도록 함
		defer func() {
			if closer != nil {
				closer.Close()
				// 로거를 표준 출력으로 복원
				log.SetOutput(os.Stdout)
			}
		}()

		assert.NotNil(t, closer, "프로덕션 모드에서는 closer를 반환해야 합니다")

		// 로그 디렉토리가 생성되었는지 확인
		logDir := filepath.Join(tempDir, defaultLogDirectoryName)
		_, err := os.Stat(logDir)
		assert.NoError(t, err, "로그 디렉토리가 생성되어야 합니다")

		// 로그 파일이 생성되었는지 확인
		files, err := os.ReadDir(logDir)
		assert.NoError(t, err, "로그 디렉토리를 읽을 수 있어야 합니다")
		assert.Greater(t, len(files), 0, "최소 1개의 로그 파일이 생성되어야 합니다")

		// 로그 파일명 확인
		found := false
		for _, file := range files {
			if strings.HasPrefix(file.Name(), appName) && strings.HasSuffix(file.Name(), "."+defaultLogFileExtension) {
				found = true
				break
			}
		}
		assert.True(t, found, "앱 이름으로 시작하는 로그 파일이 있어야 합니다")
	})
}

func TestCleanOutOfLogFiles(t *testing.T) {
	t.Run("오래된 로그 파일 삭제", func(t *testing.T) {
		// 테스트용 임시 디렉토리 사용
		tempDir := t.TempDir()
		originalLogDirParentPath := logDirectoryBasePath
		logDirectoryBasePath = tempDir + string(os.PathSeparator)
		defer func() {
			logDirectoryBasePath = originalLogDirParentPath
		}()

		// 로그 디렉토리 생성
		logDir := filepath.Join(tempDir, defaultLogDirectoryName)
		err := os.MkdirAll(logDir, 0755)
		assert.NoError(t, err, "로그 디렉토리를 생성할 수 있어야 합니다")

		appName := "test-app"

		// 오래된 로그 파일 생성 (10일 전)
		oldLogFile := filepath.Join(logDir, appName+"-old."+defaultLogFileExtension)
		f, err := os.Create(oldLogFile)
		assert.NoError(t, err, "오래된 로그 파일을 생성할 수 있어야 합니다")
		f.Close()

		// 파일의 수정 시간을 10일 전으로 변경
		oldTime := time.Now().Add(-10 * 24 * time.Hour)
		err = os.Chtimes(oldLogFile, oldTime, oldTime)
		assert.NoError(t, err, "파일 시간을 변경할 수 있어야 합니다")

		// 최근 로그 파일 생성
		recentLogFile := filepath.Join(logDir, appName+"-recent."+defaultLogFileExtension)
		f, err = os.Create(recentLogFile)
		assert.NoError(t, err, "최근 로그 파일을 생성할 수 있어야 합니다")
		f.Close()

		// removeExpiredLogFiles 실행 (7일 이상 된 파일 삭제)
		removeExpiredLogFiles(appName, 7.0)

		// 오래된 파일이 삭제되었는지 확인
		_, err = os.Stat(oldLogFile)
		assert.True(t, os.IsNotExist(err), "오래된 로그 파일이 삭제되어야 합니다")

		// 최근 파일은 남아있는지 확인
		_, err = os.Stat(recentLogFile)
		assert.NoError(t, err, "최근 로그 파일은 남아있어야 합니다")
	})

	t.Run("다른 앱의 로그 파일은 삭제하지 않음", func(t *testing.T) {
		// 테스트용 임시 디렉토리 사용
		tempDir := t.TempDir()
		originalLogDirParentPath := logDirectoryBasePath
		logDirectoryBasePath = tempDir + string(os.PathSeparator)
		defer func() {
			logDirectoryBasePath = originalLogDirParentPath
		}()

		// 로그 디렉토리 생성
		logDir := filepath.Join(tempDir, defaultLogDirectoryName)
		err := os.MkdirAll(logDir, 0755)
		assert.NoError(t, err, "로그 디렉토리를 생성할 수 있어야 합니다")

		// 다른 앱의 오래된 로그 파일 생성
		otherAppLogFile := filepath.Join(logDir, "other-app-old."+defaultLogFileExtension)
		f, err := os.Create(otherAppLogFile)
		assert.NoError(t, err, "다른 앱의 로그 파일을 생성할 수 있어야 합니다")
		f.Close()

		// 파일의 수정 시간을 10일 전으로 변경
		oldTime := time.Now().Add(-10 * 24 * time.Hour)
		err = os.Chtimes(otherAppLogFile, oldTime, oldTime)
		assert.NoError(t, err, "파일 시간을 변경할 수 있어야 합니다")

		// removeExpiredLogFiles 실행 (test-app의 로그만 삭제)
		removeExpiredLogFiles("test-app", 7.0)

		// 다른 앱의 파일은 남아있는지 확인
		_, err = os.Stat(otherAppLogFile)
		assert.NoError(t, err, "다른 앱의 로그 파일은 삭제되지 않아야 합니다")
	})
}

func TestLogFileExtension(t *testing.T) {
	t.Run("로그 파일 확장자 확인", func(t *testing.T) {
		assert.Equal(t, "log", defaultLogFileExtension, "로그 파일 확장자는 'log'여야 합니다")
	})
}

func TestLogDirName(t *testing.T) {
	t.Run("로그 디렉토리 이름 확인", func(t *testing.T) {
		assert.Equal(t, "logs", defaultLogDirectoryName, "로그 디렉토리 이름은 'logs'여야 합니다")
	})
}

func TestSetCallerPathPrefix(t *testing.T) {
	t.Run("CallerPathPrefix 설정 확인", func(t *testing.T) {
		// 기본값 백업 및 복원
		originalPrefix := callerFunctionPathPrefix
		defer func() {
			callerFunctionPathPrefix = originalPrefix
		}()

		prefix := "github.com/test/pkg"
		SetCallerPathPrefix(prefix)

		assert.Equal(t, prefix, callerFunctionPathPrefix, "Prefix가 올바르게 설정되어야 합니다")
	})
}

func TestWithComponent(t *testing.T) {
	t.Run("Component 필드가 추가되는지 확인", func(t *testing.T) {
		entry := WithComponent("test-component")
		assert.NotNil(t, entry)
		assert.Equal(t, "test-component", entry.Data["component"])
	})
}

func TestWithComponentAndFields(t *testing.T) {
	t.Run("Component와 추가 필드가 모두 포함되는지 확인", func(t *testing.T) {
		fields := log.Fields{
			"key1": "value1",
			"key2": 123,
		}
		entry := WithComponentAndFields("test-component", fields)

		assert.NotNil(t, entry)
		assert.Equal(t, "test-component", entry.Data["component"])
		assert.Equal(t, "value1", entry.Data["key1"])
		assert.Equal(t, 123, entry.Data["key2"])
	})
}
