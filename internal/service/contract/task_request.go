package contract

import (
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// TaskRunBy 작업의 실행 주체를 정의합니다.
type TaskRunBy int

const (
	// TaskRunByUnknown 초기화되지 않았거나 알 수 없는 상태입니다 (기본값).
	TaskRunByUnknown TaskRunBy = iota

	// TaskRunByUser 사용자의 요청에 의한 수동 실행입니다.
	TaskRunByUser

	// TaskRunByScheduler 스케줄러에 의한 자동 실행입니다.
	TaskRunByScheduler
)

func (t TaskRunBy) IsValid() bool {
	switch t {
	case TaskRunByUser, TaskRunByScheduler:
		return true
	default:
		return false
	}
}

func (t TaskRunBy) Validate() error {
	if !t.IsValid() {
		return apperrors.New(apperrors.InvalidInput, "지원하지 않는 실행 주체(TaskRunBy)입니다")
	}
	return nil
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

// TaskSubmitRequest 작업을 새로 등록하거나 실행할 때 필요한 요청 정보입니다.
type TaskSubmitRequest struct {
	// TaskID 실행할 작업의 종류를 식별하는 고유 ID입니다. (필수)
	// 예: "NAVER", "KURLY"
	TaskID TaskID

	// CommandID 해당 작업 내에서 수행할 구체적인 명령어 ID입니다. (필수)
	// 예: "CheckPrice", "MonitorStock"
	CommandID TaskCommandID

	// TaskContext 작업 실행 컨텍스트입니다. (필수)
	// 작업의 생명주기 관리(취소 등)와 로깅, 알림 제목 생성 등에 사용됩니다.
	TaskContext TaskContext

	// NotifierID 알림을 전송할 대상 채널입니다. (선택)
	// 지정하지 않을 경우(빈 값), 해당 Task 설정에 정의된 기본 Notifier가 사용됩니다.
	NotifierID NotifierID

	// NotifyOnStart 작업이 시작될 때 알림을 발송할지 여부입니다.
	// true일 경우, 작업 시작 시점에 "작업 시작" 알림이 전송됩니다.
	NotifyOnStart bool

	// RunBy 작업을 요청한 실행 주체입니다.
	RunBy TaskRunBy
}

func (r *TaskSubmitRequest) Validate() error {
	if err := r.TaskID.Validate(); err != nil {
		return err
	}
	if err := r.CommandID.Validate(); err != nil {
		return err
	}
	if r.TaskContext == nil {
		return apperrors.New(apperrors.InvalidInput, "TaskContext는 필수입니다")
	}
	if len(r.NotifierID) > 0 && strings.TrimSpace(string(r.NotifierID)) == "" {
		return apperrors.New(apperrors.InvalidInput, "NotifierID는 공백으로만 구성될 수 없습니다")
	}
	if err := r.RunBy.Validate(); err != nil {
		return err
	}
	return nil
}
