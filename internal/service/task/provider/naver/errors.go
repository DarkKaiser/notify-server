package naver

import (
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrEmptyQuery 설정 파일의 필수 입력값인 Query가 비어 있거나 공백만 포함된 경우 발생하는 에러입니다.
	ErrEmptyQuery = apperrors.New(apperrors.InvalidInput, "query가 입력되지 않았거나 공백입니다")
)
