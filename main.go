package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/darkkaiser/notify-server/g"
	_log_ "github.com/darkkaiser/notify-server/log"
	"github.com/darkkaiser/notify-server/service"
	"github.com/darkkaiser/notify-server/service/api"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/darkkaiser/notify-server/service/task"
	log "github.com/sirupsen/logrus"
)

// @title Notify Server API
// @version 1.0
// @description 외부 프로그램으로부터 수신된 메시지 및 등록된 태스크들의 실행 결과를 알리는 서버입니다.
// @termsOfService http://swagger.io/terms/

// @contact.name DarkKaiser
// @contact.url https://github.com/DarkKaiser
// @contact.email darkkaiser@gmail.com

// @license.name MIT
// @license.url https://github.com/DarkKaiser/notify-server/blob/master/LICENSE

// @BasePath /api/v1

// 빌드 정보 변수 (Dockerfile의 ldflags로 주입됨)
var (
	Version     = "dev"     // Git 커밋 해시
	BuildDate   = "unknown" // 빌드 날짜
	BuildNumber = "0"       // 빌드 번호
)

const (
	banner = `
  _   _         _    _   __          ____
 | \ | |  ___  | |_ (_) / _| _   _  / ___|   ___  _ __ __   __  ___  _ __
 |  \| | / _ \ | __|| || |_ | | | | \___ \  / _ \| '__|\ \ / / / _ \| '__|
 | |\  || (_) || |_ | ||  _|| |_| |  ___) ||  __/| |    \ V / |  __/| |
 |_| \_| \___/  \__||_||_|   \__, | |____/  \___||_|     \_/   \___||_|
                             |___/                                       v%s
                                                        developed by DarkKaiser
--------------------------------------------------------------------------------
`
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU()) // 모든 CPU 사용

	// 환경설정 정보를 읽어들인다.
	config := g.InitAppConfig()

	// 로그를 초기화하고, 일정 시간이 지난 로그 파일을 모두 삭제한다.
	_log_.Init(config.Debug, g.AppName, 30.)

	// 아스키아트 출력(https://ko.rakko.tools/tools/68/, 폰트:standard)
	fmt.Printf(banner, g.AppVersion)

	// 빌드 정보 출력
	log.Infof("빌드 정보 - 버전: %s, 빌드 날짜: %s, 빌드 번호: %s", Version, BuildDate, BuildNumber)
	log.Infof("Go 버전: %s, OS/Arch: %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	// 서비스를 생성하고 초기화한다.
	taskService := task.NewService(config)
	notificationService := notification.NewService(config, taskService)
	notifyAPIService := api.NewNotifyAPIService(config, notificationService)

	taskService.SetTaskNotificationSender(notificationService)

	// Set up cancellation context and waitgroup
	serviceStopCtx, cancel := context.WithCancel(context.Background())
	serviceStopWaiter := &sync.WaitGroup{}

	// 서비스를 시작한다.
	services := []service.Service{taskService, notificationService, notifyAPIService}
	for _, s := range services {
		serviceStopWaiter.Add(1)
		if err := s.Run(serviceStopCtx, serviceStopWaiter); err != nil {
			log.Errorf("서비스 시작 실패: %v", err)
			cancel() // 다른 서비스들도 종료
			serviceStopWaiter.Wait()
			log.Fatal("서비스 초기화 실패로 프로그램을 종료합니다")
		}
	}

	// Handle sigterm and await termC signal
	termC := make(chan os.Signal, 1)
	signal.Notify(termC, syscall.SIGINT, syscall.SIGTERM)

	<-termC // Blocks here until interrupted

	// Handle shutdown
	log.Info("Shutdown signal received")
	cancel()                 // Signal cancellation to context.Context
	serviceStopWaiter.Wait() // Block here until are workers are done
}
