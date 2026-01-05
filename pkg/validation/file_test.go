package validation_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/darkkaiser/notify-server/pkg/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateFile_Comprehensive는 파일 검증의 모든 시나리오를 테스트합니다.
func TestValidateFile_Comprehensive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// 1. 정상 파일 생성
	validFile := filepath.Join(tmpDir, "valid.txt")
	f, err := os.Create(validFile)
	require.NoError(t, err)
	f.Close()

	var symlinkFile, brokenSymlink string
	if runtime.GOOS != "windows" {
		// 2. 심볼릭 링크 생성 (파일) - 유효한 링크
		symlinkFile = filepath.Join(tmpDir, "symlink_to_file.txt")
		require.NoError(t, os.Symlink(validFile, symlinkFile))

		// 3. 깨진 심볼릭 링크 생성
		brokenSymlink = filepath.Join(tmpDir, "broken_link.txt")
		require.NoError(t, os.Symlink(filepath.Join(tmpDir, "nonexistent"), brokenSymlink))
	}

	tests := []struct {
		name          string
		path          string
		shouldError   bool
		errorContains string
	}{
		// [Input Validation]
		{name: "Empty Path", path: "", shouldError: true, errorContains: "파일 경로가 비어 있습니다"},
		{name: "Whitespace Path", path: "   ", shouldError: true, errorContains: "파일 경로가 비어 있습니다"},

		// [Existence & Type]
		{name: "Valid File", path: validFile, shouldError: false},
		{name: "Non-existent File", path: filepath.Join(tmpDir, "missing.txt"), shouldError: true, errorContains: "파일이 존재하지 않습니다"},
		{name: "Path is Directory", path: tmpDir, shouldError: true, errorContains: "일반 파일이어야 합니다"},

		// [Relative Path]
		{name: "Relative Path (Current Dir)", path: "file.go", shouldError: false},
	}

	// Windows가 아닌 경우 심볼릭 링크 테스트 추가
	if runtime.GOOS != "windows" {
		tests = append(tests, []struct {
			name          string
			path          string
			shouldError   bool
			errorContains string
		}{
			{name: "Valid Symlink to File", path: symlinkFile, shouldError: false},
			{name: "Broken Symlink", path: brokenSymlink, shouldError: true, errorContains: "파일이 존재하지 않습니다"},
		}...)
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validation.ValidateFile(tc.path)
			if tc.shouldError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateDir_Comprehensive는 디렉터리 검증의 모든 시나리오를 테스트합니다.
func TestValidateDir_Comprehensive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// 1. 파일 생성 (디렉터리 아님)
	validFile := filepath.Join(tmpDir, "testfile.txt")
	f, err := os.Create(validFile)
	require.NoError(t, err)
	f.Close()

	// 2. 하위 디렉터리
	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755))

	var symlinkDir string
	if runtime.GOOS != "windows" {
		// 3. 심볼릭 링크 (디렉터리)
		symlinkDir = filepath.Join(tmpDir, "symlink_to_dir")
		require.NoError(t, os.Symlink(subDir, symlinkDir))
	}

	tests := []struct {
		name          string
		path          string
		shouldError   bool
		errorContains string
	}{
		// [Input Validation]
		{name: "Empty Path", path: "", shouldError: true, errorContains: "디렉터리 경로가 비어 있습니다"},
		{name: "Whitespace Path", path: "   ", shouldError: true, errorContains: "디렉터리 경로가 비어 있습니다"},

		// [Existence & Type]
		{name: "Valid Directory", path: subDir, shouldError: false},
		{name: "Non-existent Dir", path: filepath.Join(tmpDir, "missing_dir"), shouldError: true, errorContains: "디렉터리가 존재하지 않습니다"},
		{name: "Path is File", path: validFile, shouldError: true, errorContains: "디렉터리가 아닙니다"},

		// [Relative Path]
		{name: "Relative Path (Dot)", path: ".", shouldError: false},
	}

	if runtime.GOOS != "windows" {
		tests = append(tests, struct {
			name          string
			path          string
			shouldError   bool
			errorContains string
		}{name: "Valid Symlink to Dir", path: symlinkDir, shouldError: false})
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validation.ValidateDir(tc.path)
			if tc.shouldError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPermissions_UnixOnly 는 유닉스 계열 OS에서 파일/디렉터리 권한 검증을 수행합니다.
func TestPermissions_UnixOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission tests on Windows")
	}

	t.Parallel()
	tmpDir := t.TempDir()

	// 1. 권한 없는 파일 (000)
	noPermFile := filepath.Join(tmpDir, "noperm.txt")
	f, err := os.Create(noPermFile)
	require.NoError(t, err)
	f.Close()
	require.NoError(t, os.Chmod(noPermFile, 0000))

	// 2. 권한 없는 디렉터리 (000)
	noPermDir := filepath.Join(tmpDir, "noperm_dir")
	require.NoError(t, os.Mkdir(noPermDir, 0000))

	t.Run("File Without Read Permission", func(t *testing.T) {
		err := validation.ValidateFile(noPermFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "권한이 없습니다")
	})

	t.Run("Directory Without Read Permission", func(t *testing.T) {
		err := validation.ValidateDir(noPermDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "권한이 없습니다")
	})
}
