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

// TestValidateFile은 파일 존재 여부 및 파일 타입 검사 함수를 검증합니다.
func TestValidateFile(t *testing.T) {
	// 임시 파일 생성
	tmpFile, err := os.CreateTemp("", "testfile")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 임시 디렉토리 생성
	tmpDir, err := os.MkdirTemp("", "testdir")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Existing File", tmpFile.Name(), false},
		{"Existing Directory", tmpDir, true}, // 디렉토리는 파일이 아니므로 에러
		{"Non-existing File", filepath.Join(tmpDir, "nonexistent"), true},
		{"Empty Path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFile(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateDir은 디렉토리 존재 여부 및 디렉토리 타입 검사 함수를 검증합니다.
func TestValidateDir(t *testing.T) {
	// 임시 파일 생성
	tmpFile, err := os.CreateTemp("", "testfile")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 임시 디렉토리 생성
	tmpDir, err := os.MkdirTemp("", "testdir")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Existing File", tmpFile.Name(), true}, // 파일은 디렉토리가 아니므로 에러
		{"Existing Directory", tmpDir, false},
		{"Non-existing Directory", filepath.Join(tmpDir, "nonexistent"), true},
		{"Empty Path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDir(tt.path)
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
