// Package errors 애플리케이션 전용 에러 처리 시스템을 제공합니다.
//
// 이 패키지는 표준 errors 패키지를 확장하여 타입 기반 에러 분류와
// 에러 체이닝을 지원합니다. 모든 에러는 ErrorType으로 분류되며,
// Wrap 함수를 통해 컨텍스트를 누적할 수 있습니다.
//
// # 기본 사용법
//
// 새 에러 생성:
//
//	err := errors.New(errors.NotFound, "사용자를 찾을 수 없습니다")
//
// 에러 래핑 (컨텍스트 추가):
//
//	if err != nil {
//	    return errors.Wrap(err, errors.Internal, "데이터베이스 조회 실패")
//	}
//
// 에러 타입 검사:
//
//	if errors.Is(err, errors.NotFound) {
//	    // NotFound 타입 에러 처리
//	}
//
// 에러 체인 탐색:
//
//	rootErr := errors.RootCause(err)  // 최상위 원인 에러 반환
//	errType := errors.GetType(err)    // 에러 타입 추출
package errors

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// AppError 애플리케이션에서 발생하는 모든 에러를 표준화하여 표현하는 구조체입니다.
type AppError struct {
	errType ErrorType    // 에러의 종류
	message string       // 사용자에게 보여줄 메시지
	cause   error        // 이 에러가 발생하게 된 근본 원인 (에러 체이닝)
	stack   []StackFrame // 에러 발생 시점의 함수 호출 스택 정보
}

// Message 에러 메시지를 반환합니다.
func (e *AppError) Message() string {
	return e.message
}

// Stack 스택 트레이스를 반환합니다.
func (e *AppError) Stack() []StackFrame {
	if e.stack == nil {
		return nil
	}
	return e.stack
}

// Error 표준 errors.Error 인터페이스를 구현합니다.
func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.errType, e.message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.errType, e.message)
}

// Unwrap 표준 errors.Unwrap 인터페이스를 구현합니다.
func (e *AppError) Unwrap() error {
	return e.cause
}

// Is 표준 errors.Is 인터페이스를 구현합니다.
func (e *AppError) Is(target error) bool {
	t, ok := target.(ErrorType)
	if !ok {
		return false
	}
	return e.errType == t
}

// Format fmt.Formatter 인터페이스를 구현합니다.
// %+v 사용 시 에러 체인과 스택 트레이스를 상세히 출력합니다.
func (e *AppError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			// 에러 타입과 메시지
			fmt.Fprintf(s, "[%s] %s", e.errType, e.message)

			// 스택 트레이스 출력 (원인 에러가 AppError가 아닐 때만, 또는 원인이 없을 때)
			// 즉, 체인의 가장 마지막(Root)이나, 외부 에러를 감싼 지점에서만 스택을 출력하여 중복 방지
			var target *AppError
			if e.cause == nil || !errors.As(e.cause, &target) {
				if len(e.stack) > 0 {
					fmt.Fprint(s, "\nStack trace:")
					for _, frame := range e.stack {
						// 함수명에서 패키지 경로 간소화
						funcName := frame.Function
						if idx := strings.LastIndex(funcName, "/"); idx != -1 {
							funcName = funcName[idx+1:]
						}
						fmt.Fprintf(s, "\n\t%s:%d %s", frame.File, frame.Line, funcName)
					}
				}
			}

			// Cause 출력
			if e.cause != nil {
				fmt.Fprint(s, "\nCaused by:\n")
				if formatter, ok := e.cause.(fmt.Formatter); ok {
					formatter.Format(s, verb)
				} else {
					fmt.Fprintf(s, "\t%v", e.cause)
				}
			}
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, e.Error())
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	}
}

// New 새로운 에러를 생성합니다.
func New(errType ErrorType, message string) error {
	return &AppError{
		errType: errType,
		message: message,
		stack:   captureStack(defaultCallerSkip),
	}
}

// Newf 포맷 문자열을 사용하여 새로운 에러를 생성합니다.
func Newf(errType ErrorType, format string, args ...interface{}) error {
	return &AppError{
		errType: errType,
		message: fmt.Sprintf(format, args...),
		stack:   captureStack(defaultCallerSkip),
	}
}

// Wrap 기존 에러를 감싸서 새로운 에러를 생성합니다.
// err이 nil인 경우 nil을 반환합니다.
func Wrap(err error, errType ErrorType, message string) error {
	if err == nil {
		return nil
	}
	return &AppError{
		errType: errType,
		message: message,
		cause:   err,
		stack:   captureStack(defaultCallerSkip),
	}
}

// Wrapf 포맷 문자열을 사용하여 기존 에러를 감쌉니다.
// err이 nil인 경우 nil을 반환합니다.
func Wrapf(err error, errType ErrorType, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return &AppError{
		errType: errType,
		message: fmt.Sprintf(format, args...),
		cause:   err,
		stack:   captureStack(defaultCallerSkip),
	}
}

// Is 에러 체인에 특정 에러 타입이 포함되어 있는지 확인합니다.
func Is(err error, errType ErrorType) bool {
	for err != nil {
		if appErr, ok := err.(*AppError); ok {
			if appErr.errType == errType {
				return true
			}
		}
		err = errors.Unwrap(err)
	}
	return false
}

// As 에러 체인에서 특정 타입의 에러를 찾아 대상 변수에 할당합니다.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// RootCause 에러가 발생한 가장 근본적인 원인 에러를 찾습니다.
func RootCause(err error) error {
	if err == nil {
		return nil
	}

	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}

// GetType 에러의 타입을 반환합니다. (알 수 없는 경우 Unknown 반환)
func GetType(err error) ErrorType {
	if err == nil {
		return Unknown
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.errType
	}
	return Unknown
}
