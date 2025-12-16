package task

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
)

// ------------------------------------------------------------------------------------------------
// [에러 정의 가이드]
//
// 본 패키지는 에러 처리를 위해 두 가지 방식을 혼용하고 있습니다:
//
// 1. 에러 타입 (const): 논리적인 에러의 분류(Category)를 정의합니다.
//   - 상황에 따라 동적으로 생성되는 에러들의 공통된 '성격'을 나타냅니다.
//   - `apperrors.New(Type, "detail")` 또는 `apperrors.Wrap(err, Type, "context")` 형태로 사용하여,
//     구체적인 실패 사유와 함께 에러의 대분류 정보를 포함시킵니다.
//   - 주 용도: 로그 분석, HTTP 상태 코드 매핑(404 vs 500), 메트릭 집계 등.
//
// 2. 에러 인스턴스 (var): 재사용 가능한 불변의 에러 객체(Sentinel Error)입니다.
//   - 특정 조건에서 발생하는 고정된 형태의 에러를 정의합니다.
//   - `apperrors.Is(err, ErrInstance)`를 통해 특정 에러의 발생 여부를 판별할 때 사용됩니다.
//   - 주 용도: 프로그램 흐름 제어, 불필요한 메모리 할당 방지, 일관된 에러 메시지 제공.
//
// ------------------------------------------------------------------------------------------------

var (
	// ErrTaskNotSupported 지원하지 않는 작업(Task)에 접근하려 할 때 반환됩니다.
	ErrTaskNotSupported = apperrors.New(apperrors.ErrInvalidInput, "지원하지 않는 작업입니다")

	// ErrCommandNotSupported 해당 작업(Task)은 존재하지만, 요청된 명령(Command)을 지원하지 않을 때 반환됩니다.
	ErrCommandNotSupported = apperrors.New(apperrors.ErrInvalidInput, "지원하지 않는 명령입니다")

	// ErrCommandNotImplemented 명령(Command)이 정의되어 있으나, 실제 실행 로직이 구현되지 않았을 때 반환됩니다.
	ErrCommandNotImplemented = apperrors.New(apperrors.ErrInternal, "작업 명령에 대한 구현이 없습니다")

	// ErrTaskUnregistered 등록되지 않은 작업에 접근하려 할 때 반환됩니다.
	ErrTaskUnregistered = apperrors.New(apperrors.ErrNotFound, "등록되지 않은 작업입니다.😱")

	// ErrInvalidTaskData 작업 설정 데이터(JSON/Map) 디코딩 실패 시 반환됩니다.
	ErrInvalidTaskData = apperrors.New(apperrors.ErrInvalidInput, "작업 데이터가 유효하지 않습니다")

	// ErrHTMLStructureChanged HTML 페이지 구조가 변경되어 파싱에 실패했을 때 반환됩니다.
	ErrHTMLStructureChanged = apperrors.New(apperrors.ErrExecutionFailed, "불러온 페이지의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요")
)

// NewErrCommandNotSupported 지원하지 않는 명령(Command)일 때 상세 메시지와 함께 에러를 반환합니다.
func NewErrCommandNotSupported(commandID CommandID) error {
	return apperrors.New(apperrors.ErrInvalidInput, fmt.Sprintf("지원하지 않는 명령입니다: %s", commandID))
}

// NewErrTypeAssertionFailed 타입 단언(Type Assertion) 실패 시 사용하는 에러를 생성합니다.
// targetName: 변환 대상의 이름 (예: "TaskResultData", "Product")
func NewErrTypeAssertionFailed(targetName string, expected, got interface{}) error {
	return apperrors.New(apperrors.ErrInternal, fmt.Sprintf("%s의 타입 변환이 실패하였습니다 (expected: %T, got: %T)", targetName, expected, got))
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
	return apperrors.New(apperrors.ErrExecutionFailed, message)
}
