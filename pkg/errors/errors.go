package errors

import (
	"errors"
	"fmt"
)

// ErrorType 에러의 종류를 나타내는 타입
type ErrorType string

// 에러 타입 상수
//
// 각 에러 타입은 특정 상황에서 사용되며, 에러 처리 로직에서 타입별로 다른 처리를 할 수 있습니다.
const (
	// 일반적인 에러 타입

	// ErrUnknown 알 수 없는 에러 (기본값)
	// 사용 시나리오: 에러 타입을 특정할 수 없거나, AppError가 아닌 표준 에러인 경우
	ErrUnknown ErrorType = "Unknown"

	// ErrInvalidInput 잘못된 입력값
	// 사용 시나리오:
	//   - 유효성 검사 실패 (예: 잘못된 이메일 형식, 범위 초과)
	//   - JSON 파싱 실패
	//   - 필수 파라미터 누락
	//   - 잘못된 설정 값
	ErrInvalidInput ErrorType = "InvalidInput"

	// ErrNotFound 리소스를 찾을 수 없음
	// 사용 시나리오:
	//   - 파일이 존재하지 않음
	//   - 데이터베이스 레코드가 없음
	//   - API 엔드포인트가 존재하지 않음
	//   - 설정에서 참조하는 ID가 없음
	ErrNotFound ErrorType = "NotFound"

	// ErrInternal 내부 처리 오류
	// 사용 시나리오:
	//   - 예상치 못한 내부 로직 오류
	//   - 복구 불가능한 상태
	//   - 프로그래밍 오류 (버그)
	ErrInternal ErrorType = "Internal"

	// ErrUnauthorized 인증 실패
	// 사용 시나리오:
	//   - 인증 토큰이 없거나 만료됨
	//   - 잘못된 자격 증명
	//   - API 키가 유효하지 않음
	ErrUnauthorized ErrorType = "Unauthorized"

	// ErrForbidden 권한 없음
	// 사용 시나리오:
	//   - 인증은 되었으나 해당 리소스에 접근 권한이 없음
	//   - 역할 기반 접근 제어(RBAC) 위반
	ErrForbidden ErrorType = "Forbidden"

	// ErrSystem 시스템 레벨 오류
	// 사용 시나리오:
	//   - 파일 시스템 오류 (읽기/쓰기 실패)
	//   - 네트워크 오류
	//   - 외부 시스템 연동 실패
	//   - 리소스 부족 (메모리, 디스크)
	ErrSystem ErrorType = "System"
)

// AppError 애플리케이션 전용 에러 구조체
type AppError struct {
	Type    ErrorType // 에러 종류
	Message string    // 사용자에게 보여줄 메시지
	Cause   error     // 원인 에러 (Wrapping)
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// New 새로운 에러를 생성합니다.
func New(errType ErrorType, msg string) error {
	return &AppError{
		Type:    errType,
		Message: msg,
	}
}

// Wrap 기존 에러를 감싸서 새로운 에러를 생성합니다.
func Wrap(err error, errType ErrorType, msg string) error {
	return &AppError{
		Type:    errType,
		Message: msg,
		Cause:   err,
	}
}

// Is 에러 타입이 일치하는지 확인합니다.
func Is(err error, errType ErrorType) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == errType
	}
	return false
}

// As 표준 errors.As 함수를 래핑합니다.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Cause 원인 에러를 반환합니다.
func Cause(err error) error {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Cause
	}
	return nil
}

// RootCause 에러 체인의 최상위 원인 에러를 반환합니다.
// 중첩된 에러를 재귀적으로 unwrap하여 가장 근본적인 원인을 찾습니다.
func RootCause(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}

// GetType 에러 타입을 반환합니다. AppError가 아니거나 nil이면 ErrUnknown을 반환합니다.
func GetType(err error) ErrorType {
	if err == nil {
		return ErrUnknown
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type
	}
	return ErrUnknown
}
