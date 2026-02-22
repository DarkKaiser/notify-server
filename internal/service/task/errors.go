package task

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrNotificationSenderNotInitialized 서비스 시작 시 핵심 의존성 객체인 NotificationSender가 올바르게 초기화되지 않았을 때 반환되는 에러입니다.
	ErrNotificationSenderNotInitialized = apperrors.New(apperrors.Internal, "NotificationSender 객체가 초기화되지 않았습니다")

	// ErrServiceNotRunning Task 서비스가 실행 중이 아닐 때 반환되는 에러입니다.
	ErrServiceNotRunning = apperrors.New(apperrors.Internal, "Task 서비스가 현재 실행 중이지 않아 요청을 수행할 수 없습니다")

	// ErrInvalidTaskSubmitRequest Submit() 호출 시 전달된 요청 객체가 유효하지 않을 때 반환되는 에러입니다.
	ErrInvalidTaskSubmitRequest = apperrors.New(apperrors.Internal, "작업 실행 요청 정보가 유효하지 않아 요청을 처리할 수 없습니다")

	// ErrCancelQueueFull Cancel() 호출 시 취소 요청 대기열이 가득 차 있을 때 반환되는 에러입니다.
	// 비블로킹 방식으로 채널 전송을 시도하여, 즉시 수신이 불가능하면 재시도 없이 바로 반환합니다.
	ErrCancelQueueFull = apperrors.New(apperrors.Internal, "작업 취소 대기열이 포화 상태에 도달하여 일시적으로 요청을 접수할 수 없습니다")
)

// newTaskSubmitPanicError Submit() 처리 중 패닉이 발생했을 때 반환할 표준 에러를 생성합니다.
//
// Submit()은 닫힌 taskSubmitC 채널에 전송을 시도할 경우 패닉이 발생할 수 있으며,
// defer + recover를 통해 잡은 패닉 값을 이 함수로 전달하여 호출자에게 안전하게 반환합니다.
//
// 매개변수:
//   - v: recover()로 잡은 패닉 값입니다.
//
// 반환값: apperrors.Internal 유형의 에러 객체를 반환합니다.
func newTaskSubmitPanicError(v any) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("작업 실행 요청 처리 중 예기치 않은 내부 오류가 발생하였습니다 (상세: %v)", v))
}

// newTaskCancelPanicError Cancel() 처리 중 패닉이 발생했을 때 반환할 표준 에러를 생성합니다.
//
// Cancel()은 닫힌 taskCancelC 채널에 전송을 시도할 경우 패닉이 발생할 수 있으며,
// defer + recover를 통해 잡은 패닉 값을 이 함수로 전달하여 호출자에게 안전하게 반환합니다.
//
// 매개변수:
//   - v: recover()로 잡은 패닉 값입니다.
//
// 반환값: apperrors.Internal 유형의 에러 객체를 반환합니다.
func newTaskCancelPanicError(v any) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("작업 취소 처리 중 예기치 않은 내부 오류가 발생하였습니다 (상세: %v)", v))
}
