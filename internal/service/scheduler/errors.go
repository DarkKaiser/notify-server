package scheduler

import (
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrTaskSubmitterNotInitialized 서비스 시작 시 핵심 의존성 객체인 TaskSubmitter가 올바르게 초기화되지 않았을 때 반환하는 에러입니다.
	ErrTaskSubmitterNotInitialized = apperrors.New(apperrors.Internal, "TaskSubmitter 객체가 초기화되지 않았습니다")

	// ErrNotificationSenderNotInitialized 서비스 시작 시 핵심 의존성 객체인 NotificationSender가 올바르게 초기화되지 않았을 때 반환하는 에러입니다.
	ErrNotificationSenderNotInitialized = apperrors.New(apperrors.Internal, "NotificationSender 객체가 초기화되지 않았습니다")
)
