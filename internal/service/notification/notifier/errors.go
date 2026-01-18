package notifier

import (
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrQueueFull 내부 처리 대기열이 포화 상태에 도달하여 새로운 알림 요청을 즉시 수락할 수 없을 때 반환됩니다. (일시적 부하 상태)
	ErrQueueFull = apperrors.New(apperrors.Unavailable, "현재 알림 발송 대기열이 가득 차서 요청을 처리할 수 없습니다. 잠시 후 다시 시도해 주세요")

	// ErrClosed 알림 발송 서비스가 종료 절차를 밟고 있거나 이미 종료되어, 더 이상 서비스가 불가능할 때 반환됩니다. (영구적 불가 상태)
	ErrClosed = apperrors.New(apperrors.Unavailable, "알림 발송 서비스가 종료되었기 때문에 새로운 요청을 수락할 수 없습니다")

	// ErrContextCanceled 호출자가 제공한 컨텍스트(Context)가 만료되거나 취소되어, 알림 발송 프로세스가 중단되었을 때 반환됩니다.
	ErrContextCanceled = apperrors.New(apperrors.Unavailable, "요청하신 작업의 컨텍스트가 취소되어 알림 발송 처리가 중단되었습니다")

	// ErrPanicRecovered 실행 중 예기치 않은 런타임 패닉(Panic)이 발생하여, 시스템 안정성을 위해 복구된 후 반환됩니다.
	ErrPanicRecovered = apperrors.New(apperrors.Internal, "내부 시스템 로직 수행 중 심각한 오류가 발생하여 안전하게 복구되었습니다")
)
