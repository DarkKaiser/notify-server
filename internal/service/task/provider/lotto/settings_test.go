package lotto

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskSettings_Validate(t *testing.T) {
	// 임시 디렉터리 생성 (존재하는 경로 테스트용)
	tmpDir := t.TempDir()

	// 임시 파일 생성 (파일 경로 테스트용)
	tmpFile := filepath.Join(tmpDir, "testfile")
	f, err := os.Create(tmpFile)
	require.NoError(t, err)
	f.Close()

	tests := []struct {
		name        string
		settings    *taskSettings
		wantErr     bool
		errContains string
		check       func(t *testing.T, s *taskSettings)
	}{
		{
			name: "Success_AbsolutePath",
			settings: &taskSettings{
				AppPath: tmpDir,
			},
			wantErr: false,
			check: func(t *testing.T, s *taskSettings) {
				assert.Equal(t, tmpDir, s.AppPath)
			},
		},
		{
			name: "Success_RelativePath",
			settings: &taskSettings{
				AppPath: ".", // 현재 디렉터리 (상대 경로)
			},
			wantErr: false,
			check: func(t *testing.T, s *taskSettings) {
				abs, _ := filepath.Abs(".")
				assert.Equal(t, abs, s.AppPath)
			},
		},
		{
			name: "Error_AppPathMissing",
			settings: &taskSettings{
				AppPath: "",
			},
			wantErr:     true,
			errContains: ErrAppPathMissing.Error(),
		},
		{
			name: "Error_AppPathNotDirectory",
			settings: &taskSettings{
				AppPath: tmpFile,
			},
			wantErr:     true,
			errContains: "app_path로 지정된 디렉터리 검증에 실패하였습니다",
		},
		{
			name: "Error_AppPathDoesNotExist",
			settings: &taskSettings{
				AppPath: filepath.Join(tmpDir, "non_existent_dir"),
			},
			wantErr:     true,
			errContains: "app_path로 지정된 디렉터리 검증에 실패하였습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				if tt.check != nil {
					tt.check(t, tt.settings)
				}
			}
		})
	}
}

func TestTaskSettings_Validate_AbsError(t *testing.T) {
	// filepath.Abs 실패 케이스를 강제로 만들기 어렵지만,
	// 구조적으로 에러 처리가 되어 있는지 확인하기 위해 에러 래핑 함수 등을 테스트할 수 있습니다.
	// 하지만 여기서는 Validate 로직 내의 AppPath 공백 처리나 기본적인 로직 위주로 검증합니다.
	// filepath.Abs 에러는 OS 레벨의 매우 희귀한 상황이므로 통합 테스트 레벨에서 커버하기 어렵습니다.
	// 대신 에러 생성자 함수를 테스트합니다.

	mockErr := errors.New("mock error")
	err := newErrAppPathAbsFailed(mockErr)
	assert.ErrorIs(t, err, mockErr)
	assert.Contains(t, err.Error(), "app_path의 절대 경로 변환에 실패하였습니다")
}
