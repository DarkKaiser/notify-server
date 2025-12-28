package task

import (
	"strings"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
)

// ID 작업의 고유 식별자입니다.
type ID string

func (id ID) IsEmpty() bool {
	return len(id) == 0
}

func (id ID) Validate() error {
	if len(id) == 0 {
		return apperrors.New(apperrors.InvalidInput, "ID는 필수입니다")
	}
	return nil
}

func (id ID) String() string {
	return string(id)
}

// CommandID 작업 내에서 실행할 구체적인 명령어의 식별자입니다.
type CommandID string

func (id CommandID) IsEmpty() bool {
	return len(id) == 0
}

func (id CommandID) Validate() error {
	if len(id) == 0 {
		return apperrors.New(apperrors.InvalidInput, "CommandID는 필수입니다")
	}
	return nil
}

// Match 대상 커맨드 ID(target)가 현재 커맨드 ID와 일치하는지, 또는 정의된 패턴에 부합하는지 검증합니다.
//
// 단순 일치(Exact Match)뿐만 아니라, 접미사 와일드카드('*')를 사용한 접두어 매칭(Prefix Match)을 지원합니다.
// 예: "CMD_*"는 "CMD_A", "CMD_B" 등과 일치한다고 판단합니다.
func (id CommandID) Match(target CommandID) bool {
	const wildcard = "*"

	s := string(id)
	if strings.HasSuffix(s, wildcard) {
		prefix := strings.TrimSuffix(s, wildcard)
		return strings.HasPrefix(string(target), prefix)
	}

	return id == target
}

func (id CommandID) String() string {
	return string(id)
}

// InstanceID 실행 중인 작업 인스턴스의 고유 식별자입니다.
type InstanceID string

func (id InstanceID) IsEmpty() bool {
	return len(id) == 0
}

func (id InstanceID) Validate() error {
	if len(id) == 0 {
		return apperrors.New(apperrors.InvalidInput, "InstanceID는 필수입니다")
	}
	return nil
}

func (id InstanceID) String() string {
	return string(id)
}

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

// IsValid 유효한 RunBy 값인지 확인합니다.
func (t RunBy) IsValid() bool {
	switch t {
	case RunByUser, RunByScheduler:
		return true
	default:
		return false
	}
}

// Validate 유효한 실행 주체인지 검증합니다.
func (t RunBy) Validate() error {
	if !t.IsValid() {
		return apperrors.New(apperrors.InvalidInput, "유효하지 않은 실행 주체(RunBy)입니다")
	}
	return nil
}

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

// SubmitRequest 작업 식별자, 커맨드, 컨텍스트 등 작업(Task) 실행에 필요한 모든 메타데이터와 요청 정보를 캡슐화한 구조체입니다.
// Scheduler 또는 API 요청 등을 통해 작업을 트리거할 때 사용됩니다.
type SubmitRequest struct {
	// TaskID 실행할 작업의 고유 식별자입니다. (예: "NAVER", "KURLY")
	TaskID ID

	// CommandID 작업 내에서 수행할 구체적인 명령어 식별자입니다. (예: "CheckPrice")
	CommandID CommandID

	// TaskContext 작업 실행 컨텍스트입니다.
	// 실행 흐름 전반에 걸쳐 메타데이터(Title, ID 등)를 전달하고, 취소 신호(Cancellation)를 전파하는 데 사용됩니다.
	TaskContext TaskContext

	// NotifierID 알림을 전송할 대상 채널 또는 수단(Notifier)의 식별자입니다.
	// 지정하지 않을 경우, Task 설정에 정의된 기본 Notifier가 사용됩니다.
	NotifierID string

	// NotifyOnStart 작업 시작 시점에 '시작 알림'을 발송할지 여부를 결정하는 플래그입니다.
	NotifyOnStart bool

	// RunBy 해당 작업을 누가/무엇이 실행 요청했는지를 나타냅니다.
	// (예: RunByUser - 사용자 수동 실행, RunByScheduler - 스케줄러 자동 실행)
	RunBy RunBy
}

// Validate 유효한 요청인지 검증합니다.
func (r *SubmitRequest) Validate() error {
	if err := r.TaskID.Validate(); err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, "TaskID 검증 실패")
	}
	if err := r.CommandID.Validate(); err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, "CommandID 검증 실패")
	}
	if len(r.NotifierID) > 0 && strings.TrimSpace(r.NotifierID) == "" {
		return apperrors.New(apperrors.InvalidInput, "NotifierID는 공백일 수 없습니다")
	}
	if err := r.RunBy.Validate(); err != nil {
		return err
	}
	return nil
}
