package notifier

import (
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrQueueFull 일시적인 트래픽 폭주로 내부 대기열이 가득 차서, 요청을 즉시 처리할 수 없을 때 반환됩니다.
	ErrQueueFull = apperrors.New(apperrors.Unavailable, "현재 알림 발송 대기열이 가득 차서 요청을 처리할 수 없습니다. 잠시 후 다시 시도해 주세요")

	// ErrClosed 서비스가 종료 절차를 진행 중이거나 이미 중단되어, 더 이상 요청을 수락할 수 없는 상태입니다.
	ErrClosed = apperrors.New(apperrors.Unavailable, "알림 발송 서비스가 종료되었기 때문에 새로운 요청을 수락할 수 없습니다")

	// ErrContextCanceled 호출 측의 요청 취소 또는 타임아웃으로 인해, 작업 처리가 중단되었을 때 반환됩니다.
	ErrContextCanceled = apperrors.New(apperrors.Unavailable, "요청하신 작업의 컨텍스트가 취소되어 알림 발송 처리가 중단되었습니다")

	// ErrPanicRecovered 로직 수행 중 치명적인 런타임 패닉이 발생했으나, 시스템 보호를 위해 안전하게 복구되었습니다.
	ErrPanicRecovered = apperrors.New(apperrors.Internal, "내부 시스템 로직 수행 중 심각한 오류가 발생하여 안전하게 복구되었습니다")
)
