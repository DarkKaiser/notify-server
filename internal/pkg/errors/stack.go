package errors

import (
	"path/filepath"
	"runtime"
)

// defaultCallerSkip runtime.Callers를 통해 스택 트레이스를 수집할 때 건너뛸 프레임 수입니다.
// 에러 생성 함수(New, Wrap 등)와 내부 유틸리티 함수(captureStack)의 호출 스택을 제외하고,
// 실제 에러가 발생한 지점(Caller)부터 추적하기 위해 설정된 값입니다.
const defaultCallerSkip = 3

// StackFrame 단일 함수 호출 스택의 실행 컨텍스트 정보를 캡슐화한 구조체입니다.
type StackFrame struct {
	File     string // 파일 이름
	Line     int    // 줄 번호
	Function string // 함수 이름
}

// captureStack 현재 실행 위치의 스택 정보를 수집하여 반환합니다. (최대 5단계)
func captureStack(skip int) []StackFrame {
	const maxFrames = 5
	pc := make([]uintptr, maxFrames)
	n := runtime.Callers(skip, pc)

	if n == 0 {
		return nil
	}

	callersFrames := runtime.CallersFrames(pc[:n])

	frames := make([]StackFrame, 0, n)
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
