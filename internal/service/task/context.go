package task

import (
	"context"
)

// ctxKey TaskContext에서 값을 저장하고 조회할 때 사용하는 키 타입입니다.
type ctxKey string

const (
	// ctxKeyID 작업 ID를 저장하는 키입니다.
	ctxKeyID ctxKey = "Task.ID"
	// ctxKeyCommandID 작업 커맨드 ID를 저장하는 키입니다.
	ctxKeyCommandID ctxKey = "Task.CommandID"
	// ctxKeyInstanceID 작업 인스턴스 ID를 저장하는 키입니다.
	ctxKeyInstanceID ctxKey = "Task.InstanceID"
	// ctxKeyTitle 알림 메시지의 제목을 지정하는 키입니다.
	ctxKeyTitle ctxKey = "Title"
	// ctxKeyCancelable 작업의 취소 가능 여부를 제어하는 컨텍스트 키입니다.
	ctxKeyCancelable ctxKey = "Cancelable"
	// ctxKeyElapsedTimeAfterRun 작업 실행 후 경과 시간을 저장하는 키입니다.
	ctxKeyElapsedTimeAfterRun ctxKey = "Task.ElapsedTimeAfterRun"
	// ctxKeyErrorOccurred 작업 실행 중 에러 발생 여부를 나타내는 키입니다.
	ctxKeyErrorOccurred ctxKey = "ErrorOccurred"
)

// TaskContext 작업 실행 흐름 전반에서 메타데이터를 전달하는 컨텍스트 인터페이스입니다.
type TaskContext interface {
	context.Context // 표준 Context 인터페이스 임베딩 (DeadLine, Done, Err, Value 등 지원)

	With(key, val interface{}) TaskContext
	WithTask(taskID ID, commandID CommandID) TaskContext
	WithInstanceID(instanceID InstanceID, elapsedTimeAfterRun int64) TaskContext
	WithTitle(title string) TaskContext
	WithCancelable(cancelable bool) TaskContext
	WithError() TaskContext

	GetID() ID
	GetCommandID() CommandID
	GetInstanceID() InstanceID
	GetTitle() string
	GetElapsedTimeAfterRun() int64
	IsCancelable() bool
	IsErrorOccurred() bool
}

// taskContext TaskContext 인터페이스의 구현체입니다.
// 불변성(Immutability)을 보장하기 위해 모든 With 메서드는 새로운 인스턴스를 반환합니다.
type taskContext struct {
	context.Context // 표준 Context 구현체 임베딩 (자동 델리게이션)
}

// NewTaskContext 새로운 TaskContext를 생성합니다.
func NewTaskContext() TaskContext {
	return &taskContext{
		Context: context.Background(),
	}
}

// With 키-값 쌍을 저장한 새로운 TaskContext를 반환합니다.
func (c *taskContext) With(key, val interface{}) TaskContext {
	return &taskContext{
		Context: context.WithValue(c.Context, key, val),
	}
}

// WithTask 작업 및 커맨드 식별자를 컨텍스트에 추가합니다.
func (c *taskContext) WithTask(taskID ID, commandID CommandID) TaskContext {
	ctx := context.WithValue(c.Context, ctxKeyID, taskID)
	ctx = context.WithValue(ctx, ctxKeyCommandID, commandID)
	return &taskContext{Context: ctx}
}

// WithInstanceID 실행 인스턴스 정보(ID, 경과 시간)를 컨텍스트에 추가합니다.
func (c *taskContext) WithInstanceID(instanceID InstanceID, elapsedTimeAfterRun int64) TaskContext {
	ctx := context.WithValue(c.Context, ctxKeyInstanceID, instanceID)
	ctx = context.WithValue(ctx, ctxKeyElapsedTimeAfterRun, elapsedTimeAfterRun)
	return &taskContext{Context: ctx}
}

// WithTitle 알림 제목을 컨텍스트에 추가합니다.
func (c *taskContext) WithTitle(title string) TaskContext {
	return &taskContext{
		Context: context.WithValue(c.Context, ctxKeyTitle, title),
	}
}

// WithCancelable 취소 가능 여부를 컨텍스트에 추가합니다.
func (c *taskContext) WithCancelable(cancelable bool) TaskContext {
	return &taskContext{
		Context: context.WithValue(c.Context, ctxKeyCancelable, cancelable),
	}
}

// WithError 에러 발생 플래그를 true로 설정하여 컨텍스트에 추가합니다.
func (c *taskContext) WithError() TaskContext {
	return &taskContext{
		Context: context.WithValue(c.Context, ctxKeyErrorOccurred, true),
	}
}

// GetID 컨텍스트에서 Task ID를 안전하게 타입 캐스팅하여 반환합니다.
func (c *taskContext) GetID() ID {
	if v, ok := c.Context.Value(ctxKeyID).(ID); ok {
		return v
	}
	return ""
}

// GetCommandID 컨텍스트에서 Command ID를 안전하게 타입 캐스팅하여 반환합니다.
func (c *taskContext) GetCommandID() CommandID {
	if v, ok := c.Context.Value(ctxKeyCommandID).(CommandID); ok {
		return v
	}
	return ""
}

// GetInstanceID 컨텍스트에서 Instance ID를 안전하게 타입 캐스팅하여 반환합니다.
func (c *taskContext) GetInstanceID() InstanceID {
	if v, ok := c.Context.Value(ctxKeyInstanceID).(InstanceID); ok {
		return v
	}
	return ""
}

// GetTitle 컨텍스트에서 제목을 반환합니다.
func (c *taskContext) GetTitle() string {
	if v, ok := c.Context.Value(ctxKeyTitle).(string); ok {
		return v
	}
	return ""
}

// GetElapsedTimeAfterRun 컨텍스트에서 경과 시간을 반환합니다. (기본값: 0)
func (c *taskContext) GetElapsedTimeAfterRun() int64 {
	if v, ok := c.Context.Value(ctxKeyElapsedTimeAfterRun).(int64); ok {
		return v
	}
	return 0
}

// IsCancelable 취소 가능 여부를 확인합니다. (기본값: false)
func (c *taskContext) IsCancelable() bool {
	if v, ok := c.Context.Value(ctxKeyCancelable).(bool); ok {
		return v
	}
	return false
}

// IsErrorOccurred 에러 발생 여부를 확인합니다. (기본값: false)
func (c *taskContext) IsErrorOccurred() bool {
	if v, ok := c.Context.Value(ctxKeyErrorOccurred).(bool); ok {
		return v
	}
	return false
}
