package provider

import (
	"fmt"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 리소스 조회 실패
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// ErrTaskNotFound 지정된 Task ID에 해당하는 Task를 찾을 수 없을 때 반환됩니다.
var ErrTaskNotFound = apperrors.New(apperrors.NotFound, "해당 작업을 찾을 수 없습니다")

// newErrTaskNotFound 지정된 Task ID에 해당하는 Task를 찾을 수 없을 때 상세 에러를 생성합니다.
//
// 매개변수:
//   - taskID: Task의 고유 식별자
//
// 반환값: Task ID를 포함한 상세 에러 메시지
func newErrTaskNotFound(taskID contract.TaskID) error {
	return apperrors.Wrapf(ErrTaskNotFound, apperrors.NotFound, "해당 작업을 찾을 수 없습니다 (task_id: %s)", taskID)
}

// ErrCommandNotFound 지정된 Task 내에서 Command ID에 해당하는 Command를 찾을 수 없을 때 반환됩니다.
var ErrCommandNotFound = apperrors.New(apperrors.NotFound, "해당 명령을 찾을 수 없습니다")

// newErrCommandNotFound 지정된 Task 내에서 Command ID에 해당하는 Command를 찾을 수 없을 때 상세 에러를 생성합니다.
//
// 매개변수:
//   - taskID: Task의 고유 식별자
//   - commandID: Command의 고유 식별자
//
// 반환값: Task ID와 Command ID를 포함한 상세 에러 메시지
func newErrCommandNotFound(taskID contract.TaskID, commandID contract.TaskCommandID) error {
	return apperrors.Wrapf(ErrCommandNotFound, apperrors.NotFound, "해당 명령을 찾을 수 없습니다 (task_id: %s, command_id: %s)", taskID, commandID)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 기능 미지원
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// ErrTaskNotSupported 지원하지 않는 Task 요청 시 반환되는 기본 에러입니다.
var ErrTaskNotSupported = apperrors.New(apperrors.InvalidInput, "지원하지 않는 작업입니다")

// NewErrTaskNotSupported 지원하지 않는 Task 요청 시 에러를 생성합니다.
//
// 매개변수:
//   - taskID: 요청된 Task의 고유 식별자
//
// 반환값: 요청된 TaskID를 포함한 상세 에러 메시지
func NewErrTaskNotSupported(taskID contract.TaskID) error {
	return apperrors.Wrapf(ErrTaskNotSupported, apperrors.InvalidInput, "지원하지 않는 작업입니다: %s", taskID)
}

// ErrCommandNotSupported 지원하지 않는 Command 요청 시 반환되는 기본 에러입니다.
var ErrCommandNotSupported = apperrors.New(apperrors.InvalidInput, "지원하지 않는 명령입니다")

// NewErrCommandNotSupported 지원하지 않는 Command 요청 시 에러를 생성합니다.
//
// 매개변수:
//   - commandID: 요청된 Command의 고유 식별자
//   - supportedCommandIDs: 해당 Task가 지원하는 Command ID 목록 (빈 배열인 경우 목록 없이 기본 메시지만 표시)
//
// 반환값: 요청된 CommandID와 지원 가능한 Command ID 목록을 포함한 상세 에러 메시지
func NewErrCommandNotSupported(commandID contract.TaskCommandID, supportedCommandIDs []contract.TaskCommandID) error {
	message := fmt.Sprintf("지원하지 않는 명령입니다: %s", commandID)
	if len(supportedCommandIDs) > 0 {
		commands := make([]string, len(supportedCommandIDs))
		for i, id := range supportedCommandIDs {
			commands[i] = string(id)
		}
		message = fmt.Sprintf("%s (사용 가능한 명령: %s)", message, strings.Join(commands, ", "))
	}
	return apperrors.Wrap(ErrCommandNotSupported, apperrors.InvalidInput, message)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 식별자 유효성 및 중복
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// newErrInvalidTaskID TaskID 유효성 검증 실패 시 상세 에러를 생성합니다.
//
// 매개변수:
//   - cause: taskID.Validate()에서 반환된 원인 에러
//   - taskID: 유효성 검증에 실패한 Task의 고유 식별자
//
// 반환값: 원인 에러를 래핑하고 TaskID를 포함한 상세 에러 메시지
func newErrInvalidTaskID(cause error, taskID contract.TaskID) error {
	return apperrors.Wrapf(cause, apperrors.InvalidInput, "유효하지 않은 TaskID입니다: %s", taskID)
}

// ErrDuplicateTaskID 중복된 TaskID가 발견된 경우 반환되는 기본 에러입니다.
var ErrDuplicateTaskID = apperrors.New(apperrors.Conflict, "중복된 TaskID입니다")

// newErrDuplicateTaskID 중복된 TaskID가 발견된 경우 상세 에러를 생성합니다.
//
// 매개변수:
//   - taskID: 이미 등록되어 있는 중복된 Task의 고유 식별자
//
// 반환값: ErrDuplicateTaskID를 래핑하고 TaskID를 포함한 상세 에러 메시지
func newErrDuplicateTaskID(taskID contract.TaskID) error {
	return apperrors.Wrapf(ErrDuplicateTaskID, apperrors.Conflict, "중복된 TaskID입니다: %s", taskID)
}

// newErrInvalidCommandID CommandID 유효성 검증 실패 시 상세 에러를 생성합니다.
//
// 매개변수:
//   - cause: commandID.Validate()에서 반환된 원인 에러
//   - commandID: 유효성 검증에 실패한 Command의 고유 식별자
//
// 반환값: 원인 에러를 래핑하고 CommandID를 포함한 상세 에러 메시지
func newErrInvalidCommandID(cause error, commandID contract.TaskCommandID) error {
	return apperrors.Wrapf(cause, apperrors.InvalidInput, "유효하지 않은 CommandID입니다: %s", commandID)
}

// ErrDuplicateCommandID 동일한 Task 내에서 중복된 CommandID가 발견된 경우 반환되는 기본 에러입니다.
var ErrDuplicateCommandID = apperrors.New(apperrors.Conflict, "중복된 CommandID입니다")

// newErrDuplicateCommandID 동일한 Task 내에서 중복된 CommandID가 발견된 경우 상세 에러를 생성합니다.
//
// 매개변수:
//   - commandID: 이미 등록되어 있는 중복된 Command의 고유 식별자
//
// 반환값: ErrDuplicateCommandID를 래핑하고 CommandID를 포함한 상세 에러 메시지
func newErrDuplicateCommandID(commandID contract.TaskCommandID) error {
	return apperrors.Wrapf(ErrDuplicateCommandID, apperrors.Conflict, "중복된 CommandID입니다: %s", commandID)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 설정 및 Registry 검증
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// ErrTaskConfigNil Task 설정 등록 시 Task 설정 객체가 nil인 경우 반환되는 에러입니다.
var ErrTaskConfigNil = apperrors.New(apperrors.InvalidInput, "Task 설정은 필수값입니다")

// ErrCommandConfigNil Task 설정 내의 Command 설정 목록 중 nil 설정이 포함된 경우 반환되는 에러입니다.
var ErrCommandConfigNil = apperrors.New(apperrors.InvalidInput, "Command 설정은 nil일 수 없습니다")

// ErrCommandConfigsEmpty Task 설정 내의 Command 설정이 하나도 없는 경우 반환되는 에러입니다.
var ErrCommandConfigsEmpty = apperrors.New(apperrors.InvalidInput, "최소 하나 이상의 Command 설정이 필요합니다")

// ErrNewTaskNil NewTask 팩토리 함수가 누락된 경우 반환되는 에러입니다.
var ErrNewTaskNil = apperrors.New(apperrors.InvalidInput, "NewTask 팩토리 함수는 필수값입니다")

// ErrNewSnapshotNil NewSnapshot 팩토리 함수가 누락된 경우 반환되는 에러입니다.
var ErrNewSnapshotNil = apperrors.New(apperrors.InvalidInput, "NewSnapshot 팩토리 함수는 필수값입니다")

// newErrSnapshotFactoryReturnedNil NewSnapshot 팩토리 함수가 nil 객체를 반환한 경우 상세 에러를 생성합니다.
//
// 이 함수는 NewSnapshot 팩토리 함수가 올바르게 구현되었는지 사전 검증합니다.
// 팩토리 함수가 nil을 반환하면 런타임에 Storage.Load/Save 시 패닉이 발생할 수 있으므로,
// 등록 단계에서 조기 차단하여 개발자의 구현 오류를 즉시 감지합니다.
//
// 매개변수:
//   - commandID: 문제가 발생한 Command의 고유 식별자
//
// 반환값: CommandID를 포함한 상세 에러 메시지
func newErrSnapshotFactoryReturnedNil(commandID contract.TaskCommandID) error {
	return apperrors.Newf(apperrors.Internal, "Command(%s)의 NewSnapshot 팩토리 함수가 nil을 반환했습니다", commandID)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 설정 처리
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// newErrTaskSettingsProcessingFailed Task의 추가 설정 정보 처리 실패 시 원인 에러를 포함한 상세 에러를 생성합니다.
//
// 이 함수는 FindTaskSettings에서 추가 설정 정보를 디코딩하거나 유효성 검증하는 과정에서
// 실패했을 때 호출되며, 원인 에러(cause)와 Task ID를 포함한 구체적인 에러 메시지를 반환합니다.
//
// 매개변수:
//   - cause: 처리 실패의 원인이 된 에러 (디코딩 실패 또는 Validate() 메서드의 검증 실패)
//   - taskID: Task의 고유 식별자
//
// 반환값: 원인 에러를 래핑하고 Task ID를 포함한 상세 에러 메시지
func newErrTaskSettingsProcessingFailed(cause error, taskID contract.TaskID) error {
	return apperrors.Wrapf(cause, apperrors.InvalidInput, "추가 설정 정보 처리에 실패했습니다 (task_id: %s)", taskID)
}

// newErrCommandSettingsProcessingFailed Command의 추가 설정 정보 처리 실패 시 원인 에러를 포함한 상세 에러를 생성합니다.
//
// 이 함수는 FindCommandSettings에서 추가 설정 정보를 디코딩하거나 유효성 검증하는 과정에서
// 실패했을 때 호출되며, 원인 에러(cause)와 Task ID, Command ID를 포함한 구체적인 에러 메시지를 반환합니다.
//
// 매개변수:
//   - cause: 처리 실패의 원인이 된 에러 (디코딩 실패 또는 Validate() 메서드의 검증 실패)
//   - taskID: Task의 고유 식별자
//   - commandID: Command의 고유 식별자
//
// 반환값: 원인 에러를 래핑하고 Task ID, Command ID를 포함한 상세 에러 메시지
func newErrCommandSettingsProcessingFailed(cause error, taskID contract.TaskID, commandID contract.TaskCommandID) error {
	return apperrors.Wrapf(cause, apperrors.InvalidInput, "추가 설정 정보 처리에 실패했습니다 (task_id: %s, command_id: %s)", taskID, commandID)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 런타임 내부 오류
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// NewErrTypeAssertionFailed snapshot의 타입 단언 실패 시 에러를 생성합니다.
//
// 이 함수는 Command 실행 시 이전 실행 결과의 snapshot을 특정 타입으로 단언할 때 실패하는 경우 호출됩니다.
// 타입 불일치는 일반적으로 Command 설정 변경, snapshot 구조 변경, 또는 코드 버그로 인해 발생하는 내부 오류입니다.
//
// 매개변수:
//   - expected: 기대했던 snapshot 타입의 제로값 (타입 정보 추출용)
//   - got: 실제로 전달된 snapshot 값
//
// 반환값: 기대 타입과 실제 타입을 포함한 상세 에러 메시지
func NewErrTypeAssertionFailed(expected, got any) error {
	return apperrors.Newf(apperrors.Internal, "snapshot의 타입 단언에 실패하였습니다 (expected: %T, got: %T)", expected, got)
}
