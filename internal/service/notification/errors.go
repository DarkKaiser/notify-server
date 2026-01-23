package notification

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// TODO 미완료

var (
	// ErrServiceNotRunning 시스템 종료 절차가 진행 중이거나, 필수 컴포넌트가 초기화되지 않아 알림 요청을 처리할 수 없는 경우 반환하는 에러입니다.
	ErrServiceNotRunning = apperrors.New(apperrors.Unavailable, "시스템 종료 절차가 진행 중이거나, 초기화되지 않아 알림을 보낼 수 없습니다")

	// ErrNotifierNotFound 지정된 알림 채널(Notifier)을 찾을 수 없거나, 설정 파일에 등록되지 않은 채널 ID가 요청되었을 때 반환하는 에러입니다.
	ErrNotifierNotFound = apperrors.New(apperrors.NotFound, "등록되지 않은 알림 채널입니다. 설정 파일을 확인해 주세요")
)

// NewErrDuplicateNotifierID 설정 로딩 시 동일한 Notifier ID가 중복으로 감지되었을 때 반환하는 에러를 생성합니다.
func NewErrDuplicateNotifierID(id string) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("중복된 Notifier ID('%s')가 감지되었습니다. 설정을 확인해주세요.", id))
}

// NewErrDefaultNotifierNotFound 시스템 필수 설정인 기본 Notifier ID가 누락되었거나 찾을 수 없을 때 반환하는 에러를 생성합니다.
func NewErrDefaultNotifierNotFound(id string) error {
	return apperrors.New(apperrors.NotFound, fmt.Sprintf("기본 Notifier('%s')를 찾을 수 없습니다", id))
}

// NewErrExecutorNotInitialized 서비스 시작 시 필수 컴포넌트인 Task Executor가 초기화되지 않았을 때 반환하는 에러를 생성합니다.
func NewErrExecutorNotInitialized() error {
	return apperrors.New(apperrors.Internal, "Executor 객체가 초기화되지 않았습니다")
}

// NewErrNotifierInitFailed Notifier 생성 또는 연결 설정 중 예상치 못한 에러가 발생했을 때 반환하는 에러를 생성합니다.
func NewErrNotifierInitFailed(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "Notifier 초기화 중 에러가 발생했습니다")
}
