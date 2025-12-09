package task

import (
	"github.com/darkkaiser/notify-server/pkg/errors"
)

// ------------------------------------------------------------------------------------------------
// [에러 정의 구분]
//
// 1. const 정의: "에러 타입(Error Type)"
//   - 에러의 '종류'나 '카테고리'를 정의합니다.
//   - 실제 에러 객체가 아니며, errors.New()를 통해 구체적인 에러를 만들 때 '분류 기준'으로 사용됩니다.
//   - 예: errors.New(ErrTaskNotFound, "ID 123번 Task가 없습니다") -> "TaskNotFound" 타입의 에러 생성
//
// 2. var 정의: "에러 인스턴스(Error Instance)"
//   - 미리 만들어진 '완전한 에러 객체'입니다. (메시지가 고정됨)
//   - 별도의 메시지 포맷팅 없이, 자주 쓰이는 에러를 바로 반환할 때 사용합니다.
//   - 예: return ErrNotSupportedTask
//
// ------------------------------------------------------------------------------------------------
const (
	// ErrTaskNotFound Task를 찾을 수 없음
	// 사용 시나리오: 설정에 정의되지 않은 Task ID를 참조할 때
	// (기존 주석 참조)
	ErrTaskNotFound errors.ErrorType = "TaskNotFound"

	// ErrTaskExecutionFailed Task 실행 실패
	// 사용 시나리오: Task 실행 중 오류가 발생했을 때
	ErrTaskExecutionFailed errors.ErrorType = "TaskExecutionFailed"
)

var (
	ErrNotSupportedTask      = errors.New(errors.ErrInvalidInput, "지원되지 않는 작업입니다")
	ErrNotSupportedCommand   = errors.New(errors.ErrInvalidInput, "지원되지 않는 작업 커맨드입니다")
	ErrNotImplementedCommand = errors.New(errors.ErrInternal, "작업 커맨드에 대한 구현이 없습니다")
)
