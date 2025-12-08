package task

import (
	"github.com/darkkaiser/notify-server/pkg/errors"
)

const (
	// ErrTaskNotFound Task를 찾을 수 없음
	// 사용 시나리오: 설정에 정의되지 않은 Task ID를 참조할 때
	ErrTaskNotFound errors.ErrorType = "TaskNotFound"

	// ErrTaskExecutionFailed Task 실행 실패
	// 사용 시나리오: Task 실행 중 오류가 발생했을 때
	ErrTaskExecutionFailed errors.ErrorType = "TaskExecutionFailed"
)
