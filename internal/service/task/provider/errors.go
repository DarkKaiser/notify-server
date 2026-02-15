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
// 설정 및 Registry 검증 (정적 검증)
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
// 설정 처리 (파싱 및 디코딩)
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
// 실행 전 의존성 및 초기화 검증 (prepareExecution)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// newErrExecuteFuncNotInitialized Task의 핵심 비즈니스 로직(ExecuteFunc)이 주입되지 않았을 때 상세 에러를 생성합니다.
//
// 이 함수는 prepareExecution 단계에서 execute 함수가 nil인지 검증할 때 호출됩니다.
// ExecuteFunc는 개별 Task 구현체에서 SetExecute()를 통해 주입되어야 하며,
// 이 에러는 Task 생성 시점의 의존성 주입 누락(개발자 실수)을 의미합니다.
//
// 매개변수:
//   - taskID: Task의 고유 식별자
//   - commandID: Command의 고유 식별자
//
// 반환값: Task ID와 Command ID를 포함한 상세 에러 메시지
func newErrExecuteFuncNotInitialized(taskID contract.TaskID, commandID contract.TaskCommandID) error {
	return apperrors.Newf(apperrors.Internal, "%s (task_id: %s, command_id: %s)", errMsgExecuteFuncNotInitialized, taskID, commandID)
}

// newErrScraperNotInitialized 스크래핑이 필요한 Task임에도 Scraper 의존성이 주입되지 않았을 때 상세 에러를 생성합니다.
//
// 이 함수는 prepareExecution 단계에서 requireScraper가 true인데 scraper가 nil인 경우 호출됩니다.
// 웹 스크래핑 기능이 필요한 Task는 반드시 Fetcher를 주입받아 Scraper를 초기화해야 하며,
// 이 에러는 Task 생성 시점의 의존성 주입 누락(개발자 실수)을 의미합니다.
//
// 매개변수:
//   - taskID: Task의 고유 식별자
//   - commandID: Command의 고유 식별자
//
// 반환값: Task ID와 Command ID를 포함한 상세 에러 메시지
func newErrScraperNotInitialized(taskID contract.TaskID, commandID contract.TaskCommandID) error {
	return apperrors.Newf(apperrors.Internal, "%s (task_id: %s, command_id: %s)", errMsgScraperNotInitialized, taskID, commandID)
}

// newErrStorageNotInitialized 스냅샷 관리가 필요한 Task임에도 Storage 의존성이 주입되지 않았을 때 상세 에러를 생성합니다.
//
// 이 함수는 다음 두 상황에서 호출됩니다:
//  1. prepareExecution: newSnapshot이 존재하여 이전 스냅샷을 로드하려는데 storage가 nil인 경우
//  2. finalizeExecution: 새로운 스냅샷을 저장하려는데 storage가 nil인 경우
//
// NewSnapshot 팩토리 함수가 등록되어 스냅샷을 관리하겠다는 의도를 보였음에도
// 실제 저장소(Storage)를 주입하지 않은 것은 명백한 구현 실수(버그)입니다.
//
// 매개변수:
//   - taskID: Task의 고유 식별자
//   - commandID: Command의 고유 식별자
//
// 반환값: Task ID와 Command ID를 포함한 상세 에러 메시지
func newErrStorageNotInitialized(taskID contract.TaskID, commandID contract.TaskCommandID) error {
	return apperrors.Newf(apperrors.Internal, "%s (task_id: %s, command_id: %s)", errMsgStorageNotInitialized, taskID, commandID)
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 런타임 및 실행 오류
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// newErrSnapshotCreationFailed 새로운 스냅샷 객체 생성 실패 시 상세 에러를 생성합니다.
//
// 이 함수는 prepareExecution 단계에서 newSnapshot() 팩토리 함수를 호출했는데
// nil이 반환된 경우에 호출됩니다. NewSnapshot 팩토리 함수는 항상 유효한 스냅샷 인스턴스를
// 반환해야 하며, nil을 반환하는 것은 팩토리 함수의 구현 오류(버그)를 의미합니다.
//
// 매개변수:
//   - taskID: Task의 고유 식별자
//   - commandID: Command의 고유 식별자
//
// 반환값: Task ID와 Command ID를 포함한 상세 에러 메시지
func newErrSnapshotCreationFailed(taskID contract.TaskID, commandID contract.TaskCommandID) error {
	return apperrors.Newf(apperrors.Internal, "%s (task_id: %s, command_id: %s)", errMsgSnapshotCreationFailed, taskID, commandID)
}

// newErrSnapshotLoadingFailed 이전 작업 결과(Snapshot) 로딩 실패 시 상세 에러를 생성합니다.
//
// 이 함수는 prepareExecution 단계에서 storage.Load() 호출 중 에러가 발생했을 때 호출됩니다.
// 단, ErrTaskResultNotFound(최초 실행)는 정상 상황으로 간주하여 이 함수를 호출하지 않습니다.
// 로딩 실패는 파일 시스템 오류, 네트워크 문제, 데이터 손상 등 다양한 원인으로 발생할 수 있으며, 시스템 운영 관점에서 해결해야 할 문제입니다.
//
// 매개변수:
//   - cause: Storage.Load()에서 반환된 원인 에러
//   - taskID: Task의 고유 식별자
//   - commandID: Command의 고유 식별자
//
// 반환값: 원인 에러를 래핑하고 Task ID와 Command ID를 포함한 상세 에러 메시지
func newErrSnapshotLoadingFailed(cause error, taskID contract.TaskID, commandID contract.TaskCommandID) error {
	return apperrors.Wrapf(cause, apperrors.Internal, "이전 작업 결과(Snapshot) 로딩 중 Storage 오류 발생 (task_id: %s, command_id: %s)", taskID, commandID)
}

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

// newErrRuntimePanic Task 실행 중 패닉이 발생했을 때 상세 에러를 생성합니다.
//
// 이 함수는 Run 메서드의 defer 블록에서 recover()로 패닉을 복구한 후 호출됩니다.
// 패닉은 예상치 못한 코드 버그나 심각한 런타임 오류(nil 포인터 참조, 인덱스 초과 등)로 인해 발생하며,
// 전체 서비스가 중단되지 않도록 복구하여 로그에 기록하고 사용자에게 에러 알림을 전송합니다.
//
// 매개변수:
//   - r: recover()가 반환한 패닉 값 (any 타입)
//   - taskID: Task의 고유 식별자
//   - commandID: Command의 고유 식별자
//
// 반환값: 패닉 값과 Task ID, Command ID를 포함한 상세 에러 메시지
func newErrRuntimePanic(r any, taskID contract.TaskID, commandID contract.TaskCommandID) error {
	return apperrors.Newf(apperrors.Internal, "Task 실행 중 런타임 패닉(Panic) 발생: %v (task_id: %s, command_id: %s)", r, taskID, commandID)
}
