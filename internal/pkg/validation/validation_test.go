package validation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Port Validation Tests
// =============================================================================

// =============================================================================
// File Existence Validation Tests
// =============================================================================

// TestValidateFileExists는 파일 존재 여부 검사를 검증합니다.
//
// 검증 항목:
//   - 존재하는 파일
//   - 존재하는 디렉토리
//   - 존재하지 않는 파일
//   - warnOnly 옵션 (경고만 출력)
//   - 빈 경로 (허용됨)
func TestValidateFileExists(t *testing.T) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "testfile")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "testdir")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		path     string
		warnOnly bool
		wantErr  bool
	}{
		{"Existing File", tmpFile.Name(), false, false},
		{"Existing Directory", tmpDir, false, false},
		{"Non-existing File", filepath.Join(tmpDir, "nonexistent"), false, true},
		{"Non-existing File (WarnOnly)", filepath.Join(tmpDir, "nonexistent"), true, false}, // Error logged but nil returned
		{"Empty Path", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileExists(tt.path, tt.warnOnly)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// Duplicate Validation Tests
// =============================================================================

// =============================================================================
// Examples (Documentation)
// =============================================================================
