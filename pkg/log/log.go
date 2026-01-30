package log

import (
	"context"
	"io"
	"time"

	"github.com/sirupsen/logrus"
)

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

// SetLevel 전역 로거의 레벨을 설정합니다.
func SetLevel(level Level) {
	logrus.SetLevel(level)
}

// WithField 단일 키-값 쌍을 로그 컨텍스트에 추가합니다.
func WithField(key string, value interface{}) *Entry {
	return logrus.WithField(key, value)
}

// WithFields 구조화된 필드(Key-Value)를 로그 컨텍스트에 추가합니다.
func WithFields(fields Fields) *Entry {
	return logrus.WithFields(fields)
}

// WithContext 컨텍스트를 로그에 연동합니다. (Trace ID 추적 등에 활용)
func WithContext(ctx context.Context) *Entry {
	return logrus.WithContext(ctx)
}

// WithError 에러를 로그 컨텍스트에 추가합니다. ("error" 필드)
func WithError(err error) *Entry {
	return logrus.WithError(err)
}

// WithTime 커스텀 타임스탬프를 로그 컨텍스트에 설정합니다.
func WithTime(t time.Time) *Entry {
	return logrus.WithTime(t)
}

// WithComponent 로그의 출처(Component)를 명시하는 'component' 필드를 컨텍스트에 추가합니다.
func WithComponent(component string) *Entry {
	return logrus.WithField("component", component)
}

// WithComponentAndFields 'component' 필드와 추가적인 구조화된 필드들을 동시에 컨텍스트에 추가합니다.
func WithComponentAndFields(component string, fields Fields) *Entry {
	return logrus.WithFields(fields).WithField("component", component)
}

// Trace 레벨 로그를 기록합니다.
var Trace = logrus.Trace

// Debug 레벨 로그를 기록합니다.
var Debug = logrus.Debug

// Info 레벨 로그를 기록합니다.
var Info = logrus.Info

// Warn 레벨 로그를 기록합니다.
var Warn = logrus.Warn

// Error 레벨 로그를 기록합니다.
var Error = logrus.Error

// Fatal 레벨 로그를 기록합니다. (이후 os.Exit(1) 호출)
var Fatal = logrus.Fatal

// Panic 레벨 로그를 기록합니다. (이후 panic() 발생)
var Panic = logrus.Panic

// 포맷팅된 Trace 레벨 로그를 기록합니다.
var Tracef = logrus.Tracef

// 포맷팅된 Debug 레벨 로그를 기록합니다.
var Debugf = logrus.Debugf

// 포맷팅된 Info 레벨 로그를 기록합니다.
var Infof = logrus.Infof

// 포맷팅된 Warn 레벨 로그를 기록합니다.
var Warnf = logrus.Warnf

// 포맷팅된 Error 레벨 로그를 기록합니다.
var Errorf = logrus.Errorf

// 포맷팅된 Fatal 레벨 로그를 기록합니다. (이후 os.Exit(1) 호출)
var Fatalf = logrus.Fatalf

// 포맷팅된 Panic 레벨 로그를 기록합니다. (이후 panic() 발생)
var Panicf = logrus.Panicf
