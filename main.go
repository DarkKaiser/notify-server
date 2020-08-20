package main

import (
	"github.com/darkkaiser/notify-server/cron1"
	"github.com/darkkaiser/notify-server/global"
	_log_ "github.com/darkkaiser/notify-server/log"
	"github.com/darkkaiser/notify-server/notifiers"
	"github.com/darkkaiser/notify-server/task"
	log "github.com/sirupsen/logrus"
	"time"
)

func main() {
	// 환경설정 정보를 읽어들인다.
	appConfig := global.InitAppConfig()

	// 로그를 초기화한다.
	_log_.InitLog(appConfig)

	log.Info("##########################################################")
	log.Info("###                                                    ###")
	log.Infof("###                %s %s                 ###", global.AppName, global.AppVersion)
	log.Info("###                                                    ###")
	log.Info("###                           developed by DarkKaiser  ###")
	log.Info("###                                                    ###")
	log.Info("##########################################################")

	// 일정 시간이 지난 로그파일을 모두 삭제한다.
	_log_.CleanOutOfLogFiles(30.)

	log.Print(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>> START")

	// @@@@@
	tm := task.TaskManager{}
	tm.Init(appConfig)

	var c cron1.CronServer
	c.Tmm = &tm
	c.Start(appConfig)

	n := notifiers.NotifierManager{}
	n.Start(appConfig)
	//time.Sleep(3 * time.Second)
	//n.Notify(server.NOTIFIER_TELEGRAM, "테스트메시지")

	time.Sleep(3000 * time.Second)

	log.Print("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<< END")
}
