package errors

import (
	"errors"
	"fmt"
)

// ErrorType 에러의 종류를 나타내는 타입
type ErrorType string

const (
	// 일반적인 에러 타입
	ErrUnknown      ErrorType = "Unknown"
	ErrInvalidInput ErrorType = "InvalidInput"
	ErrNotFound     ErrorType = "NotFound"
	ErrInternal     ErrorType = "Internal"
	ErrUnauthorized ErrorType = "Unauthorized"
	ErrForbidden    ErrorType = "Forbidden"
	ErrSystem       ErrorType = "System"

	// Domain Specific Errors
	ErrTaskNotFound        ErrorType = "TaskNotFound"
	ErrTaskExecutionFailed ErrorType = "TaskExecutionFailed"
	ErrNotificationFailed  ErrorType = "NotificationFailed"
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

// GetType 에러 타입을 반환합니다. AppError가 아니면 ErrUnknown을 반환합니다.
func GetType(err error) ErrorType {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type
	}
	return ErrUnknown
}
