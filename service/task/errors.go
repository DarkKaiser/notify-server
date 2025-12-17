package task

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
)

var (
	// ErrTaskNotSupported 지원하지 않는 작업(Task)에 접근하려 할 때 반환됩니다.
	ErrTaskNotSupported = apperrors.New(apperrors.InvalidInput, "지원하지 않는 작업입니다")

	// ErrCommandNotSupported 해당 작업(Task)은 존재하지만, 요청된 명령(Command)을 지원하지 않을 때 반환됩니다.
	ErrCommandNotSupported = apperrors.New(apperrors.InvalidInput, "지원하지 않는 명령입니다")

	// ErrTaskUnregistered 등록되지 않은 작업에 접근하려 할 때 반환됩니다.
	ErrTaskUnregistered = apperrors.New(apperrors.NotFound, "등록되지 않은 작업입니다.😱")

	// ErrInvalidTaskData 작업 설정 데이터(JSON/Map) 디코딩 실패 시 반환됩니다.
	ErrInvalidTaskData = apperrors.New(apperrors.InvalidInput, "작업 데이터가 유효하지 않습니다")

	// ErrHTMLStructureChanged HTML 페이지 구조가 변경되어 파싱에 실패했을 때 반환됩니다.
	ErrHTMLStructureChanged = apperrors.New(apperrors.ExecutionFailed, "불러온 페이지의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요")
)

// NewErrCommandNotSupported 지원하지 않는 명령(Command)일 때 상세 메시지와 함께 에러를 반환합니다.
func NewErrCommandNotSupported(commandID CommandID) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("지원하지 않는 명령입니다: %s", commandID))
}

// NewErrHTMLStructureChanged HTML 구조 변경 에러에 상세 정보(URL, 추가 설명 등)를 덧붙여 반환합니다.
func NewErrHTMLStructureChanged(url, details string) error {
	message := ErrHTMLStructureChanged.Error()
	if url != "" {
		message += fmt.Sprintf(" (%s)", url)
	}
	if details != "" {
		message += fmt.Sprintf(": %s", details)
	}
	return apperrors.New(apperrors.ExecutionFailed, message)
}

// NewErrTypeAssertionFailed 타입 단언(Type Assertion) 실패 시 사용하는 에러를 생성합니다.
func NewErrTypeAssertionFailed(targetName string, expected, got interface{}) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("%s의 타입 변환이 실패하였습니다 (expected: %T, got: %T)", targetName, expected, got))
}
