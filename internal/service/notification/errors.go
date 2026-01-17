package notification

import "errors"

// TODO 미완료

var (
	// ErrServiceStopped 서비스가 중지되었거나, 초기화되지 않아 알림 요청을 처리할 수 없는 경우 반환됩니다.
	ErrServiceStopped = errors.New("시스템 종료 절차가 진행 중이거나, 초기화되지 않아 알림을 보낼 수 없습니다")

	// ErrNotFoundNotifier 요청된 ID에 해당하는 Notifier 설정을 찾을 수 없는 경우 반환됩니다.
	ErrNotFoundNotifier = errors.New("등록되지 않은 알림 채널입니다. 설정 파일을 확인해 주세요")
)
