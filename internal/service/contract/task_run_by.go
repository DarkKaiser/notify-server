package contract

import (
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// TaskRunBy 작업의 실행 주체를 정의합니다.
type TaskRunBy int

const (
	// TaskRunByUnknown 초기화되지 않았거나 알 수 없는 상태입니다 (기본값).
	TaskRunByUnknown TaskRunBy = iota

	// TaskRunByUser 사용자의 요청에 의한 수동 실행입니다.
	TaskRunByUser

	// TaskRunByScheduler 스케줄러에 의한 자동 실행입니다.
	TaskRunByScheduler
)

func (t TaskRunBy) IsValid() bool {
	switch t {
	case TaskRunByUser, TaskRunByScheduler:
		return true
	default:
		return false
	}
}

func (t TaskRunBy) Validate() error {
	if !t.IsValid() {
		return apperrors.New(apperrors.InvalidInput, "지원하지 않는 실행 주체(TaskRunBy)입니다")
	}
	return nil
}

func (t TaskRunBy) String() string {
	switch t {
	case TaskRunByUser:
		return "User"
	case TaskRunByScheduler:
		return "Scheduler"
	default:
		return "Unknown"
	}
}
