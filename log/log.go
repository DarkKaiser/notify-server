package log

import (
	"fmt"
	"github.com/darkkaiser/notify-server/global"
	"github.com/darkkaiser/notify-server/utils"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strings"
	"time"
)

const (
	logFileDir       string = "logs"
	logFilePrefix           = global.AppName
	logFileExtension string = "log"
)

func InitLog(appConfig *global.AppConfig) {
	if appConfig.DebugMode == true {
		return
	}

	// 로그 파일이 쌓이는 폴더를 생성한다.
	_, err := os.Stat(logFileDir)
	if os.IsNotExist(err) == true {
		utils.CheckErr(os.MkdirAll(logFileDir, 0755))
	}

	// 로그 파일을 생성한다.
	t := time.Now()
	filePath := fmt.Sprintf("%s%s%s-%d%02d%02d%02d%02d%02d.%s", logFileDir, string(os.PathSeparator), logFilePrefix, t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), logFileExtension)
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	utils.CheckErr(err)

	log.SetOutput(file)
}

func CleanOutOfLogFiles() {
	fiList, err := ioutil.ReadDir(logFileDir)
	if err != nil {
		return
	}

	t := time.Now()
	for _, fi := range fiList {
		fileName := fi.Name()
		if strings.HasPrefix(fileName, logFilePrefix) == false || strings.HasSuffix(fileName, logFileExtension) == false {
			continue
		}

		outOfDays := math.Abs(t.Sub(fi.ModTime()).Hours()) / 24
		if outOfDays >= 15. {
			filePath := logFileDir + string(os.PathSeparator) + fileName

			err = os.Remove(filePath)
			if err == nil {
				log.Printf("오래된 로그파일 삭제 성공(%s)", filePath)
			} else {
				log.Printf("오래된 로그파일 삭제 실패(%s), %s", filePath, err)
			}
		}
	}
}
