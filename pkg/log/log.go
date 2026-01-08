package log

import (
	"io"

	"github.com/sirupsen/logrus"
)

// New 새로운 로거 인스턴스를 생성합니다.
func New() *Logger {
	return logrus.New()
}

// StandardLogger logrus 라이브러리의 전역 표준 로거(Standard Logger) 인스턴스를 반환합니다.
// (참고: 이 인스턴스는 pkg/log.Setup()을 통해 모든 설정이 완료된 상태입니다.)
func StandardLogger() *Logger {
	return logrus.StandardLogger()
}

// SetOutput 전역 로거의 출력을 설정합니다.
func SetOutput(out io.Writer) {
	logrus.SetOutput(out)
}

// SetFormatter 전역 로거의 포맷터를 설정합니다.
func SetFormatter(formatter Formatter) {
	logrus.SetFormatter(formatter)
}

// WithFields 구조화된 필드(Key-Value)를 로그 컨텍스트에 추가합니다.
func WithFields(fields Fields) *Entry {
	return logrus.WithFields(fields)
}

// WithComponent 로그의 출처(Component)를 명시하는 'component' 필드를 컨텍스트에 추가합니다.
func WithComponent(component string) *Entry {
	return logrus.WithField("component", component)
}

// WithComponentAndFields 'component' 필드와 추가적인 구조화된 필드들을 동시에 컨텍스트에 추가합니다.
func WithComponentAndFields(component string, fields Fields) *Entry {
	return logrus.WithFields(fields).WithField("component", component)
}
