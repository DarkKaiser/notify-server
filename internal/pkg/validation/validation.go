package validation

import (
	"fmt"
	"os"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

// ValidateFileExists 파일 존재 여부를 검사합니다
// warnOnly가 true면 경고만 출력하고 에러는 반환하지 않습니다.
func ValidateFileExists(path string, warnOnly bool) error {
	if path == "" {
		return nil // 빈 경로는 검사하지 않음
	}

	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			errMsg := fmt.Errorf("파일이 존재하지 않습니다: %s", path)
			if warnOnly {
				applog.WithComponentAndFields("validation", log.Fields{
					"file_path": path,
				}).Warn(errMsg.Error())
				return nil
			}
			return errMsg
		}
		return fmt.Errorf("파일 접근 오류: %s (원인: %w)", path, err)
	}
	return nil
}
