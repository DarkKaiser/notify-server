package telegram

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// NewErrInvalidBotToken 제공된 Bot Token이 유효하지 않아 텔레그램 API 클라이언트 초기화가 실패하였을 때 반환하는 에러를 생성합니다.
func NewErrInvalidBotToken(err error) error {
	return apperrors.Wrap(err, apperrors.InvalidInput, "텔레그램 봇 API 클라이언트 초기화가 실패하였습니다. BotToken이 올바른지 확인해주세요")
}

// NewErrInvalidCommandIDs 명령어 등록 과정에서 필수 식별자인 TaskID 또는 CommandID가 누락된 경우 반환하는 에러를 생성합니다.
func NewErrInvalidCommandIDs(taskID, commandID string) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("TaskID와 CommandID는 필수입니다. 설정 파일을 확인해주세요 (Task:'%s', Command:'%s')", taskID, commandID))
}

// NewErrDuplicateCommandName 이미 등록된 명령어와 이름이 중복되는 충돌이 감지되었을 때 반환하는 에러를 생성합니다.
func NewErrDuplicateCommandName(cmdName, existingTaskID, existingCommandID, newTaskID, newCommandID string) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("명령어 충돌: /%s (기존: %s > %s vs 신규: %s > %s)", cmdName, existingTaskID, existingCommandID, newTaskID, newCommandID))
}
