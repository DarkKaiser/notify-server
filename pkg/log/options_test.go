package log

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOptions_Validate는 Options 검증 로직을 테스트합니다.
func TestOptions_Validate(t *testing.T) {
	t.Parallel()

	// 임시 디렉토리 생성 (유효한 Dir 테스트용)
	tempDir := t.TempDir()

	// 임시 파일 생성 (Dir이 파일인 경우 테스트용)
	tempFile := filepath.Join(tempDir, "testfile")
	err := os.WriteFile(tempFile, []byte("test"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name        string
		opts        Options
		expectError bool
		errorMsg    string
	}{
		{
			name: "유효한 설정값 (기본)",
			opts: Options{
				Name:       "test-app",
				MaxAge:     7,
				MaxSizeMB:  100,
				MaxBackups: 20,
			},
			expectError: false,
		},
		{
			name: "유효한 설정값 (Dir 포함)",
			opts: Options{
				Name:       "test-app",
				Dir:        tempDir,
				MaxAge:     7,
				MaxSizeMB:  100,
				MaxBackups: 20,
			},
			expectError: false,
		},
		{
			name: "Name 누락",
			opts: Options{
				MaxAge:     7,
				MaxSizeMB:  100,
				MaxBackups: 20,
			},
			expectError: true,
			errorMsg:    "애플리케이션 식별자(Name)가 설정되지 않았습니다",
		},
		{
			name: "Dir이 파일인 경우 (경로 충돌)",
			opts: Options{
				Name:       "test-app",
				Dir:        tempFile, // 파일 경로를 Dir로 설정
				MaxAge:     7,
				MaxSizeMB:  100,
				MaxBackups: 20,
			},
			expectError: true,
			errorMsg:    "로그 디렉토리 경로(" + tempFile + ")가 이미 파일로 존재합니다",
		},
		{
			name: "모든 값이 0 (기본값 사용)",
			opts: Options{
				Name:       "test-app",
				MaxAge:     0,
				MaxSizeMB:  0,
				MaxBackups: 0,
			},
			expectError: false,
		},
		{
			name: "음수 MaxAge",
			opts: Options{
				Name:       "test-app",
				MaxAge:     -1,
				MaxSizeMB:  100,
				MaxBackups: 20,
			},
			expectError: true,
			errorMsg:    "MaxAge는 0 이상이어야 합니다",
		},
		{
			name: "음수 MaxSizeMB",
			opts: Options{
				Name:       "test-app",
				MaxAge:     7,
				MaxSizeMB:  -100,
				MaxBackups: 20,
			},
			expectError: true,
			errorMsg:    "MaxSizeMB는 0 이상이어야 합니다",
		},
		{
			name: "음수 MaxBackups",
			opts: Options{
				Name:       "test-app",
				MaxAge:     7,
				MaxSizeMB:  100,
				MaxBackups: -5,
			},
			expectError: true,
			errorMsg:    "MaxBackups는 0 이상이어야 합니다",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.opts.Validate()

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
