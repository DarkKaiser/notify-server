package task

import (
	"context"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
)

var (
	ErrNotSupportedTask      = apperrors.New(apperrors.ErrInvalidInput, "지원되지 않는 작업입니다")
	ErrNotSupportedCommand   = apperrors.New(apperrors.ErrInvalidInput, "지원되지 않는 작업 커맨드입니다")
	ErrNotImplementedCommand = apperrors.New(apperrors.ErrInternal, "작업 커맨드에 대한 구현이 없습니다")
)

// ID 작업의 고유 식별자입니다.
type ID string

func (id ID) IsEmpty() bool {
	return len(id) == 0
}

func (id ID) String() string {
	return string(id)
}

// CommandID 작업 내에서 실행할 구체적인 명령어의 식별자입니다.
type CommandID string

func (id CommandID) IsEmpty() bool {
	return len(id) == 0
}

func (id CommandID) String() string {
	return string(id)
}

// InstanceID 실행 중인 작업 인스턴스의 고유 식별자입니다.
type InstanceID string

func (id InstanceID) IsEmpty() bool {
	return len(id) == 0
}

func (id InstanceID) String() string {
	return string(id)
}

// Match 주어진 대상 커맨드 ID(target)가 현재 커맨드 ID 패턴과 일치하는지 확인합니다.
// 와일드카드('*') 접미사를 지원하여, 패턴 매칭을 수행할 수 있습니다.
func (id CommandID) Match(target CommandID) bool {
	const wildcard = "*"

	s := string(id)
	if strings.HasSuffix(s, wildcard) {
		prefix := strings.TrimSuffix(s, wildcard)
		return strings.HasPrefix(string(target), prefix)
	}

	return id == target
}

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

// RunBy 누가 작업을 실행했는지를 나타내는 타입입니다.
type RunBy int

const (
	// RunByUnknown 실행 주체가 명확하지 않은 상태 (Zero Value 안전성 확보)
	RunByUnknown RunBy = iota
	// RunByUser 사용자가 직접 실행 요청한 경우입니다.
	RunByUser
	// RunByScheduler 스케줄러에 의해 자동으로 실행된 경우입니다.
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

// RunRequest 작업 실행 요청 정보를 담고 있는 구조체입니다.
type RunRequest struct {
	TaskID        ID
	TaskCommandID CommandID

	// TaskCtx 작업 실행 컨텍스트입니다. 실행 중 필요한 메타데이터를 전달하는 데 사용됩니다.
	TaskCtx TaskContext

	NotifierID string

	// NotifyOnStart 작업 시작 시 알림을 보낼지 여부입니다.
	NotifyOnStart bool

	RunBy RunBy
}

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

type Runner interface {
	// Run 작업을 실행합니다. 실행 성공 여부를 반환합니다.
	Run(req *RunRequest) (succeeded bool)
}

type Canceler interface {
	// Cancel 특정 작업 인스턴스를 취소합니다. 취소 성공 여부를 반환합니다.
	Cancel(taskInstanceID InstanceID) (succeeded bool)
}

type Executor interface {
	Runner
	Canceler
}

// TaskNotificationSender 작업 상태나 결과를 알림으로 전송하는 인터페이스입니다.
type TaskNotificationSender interface {
	// NotifyToDefault 기본 설정된 알림 채널로 메시지를 전송합니다.
	NotifyToDefault(message string) bool
	// NotifyWithTaskContext 특정 알림 채널로 컨텍스트 정보와 함께 메시지를 전송합니다.
	NotifyWithTaskContext(notifierID string, message string, taskCtx TaskContext) bool

	// SupportsHTMLMessage 해당 알림 채널이 HTML 메시지 형식을 지원하는지 확인합니다.
	SupportsHTMLMessage(notifierID string) bool
}
