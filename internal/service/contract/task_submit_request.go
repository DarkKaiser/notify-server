package contract

import (
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// TaskSubmitRequest 작업 실행 요청을 캡슐화한 데이터 전송 객체(DTO)입니다.
//
// API 서비스 또는 내부 스케줄러로부터 작업 실행 명령을 받을 때 사용되며,
// 실행할 작업의 종류, 세부 명령어, 알림 옵션, 실행 주체 등 작업 컨텍스트 설정에 필요한 모든 정보를 포함합니다.
type TaskSubmitRequest struct {
	// TaskID 실행하고자 하는 작업의 종류를 식별하는 고유 ID입니다. (Required)
	// 예: "NAVER", "KURLY"
	TaskID TaskID

	// CommandID 해당 작업 내에서 수행할 구체적인 명령어 ID입니다. (Required)
	// 예: "CheckPrice", "MonitorStock"
	CommandID TaskCommandID

	// NotifierID 알림을 전송할 대상 채널(Notifier)의 식별자입니다. (Optional)
	// 특정 메신저나 채널로 알림을 강제하고 싶을 때 사용합니다.
	// 이 값을 지정하지 않거나 빈 문자열("")일 경우, 해당 Task 설정에 정의된 기본 Notifier가 자동으로 사용됩니다.
	NotifierID NotifierID

	// NotifyOnStart 작업 실행 시작 시점에 즉시 알림을 발송할지 여부를 결정합니다. (Optional)
	// - true: 작업이 큐에서 꺼내져 실행되는 즉시 "작업 시작" 알림을 전송합니다. 장기 실행 작업의 경우 즉각적인 피드백을 제공하여 UX를 향상시킬 수 있습니다.
	// - false: 시작 알림을 보내지 않고, 작업 결과(성공/실패)에 대한 알림만 전송합니다. (기본값)
	NotifyOnStart bool

	// RunBy 이 작업을 요청한 실행 주체를 나타냅니다. (Required)
	// 로깅, 감사, 그리고 알림 메시지 포맷팅 시 "누가 실행했는지"를 구별하기 위해 사용됩니다.
	// 예: TaskRunByUser(사용자 수동 실행), TaskRunByScheduler(스케줄러 자동 실행)
	RunBy TaskRunBy
}

func (r *TaskSubmitRequest) Validate() error {
	if err := r.TaskID.Validate(); err != nil {
		return err
	}
	if err := r.CommandID.Validate(); err != nil {
		return err
	}
	if len(r.NotifierID) > 0 && strings.TrimSpace(string(r.NotifierID)) == "" {
		return apperrors.New(apperrors.InvalidInput, "NotifierID 유효성 검증 실패: 공백 이외의 유효한 문자열을 포함해야 합니다")
	}
	if err := r.RunBy.Validate(); err != nil {
		return err
	}
	return nil
}
