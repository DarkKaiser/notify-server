package task

import (
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
)

type TaskID string
type TaskCommandID string
type TaskInstanceID string
type TaskRunBy int

// TaskCommandID의 마지막에 들어가는 특별한 문자
// 이 문자는 환경설정 파일(JSON)에서는 사용되지 않으며 오직 소스코드 상에서만 사용한다.
const taskCommandIDAnyString string = "*"

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
