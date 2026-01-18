package telegram

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// NewErrInvalidBotToken 텔레그램 봇 API 클라이언트 초기화 실패(주로 토큰 오류) 시 반환되는 에러를 생성합니다.
func NewErrInvalidBotToken(err error) error {
	return apperrors.Wrap(err, apperrors.InvalidInput, "텔레그램 봇 API 클라이언트 초기화에 실패했습니다. BotToken이 올바른지 확인해주세요.")
}

// NewErrInvalidCommandIDs TaskID 또는 CommandID가 비어있어 명령어를 생성할 수 없을 때 반환되는 에러를 생성합니다.
func NewErrInvalidCommandIDs(taskID, commandID string) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("TaskID와 CommandID는 필수입니다. 설정 파일을 확인해주세요. (Task:'%s', Command:'%s')", taskID, commandID))
}

// NewErrDuplicateCommandName 명령어 이름 중복이 감지되었을 때 반환되는 에러를 생성합니다.
func NewErrDuplicateCommandName(cmdName, existingTaskID, existingCommandID, newTaskID, newCommandID string) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("명령어 충돌: /%s (기존: %s > %s vs 신규: %s > %s)", cmdName, existingTaskID, existingCommandID, newTaskID, newCommandID))
}
