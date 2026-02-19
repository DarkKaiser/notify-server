package lotto

import (
	"path/filepath"
	"strings"

	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/pkg/validation"
)

type taskSettings struct {
	AppPath string `json:"app_path"`
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Validator = (*taskSettings)(nil)

func (s *taskSettings) Validate() error {
	s.AppPath = strings.TrimSpace(s.AppPath)
	if s.AppPath == "" {
		return ErrAppPathMissing
	}

	// 절대 경로로 변환하여 실행 위치(CWD)에 독립적으로 만듭니다.
	absPath, err := filepath.Abs(s.AppPath)
	if err != nil {
		return newErrAppPathAbsFailed(err)
	}
	s.AppPath = absPath

	if err := validation.ValidateDir(s.AppPath); err != nil {
		return newErrAppPathDirValidationFailed(err)
	}

	return nil
}
