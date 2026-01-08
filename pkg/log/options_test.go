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

	// 테스트 환경 셋업을 위한 헬퍼 함수
	createTempFile := func(t *testing.T) string {
		t.Helper()
		tempDir := t.TempDir()
		tempFile := filepath.Join(tempDir, "conflict_file")
		err := os.WriteFile(tempFile, []byte("conflict"), 0644)
		require.NoError(t, err)
		return tempFile
	}

	tests := []struct {
		name        string
		buildOpts   func(t *testing.T) Options // 동적 설정 생성을 위해 함수로 변경 (Setup 격리)
		expectError bool
		errorMsg    string
	}{
		{
			name: "Success_DefaultDefaults",
			buildOpts: func(t *testing.T) Options {
				return Options{
					Name:       "test-app",
					MaxAge:     7,
					MaxSizeMB:  100,
					MaxBackups: 20,
				}
			},
			expectError: false,
		},
		{
			name: "Success_WithValidDir",
			buildOpts: func(t *testing.T) Options {
				return Options{
					Name:       "test-app",
					Dir:        t.TempDir(), // 각 테스트마다 독립된 임시 디렉토리 사용
					MaxAge:     7,
					MaxSizeMB:  100,
					MaxBackups: 20,
				}
			},
			expectError: false,
		},
		{
			name: "Success_ZeroValues_UseDefaults",
			buildOpts: func(t *testing.T) Options {
				return Options{
					Name:       "test-app",
					MaxAge:     0,
					MaxSizeMB:  0,
					MaxBackups: 0,
				}
			},
			expectError: false,
		},
		{
			name: "Error_MissingName",
			buildOpts: func(t *testing.T) Options {
				return Options{
					MaxAge:     7,
					MaxSizeMB:  100,
					MaxBackups: 20,
				}
			},
			expectError: true,
			errorMsg:    "애플리케이션 식별자(Name)가 설정되지 않았습니다",
		},
		{
			name: "Error_DirConflictWithFile",
			buildOpts: func(t *testing.T) Options {
				conflictFile := createTempFile(t)
				return Options{
					Name:       "test-app",
					Dir:        conflictFile, // 파일 경로를 Dir로 설정하여 에러 유도
					MaxAge:     7,
					MaxSizeMB:  100,
					MaxBackups: 20,
				}
			},
			expectError: true,
			errorMsg:    "이미 파일로 존재합니다", // 부분 일치 검증
		},
		{
			name: "Error_NegativeMaxAge",
			buildOpts: func(t *testing.T) Options {
				return Options{Name: "app", MaxAge: -1}
			},
			expectError: true,
			errorMsg:    "MaxAge는 0 이상이어야 합니다",
		},
		{
			name: "Error_NegativeMaxSizeMB",
			buildOpts: func(t *testing.T) Options {
				return Options{Name: "app", MaxSizeMB: -1}
			},
			expectError: true,
			errorMsg:    "MaxSizeMB는 0 이상이어야 합니다",
		},
		{
			name: "Error_NegativeMaxBackups",
			buildOpts: func(t *testing.T) Options {
				return Options{Name: "app", MaxBackups: -1}
			},
			expectError: true,
			errorMsg:    "MaxBackups는 0 이상이어야 합니다",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := tc.buildOpts(t)
			err := opts.Validate()

			if tc.expectError {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
