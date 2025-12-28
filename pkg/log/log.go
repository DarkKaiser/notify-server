package log

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
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

	EnableCriticalLog bool // Error 이상(Error, Fatal, Panic)의 치명적 로그를 별도 파일로 분리 저장할지 여부
	EnableVerboseLog  bool // Debug 이하(Trace, Debug)의 상세 로그를 별도 파일로 분리 저장할지 여부

	// ReportCaller: 로그를 호출한 소스 코드의 위치(파일명:라인번호)를 함께 기록할지 여부
	// 예: true로 설정 시 "main.go:55" 처럼 로그가 발생한 위치를 알 수 있어 디버깅에 유용합니다.
	ReportCaller bool

	// CallerPathPrefix: 로그에 출력되는 파일 경로가 너무 길 때, 앞부분을 잘라내어 보기 좋게 만듭니다.
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
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			function = fmt.Sprintf("%s(line:%d)", frame.Function, frame.Line)
			if opts.CallerPathPrefix != "" && strings.HasPrefix(function, opts.CallerPathPrefix) {
				function = "..." + function[len(opts.CallerPathPrefix):]
			}
			return
		},
	})

	// 파일 로깅을 위해서는 애플리케이션 식별자(Name)가 필수입니다.
	// Name이 비어있는 경우, 전역 설정(Formatter 등)만 적용하고 파일 출력 설정은 생략합니다.
	// 이는 에러가 아니므로 nil error를 반환합니다.
	if opts.Name == "" {
		return nil, nil
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

	timestamp := time.Now().Format(timestampFormat)

	// 메인 로그 파일 생성
	mainLogFile, err := createLogFile(logDir, opts.Name, timestamp, "")
	if err != nil {
		return nil, fmt.Errorf("메인 로그 파일 생성 실패: %w", err)
	}
	log.SetOutput(mainLogFile)

	// 레벨별 로그 파일 생성 및 Hook 등록
	var hook *LogLevelHook
	var criticalLogFile, verboseLogFile *os.File

	if opts.EnableCriticalLog {
		// Error, Fatal, Panic 레벨을 저장할 파일
		criticalLogFile, err = createLogFile(logDir, opts.Name, timestamp, "critical")
		if err != nil {
			return nil, fmt.Errorf("에러 로그 파일 생성 실패: %w", err)
		}
	}

	if opts.EnableVerboseLog {
		// Debug, Trace 레벨을 저장할 파일
		verboseLogFile, err = createLogFile(logDir, opts.Name, timestamp, "verbose")
		if err != nil {
			return nil, fmt.Errorf("디버그 로그 파일 생성 실패: %w", err)
		}
	}

	if criticalLogFile != nil || verboseLogFile != nil {
		hook = &LogLevelHook{
			criticalWriter: criticalLogFile,
			verboseWriter:  verboseLogFile,
			formatter:      log.StandardLogger().Formatter,
		}
		log.AddHook(hook)
	}

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
func createLogFile(dirPath, appName, timestamp, suffix string) (*os.File, error) {
	var fileName string
	if suffix == "" {
		fileName = fmt.Sprintf("%s-%s.%s", appName, timestamp, fileExt)
	} else {
		fileName = fmt.Sprintf("%s-%s.%s.%s", appName, timestamp, suffix, fileExt)
	}

	filePath := filepath.Join(dirPath, fileName)
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func removeExpiredLogFiles(logDir, appName string, retentionDays int) {
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
			log.WithFields(log.Fields{
				"component": "log",
				"file_name": fileName,
				"error":     err,
			}).Warn("로그 파일 정보 읽기 실패")

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
	newFields := make(log.Fields, len(fields)+1)
	for k, v := range fields {
		newFields[k] = v
	}
	newFields["component"] = component
	return log.WithFields(newFields)
}
