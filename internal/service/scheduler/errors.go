package scheduler

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrTaskSubmitterNotInitialized 서비스 시작 시 핵심 의존성 객체인 TaskSubmitter가 올바르게 초기화되지 않았을 때 반환하는 에러입니다.
	ErrTaskSubmitterNotInitialized = apperrors.New(apperrors.Internal, "TaskSubmitter 객체가 초기화되지 않았습니다")

	// ErrNotificationSenderNotInitialized 서비스 시작 시 핵심 의존성 객체인 NotificationSender가 올바르게 초기화되지 않았을 때 반환하는 에러입니다.
	ErrNotificationSenderNotInitialized = apperrors.New(apperrors.Internal, "NotificationSender 객체가 초기화되지 않았습니다")
)

// NewErrInvalidCronSpec Cron 표현식이 올바르지 않아 스케줄 등록에 실패했을 때 반환하는 에러를 생성합니다.
func NewErrInvalidCronSpec(taskID, timeSpec string, cause error) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("스케줄 등록 실패: 잘못된 Cron 표현식입니다 (TaskID=%s, TimeSpec='%s'): %v", taskID, timeSpec, cause))
}
