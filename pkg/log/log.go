package log

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// 로그 파일 확장자
	fileExt = "log"

	// 로그 파일명에 사용되는 타임스탬프 포맷
	timestampFormat = "20060102150405"
)

// Options 로그 초기화 옵션입니다.
type Options struct {
	Dir           string // 로그 파일이 저장될 디렉토리 경로 (기본값: "logs")
	Name          string // 로그 파일명 생성에 사용될 애플리케이션 식별자 (필수)
	RetentionDays int    // 오래된 로그 삭제 기준일 (일 단위, 0: 삭제 안 함)

	EnableCriticalLog bool // ERROR 이상(ERROR, FATAL, PANIC)의 치명적 로그를 별도 파일로 분리 저장할지 여부
	EnableVerboseLog  bool // DEBUG 이하(DEBUG, TRACE)의 상세 로그를 별도 파일로 분리 저장할지 여부
	EnableConsoleLog  bool // 표준 출력(Stdout)에도 로그를 출력할지 여부 (개발 환경 권장)

	// 로그 파일 생성 시의 권한 (기본값: 0644 - 소유자 쓰기/읽기, 그룹/기타 읽기)
	FileMode os.FileMode

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
//   - 옵션에 따른 로그 파일 생성 및 로테이션 정책 적용
//   - 우아한 종료(Graceful Shutdown)를 위한 Closer 반환
//
// 주의:
//   - 애플리케이션 시작 시점(main 함수 도입부)에 **단 한 번만** 호출해야 합니다.
//   - 반환된 Closer는 반드시 defer를 통해 리소스가 해제되도록 보장해야 합니다.
func Setup(opts Options) (io.Closer, error) {
	log.SetLevel(log.TraceLevel)
	log.SetReportCaller(opts.ReportCaller)
	log.SetFormatter(&log.TextFormatter{
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

	// 기본 권한 설정
	if opts.FileMode == 0 {
		opts.FileMode = 0644
	}

	// 로그 디렉토리 생성
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("로그 디렉토리 생성 실패: %w", err)
	}

	timestamp := time.Now().Format(timestampFormat)

	// 메인 로그 파일 생성
	mainLogFile, err := createLogFile(logDir, opts.Name, timestamp, "", opts.FileMode)
	if err != nil {
		return nil, fmt.Errorf("메인 로그 파일 생성 실패: %w", err)
	}

	// 로그 출력 전략을 '콘솔(Stdout)'과 '파일(File)'로 이원화하여 구성합니다.
	//
	// 1. 콘솔 출력 (Stdout):
	//    - 컨테이너 및 클라우드 환경의 표준인 '12-Factor App' 로깅 원칙을 준수합니다.
	//    - 개발 시에는 터미널 직관성을, 운영 시에는 로그 수집 파이프라인(Fluentd 등)과의 호환성을 보장합니다.
	if opts.EnableConsoleLog {
		log.SetOutput(os.Stdout)
	} else {
		log.SetOutput(io.Discard) // 콘솔 출력 비활성화
	}

	// 2. 파일 출력 (File Logging):
	//    - 단순한 파일 기록을 넘어, 로그 레벨에 따른 '지능형 라우팅'을 수행합니다. (Logrus Hook 활용)
	//    - 전략:
	//      * Critical (Error/Fatal): 별도 파일로 격리하여 장애 발생 시 즉각적인 원인 분석을 지원합니다.
	//      * Verbose (Debug/Trace): 메인 로그의 가독성을 해치지 않도록 별도 파일로 분리하여 저장합니다.
	var hook *LogLevelHook
	var criticalLogFile, verboseLogFile *os.File

	if opts.EnableCriticalLog {
		// Error, Fatal, Panic 레벨을 저장할 파일
		criticalLogFile, err = createLogFile(logDir, opts.Name, timestamp, "critical", opts.FileMode)
		if err != nil {
			_ = mainLogFile.Close() // 메인 로그 파일 정리
			return nil, fmt.Errorf("에러 로그 파일 생성 실패: %w", err)
		}
	}

	if opts.EnableVerboseLog {
		// Debug, Trace 레벨을 저장할 파일
		verboseLogFile, err = createLogFile(logDir, opts.Name, timestamp, "verbose", opts.FileMode)
		if err != nil {
			// 이미 열린 파일들 정리
			if criticalLogFile != nil {
				_ = criticalLogFile.Close()
			}
			_ = mainLogFile.Close()
			return nil, fmt.Errorf("디버그 로그 파일 생성 실패: %w", err)
		}
	}

	// 모든 파일 로깅을 Hook이 전담
	hook = &LogLevelHook{
		mainWriter:     mainLogFile,
		criticalWriter: criticalLogFile,
		verboseWriter:  verboseLogFile,

		formatter: log.StandardLogger().Formatter,
	}
	log.AddHook(hook)

	// 만료된 로그 파일 삭제
	if opts.RetentionDays > 0 {
		go removeExpiredLogFiles(logDir, opts.Name, opts.RetentionDays)
	}

	return &multiCloser{
		closers: []io.Closer{mainLogFile, criticalLogFile, verboseLogFile},
		hook:    hook,
	}, nil
}

// createLogFile 로그 파일을 생성합니다.
func createLogFile(dirPath, appName, timestamp, suffix string, perm os.FileMode) (*os.File, error) {
	var fileName string
	if suffix == "" {
		fileName = fmt.Sprintf("%s-%s.%s", appName, timestamp, fileExt)
	} else {
		fileName = fmt.Sprintf("%s-%s.%s.%s", appName, timestamp, suffix, fileExt)
	}

	filePath := filepath.Join(dirPath, fileName)
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, perm)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func removeExpiredLogFiles(logDir, appName string, retentionDays int) {
	// 백그라운드 고루틴이므로 패닉 발생 시 앱 전체가 죽지 않도록 방어
	defer func() {
		if r := recover(); r != nil {
			log.WithFields(log.Fields{
				"component": "log",
				"recover":   r,
			}).Error("로그 정리 중 패닉 발생 (복구됨)")
		}
	}()

	entries, err := os.ReadDir(logDir)
	if err != nil {
		log.WithFields(log.Fields{
			"component": "log",
			"directory": logDir,
			"error":     err,
		}).Warn("로그 디렉토리 읽기 실패")

		return
	}

	const hoursPerDay = 24
	now := time.Now()

	for _, entry := range entries {
		// 디렉토리는 건너뛴다
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()

		// 앱 이름으로 시작하지 않거나 로그 파일 확장자가 아니면 건너뛴다
		if !strings.HasPrefix(fileName, appName) || filepath.Ext(fileName) != "."+fileExt {
			continue
		}

		// 파일 정보 가져오기
		fileInfo, err := entry.Info()
		if err != nil {
			// 파일이 이미 삭제되었거나 접근 불가능한 경우 (Race Condition 등)
			// 삭제 프로세스이므로 치명적이지 않음. Debug 레벨로 낮춰서 노이즈 감소.
			log.WithFields(log.Fields{
				"component": "log",
				"file_name": fileName,
				"error":     err,
			}).Debug("로그 파일 정보 읽기 실패 (건너뜀)")

			continue
		}

		// 파일이 만료되었는지 확인
		daysAgo := math.Abs(now.Sub(fileInfo.ModTime()).Hours()) / hoursPerDay
		if daysAgo < float64(retentionDays) {
			continue
		}

		// 만료된 파일 삭제
		filePath := filepath.Join(logDir, fileName)
		if err := os.Remove(filePath); err != nil {
			log.WithFields(log.Fields{
				"component": "log",
				"file_path": filePath,
				"error":     err,
			}).Error("오래된 로그파일 삭제 실패")
		} else {
			log.WithFields(log.Fields{
				"component": "log",
				"file_path": filePath,
			}).Info("오래된 로그파일 삭제 성공")
		}
	}
}

func SetDebugMode(debug bool) {
	if debug {
		log.SetLevel(log.TraceLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

// WithComponent component 필드를 포함한 로그 Entry를 반환합니다.
// 모든 로그에 component 필드를 일관되게 추가하기 위해 사용합니다.
func WithComponent(component string) *log.Entry {
	return log.WithFields(log.Fields{
		"component": component,
	})
}

// WithComponentAndFields component 필드와 추가 필드를 포함한 로그 Entry를 반환합니다.
func WithComponentAndFields(component string, fields log.Fields) *log.Entry {
	l := len(fields)
	newFields := make(log.Fields, l+1)
	newFields["component"] = component

	for k, v := range fields {
		newFields[k] = v
	}
	return log.WithFields(newFields)
}
