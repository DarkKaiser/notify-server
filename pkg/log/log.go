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

var (
	// 로그 디렉토리의 기본 경로 (빈 문자열 = 현재 디렉토리)
	logDirectoryBasePath = ""

	// 호출자 경로에서 축약할 prefix
	callerFunctionPathPrefix = ""
)

const (
	// 로그 파일이 저장될 디렉토리 이름
	defaultLogDirectoryName = "logs"

	// 로그 파일의 확장자
	defaultLogFileExtension = "log"
)

// InitFileOptions 로그 파일 초기화 옵션입니다.
type InitFileOptions struct {
	// EnableCriticalLog 치명적인 오류(Error, Fatal, Panic) 레벨의 로그를 별도 파일로 분리합니다.
	EnableCriticalLog bool

	// EnableVerboseLog 상세 정보(Debug, Trace) 레벨의 로그를 별도 파일로 분리합니다.
	EnableVerboseLog bool
}

func init() {
	log.SetLevel(log.TraceLevel)
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			function = fmt.Sprintf("%s(line:%d)", frame.Function, frame.Line)
			if callerFunctionPathPrefix != "" && strings.HasPrefix(function, callerFunctionPathPrefix) {
				function = "..." + function[len(callerFunctionPathPrefix):]
			}

			return
		},
	})
}

// InitFile 로그 파일 출력을 초기화합니다.
// 이 함수는 환경설정 로드 전에 호출하여 모든 로그가 파일에 기록되도록 합니다.
// Debug 모드 설정은 SetDebugMode()를 통해 별도로 수행합니다.
func InitFile(appName string, retentionDays float64) io.Closer {
	return InitFileWithOptions(appName, retentionDays, InitFileOptions{})
}

// InitFileWithOptions는 옵션을 사용하여 로그 파일 출력을 초기화합니다.
// 레벨별 로그 파일 분리 기능을 사용하려면 이 함수를 사용하세요.
func InitFileWithOptions(appName string, retentionDays float64, opts InitFileOptions) io.Closer {
	logDirectoryPath := filepath.Join(logDirectoryBasePath, defaultLogDirectoryName)

	// 로그 디렉토리 생성
	if err := os.MkdirAll(logDirectoryPath, 0755); err != nil {
		log.WithError(err).Fatal("로그 디렉토리 생성 실패")
	}

	timestamp := time.Now().Format("20060102150405")

	// 메인 로그 파일 생성
	mainLogFile, err := createLogFile(logDirectoryPath, appName, timestamp, "")
	if err != nil {
		log.WithError(err).Fatal("메인 로그 파일 생성 실패")
	}
	log.SetOutput(mainLogFile)

	// 레벨별 로그 파일 생성 및 Hook 등록
	var hook *LogLevelFileHook
	var errorLogFile, debugLogFile *os.File

	if opts.EnableCriticalLog {
		// Error, Fatal, Panic 레벨을 저장할 파일
		errorLogFile, err = createLogFile(logDirectoryPath, appName, timestamp, "error")
		if err != nil {
			log.WithError(err).Fatal("에러 로그 파일 생성 실패")
		}
	}

	if opts.EnableVerboseLog {
		// Debug, Trace 레벨을 저장할 파일
		debugLogFile, err = createLogFile(logDirectoryPath, appName, timestamp, "debug")
		if err != nil {
			log.WithError(err).Fatal("디버그 로그 파일 생성 실패")
		}
	}

	if errorLogFile != nil || debugLogFile != nil {
		hook = &LogLevelFileHook{
			criticalWriter: errorLogFile,
			verboseWriter:  debugLogFile,
			formatter:      log.StandardLogger().Formatter,
		}
		log.AddHook(hook)
	}

	// 만료된 로그 파일 삭제
	removeExpiredLogFiles(appName, retentionDays)

	return &multiCloser{
		closers: []io.Closer{mainLogFile, errorLogFile, debugLogFile},
		hook:    hook,
	}
}

// createLogFile 로그 파일을 생성합니다.
func createLogFile(dirPath, appName, timestamp, suffix string) (*os.File, error) {
	var fileName string
	if suffix == "" {
		fileName = fmt.Sprintf("%s-%s.%s", appName, timestamp, defaultLogFileExtension)
	} else {
		fileName = fmt.Sprintf("%s-%s.%s.%s", appName, timestamp, suffix, defaultLogFileExtension)
	}

	filePath := filepath.Join(dirPath, fileName)
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// SetCallerPathPrefix 호출자 정보에서 축약할 경로 prefix를 설정합니다.
// main() 함수 초기에 호출하여 호출자 경로 표시를 커스터마이징할 수 있습니다.
// 예제: SetCallerPathPrefix("github.com/darkkaiser")
func SetCallerPathPrefix(prefix string) {
	callerFunctionPathPrefix = prefix
}

// SetDebugMode Debug 모드에 따라 로그 레벨을 설정합니다.
// - Debug 모드: Trace 레벨 (모든 로그 출력)
// - 운영 모드: Info 레벨 (Info, Warn, Error, Fatal만 출력)
func SetDebugMode(debug bool) {
	if debug {
		log.SetLevel(log.TraceLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

func removeExpiredLogFiles(appName string, retentionDays float64) {
	logDirectoryPath := filepath.Join(logDirectoryBasePath, defaultLogDirectoryName)

	entries, err := os.ReadDir(logDirectoryPath)
	if err != nil {
		log.WithFields(log.Fields{
			"component": "log",
			"directory": logDirectoryPath,
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
		if !strings.HasPrefix(fileName, appName) || filepath.Ext(fileName) != "."+defaultLogFileExtension {
			continue
		}

		// 파일 정보 가져오기
		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}

		// 파일이 만료되었는지 확인
		daysAgo := math.Abs(now.Sub(fileInfo.ModTime()).Hours()) / hoursPerDay
		if daysAgo < retentionDays {
			continue
		}

		// 만료된 파일 삭제
		filePath := filepath.Join(logDirectoryPath, fileName)
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

// MaskSensitiveData 민감한 정보를 마스킹합니다.
// 토큰, 키 등의 민감 정보를 안전하게 로깅하기 위해 사용합니다.
func MaskSensitiveData(data string) string {
	if data == "" {
		return ""
	}

	// 3자 이하는 전체 마스킹
	if len(data) <= 3 {
		return "***"
	}

	// 앞 4자만 표시하고 나머지는 마스킹
	if len(data) <= 12 {
		return data[:4] + "***"
	}

	// 긴 토큰은 앞 4자 + 마스킹 + 뒤 4자
	return data[:4] + "***" + data[len(data)-4:]
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
