package task

import (
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
const (
	// ErrTaskNotFound 요청된 작업을 찾을 수 없을 때 발생하는 에러 타입입니다.
	//
	// [사용 시나리오]
	//  - 유효하지 않거나 등록되지 않은 Task ID로 작업을 조회하거나 실행하려 할 때 사용됩니다.
	//  - 예: DB나 실행 목록에 해당 ID의 작업이 존재하지 않음.
	ErrTaskNotFound apperrors.ErrorType = "TaskNotFound"

	// ErrTaskExecutionFailed 작업 실행 중에 예기치 않은 오류가 발생했을 때 사용하는 에러 타입입니다.
	//
	// [사용 시나리오]
	//  - 네트워크 타임아웃, 파싱 오류, 외부 프로세스 비정상 종료 등 실행 로직 내부의 실패.
	//  - Fetcher, Parser, CommandExecutor 등 실행 흐름 전반에서 발생할 수 있습니다.
	ErrTaskExecutionFailed apperrors.ErrorType = "TaskExecutionFailed"
)

var (
	// ErrTaskNotSupported 지원되지 않는 작업(Task)에 접근하려 할 때 반환됩니다.
	ErrTaskNotSupported = apperrors.New(apperrors.ErrInvalidInput, "지원되지 않는 작업입니다")

	// ErrCommandNotSupported 해당 작업(Task)은 존재하지만, 요청된 커맨드(Command)가 지원되지 않을 때 반환됩니다.
	ErrCommandNotSupported = apperrors.New(apperrors.ErrInvalidInput, "지원되지 않는 작업 커맨드입니다")

	// ErrCommandNotImplemented 커맨드가 정의되어 있으나, 실제 실행 로직(Handler)이 구현되지 않았을 때 반환됩니다.
	ErrCommandNotImplemented = apperrors.New(apperrors.ErrInternal, "작업 커맨드에 대한 구현이 없습니다")
)
