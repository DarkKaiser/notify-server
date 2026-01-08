package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// 생성되는 로그 파일의 기본 확장자
	fileExt = "log"

	// 기본 로그 로테이션 정책
	defaultMaxSizeMB  = 100 // 로그 파일 하나당 최대 크기 (단위: MB)
	defaultMaxBackups = 20  // 로테이션 된 로그 파일의 최대 보관 개수
)

var (
	// Setup() 함수의 중복 호출을 방지하기 위한 동기화 객체
	// 프로세스 생명주기 동안 Setup()이 단 한 번만 실행되도록 보장합니다.
	setupOnce sync.Once

	// 전역 로깅 리소스의 해제 객체(Closer)를 보관합니다.
	// 최초 초기화 시 생성된 객체를 유지하여, Setup 재호출 시 동일한 인스턴스를 반환합니다.
	globalCloser io.Closer

	// 로깅 시스템 초기화 단계에서 발생한 에러를 보관합니다.
	// 초기화에 실패한 경우, 이후 Setup()이 재호출되더라도 재시도하지 않고 최초의 에러를 그대로 반환하여 일관성을 보장합니다.
	globalSetupErr error
)

// Setup 전역 로깅 시스템을 초기화하고 설정된 옵션에 따라 파일 출력을 구성합니다.
//
// 주의:
//   - 애플리케이션 시작 시점(main 함수 도입부)에 호출하는 것을 권장합니다.
//   - 반환된 Closer는 반드시 defer를 통해 리소스가 해제되도록 보장해야 합니다.
func Setup(opts Options) (io.Closer, error) {
	setupOnce.Do(func() {
		globalCloser, globalSetupErr = setupInternal(opts)
	})

	return globalCloser, globalSetupErr
}

// setupInternal 실제 로깅 시스템 초기화 로직을 수행합니다.
// 이 함수는 Setup()에서 sync.Once를 통해 단 한 번만 호출됩니다.
func setupInternal(opts Options) (io.Closer, error) {
	// Options 검증 (잘못된 설정값으로 인한 런타임 에러 방지)
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("유효하지 않은 로그 설정: %w", err)
	}

	// 로그 레벨 설정
	level := opts.Level
	if level == 0 {
		level = InfoLevel
	}
	logrus.SetLevel(level)

	// 호출자 정보(파일명, 라인번호) 기록 여부를 설정합니다.
	logrus.SetReportCaller(opts.ReportCaller)

	// Logrus는 io.Discard라도 포맷팅을 수행하므로, 이를 막기 위해 아무것도 안 하는 포맷터를 설정합니다.
	logrus.SetFormatter(&silentFormatter{})

	// 실제 파일/콘솔 출력에 사용할 TextFormatter를 설정합니다. (hook에서 사용)
	textFormatter := &logrus.TextFormatter{
		FullTimestamp:   true,         // TTY가 아니어도 타임스탬프를 항상 출력
		TimestampFormat: time.RFC3339, // "2006-01-02T15:04:05Z07:00" (ISO8601 표준)
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			function = frame.Function + "(line:" + strconv.Itoa(frame.Line) + ")"
			if opts.CallerPathPrefix != "" {
				if cut, found := strings.CutPrefix(function, opts.CallerPathPrefix); found {
					function = "..." + cut
				}
			}
			return
		},
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

	// 로그 로테이션 설정값 결정
	maxSize := opts.MaxSizeMB
	if maxSize == 0 {
		maxSize = defaultMaxSizeMB
	}
	maxBackups := opts.MaxBackups
	if maxBackups == 0 {
		maxBackups = defaultMaxBackups
	}

	// Logrus의 기본 출력(os.Stderr)은 비활성화하고, 모든 로그 처리를 Hook 시스템에 위임합니다.
	// 이는 중복 출력을 방지하고, 파일(Critical/Verbose)과 콘솔 출력을 중앙에서 정교하게 제어하기 위함입니다.
	logrus.SetOutput(io.Discard)

	// 콘솔 로깅이 활성화된 경우에만 표준 출력(stdout)을 대상으로 설정합니다.
	var consoleWriter io.Writer
	if opts.EnableConsoleLog {
		consoleWriter = os.Stdout
	}

	// 생성된 리소스(파일 핸들 등)를 추적하여, 초기화 실패 시 롤백하거나 종료 시 해제하기 위해 사용합니다.
	var closers []io.Closer
	succeeded := false

	// 에러 발생 시 이미 생성된 리소스를 정리합니다.
	// 성공 시에는 succeeded 플래그가 true로 설정되어 정리를 건너뜁니다.
	defer func() {
		if !succeeded {
			for _, c := range closers {
				if c != nil {
					_ = c.Close()
				}
			}
		}
	}()

	// 메인 로그 파일을 위한 Logger를 초기화합니다.
	mainLogger := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, fmt.Sprintf("%s.%s", opts.Name, fileExt)),
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     opts.MaxAge,
		Compress:   false,
		LocalTime:  true,
	}
	closers = append(closers, mainLogger)

	var h *hook
	var criticalLogger, verboseLogger *lumberjack.Logger

	// 중요 로그(Critical) 파일을 위한 Logger를 초기화합니다.
	if opts.EnableCriticalLog {
		criticalLogger = &lumberjack.Logger{
			Filename:   filepath.Join(logDir, fmt.Sprintf("%s.critical.%s", opts.Name, fileExt)),
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     opts.MaxAge,
			Compress:   false,
			LocalTime:  true,
		}
		closers = append(closers, criticalLogger)
	}

	// 상세 로그(Verbose) 파일을 위한 Logger를 초기화합니다.
	if opts.EnableVerboseLog {
		verboseLogger = &lumberjack.Logger{
			Filename:   filepath.Join(logDir, fmt.Sprintf("%s.verbose.%s", opts.Name, fileExt)),
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     opts.MaxAge,
			Compress:   false,
			LocalTime:  true,
		}
		closers = append(closers, verboseLogger)
	}

	// 메인 로그, 중요 로그(Critical), 상세 로그(Verbose), 콘솔 출력을 중앙에서 분배할 Hook을 생성합니다.
	h = &hook{
		mainWriter: mainLogger,
		formatter:  textFormatter,
	}

	// 활성화된 옵션에 따라 추가적인 Writer(Critical, Verbose, Console)를 연결합니다.
	if criticalLogger != nil {
		h.criticalWriter = criticalLogger
	}
	if verboseLogger != nil {
		h.verboseWriter = verboseLogger
	}
	if consoleWriter != nil {
		h.consoleWriter = consoleWriter
	}

	// 구성된 Hook을 등록하여 실제 로깅 라우팅 시스템을 활성화합니다.
	logrus.AddHook(h)

	// 모든 초기화가 성공했음을 표시하여 defer의 리소스 정리를 건너뜁니다.
	succeeded = true

	// 리소스 해제 객체(Closer)를 생성하여 모든 리소스를 추적하고 관리합니다.
	c := &closer{
		closers: closers,
		hook:    h,
	}

	// Fatal 로그 발생 시(os.Exit 호출 직전) 버퍼에 남은 로그를 디스크에 쓰고 리소스를 안전하게 해제하도록 핸들러를 등록합니다.
	logrus.RegisterExitHandler(func() {
		_ = c.Close()
	})

	return c, nil
}
