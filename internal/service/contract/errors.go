package contract

import (
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrMessageRequired 알림의 본문 내용이 비어있거나 공백 문자로만 구성되어 있어 유효하지 않을 때 반환하는 에러입니다.
	ErrMessageRequired = apperrors.New(apperrors.InvalidInput, "알림 메시지 본문은 비워둘 수 없습니다")
)
