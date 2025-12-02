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

func Init(debug bool, appName string, checkDaysAgo float64) io.Closer {
	// Debug 모드에 따라 로그 레벨 설정
	// - Debug 모드: Trace 레벨 (모든 로그 출력)
	// - 운영 모드: Info 레벨 (Info, Warn, Error, Fatal만 출력)
	if debug == true {
		log.SetLevel(log.TraceLevel)
	} else {
		// 운영 환경에서는 Info 레벨 이상만 출력
		log.SetLevel(log.InfoLevel)
	}

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
