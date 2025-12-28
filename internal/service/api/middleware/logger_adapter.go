package middleware

import (
	"io"

	"github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
)

// Logger Echo의 log.Logger 인터페이스를 구현하는 Logrus 어댑터입니다.
// 이 어댑터 패턴을 통해 Echo 프레임워크가 Logrus를 사용하여 로깅할 수 있도록 합니다.
//
// Echo는 자체 Logger 인터페이스(github.com/labstack/gommon/log.Logger)를 정의하고 있으며,
// 이 인터페이스의 모든 메서드를 구현해야 Echo와 통합할 수 있습니다.
// 아래의 메서드들은 대부분 Logrus의 해당 메서드로 단순 위임하는 보일러플레이트 코드입니다.
type Logger struct {
	*logrus.Logger
}

// Output 현재 출력 Writer를 반환합니다.
func (l Logger) Output() io.Writer {
	return l.Logger.Out
}

func (l Logger) SetOutput(w io.Writer) {
	l.Logger.SetOutput(w)
}

func (l Logger) Prefix() string {
	return ""
}

func (l Logger) SetPrefix(string) {
	// Echo의 Prefix 기능은 사용하지 않음
}

// Level Logrus의 로그 레벨을 Echo의 로그 레벨로 변환합니다.
func (l Logger) Level() log.Lvl {
	switch l.Logger.Level {
	case logrus.DebugLevel:
		return log.DEBUG
	case logrus.WarnLevel:
		return log.WARN
	case logrus.ErrorLevel:
		return log.ERROR
	case logrus.InfoLevel:
		return log.INFO
	case logrus.PanicLevel:
		// Echo에 대응하는 레벨이 없으므로 OFF 반환
	case logrus.FatalLevel:
		// Echo에 대응하는 레벨이 없으므로 OFF 반환
	case logrus.TraceLevel:
		// Echo에 대응하는 레벨이 없으므로 OFF 반환
	}

	return log.OFF
}

// SetLevel Echo의 로그 레벨을 Logrus의 로그 레벨로 변환하여 설정합니다.
func (l Logger) SetLevel(lvl log.Lvl) {
	switch lvl {
	case log.DEBUG:
		l.Logger.SetLevel(logrus.DebugLevel)
	case log.WARN:
		l.Logger.SetLevel(logrus.WarnLevel)
	case log.ERROR:
		l.Logger.SetLevel(logrus.ErrorLevel)
	case log.INFO:
		l.Logger.SetLevel(logrus.InfoLevel)
	case log.OFF:
		// log.OFF는 Logrus에 대응하는 레벨이 없으므로 무시
	}
}

func (l Logger) SetHeader(string) {
	// Echo의 Header 기능은 사용하지 않음
}

// 아래 메서드들은 Echo의 Logger 인터페이스 요구사항을 충족하기 위해
// Logrus의 해당 메서드로 단순 위임합니다.

func (l Logger) Print(i ...interface{}) {
	l.Logger.Print(i...)
}

func (l Logger) Printf(format string, args ...interface{}) {
	l.Logger.Printf(format, args...)
}

func (l Logger) Printj(j log.JSON) {
	l.Logger.WithFields(logrus.Fields(j)).Print()
}

func (l Logger) Debug(i ...interface{}) {
	l.Logger.Debug(i...)
}

func (l Logger) Debugf(format string, args ...interface{}) {
	l.Logger.Debugf(format, args...)
}

func (l Logger) Debugj(j log.JSON) {
	l.Logger.WithFields(logrus.Fields(j)).Debug()
}

func (l Logger) Info(i ...interface{}) {
	l.Logger.Info(i...)
}

func (l Logger) Infof(format string, args ...interface{}) {
	l.Logger.Infof(format, args...)
}

func (l Logger) Infoj(j log.JSON) {
	l.Logger.WithFields(logrus.Fields(j)).Info()
}

func (l Logger) Warn(i ...interface{}) {
	l.Logger.Warn(i...)
}

func (l Logger) Warnf(format string, args ...interface{}) {
	l.Logger.Warnf(format, args...)
}

func (l Logger) Warnj(j log.JSON) {
	l.Logger.WithFields(logrus.Fields(j)).Warn()
}

func (l Logger) Error(i ...interface{}) {
	l.Logger.Error(i...)
}

func (l Logger) Errorf(format string, args ...interface{}) {
	l.Logger.Errorf(format, args...)
}

func (l Logger) Errorj(j log.JSON) {
	l.Logger.WithFields(logrus.Fields(j)).Error()
}

func (l Logger) Fatal(i ...interface{}) {
	l.Logger.Fatal(i...)
}

func (l Logger) Fatalf(format string, args ...interface{}) {
	l.Logger.Fatalf(format, args...)
}

func (l Logger) Fatalj(j log.JSON) {
	l.Logger.WithFields(logrus.Fields(j)).Fatal()
}

func (l Logger) Panic(i ...interface{}) {
	l.Logger.Panic(i...)
}

func (l Logger) Panicf(format string, args ...interface{}) {
	l.Logger.Panicf(format, args...)
}

func (l Logger) Panicj(j log.JSON) {
	l.Logger.WithFields(logrus.Fields(j)).Panic()
}
