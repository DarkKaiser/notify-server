package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestInitFileWithOptions_ErrorLog(t *testing.T) {
	t.Run("에러 로그 파일 생성", func(t *testing.T) {
		// 테스트용 임시 디렉토리 사용
		tempDir := t.TempDir()
		originalLogDirBasePath := logDirectoryBasePath
		logDirectoryBasePath = tempDir + string(os.PathSeparator)
		defer func() {
			logDirectoryBasePath = originalLogDirBasePath
		}()

		appName := "test-app"
		closer := InitFileWithOptions(appName, 7.0, InitFileOptions{
			EnableCriticalLog: true,
		})

		// 테스트 종료 시 로거를 표준 출력으로 복원
		defer func() {
			if closer != nil {
				closer.Close()
				log.SetOutput(os.Stdout)
			}
		}()

		assert.NotNil(t, closer, "closer를 반환해야 합니다")

		// 로그 디렉토리 확인
		logDir := filepath.Join(tempDir, defaultLogDirectoryName)
		files, err := os.ReadDir(logDir)
		assert.NoError(t, err, "로그 디렉토리를 읽을 수 있어야 합니다")

		// 메인 로그 파일과 에러 로그 파일이 생성되었는지 확인
		var hasMainLog, hasCriticalLog bool
		for _, file := range files {
			name := file.Name()
			if strings.HasPrefix(name, appName) {
				if strings.Contains(name, ".critical.") {
					hasCriticalLog = true
				} else if strings.HasSuffix(name, "."+defaultLogFileExtension) {
					hasMainLog = true
				}
			}
		}

		assert.True(t, hasMainLog, "메인 로그 파일이 생성되어야 합니다")
		assert.True(t, hasCriticalLog, "에러 로그 파일이 생성되어야 합니다")
	})
}

func TestInitFileWithOptions_BothLogs(t *testing.T) {
	t.Run("에러 및 디버그 로그 파일 모두 생성", func(t *testing.T) {
		// 테스트용 임시 디렉토리 사용
		tempDir := t.TempDir()
		originalLogDirBasePath := logDirectoryBasePath
		logDirectoryBasePath = tempDir + string(os.PathSeparator)
		defer func() {
			logDirectoryBasePath = originalLogDirBasePath
		}()

		appName := "test-app"
		closer := InitFileWithOptions(appName, 7.0, InitFileOptions{
			EnableCriticalLog: true,
			EnableVerboseLog:  true,
		})

		// 테스트 종료 시 로거를 표준 출력으로 복원
		defer func() {
			if closer != nil {
				closer.Close()
				log.SetOutput(os.Stdout)
			}
		}()

		assert.NotNil(t, closer, "closer를 반환해야 합니다")

		// 로그 디렉토리 확인
		logDir := filepath.Join(tempDir, defaultLogDirectoryName)
		files, err := os.ReadDir(logDir)
		assert.NoError(t, err, "로그 디렉토리를 읽을 수 있어야 합니다")

		// 3개의 로그 파일이 생성되었는지 확인
		var hasMainLog, hasCriticalLog, hasVerboseLog bool
		for _, file := range files {
			name := file.Name()
			if strings.HasPrefix(name, appName) {
				if strings.Contains(name, ".critical.") {
					hasCriticalLog = true
				} else if strings.Contains(name, ".verbose.") {
					hasVerboseLog = true
				} else if strings.HasSuffix(name, "."+defaultLogFileExtension) {
					hasMainLog = true
				}
			}
		}

		assert.True(t, hasMainLog, "메인 로그 파일이 생성되어야 합니다")
		assert.True(t, hasCriticalLog, "에러 로그 파일이 생성되어야 합니다")
		assert.True(t, hasVerboseLog, "디버그 로그 파일이 생성되어야 합니다")
		assert.Equal(t, 3, len(files), "총 3개의 로그 파일이 생성되어야 합니다")
	})
}
