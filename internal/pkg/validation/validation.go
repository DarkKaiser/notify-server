package validation

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateFile 파일 존재 여부 및 파일 타입인지 검사합니다.
func ValidateFile(path string) error {
	if path == "" {
		return nil
	}

	if err := validate.Var(path, "file"); err != nil {
		return fmt.Errorf("유효하지 않은 파일 경로입니다 (존재하지 않거나 디렉터리임): %s", path)
	}
	return nil
}

// ValidateDir 디렉터리 존재 여부 및 디렉터리 타입인지 검사합니다.
func ValidateDir(path string) error {
	if path == "" {
		return nil
	}

	if err := validate.Var(path, "dir"); err != nil {
		return fmt.Errorf("유효하지 않은 디렉터리 경로입니다 (존재하지 않거나 파일임): %s", path)
	}
	return nil
}
