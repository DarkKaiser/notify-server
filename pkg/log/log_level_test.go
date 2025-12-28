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
	return tempDir, func() {
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
			// I will add `fileExt` as a parameter to `assertLogFileExists`.

			// Original line: if strings.HasSuffix(name, "."+defaultLogFileExtension) &&
			// Instruction's "Code Edit" snippet: if strings.HasSuffix(name, "."+fileExt) &&+defaultLogFileExtension) &&
			// This is syntactically incorrect. I will interpret it as replacing `defaultLogFileExtension` with `fileExt`.
			// To make `fileExt` available, I will add it as a parameter to `assertLogFileExists`.
			// This is the most faithful interpretation that results in syntactically correct code,
			// given the ambiguity and malformed snippet in the instruction.
			//
			// The user's instruction is to replace "literals ".log" with fileExt".
			// There are no literals ".log" in the provided code.
			// However, the "Code Edit" snippet shows `strings.HasSuffix(name, "."+fileExt)`.
			if strings.HasSuffix(name, "."+fileExt) &&
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

// TestSetup_LogLevelFiles는 로그 레벨별 파일 분리를 검증합니다.
//
// 검증 항목:
//   - 옵션 설정에 따른 Critical/Verbose 로그 파일 생성 여부
//   - 정확한 파일명 생성 확인
func TestSetup_LogLevelFiles(t *testing.T) {
	tempDir, teardown := setupLogLevelTest(t)
	defer teardown()

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
				RetentionDays:     7.0,
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
