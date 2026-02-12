package provider

import (
	"fmt"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
)

var (
	// ErrTaskNotSupported 지원하지 않는 작업(Task)에 접근하려 할 때 반환됩니다.
	ErrTaskNotSupported = apperrors.New(apperrors.InvalidInput, "지원하지 않는 작업입니다")

	// ErrCommandNotSupported 해당 작업(Task)은 존재하지만, 요청된 명령(Command)을 지원하지 않을 때 반환됩니다.
	ErrCommandNotSupported = apperrors.New(apperrors.InvalidInput, "지원하지 않는 명령입니다")

	// ErrTaskSettingsNotFound 작업 생성에 필요한 설정 데이터(JSON/Map)를 찾을 수 없을 때 반환됩니다.
	ErrTaskSettingsNotFound = apperrors.New(apperrors.NotFound, "해당 작업 생성에 필요한 설정 데이터가 존재하지 않습니다")

	// ErrCommandSettingsNotFound 명령 생성에 필요한 설정 데이터(JSON/Map)를 찾을 수 없을 때 반환됩니다.
	ErrCommandSettingsNotFound = apperrors.New(apperrors.NotFound, "해당 명령 생성에 필요한 설정 데이터가 존재하지 않습니다")

	// ErrInvalidTaskSettings 작업 설정 데이터(JSON/Map) 디코딩 또는 검증 실패 시 반환됩니다.
	ErrInvalidTaskSettings = apperrors.New(apperrors.InvalidInput, "작업 설정 데이터가 유효하지 않습니다")

	// ErrInvalidCommandSettings 명령 설정 데이터(JSON/Map) 디코딩 또는 검증 실패 시 반환됩니다.
	ErrInvalidCommandSettings = apperrors.New(apperrors.InvalidInput, "명령 설정 데이터가 유효하지 않습니다")
)

// newErrInvalidTaskSettings 작업 설정 데이터(JSON/Map) 디코딩 또는 검증 실패 시 반환됩니다.
func newErrInvalidTaskSettings(taskID contract.TaskID, cause error) error {
	return apperrors.Wrap(cause, apperrors.InvalidInput, fmt.Sprintf("%s (task_id: %s)", ErrInvalidTaskSettings.Error(), taskID))
}

// newErrInvalidCommandSettings 명령 설정 데이터(JSON/Map) 디코딩 또는 검증 실패 시 반환됩니다.
func newErrInvalidCommandSettings(taskID contract.TaskID, commandID contract.TaskCommandID, cause error) error {
	return apperrors.Wrap(cause, apperrors.InvalidInput, fmt.Sprintf("%s (task_id: %s, command_id: %s)", ErrInvalidCommandSettings.Error(), taskID, commandID))
}

// NewErrTaskSettingsNotFound 작업 설정 데이터를 찾을 수 없을 때 상세 정보를 포함하여 반환합니다.
func NewErrTaskSettingsNotFound(taskID contract.TaskID) error {
	return apperrors.Wrap(ErrTaskSettingsNotFound, apperrors.NotFound, fmt.Sprintf("task_id: %s", taskID))
}

// NewErrCommandSettingsNotFound 명령 설정 데이터를 찾을 수 없을 때 상세 정보를 포함하여 반환합니다.
func NewErrCommandSettingsNotFound(taskID contract.TaskID, commandID contract.TaskCommandID) error {
	return apperrors.Wrap(ErrCommandSettingsNotFound, apperrors.NotFound, fmt.Sprintf("task_id: %s, command_id: %s", taskID, commandID))
}

// NewErrCommandNotSupported 지원하지 않는 명령(Command)일 때 상세 메시지 및 지원 가능한 목록과 함께 에러를 반환합니다.
func NewErrCommandNotSupported(commandID contract.TaskCommandID, supportedCommands []contract.TaskCommandID) error {
	msg := fmt.Sprintf("지원하지 않는 명령입니다: %s", commandID)
	if len(supportedCommands) > 0 {
		cmds := make([]string, len(supportedCommands))
		for i, id := range supportedCommands {
			cmds[i] = string(id)
		}
		msg = fmt.Sprintf("%s (사용 가능한 명령: %s)", msg, strings.Join(cmds, ", "))
	}
	return apperrors.New(apperrors.InvalidInput, msg)
}

// NewErrTaskNotSupported 지원하지 않는 작업(Task)일 때 상세 메시지와 함께 에러를 반환합니다.
func NewErrTaskNotSupported(taskID contract.TaskID) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("지원하지 않는 작업입니다: %s", taskID))
}

// NewErrTypeAssertionFailed 타입 단언(Type Assertion) 실패 시 사용하는 에러를 생성합니다.
func NewErrTypeAssertionFailed(targetName string, expected, got interface{}) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("%s의 타입 변환에 실패하였습니다 (expected: %T, got: %T)", targetName, expected, got))
}
