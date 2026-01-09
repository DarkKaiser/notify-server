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

//go:generate stringer -type=ErrorType

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrorType 에러의 종류를 나타내는 타입
type ErrorType int

// 에러 타입 상수
//
// 각 에러 타입은 특정 상황에서 사용되며, 에러 처리 로직에서 타입별로 다른 처리를 할 수 있습니다.
const (
	// Unknown 알 수 없는 에러 (기본값)
	// 사용 시나리오: 에러 타입을 특정할 수 없거나, AppError가 아닌 표준 에러인 경우
	Unknown ErrorType = iota

	// Internal 내부 처리 오류
	// 사용 시나리오:
	//   - 예상치 못한 내부 로직 오류
	//   - 복구 불가능한 상태
	//   - 프로그래밍 오류 (버그)
	Internal

	// System 시스템 레벨 오류
	// 사용 시나리오:
	//   - 파일 시스템 오류 (읽기/쓰기 실패)
	//   - 네트워크 오류
	//   - 외부 시스템 연동 실패
	//   - 리소스 부족 (메모리, 디스크)
	System

	// Unauthorized 인증 실패
	// 사용 시나리오:
	//   - 인증 토큰이 없거나 만료됨
	//   - 잘못된 자격 증명
	//   - API 키가 유효하지 않음
	Unauthorized

	// Forbidden 권한 없음
	// 사용 시나리오:
	//   - 인증은 되었으나 해당 리소스에 접근 권한이 없음
	//   - 역할 기반 접근 제어(RBAC) 위반
	Forbidden

	// InvalidInput 잘못된 입력값
	// 사용 시나리오:
	//   - 유효성 검사 실패 (예: 잘못된 이메일 형식, 범위 초과)
	//   - JSON 파싱 실패
	//   - 필수 파라미터 누락
	//   - 잘못된 설정 값
	InvalidInput

	// Conflict 리소스 충돌 (이미 존재함)
	// 사용 시나리오:
	//   - 중복된 ID로 생성 시도
	//   - 데이터 무결성 위반
	Conflict

	// NotFound 리소스를 찾을 수 없음
	// 사용 시나리오:
	//   - 파일이 존재하지 않음
	//   - 데이터베이스 레코드가 없음
	//   - API 엔드포인트가 존재하지 않음
	//   - 설정에서 참조하는 ID가 없음
	NotFound

	// ExecutionFailed 비즈니스 로직이나 작업 실행 과정에서 실패가 발생했을 때 사용합니다.
	// 사용 시나리오:
	//   - 크롤링/스크래핑 작업 수행 실패 (파싱 에러, 타임아웃 등)
	//   - 외부 커맨드 또는 프로세스 실행 실패
	ExecutionFailed

	// Timeout 작업 수행 시간 초과
	// 사용 시나리오:
	//   - 외부 API 응답 지연
	//   - DB 쿼리 타임아웃
	//   - 컨텍스트 데드라인 초과
	Timeout

	// Unavailable 일시적인 서비스 사용 불가
	// 사용 시나리오:
	//   - 외부 시스템(스크래핑 대상) 장애
	//   - 트래픽 폭주로 인한 차단
	Unavailable
)

// String ErrorType을 문자열로 반환합니다.
// 이 메서드는 stringer 도구에 의해 자동 생성됩니다.
// 수동으로 수정하지 마세요. 대신 `go generate ./...` 를 실행하세요.

// StackFrame 스택 트레이스의 단일 프레임 정보
type StackFrame struct {
	File     string // 파일명 (경로 제외)
	Line     int    // 라인 번호
	Function string // 함수명
}

// AppError 애플리케이션 전용 에러 구조체
type AppError struct {
	Type    ErrorType    // 에러 종류
	Message string       // 사용자에게 보여줄 메시지
	Cause   error        // 원인 에러 (Wrapping)
	Stack   []StackFrame // 스택 트레이스 (최대 5개)
}

// captureStack 현재 호출 스택을 캡처합니다 (최대 5개 프레임)
func captureStack(skip int) []StackFrame {
	const maxFrames = 5
	pc := make([]uintptr, maxFrames)
	n := runtime.Callers(skip, pc)

	if n == 0 {
		return nil
	}

	frames := make([]StackFrame, 0, n)
	callersFrames := runtime.CallersFrames(pc[:n])

	for {
		frame, more := callersFrames.Next()
		frames = append(frames, StackFrame{
			File:     filepath.Base(frame.File),
			Line:     frame.Line,
			Function: frame.Function,
		})
		if !more {
			break
		}
	}

	return frames
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Format fmt.Formatter 인터페이스를 구현합니다.
// %+v 사용 시 에러 체인과 스택 트레이스를 상세히 출력합니다.
func (e *AppError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			// 에러 타입과 메시지
			fmt.Fprintf(s, "[%s] %s", e.Type, e.Message)

			// 스택 트레이스 출력
			if len(e.Stack) > 0 {
				fmt.Fprint(s, "\nStack trace:")
				for _, frame := range e.Stack {
					// 함수명에서 패키지 경로 간소화
					funcName := frame.Function
					if idx := strings.LastIndex(funcName, "/"); idx != -1 {
						funcName = funcName[idx+1:]
					}
					fmt.Fprintf(s, "\n\t%s:%d %s", frame.File, frame.Line, funcName)
				}
			}

			// Cause 출력
			if e.Cause != nil {
				fmt.Fprint(s, "\nCaused by:\n")
				if formatter, ok := e.Cause.(fmt.Formatter); ok {
					formatter.Format(s, verb)
				} else {
					fmt.Fprintf(s, "\t%v", e.Cause)
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
		Type:    errType,
		Message: message,
		Stack:   captureStack(3),
	}
}

// Newf 포맷 문자열을 사용하여 새로운 에러를 생성합니다.
func Newf(errType ErrorType, format string, args ...interface{}) error {
	return &AppError{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
		Stack:   captureStack(3),
	}
}

// Wrap 기존 에러를 감싸서 새로운 에러를 생성합니다.
// err이 nil인 경우 nil을 반환합니다.
func Wrap(err error, errType ErrorType, message string) error {
	if err == nil {
		return nil
	}
	return &AppError{
		Type:    errType,
		Message: message,
		Cause:   err,
		Stack:   captureStack(3),
	}
}

// Wrapf 포맷 문자열을 사용하여 기존 에러를 감쌉니다.
// err이 nil인 경우 nil을 반환합니다.
func Wrapf(err error, errType ErrorType, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return &AppError{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
		Cause:   err,
		Stack:   captureStack(3),
	}
}

// Unwrap 표준 errors.Unwrap 인터페이스를 구현합니다.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is 에러 타입이 일치하는지 확인합니다.
// 에러 체인 전체를 탐색하여 지정된 타입이 존재하는지 검사합니다.
func Is(err error, errType ErrorType) bool {
	for err != nil {
		var appErr *AppError
		if errors.As(err, &appErr) && appErr.Type == errType {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

// As 표준 errors.As 함수를 래핑합니다.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// RootCause 에러 체인의 최상위 원인 에러를 반환합니다.
// 중첩된 에러를 재귀적으로 unwrap하여 가장 근본적인 원인을 찾습니다.
//
// 한 단계만 unwrap이 필요한 경우 표준 errors.Unwrap()을 사용하세요.
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

// GetType 에러 타입을 반환합니다. AppError가 아니거나 nil이면 ErrUnknown을 반환합니다.
func GetType(err error) ErrorType {
	if err == nil {
		return Unknown
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type
	}
	return Unknown
}
