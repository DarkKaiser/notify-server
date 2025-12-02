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
// @version 1.0.0
// @description 웹 스크래핑을 통해 수집한 정보를 알림으로 전송하는 서버의 REST API입니다.
// @description
// @description 이 API를 사용하면 외부 애플리케이션에서 텔레그램 등의 메신저로 알림 메시지를 전송할 수 있습니다.
// @description
// @description ## 주요 기능
// @description - 알림 메시지 전송
// @description - 다양한 알림 채널 지원 (Telegram 등)
// @description - 애플리케이션별 인증 및 권한 관리
// @description
// @description ## 인증 방법
// @description API 사용을 위해서는 사전에 등록된 애플리케이션 ID와 App Key가 필요합니다.
// @description 설정 파일(notify-server.json)의 allowed_applications에 애플리케이션을 등록한 후 사용하세요.
// @description
// @description ## 인증 플로우
// @description 1. **사전 준비**: notify-server.json의 allowed_applications에 애플리케이션 등록
// @description    - application_id, app_key, default_notifier_id 설정
// @description 2. **API 호출**: Query Parameter로 app_key 전달
// @description    - POST /api/v1/notice/message?app_key=YOUR_KEY
// @description 3. **인증 검증**: 서버에서 application_id와 app_key 확인
// @description    - 미등록 앱: 401 Unauthorized
// @description    - 잘못된 app_key: 401 Unauthorized
// @description 4. **알림 전송**: 인증 성공 시 텔레그램으로 메시지 전송
// @description    - 성공: 200 OK
// @description
// @description 자세한 인증 플로우 다이어그램은 GitHub README를 참조하세요.

// @termsOfService http://swagger.io/terms/

// @contact.name DarkKaiser
// @contact.url https://github.com/DarkKaiser
// @contact.email darkkaiser@gmail.com

// @license.name MIT
// @license.url https://github.com/DarkKaiser/notify-server/blob/master/LICENSE

// @host api.darkkaiser.com:2443
// @BasePath /api/v1

// @securityDefinitions.apikey ApiKeyAuth
// @in query
// @name app_key
// @description Application Key for authentication

// @externalDocs.description API 인증 가이드 (인증 플로우 다이어그램 포함)
// @externalDocs.url https://github.com/DarkKaiser/notify-server#api-인증-플로우

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
	notifyAPIService := api.NewNotifyAPIService(config, notificationService, Version, BuildDate, BuildNumber)

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
