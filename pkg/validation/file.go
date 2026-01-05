package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateFile 지정된 경로가 유효한 파일인지 검증합니다.
func ValidateFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("파일 경로가 비어 있습니다")
	}

	path = filepath.Clean(path)

	// 1. 존재 여부 및 메타데이터 확인
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("파일이 존재하지 않습니다 (path=%q)", path)
		}
		return fmt.Errorf("파일 정보를 확인하는 중 오류가 발생했습니다 (path=%q): %w", path, err)
	}

	// 2. 파일 타입 확인 (정규 파일 여부)
	// 디렉터리뿐만 아니라 소켓, 파이프, 디바이스 파일 등을 모두 제외하기 위해
	// IsRegular()를 사용하여 순수 '파일'인지 확인합니다.
	if !info.Mode().IsRegular() {
		return fmt.Errorf("해당 경로는 일반 파일이어야 합니다 (path=%q, mode=%s)", path, info.Mode())
	}

	// 3. 읽기 권한 확인
	// os.Stat의 ModePerm 만으로는 ACL이나 컨테이너/클라우드 환경의 권한을 완벽히 대변하지 못하므로,
	// 실제로 파일을 열어보는 것이 가장 확실한 검증 방법입니다.
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("파일을 읽을 수 있는 권한이 없습니다 (path=%q): %w", path, err)
	}
	_ = f.Close()

	return nil
}

// ValidateDir 지정된 경로가 유효한 디렉터리인지 검증합니다.
func ValidateDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("디렉터리 경로가 비어 있습니다")
	}

	path = filepath.Clean(path)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("디렉터리가 존재하지 않습니다 (path=%q)", path)
		}
		return fmt.Errorf("디렉터리 정보를 확인하는 중 오류가 발생했습니다 (path=%q): %w", path, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("해당 경로는 디렉터리가 아닙니다 (path=%q)", path)
	}

	// 3. 읽기 권한 확인
	// 디렉터리 목록을 읽을 수 있는지 확인하기 위해 실제로 열어봅니다.
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("디렉터리를 읽을 수 있는 권한이 없습니다 (path=%q): %w", path, err)
	}
	_ = f.Close()

	return nil
}
