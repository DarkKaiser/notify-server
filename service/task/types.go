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

// ID 작업의 고유 식별자입니다. (예: "Lotto")
type ID string

// CommandID 작업 내에서 실행할 구체적인 명령어의 식별자입니다. (예: "Check")
type CommandID string

// InstanceID 실행 중인 작업 인스턴스의 고유 식별자입니다.
type InstanceID string

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
	// TaskCtxKeyTitle 알림 메시지의 제목을 지정하는 키입니다.
	TaskCtxKeyTitle taskContextKey = "Title"
	// TaskCtxKeyErrorOccurred 작업 실행 중 에러 발생 여부를 나타내는 키입니다.
	TaskCtxKeyErrorOccurred taskContextKey = "ErrorOccurred"

	// TaskCtxKeyID 작업 ID를 저장하는 키입니다.
	TaskCtxKeyID taskContextKey = "Task.ID"
	// TaskCtxKeyCommandID 작업 커맨드 ID를 저장하는 키입니다.
	TaskCtxKeyCommandID taskContextKey = "Task.CommandID"
	// TaskCtxKeyInstanceID 작업 인스턴스 ID를 저장하는 키입니다.
	TaskCtxKeyInstanceID taskContextKey = "Task.InstanceID"
	// TaskCtxKeyElapsedTimeAfterRun 작업 실행 후 경과 시간을 저장하는 키입니다.
	TaskCtxKeyElapsedTimeAfterRun taskContextKey = "Task.ElapsedTimeAfterRun"
)

// RunBy 누가 작업을 실행했는지를 나타내는 타입입니다.
type RunBy int

const (
	// RunByUser 사용자가 직접 실행 요청한 경우입니다.
	RunByUser RunBy = iota
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
