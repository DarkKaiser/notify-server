package validation

import (
	"fmt"
	"os"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
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
			errMsg := apperrors.New(apperrors.NotFound, fmt.Sprintf("파일이 존재하지 않습니다: %s", path))
			if warnOnly {
				applog.WithComponentAndFields("validation", log.Fields{
					"file_path": path,
				}).Warn(errMsg.Error())
				return nil
			}
			return errMsg
		}
		return apperrors.Wrap(err, apperrors.Internal, fmt.Sprintf("파일 접근 오류: %s", path))
	}
	return nil
}
