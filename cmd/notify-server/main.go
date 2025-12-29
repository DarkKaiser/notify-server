package main

import (
	"context"
	"runtime"

	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/pkg/version"
	"github.com/darkkaiser/notify-server/internal/service"
	"github.com/darkkaiser/notify-server/internal/service/api"
	"github.com/darkkaiser/notify-server/internal/service/notification"
	"github.com/darkkaiser/notify-server/internal/service/task"
	_ "github.com/darkkaiser/notify-server/internal/service/task/kurly"
	_ "github.com/darkkaiser/notify-server/internal/service/task/lotto"
	_ "github.com/darkkaiser/notify-server/internal/service/task/naver"
	_ "github.com/darkkaiser/notify-server/internal/service/task/navershopping"
	applog "github.com/darkkaiser/notify-server/pkg/log"
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
// @description 설정 파일(notify-server.json)의 notify_api.applications에 애플리케이션을 등록한 후 사용하세요.
// @description
// @description ## 인증 플로우
// @description 1. **사전 준비**: notify-server.json의 notify_api.applications에 애플리케이션 등록
// @description    - application_id, app_key, default_notifier_id 설정
// @description 2. **API 호출**: Query Parameter로 app_key 전달
// @description    - POST /api/v1/notifications?app_key=YOUR_KEY
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
// @BasePath /

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
                             |___/                           %s
                                                        developed by DarkKaiser
--------------------------------------------------------------------------------
`
)

func main() {
	// 1. 환경설정 로드 (로그 설정에 필요하므로 가장 먼저 수행한다)
	appConfig, err := config.InitAppConfig()
	if err != nil {
		// 로거 초기화 전이므로 표준 에러에 출력
		fmt.Fprintf(os.Stderr, "[FATAL] 환경설정 로드 실패: %v\n", err)
		os.Exit(1)
	}

	// 2. 로그 시스템 초기화
	var logOpts applog.Options
	if appConfig.Debug {
		logOpts = applog.NewDevelopmentConfig(config.AppName)
	} else {
		logOpts = applog.NewProductionConfig(config.AppName)
	}

	appLogCloser, err := applog.Setup(logOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] 로그 시스템 초기화 실패. 서버 구동을 중단합니다. (Cause: %v)\n", err)
		os.Exit(1)
	}
	defer appLogCloser.Close()

	// 3. 로그 레벨 최종 확정
	applog.SetDebugMode(appConfig.Debug)

	// 아스키아트 출력(https://ko.rakko.tools/tools/68/, 폰트:standard)
	fmt.Printf(banner, Version)

	// 빌드 정보 설정 (전역 싱글톤 등록)
	buildInfo := version.Info{
		Version:     Version,
		BuildDate:   BuildDate,
		BuildNumber: BuildNumber,
		GoVersion:   runtime.Version(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
	}
	version.Set(buildInfo)

	// 빌드 정보 출력
	applog.WithComponentAndFields("main", log.Fields{
		"version": buildInfo.String(),
		"env":     map[bool]string{true: "development", false: "production"}[appConfig.Debug],
	}).Info("서버 초기화 시작")

	// 서비스를 생성하고 초기화한다.
	taskService := task.NewService(appConfig)
	notificationService := notification.NewService(appConfig, taskService)
	apiService := api.NewService(appConfig, notificationService, buildInfo)

	taskService.SetNotificationSender(notificationService)

	// Set up cancellation context and waitgroup
	serviceStopCtx, cancel := context.WithCancel(context.Background())
	serviceStopWG := &sync.WaitGroup{}

	// 서비스를 시작한다.
	services := []service.Service{taskService, notificationService, apiService}
	for _, s := range services {
		serviceStopWG.Add(1)
		if err := s.Start(serviceStopCtx, serviceStopWG); err != nil {
			applog.WithComponentAndFields("main", log.Fields{
				"error": err,
			}).Error("서비스 초기화 실패")

			cancel() // 다른 서비스들도 종료
			serviceStopWG.Wait()

			log.Fatal("서비스 초기화 실패로 프로그램을 종료합니다")
		}
	}

	// Handle sigterm and await termC signal
	termC := make(chan os.Signal, 1)
	signal.Notify(termC, syscall.SIGINT, syscall.SIGTERM)

	applog.WithComponent("main").Info("서버 가동 완료")

	<-termC // Blocks here until interrupted

	// Handle shutdown
	applog.WithComponent("main").Info("Shutdown signal received")
	cancel()             // Signal cancellation to context.Context
	serviceStopWG.Wait() // Block here until are workers are done
}
