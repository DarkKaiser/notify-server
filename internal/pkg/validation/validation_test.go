package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// URL Validation Tests
// =============================================================================

// TestValidateURL은 URL 유효성 검사를 검증합니다.
//
// 검증 항목:
//   - 유효한 HTTP/HTTPS URL
//   - 포트, 경로, 쿼리 포함 URL
//   - Localhost 및 IP 주소
//   - 잘못된 스키마 (ftp)
//   - 잘못된 형식 (공백, 호스트 누락)
//   - 빈 문자열 (허용됨)
func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		urlStr  string
		wantErr bool
	}{
		{"Valid HTTP", "http://example.com", false},
		{"Valid HTTPS", "https://example.com", false},
		{"Valid with port", "https://example.com:8080", false},
		{"Valid with path", "https://example.com/api/v1", false},
		{"Valid with query", "https://example.com/search?q=test", false},
		{"Valid Localhost", "http://localhost:3000", false},
		{"Valid IP", "http://192.168.0.1", false},
		{"Invalid Scheme (ftp)", "ftp://example.com", true},
		{"Invalid Scheme (missing)", "example.com", true},
		{"Invalid Format (spaces)", "http://exa mple.com", true},
		{"Missing Host", "http://", true},
		{"Empty String", "", false}, // Empty is allowed by design (optional)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.urlStr)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, apperrors.Is(err, apperrors.InvalidInput))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

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
		errType  apperrors.ErrorType
	}{
		{"Existing File", tmpFile.Name(), false, false, ""},
		{"Existing Directory", tmpDir, false, false, ""},
		{"Non-existing File", filepath.Join(tmpDir, "nonexistent"), false, true, apperrors.NotFound},
		{"Non-existing File (WarnOnly)", filepath.Join(tmpDir, "nonexistent"), true, false, ""}, // Error logged but nil returned
		{"Empty Path", "", false, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileExists(tt.path, tt.warnOnly)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != "" {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s, got %v", tt.errType, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateFileExistsOrURL은 파일 또는 URL 유효성 검사를 검증합니다.
//
// 검증 항목:
//   - 유효한 URL
//   - 잘못된 URL
//   - 존재하는 파일
//   - 존재하지 않는 파일
//   - warnOnly 옵션
//   - 빈 경로 (허용됨)
func TestValidateFileExistsOrURL(t *testing.T) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "testfile")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tests := []struct {
		name     string
		path     string
		warnOnly bool
		wantErr  bool
	}{
		{"Valid URL", "https://example.com", false, false},
		{"Invalid URL", "http://", false, true},
		{"Existing File", tmpFile.Name(), false, false},
		{"Non-existing File", "nonexistent_file", false, true},
		{"Non-existing File (WarnOnly)", "nonexistent_file", true, false},
		{"Empty Path", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileExistsOrURL(tt.path, tt.warnOnly)
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

func ExampleValidateURL() {
	if err := ValidateURL("https://example.com"); err == nil {
		fmt.Println("Valid URL")
	}
	// Output: Valid URL
}
