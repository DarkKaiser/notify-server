package main

import (
	"encoding/json"
	"github.com/darkkaiser/notify-server/cron1"
	"github.com/darkkaiser/notify-server/global"
	_log_ "github.com/darkkaiser/notify-server/log"
	"github.com/darkkaiser/notify-server/server"
	"github.com/darkkaiser/notify-server/task"
	"github.com/darkkaiser/notify-server/utils"
	"io/ioutil"
	"log"
	"time"
)

func main() {
	// 환경설정 정보를 읽어들이고 초기화한다.
	appConfig := initAppConfig(global.AppConfigFileName)

	// 로그를 초기화한다.
	_log_.InitLog(appConfig)

	// @@@@@ 다시 작성
	log.Print("############################################################")
	log.Print("###                                                      ###")
	log.Printf("###                 %s %s                  ###", global.AppName, global.AppVersion)
	log.Print("###                                                      ###")
	log.Print("###                             developed by DarkKaiser  ###")
	log.Print("###                                                      ###")
	log.Print("############################################################")

	// 일정 시간이 지난 로그파일을 모두 삭제한다.
	_log_.CleanOutOfLogFiles()

	// @@@@@
	log.Print(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>> START")

	tm := task.TaskManager{}
	tm.Init()

	var c cron1.CronServer
	c.Tmm = &tm
	c.Start(appConfig)

	n := server.NotiServer{}
	n.Start(appConfig)
	//time.Sleep(3 * time.Second)
	//n.Notify(server.NOTIFIER_TELEGRAM, "테스트메시지")

	time.Sleep(3000 * time.Second)

	log.Print("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<< END")
}

func initAppConfig(path string) *global.AppConfig {
	data, err := ioutil.ReadFile(path)
	utils.CheckErr(err)

	var appConfig global.AppConfig
	err = json.Unmarshal(data, &appConfig)
	utils.CheckErr(err)

	// @@@@@
	//if os.IsPathSeparator(appConfig.Torrent.DownloadPath[len(appConfig.Torrent.DownloadPath)-1]) == false {
	//	appConfig.Torrent.DownloadPath += string(os.PathSeparator)
	//}

	return &appConfig
}
