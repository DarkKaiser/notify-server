package log

import (
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/darkkaiser/notify-server/utils"
	log "github.com/sirupsen/logrus"
)

var (
	logDirParentPath = ""
)

const (
	logDirName       string = "logs"
	logFileExtension string = "log"
)

func init() {
	log.SetLevel(log.TraceLevel)
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			const shortPath = "github.com/darkkaiser"

			function = fmt.Sprintf("%s(line:%d)", frame.Function, frame.Line)
			if strings.HasPrefix(function, shortPath) == true {
				function = "..." + function[len(shortPath):]
			}

			return
		},
	})
}

// Init 로그 파일 출력과 Debug 모드를 한 번에 초기화합니다.
// Deprecated: 대신 InitFile()과 SetDebugMode()를 순차적으로 호출하세요.
// 이렇게 하면 환경설정 로드 전에 로그 파일 출력을 먼저 활성화할 수 있습니다.
func Init(debug bool, appName string, checkDaysAgo float64) io.Closer {
	logFile := InitFile(appName, checkDaysAgo)
	SetDebugMode(debug)
	return logFile
}

// InitFile 로그 파일 출력을 초기화합니다.
// 이 함수는 환경설정 로드 전에 호출하여 모든 로그가 파일에 기록되도록 합니다.
// Debug 모드 설정은 SetDebugMode()를 통해 별도로 수행합니다.
func InitFile(appName string, checkDaysAgo float64) io.Closer {
	var logDirPath = fmt.Sprintf("%s%s", logDirParentPath, logDirName)

	// 로그 파일이 쌓이는 폴더를 생성한다.
	_, err := os.Stat(logDirPath)
	if os.IsNotExist(err) == true {
		utils.CheckErr(os.MkdirAll(logDirPath, 0755))
	}

	// 로그 파일을 생성한다.
	t := time.Now()
	logFilePath := fmt.Sprintf("%s%s%s-%d%02d%02d%02d%02d%02d.%s", logDirPath, string(os.PathSeparator), appName, t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), logFileExtension)
	logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	utils.CheckErr(err)

	log.SetOutput(logFile)

	// 일정 시간이 지난 로그 파일을 모두 삭제한다.
	cleanOutOfLogFiles(appName, checkDaysAgo)

	return logFile
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

func cleanOutOfLogFiles(appName string, checkDaysAgo float64) {
	var logDirPath = fmt.Sprintf("%s%s", logDirParentPath, logDirName)

	deList, err := os.ReadDir(logDirPath)
	if err != nil {
		return
	}
	fiList := make([]fs.FileInfo, 0, len(deList))
	for _, de := range deList {
		fi, err := de.Info()
		if err != nil {
			return
		}
		fiList = append(fiList, fi)
	}

	t := time.Now()
	for _, fi := range fiList {
		fileName := fi.Name()
		if strings.HasPrefix(fileName, appName) == false || strings.HasSuffix(fileName, logFileExtension) == false {
			continue
		}

		daysAgo := math.Abs(t.Sub(fi.ModTime()).Hours()) / 24
		if daysAgo >= checkDaysAgo {
			filePath := logDirPath + string(os.PathSeparator) + fileName

			err = os.Remove(filePath)
			if err == nil {
				log.WithFields(log.Fields{
					"component": "log",
					"file_path": filePath,
				}).Info("오래된 로그파일 삭제 성공")
			} else {
				log.WithFields(log.Fields{
					"component": "log",
					"file_path": filePath,
					"error":     err,
				}).Error("오래된 로그파일 삭제 실패")
			}
		}
	}
}
