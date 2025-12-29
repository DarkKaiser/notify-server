package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// 로그 파일 확장자
	fileExt = "log"

	// 기본 로그 로테이션 설정
	defaultMaxSizeMB  = 100 // 100MB
	defaultMaxBackups = 20  // 최대 유지 파일 수
)

// Options 로그 초기화 옵션입니다.
type Options struct {
	Dir    string // 로그 파일이 저장될 디렉토리 경로 (기본값: "logs")
	Name   string // 로그 파일명 생성에 사용될 애플리케이션 식별자 (필수)
	MaxAge int    // 오래된 로그 삭제 기준일 (일 단위, 0: 삭제 안 함)

	EnableCriticalLog bool // ERROR 이상(ERROR, FATAL, PANIC)의 치명적 로그를 별도 파일로 분리 저장할지 여부
	EnableVerboseLog  bool // DEBUG 이하(DEBUG, TRACE)의 상세 로그를 별도 파일로 분리 저장할지 여부
	EnableConsoleLog  bool // 표준 출력(Stdout)에도 로그를 출력할지 여부 (개발 환경 권장)

	// 로그를 호출한 소스 코드의 위치(파일명:라인번호)를 함께 기록할지 여부
	// 예: true로 설정 시 "main.go:55" 처럼 로그가 발생한 위치를 알 수 있어 디버깅에 유용합니다.
	ReportCaller bool

	// 로그에 출력되는 파일 경로가 너무 길 때, 앞부분을 잘라내어 보기 좋게 만듭니다.
	// 예: "github.com/my/project/pkg/server.go" -> prefix가 "github.com/my/project"라면 "pkg/server.go"만 출력됨
	CallerPathPrefix string
}

