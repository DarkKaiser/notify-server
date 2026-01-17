package contract

import (
	"context"
)

// ctxKey TaskContext에서 값을 저장하고 조회할 때 사용하는 키 타입입니다.
type ctxKey string

const (
	// ctxKeyTaskID 작업 ID를 저장하는 키입니다.
	ctxKeyTaskID ctxKey = "Task.ID"
	// ctxKeyCommandID 작업 명령 ID를 저장하는 키입니다.
	ctxKeyCommandID ctxKey = "Task.CommandID"
	// ctxKeyInstanceID 작업 인스턴스 ID를 저장하는 키입니다.
	ctxKeyInstanceID ctxKey = "Task.InstanceID"
	// ctxKeyTitle 알림 메시지의 제목을 저장하는 키입니다.
	ctxKeyTitle ctxKey = "Title"
	// ctxKeyElapsedTimeAfterRun 작업 실행 후 경과 시간을 저장하는 키입니다.
	ctxKeyElapsedTimeAfterRun ctxKey = "Task.ElapsedTimeAfterRun"
	// ctxKeyCancelable 작업의 취소 가능 여부를 저장하는 키입니다.
	ctxKeyCancelable ctxKey = "Cancelable"
	// ctxKeyErrorOccurred 작업 실행 중 에러 발생 여부를 저장하는 키입니다.
	ctxKeyErrorOccurred ctxKey = "ErrorOccurred"
)

// TaskContext 작업의 생명주기 동안 필요한 메타데이터(TaskID, CommandID 등)를 관리하고 전달하는 컨텍스트입니다.
type TaskContext interface {
	context.Context

	With(key, val interface{}) TaskContext
	WithTask(taskID TaskID, commandID TaskCommandID) TaskContext
	WithTaskInstanceID(instanceID TaskInstanceID, elapsedTimeAfterRun int64) TaskContext
	WithTitle(title string) TaskContext
	WithCancelable(cancelable bool) TaskContext
	WithError() TaskContext

	GetTaskID() TaskID
	GetTaskCommandID() TaskCommandID
	GetTaskInstanceID() TaskInstanceID
	GetTitle() string
	GetElapsedTimeAfterRun() int64
	IsCancelable() bool
	IsErrorOccurred() bool
}

// taskContext TaskContext 인터페이스의 구현체입니다.
// 불변성(Immutability)을 보장하기 위해 모든 With 메서드는 새로운 인스턴스를 반환합니다.
type taskContext struct {
	context.Context

	taskID     TaskID
	commandID  TaskCommandID
	instanceID TaskInstanceID
}

// NewTaskContext 새로운 TaskContext를 생성합니다.
func NewTaskContext() TaskContext {
	return &taskContext{
		Context: context.Background(),
	}
}

func (c *taskContext) With(key, val interface{}) TaskContext {
	return &taskContext{
		Context: context.WithValue(c.Context, key, val),

		taskID:     c.taskID,
		commandID:  c.commandID,
		instanceID: c.instanceID,
	}
}

func (c *taskContext) WithTask(taskID TaskID, commandID TaskCommandID) TaskContext {
	return &taskContext{
		Context: context.WithValue(context.WithValue(c.Context, ctxKeyTaskID, taskID), ctxKeyCommandID, commandID),

		taskID:     taskID,
		commandID:  commandID,
		instanceID: c.instanceID,
	}
}

func (c *taskContext) WithTaskInstanceID(instanceID TaskInstanceID, elapsedTimeAfterRun int64) TaskContext {
	return &taskContext{
		Context: context.WithValue(context.WithValue(c.Context, ctxKeyInstanceID, instanceID), ctxKeyElapsedTimeAfterRun, elapsedTimeAfterRun),

		taskID:     c.taskID,
		commandID:  c.commandID,
		instanceID: instanceID,
	}
}

func (c *taskContext) WithTitle(title string) TaskContext {
	return &taskContext{
		Context: context.WithValue(c.Context, ctxKeyTitle, title),

		taskID:     c.taskID,
		commandID:  c.commandID,
		instanceID: c.instanceID,
	}
}

func (c *taskContext) WithCancelable(cancelable bool) TaskContext {
	return &taskContext{
		Context: context.WithValue(c.Context, ctxKeyCancelable, cancelable),

		taskID:     c.taskID,
		commandID:  c.commandID,
		instanceID: c.instanceID,
	}
}

func (c *taskContext) WithError() TaskContext {
	return &taskContext{
		Context: context.WithValue(c.Context, ctxKeyErrorOccurred, true),

		taskID:     c.taskID,
		commandID:  c.commandID,
		instanceID: c.instanceID,
	}
}

func (c *taskContext) GetTaskID() TaskID {
	if c.taskID != "" {
		return c.taskID
	}
	if v, ok := c.Context.Value(ctxKeyTaskID).(TaskID); ok {
		return v
	}
	return ""
}

func (c *taskContext) GetTaskCommandID() TaskCommandID {
	if c.commandID != "" {
		return c.commandID
	}
	if v, ok := c.Context.Value(ctxKeyCommandID).(TaskCommandID); ok {
		return v
	}
	return ""
}

func (c *taskContext) GetTaskInstanceID() TaskInstanceID {
	if c.instanceID != "" {
		return c.instanceID
	}
	if v, ok := c.Context.Value(ctxKeyInstanceID).(TaskInstanceID); ok {
		return v
	}
	return ""
}

func (c *taskContext) GetTitle() string {
	if v, ok := c.Context.Value(ctxKeyTitle).(string); ok {
		return v
	}
	return ""
}

func (c *taskContext) GetElapsedTimeAfterRun() int64 {
	if v, ok := c.Context.Value(ctxKeyElapsedTimeAfterRun).(int64); ok {
		return v
	}
	return 0
}

func (c *taskContext) IsCancelable() bool {
	if v, ok := c.Context.Value(ctxKeyCancelable).(bool); ok {
		return v
	}
	return false
}

func (c *taskContext) IsErrorOccurred() bool {
	if v, ok := c.Context.Value(ctxKeyErrorOccurred).(bool); ok {
		return v
	}
	return false
}
