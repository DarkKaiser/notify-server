package task

import (
	"strings"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
)

var (
	ErrNotSupportedTask      = apperrors.New(apperrors.ErrInvalidInput, "지원되지 않는 작업입니다")
	ErrNotSupportedCommand   = apperrors.New(apperrors.ErrInvalidInput, "지원되지 않는 작업 커맨드입니다")
	ErrNotImplementedCommand = apperrors.New(apperrors.ErrInternal, "작업 커맨드에 대한 구현이 없습니다")
)

type TaskID string
type TaskCommandID string
type TaskInstanceID string

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

type taskContextKey string

const (
	TaskCtxKeyTitle         taskContextKey = "Title"
	TaskCtxKeyErrorOccurred taskContextKey = "ErrorOccurred"

	TaskCtxKeyTaskID              taskContextKey = "Task.TaskID"
	TaskCtxKeyTaskCommandID       taskContextKey = "Task.TaskCommandID"
	TaskCtxKeyTaskInstanceID      taskContextKey = "Task.TaskInstanceID"
	TaskCtxKeyElapsedTimeAfterRun taskContextKey = "Task.ElapsedTimeAfterRun"
)

type RunBy int

const (
	RunByUser RunBy = iota
	RunByScheduler
)

func (t RunBy) String() string {
	switch t {
	case RunByUser:
		return "User"
	case RunByScheduler:
		return "Scheduler"
	default:
		return "Unknown"
	}
}

// TaskRunData
type TaskRunData struct {
	TaskID        TaskID
	TaskCommandID TaskCommandID

	TaskCtx TaskContext

	NotifierID string

	NotifyOnStart bool

	RunBy RunBy
}

// Runner
type Runner interface {
	Run(taskRunData *TaskRunData) (succeeded bool)
}

// Canceler
type Canceler interface {
	Cancel(taskInstanceID TaskInstanceID) (succeeded bool)
}

// Executor
type Executor interface {
	Runner
	Canceler
}

// TaskNotificationSender
type TaskNotificationSender interface {
	NotifyToDefault(message string) bool
	NotifyWithTaskContext(notifierID string, message string, taskCtx TaskContext) bool

	SupportsHTMLMessage(notifierID string) bool
}
