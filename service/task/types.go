package task

import (
	"strings"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
)

type TaskID string
type TaskCommandID string
type TaskInstanceID string
type TaskRunBy int

// Match 주어진 commandID가 현재 commandID 패턴과 일치하는지 확인합니다.
// '*' 와일드카드를 지원하여 접두사가 일치하면 true를 반환합니다.
func (id TaskCommandID) Match(target TaskCommandID) bool {
	const wildcard = "*"

	s := string(id)
	if strings.HasSuffix(s, wildcard) {
		prefix := strings.TrimSuffix(s, wildcard)
		return strings.HasPrefix(string(target), prefix)
	}

	return id == target
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

type taskContextKey string

const (
	TaskCtxKeyTitle         taskContextKey = "Title"
	TaskCtxKeyErrorOccurred taskContextKey = "ErrorOccurred"

	TaskCtxKeyTaskID              taskContextKey = "Task.TaskID"
	TaskCtxKeyTaskCommandID       taskContextKey = "Task.TaskCommandID"
	TaskCtxKeyTaskInstanceID      taskContextKey = "Task.TaskInstanceID"
	TaskCtxKeyElapsedTimeAfterRun taskContextKey = "Task.ElapsedTimeAfterRun"
)

const (
	TaskRunByUser TaskRunBy = iota
	TaskRunByScheduler
)

var (
	ErrNotSupportedTask               = apperrors.New(apperrors.ErrInvalidInput, "지원되지 않는 작업입니다")
	ErrNotSupportedCommand            = apperrors.New(apperrors.ErrInvalidInput, "지원되지 않는 작업 커맨드입니다")
	ErrNoImplementationForTaskCommand = apperrors.New(apperrors.ErrInternal, "작업 커맨드에 대한 구현이 없습니다")
)

// TaskRunData
type TaskRunData struct {
	TaskID        TaskID
	TaskCommandID TaskCommandID

	TaskCtx TaskContext

	NotifierID string

	NotifyResultOfTaskRunRequest bool

	TaskRunBy TaskRunBy
}

// TaskExecutor
type TaskExecutor interface {
	TaskRun(taskID TaskID, taskCommandID TaskCommandID, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) (succeeded bool)
	TaskRunWithContext(taskID TaskID, taskCommandID TaskCommandID, taskCtx TaskContext, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) (succeeded bool)
}

// TaskCanceler
type TaskCanceler interface {
	TaskCancel(taskInstanceID TaskInstanceID) (succeeded bool)
}

// TaskRunner
type TaskRunner interface {
	TaskExecutor
	TaskCanceler
}

// TaskNotificationSender
type TaskNotificationSender interface {
	NotifyToDefault(message string) bool
	NotifyWithTaskContext(notifierID string, message string, taskCtx TaskContext) bool

	SupportsHTMLMessage(notifierID string) bool
}
