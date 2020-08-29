package main

import (
	"context"
	"github.com/darkkaiser/notify-server/global"
	_log_ "github.com/darkkaiser/notify-server/log"
	"github.com/darkkaiser/notify-server/service/notify"
	"github.com/darkkaiser/notify-server/service/task"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU()) // 모든 CPU 사용

	// 환경설정 정보를 읽어들인다.
	config := global.InitAppConfig()

	// 로그를 초기화하고, 일정 시간이 지난 로그 파일을 모두 삭제한다.
	_log_.Init(config, 30.)

	log.Info("##########################################################")
	log.Info("###                                                    ###")
	log.Infof("###                %s %s                 ###", global.AppName, global.AppVersion)
	log.Info("###                                                    ###")
	log.Info("###                           developed by DarkKaiser  ###")
	log.Info("###                                                    ###")
	log.Info("##########################################################")

	log.Info(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>> START")

	// Set up cancellation context and waitgroup
	serviceStopCtx, cancel := context.WithCancel(context.Background())
	serviceStopWaiter := &sync.WaitGroup{}

	// @@@@@
	////////////////////////////////
	serviceStopWaiter.Add(2)
	task.NewTaskService(config, serviceStopCtx, serviceStopWaiter).Run()
	notify.NewNotifyService(config, serviceStopCtx, serviceStopWaiter).Run()
	////////////////////////////////

	// Handle sigterm and await termC signal
	termC := make(chan os.Signal)
	signal.Notify(termC, syscall.SIGINT, syscall.SIGTERM)

	<-termC // Blocks here until interrupted

	// Handle shutdown
	log.Info("Shutdown signal received")
	cancel()                 // Signal cancellation to context.Context
	serviceStopWaiter.Wait() // Block here until are workers are done

	log.Info("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<< END")
}
