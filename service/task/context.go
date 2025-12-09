package task

import (
	"context"
)

// taskContextKey TaskContext에서 값을 저장하고 조회할 때 사용하는 키 타입입니다.
type taskContextKey string

const (
	// taskCtxKeyTitle 알림 메시지의 제목을 지정하는 키입니다.
	taskCtxKeyTitle taskContextKey = "Title"
	// taskCtxKeyErrorOccurred 작업 실행 중 에러 발생 여부를 나타내는 키입니다.
	taskCtxKeyErrorOccurred taskContextKey = "ErrorOccurred"

	// taskCtxKeyID 작업 ID를 저장하는 키입니다.
	taskCtxKeyID taskContextKey = "Task.ID"
	// taskCtxKeyCommandID 작업 커맨드 ID를 저장하는 키입니다.
	taskCtxKeyCommandID taskContextKey = "Task.CommandID"
	// taskCtxKeyInstanceID 작업 인스턴스 ID를 저장하는 키입니다.
	taskCtxKeyInstanceID taskContextKey = "Task.InstanceID"
	// taskCtxKeyElapsedTimeAfterRun 작업 실행 후 경과 시간을 저장하는 키입니다.
	taskCtxKeyElapsedTimeAfterRun taskContextKey = "Task.ElapsedTimeAfterRun"
)

// TaskContext 작업 실행에 필요한 컨텍스트 정보를 정의하는 인터페이스입니다.
type TaskContext interface {
	// With 주어진 키와 값으로 새로운 컨텍스트를 생성하여 반환합니다.
	With(key, val interface{}) TaskContext
	// WithTask 작업 ID와 커맨드 ID를 포함하는 새로운 컨텍스트를 생성하여 반환합니다.
	WithTask(taskID ID, taskCommandID CommandID) TaskContext
	// WithInstanceID 작업 인스턴스 ID와 실행 후 경과 시간을 포함하는 새로운 컨텍스트를 생성하여 반환합니다.
	WithInstanceID(taskInstanceID InstanceID, elapsedTimeAfterRun int64) TaskContext
	// WithTitle 알림 메시지의 제목을 설정한 새로운 컨텍스트를 생성하여 반환합니다.
	WithTitle(title string) TaskContext
	// WithError 에러 발생 여부를 설정한 새로운 컨텍스트를 생성하여 반환합니다.
	WithError() TaskContext

	// Value Context에서 해당 키의 값을 반환합니다.
	Value(key interface{}) interface{}

	// GetID Context에 저장된 작업 ID를 반환합니다.
	GetID() ID
	// GetCommandID Context에 저장된 작업 커맨드 ID를 반환합니다.
	GetCommandID() CommandID
	// GetInstanceID Context에 저장된 작업 인스턴스 ID를 반환합니다.
	GetInstanceID() InstanceID
	// GetElapsedTimeAfterRun Context에 저장된 작업 실행 후 경과 시간을 반환합니다.
	GetElapsedTimeAfterRun() int64
	// IsErrorOccurred Context에 에러 발생 여부가 설정되었는지 확인합니다.
	IsErrorOccurred() bool
	// GetTitle Context에 저장된 제목을 반환합니다.
	GetTitle() string
}

// taskContext TaskContext 인터페이스의 구현체입니다.
type taskContext struct {
	ctx context.Context
}

// NewTaskContext 새로운 TaskContext를 생성합니다.
func NewTaskContext() TaskContext {
	return &taskContext{
		ctx: context.Background(),
	}
}

func (c *taskContext) With(key, val interface{}) TaskContext {
	c.ctx = context.WithValue(c.ctx, key, val)
	return c
}

func (c *taskContext) WithTask(taskID ID, taskCommandID CommandID) TaskContext {
	c.ctx = context.WithValue(c.ctx, taskCtxKeyID, taskID)
	c.ctx = context.WithValue(c.ctx, taskCtxKeyCommandID, taskCommandID)
	return c
}

func (c *taskContext) WithInstanceID(taskInstanceID InstanceID, elapsedTimeAfterRun int64) TaskContext {
	c.ctx = context.WithValue(c.ctx, taskCtxKeyInstanceID, taskInstanceID)
	c.ctx = context.WithValue(c.ctx, taskCtxKeyElapsedTimeAfterRun, elapsedTimeAfterRun)
	return c
}

func (c *taskContext) WithTitle(title string) TaskContext {
	c.ctx = context.WithValue(c.ctx, taskCtxKeyTitle, title)
	return c
}

func (c *taskContext) WithError() TaskContext {
	c.ctx = context.WithValue(c.ctx, taskCtxKeyErrorOccurred, true)
	return c
}

func (c *taskContext) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}

func (c *taskContext) GetID() ID {
	if v, ok := c.ctx.Value(taskCtxKeyID).(ID); ok {
		return v
	}
	return ""
}

func (c *taskContext) GetCommandID() CommandID {
	if v, ok := c.ctx.Value(taskCtxKeyCommandID).(CommandID); ok {
		return v
	}
	return ""
}

func (c *taskContext) GetInstanceID() InstanceID {
	if v, ok := c.ctx.Value(taskCtxKeyInstanceID).(InstanceID); ok {
		return v
	}
	return ""
}

func (c *taskContext) GetElapsedTimeAfterRun() int64 {
	if v, ok := c.ctx.Value(taskCtxKeyElapsedTimeAfterRun).(int64); ok {
		return v
	}
	return 0
}

func (c *taskContext) IsErrorOccurred() bool {
	if v, ok := c.ctx.Value(taskCtxKeyErrorOccurred).(bool); ok {
		return v
	}
	return false
}

func (c *taskContext) GetTitle() string {
	if v, ok := c.ctx.Value(taskCtxKeyTitle).(string); ok {
		return v
	}
	return ""
}
