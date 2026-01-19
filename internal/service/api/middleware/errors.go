package middleware

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// NewErrPanicRecovered 캡처된 패닉 값을 내부 시스템 오류로 래핑하여 새로운 에러를 생성합니다.
func NewErrPanicRecovered(r any) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("%v", r))
}
