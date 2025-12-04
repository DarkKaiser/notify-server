package middleware

import (
	"io"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
)

// Logger는 Echo의 log.Logger 인터페이스를 구현하는 Logrus 어댑터입니다.
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
	return l.Out
}

func (l Logger) SetOutput(w io.Writer) {
	logrus.SetOutput(w)
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
	case logrus.FatalLevel:
	case logrus.TraceLevel:
	}

	return log.OFF
}

// SetLevel Echo의 로그 레벨을 Logrus의 로그 레벨로 변환하여 설정합니다.
func (l Logger) SetLevel(lvl log.Lvl) {
	switch lvl {
	case log.DEBUG:
		logrus.SetLevel(logrus.DebugLevel)
	case log.WARN:
		logrus.SetLevel(logrus.WarnLevel)
	case log.ERROR:
		logrus.SetLevel(logrus.ErrorLevel)
	case log.INFO:
		logrus.SetLevel(logrus.InfoLevel)
	case log.OFF:
	}
}

func (l Logger) SetHeader(string) {
	// Echo의 Header 기능은 사용하지 않음
}

// 아래 메서드들은 Echo의 Logger 인터페이스 요구사항을 충족하기 위해
// Logrus의 해당 메서드로 단순 위임합니다.

func (l Logger) Print(i ...interface{}) {
	logrus.Print(i...)
}

func (l Logger) Printf(format string, args ...interface{}) {
	logrus.Printf(format, args...)
}

func (l Logger) Printj(j log.JSON) {
	logrus.WithFields(logrus.Fields(j)).Print()
}

func (l Logger) Debug(i ...interface{}) {
	logrus.Debug(i...)
}

func (l Logger) Debugf(format string, args ...interface{}) {
	logrus.Debugf(format, args...)
}

func (l Logger) Debugj(j log.JSON) {
	logrus.WithFields(logrus.Fields(j)).Debug()
}

func (l Logger) Info(i ...interface{}) {
	logrus.Info(i...)
}

func (l Logger) Infof(format string, args ...interface{}) {
	logrus.Infof(format, args...)
}

func (l Logger) Infoj(j log.JSON) {
	logrus.WithFields(logrus.Fields(j)).Info()
}

func (l Logger) Warn(i ...interface{}) {
	logrus.Warn(i...)
}

func (l Logger) Warnf(format string, args ...interface{}) {
	logrus.Warnf(format, args...)
}

func (l Logger) Warnj(j log.JSON) {
	logrus.WithFields(logrus.Fields(j)).Warn()
}

func (l Logger) Error(i ...interface{}) {
	logrus.Error(i...)
}

func (l Logger) Errorf(format string, args ...interface{}) {
	logrus.Errorf(format, args...)
}

func (l Logger) Errorj(j log.JSON) {
	logrus.WithFields(logrus.Fields(j)).Error()
}

func (l Logger) Fatal(i ...interface{}) {
	logrus.Fatal(i...)
}

func (l Logger) Fatalf(format string, args ...interface{}) {
	logrus.Fatalf(format, args...)
}

func (l Logger) Fatalj(j log.JSON) {
	logrus.WithFields(logrus.Fields(j)).Fatal()
}

func (l Logger) Panic(i ...interface{}) {
	logrus.Panic(i...)
}

func (l Logger) Panicf(format string, args ...interface{}) {
	logrus.Panicf(format, args...)
}

func (l Logger) Panicj(j log.JSON) {
	logrus.WithFields(logrus.Fields(j)).Panic()
}

// logrusMiddlewareHandler HTTP 요청/응답 정보를 Logrus로 로깅하는 미들웨어 핸들러입니다.
// 요청 처리 시간, 상태 코드, IP 주소 등의 정보를 구조화된 로그로 기록합니다.
func logrusMiddlewareHandler(c echo.Context, next echo.HandlerFunc) error {
	req := c.Request()
	res := c.Response()
	start := time.Now()
	if err := next(c); err != nil {
		c.Error(err)
	}
	stop := time.Now()

	p := req.URL.Path
	if p == "" {
		p = "/"
	}

	bytesIn := req.Header.Get(echo.HeaderContentLength)
	if bytesIn == "" {
		bytesIn = "0"
	}

	logrus.WithFields(map[string]interface{}{
		"time_rfc3339":  time.Now().Format(time.RFC3339),
		"remote_ip":     c.RealIP(),
		"host":          req.Host,
		"uri":           req.RequestURI,
		"method":        req.Method,
		"path":          p,
		"referer":       req.Referer(),
		"user_agent":    req.UserAgent(),
		"status":        res.Status,
		"latency":       strconv.FormatInt(stop.Sub(start).Nanoseconds()/1000, 10),
		"latency_human": stop.Sub(start).String(),
		"bytes_in":      bytesIn,
		"bytes_out":     strconv.FormatInt(res.Size, 10),
		"request_id":    res.Header().Get(echo.HeaderXRequestID),
	}).Info("echo log")

	return nil
}

func logrusLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return logrusMiddlewareHandler(c, next)
	}
}

// LogrusLogger Echo 미들웨어로 사용할 수 있는 Logrus 로거를 반환합니다.
// router.New()에서 이 미들웨어를 등록하여 모든 HTTP 요청을 로깅합니다.
func LogrusLogger() echo.MiddlewareFunc {
	return logrusLogger
}