// Setup 전역 로깅 시스템을 초기화하고 설정된 옵션에 따라 파일 출력을 구성합니다.
//
// 주요 기능:
//   - Logrus 전역 설정 (Level, Formatter, Hook) 적용
//   - Lumberjack을 이용한 로그 로테이션 (100MB 단위, 최대 20개 유지, MaxAge 적용)
//   - 우아한 종료(Graceful Shutdown)를 위한 Closer 반환
//
// 주의:
//   - 애플리케이션 시작 시점(main 함수 도입부)에 **단 한 번만** 호출해야 합니다.
//   - 반환된 Closer는 반드시 defer를 통해 리소스가 해제되도록 보장해야 합니다.
func Setup(opts Options) (io.Closer, error) {
	logrus.SetLevel(logrus.TraceLevel)
	logrus.SetReportCaller(opts.ReportCaller)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,         // TTY가 아니어도 타임스탬프를 항상 출력
		TimestampFormat: time.RFC3339, // "2006-01-02T15:04:05Z07:00" (ISO8601 표준)
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			function = frame.Function + "(line:" + strconv.Itoa(frame.Line) + ")"
			if opts.CallerPathPrefix != "" && strings.HasPrefix(function, opts.CallerPathPrefix) {
				function = "..." + function[len(opts.CallerPathPrefix):]
			}
			return
		},
	})

	// 파일 로깅을 위해서는 애플리케이션 식별자(Name)가 필수입니다.
	// Name이 비어있는 경우, 전역 설정(Formatter 등)만 적용하고 파일 출력 설정은 생략합니다.
	// API 안전성을 위해 nil 대신 빈 Closer를 반환하여 호출 측의 defer nil.Close() 패닉을 방지합니다.
	if opts.Name == "" {
		return &multiCloser{}, nil
	}

	// 로그 저장 경로가 명시되지 않은 경우, 실행 위치의 'logs' 디렉토리를 기본값으로 사용합니다.
	logDir := opts.Dir
	if logDir == "" {
		logDir = "logs"
	}

	// 로그 디렉토리 생성
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("로그 디렉토리 생성 실패: %w", err)
	}

	// 로그 출력 전략을 '콘솔(Stdout)'과 '파일(File)'로 이원화하여 구성합니다.
	//
	// 1. 콘솔 출력 (Stdout):
	//    - 컨테이너 및 클라우드 환경의 표준인 '12-Factor App' 로깅 원칙을 준수합니다.
	//    - 개발 시에는 터미널 직관성을, 운영 시에는 로그 수집 파이프라인(Fluentd 등)과의 호환성을 보장합니다.
	if opts.EnableConsoleLog {
		logrus.SetOutput(os.Stdout)
	} else {
		logrus.SetOutput(io.Discard) // 콘솔 출력 비활성화
	}

	// 2. 파일 출력 (Lumberjack):
	//    - 단순한 파일 기록을 넘어, 로그 레벨에 따른 '지능형 라우팅'을 수행합니다. (Logrus Hook 활용)
	//    - 전략:
	//      * Critical (Error/Fatal): 별도 파일로 격리하여 장애 발생 시 즉각적인 원인 분석을 지원합니다.
	//      * Verbose (Debug/Trace): 메인 로그의 가독성을 해치지 않도록 별도 파일로 분리하여 저장합니다.

	mainLogger := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, fmt.Sprintf("%s.%s", opts.Name, fileExt)),
		MaxSize:    defaultMaxSizeMB,
		MaxBackups: defaultMaxBackups,
		MaxAge:     opts.MaxAge,
		Compress:   true,
		LocalTime:  true,
	}

	var hook *LogLevelHook
	var criticalLogger, verboseLogger *lumberjack.Logger

	if opts.EnableCriticalLog {
		// Error, Fatal, Panic 레벨을 저장할 파일
		criticalLogger = &lumberjack.Logger{
			Filename:   filepath.Join(logDir, fmt.Sprintf("%s.critical.%s", opts.Name, fileExt)),
			MaxSize:    defaultMaxSizeMB,
			MaxBackups: defaultMaxBackups,
			MaxAge:     opts.MaxAge,
			Compress:   true,
			LocalTime:  true,
		}
	}

	if opts.EnableVerboseLog {
		// Debug, Trace 레벨을 저장할 파일
		verboseLogger = &lumberjack.Logger{
			Filename:   filepath.Join(logDir, fmt.Sprintf("%s.verbose.%s", opts.Name, fileExt)),
			MaxSize:    defaultMaxSizeMB,
			MaxBackups: defaultMaxBackups,
			MaxAge:     opts.MaxAge,
			Compress:   true,
			LocalTime:  true,
		}
	}

	// 모든 파일 로깅을 Hook이 전담
	hook = &LogLevelHook{
		mainWriter: mainLogger,

		formatter: logrus.StandardLogger().Formatter,
	}

	if criticalLogger != nil {
		hook.criticalWriter = criticalLogger

		// 서버가 재시작될 때마다 강제로 로그 로테이션을 수행하여
		// 일자별 로그 파일 분리를 보장하고, 이전 로그와의 명확한 경계를 형성합니다.
		_ = criticalLogger.Rotate()
	}
	if verboseLogger != nil {
		hook.verboseWriter = verboseLogger

		// 서버가 재시작될 때마다 강제로 로그 로테이션을 수행하여
		// 일자별 로그 파일 분리를 보장하고, 이전 로그와의 명확한 경계를 형성합니다.
		_ = verboseLogger.Rotate()
	}
	logrus.AddHook(hook)

	// 서버가 재시작될 때마다 강제로 로그 로테이션을 수행하여
	// 일자별 로그 파일 분리를 보장하고, 이전 로그와의 명확한 경계를 형성합니다.
	_ = mainLogger.Rotate()

	// multiCloser 구성
	closers := []io.Closer{mainLogger}
	if criticalLogger != nil {
		closers = append(closers, criticalLogger)
	}
	if verboseLogger != nil {
		closers = append(closers, verboseLogger)
	}

	return &multiCloser{
		closers: closers,

		hook: hook,
	}, nil
}

func SetDebugMode(debug bool) {
	if debug {
		logrus.SetLevel(logrus.TraceLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}

// WithComponent component 필드를 포함한 로그 Entry를 반환합니다.
// 모든 로그에 component 필드를 일관되게 추가하기 위해 사용합니다.
func WithComponent(component string) *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"component": component,
	})
}

// WithComponentAndFields component 필드와 추가 필드를 포함한 로그 Entry를 반환합니다.
func WithComponentAndFields(component string, fields logrus.Fields) *logrus.Entry {
	l := len(fields)
	newFields := make(logrus.Fields, l+1)
	newFields["component"] = component

	for k, v := range fields {
		newFields[k] = v
	}
	return logrus.WithFields(newFields)
}
