package notification

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrServiceNotRunning 서비스가 정상적인 실행 상태(Running)가 아닐 때(종료 절차 진행 중 또는 미시작) 반환하는 에러입니다.
	ErrServiceNotRunning = apperrors.New(apperrors.Unavailable, "서비스가 실행 상태가 아닙니다: 시스템이 종료 중이거나 아직 시작되지 않았습니다")

	// ErrNotifierNotFound 지정된 알림 채널(Notifier)을 찾을 수 없거나, 설정 파일에 등록되지 않은 채널 ID가 요청되었을 때 반환하는 에러입니다.
	ErrNotifierNotFound = apperrors.New(apperrors.NotFound, "등록되지 않은 Notifier입니다. 설정 파일을 확인해 주세요")

	// ErrExecutorNotInitialized 서비스 시작 시 핵심 의존성 객체인 Task Executor가 올바르게 초기화되지 않았을 때 반환하는 에러입니다.
	ErrExecutorNotInitialized = apperrors.New(apperrors.Internal, "Executor 객체가 초기화되지 않았습니다")
)

// NewErrDuplicateNotifierID 설정 로딩 시 동일한 Notifier ID가 중복으로 감지되었을 때 반환하는 에러를 생성합니다.
func NewErrDuplicateNotifierID(id string) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("중복된 Notifier ID('%s')가 감지되었습니다. 설정을 확인해주세요", id))
}

// NewErrDefaultNotifierNotFound 설정 파일에 정의된 기본(Default) Notifier ID를 찾을 수 없을 때 반환하는 에러를 생성합니다.
func NewErrDefaultNotifierNotFound(id string) error {
	return apperrors.New(apperrors.NotFound, fmt.Sprintf("기본 Notifier('%s')를 찾을 수 없습니다", id))
}

// NewErrNotifierInitFailed Notifier 인스턴스 생성 및 초기화 과정에서 예기치 않은 오류가 발생했을 때 반환하는 에러를 생성합니다.
func NewErrNotifierInitFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "Notifier 인스턴스 초기화 실패: 내부 구성 오류 또는 연결 문제로 인해 생성하지 못했습니다")
}
