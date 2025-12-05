package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/darkkaiser/notify-server/config"
	_ "github.com/darkkaiser/notify-server/docs"
	"github.com/darkkaiser/notify-server/pkg/common"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/service/api/v1/httpserver"
	"github.com/darkkaiser/notify-server/service/notification"
	log "github.com/sirupsen/logrus"
)

// NotifyAPIService Notify API 서버의 생명주기를 관리하는 서비스입니다.
//
// 이 서비스는 다음과 같은 역할을 수행합니다:
//   - Echo 기반 HTTP/HTTPS 서버 시작 및 종료
//   - API 엔드포인트 라우팅 설정 (Health Check, Version, 알림 메시지 전송 등)
//   - Swagger UI 제공
//   - 서비스 상태 관리 (시작/중지)
//   - Graceful Shutdown 지원
//
// 서비스는 고루틴으로 실행되며, context를 통해 종료 신호를 받습니다.
type NotifyAPIService struct {
	appConfig *config.AppConfig

	running   bool
	runningMu sync.Mutex

	notificationSender notification.NotificationSender

	buildInfo common.BuildInfo
}

func NewNotifyAPIService(appConfig *config.AppConfig, notificationSender notification.NotificationSender, buildInfo common.BuildInfo) *NotifyAPIService {
	return &NotifyAPIService{
		appConfig: appConfig,

		running:   false,
		runningMu: sync.Mutex{},

		notificationSender: notificationSender,

		buildInfo: buildInfo,
	}
}

func (s *NotifyAPIService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent("api.service").Info("NotifyAPI 서비스 시작중...")

	if s.notificationSender == nil {
		defer serviceStopWaiter.Done()

		return apperrors.New(apperrors.ErrInternal, "NotificationSender 객체가 초기화되지 않았습니다")
	}

	if s.running {
		defer serviceStopWaiter.Done()

		applog.WithComponent("api.service").Warn("NotifyAPI 서비스가 이미 시작됨!!!")

		return nil
	}

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	applog.WithComponent("api.service").Info("NotifyAPI 서비스 시작됨")

	return nil
}

func (s *NotifyAPIService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	// main.go에서 전달받은 빌드 정보를 Handler에 전달
	h := handler.NewHandler(s.appConfig, s.notificationSender, s.buildInfo)

	// HTTP 서버 생성 (미들웨어 및 라우트 설정 포함)
	e := httpserver.New(httpserver.Config{
		Debug:        s.appConfig.Debug,
		AllowOrigins: s.appConfig.NotifyAPI.CORS.AllowOrigins,
	}, h)

	// httpServerDone은 HTTP 서버 고루틴이 종료될 때까지 대기하기 위한 채널이다.
	// 서비스 종료 시 s.notificationSender를 nil로 설정하기 전에 HTTP 서버가 완전히 종료되었음을 보장하여
	// 경쟁 상태(Race Condition)를 방지한다.
	httpServerDone := make(chan struct{})

	go func(listenPort int) {
		defer close(httpServerDone)

		applog.WithComponentAndFields("api.service", log.Fields{
			"port": listenPort,
		}).Debug("NotifyAPI 서비스 > http 서버 시작")

		var err error
		if s.appConfig.NotifyAPI.WS.TLSServer {
			err = e.StartTLS(fmt.Sprintf(":%d", listenPort), s.appConfig.NotifyAPI.WS.TLSCertFile, s.appConfig.NotifyAPI.WS.TLSKeyFile)
		} else {
			err = e.Start(fmt.Sprintf(":%d", listenPort))
		}

		// Start(), StartTLS() 함수는 항상 nil이 아닌 error를 반환한다.
		if errors.Is(err, http.ErrServerClosed) {
			applog.WithComponent("api.service").Info("NotifyAPI 서비스 > http 서버 중지됨")
		} else {
			m := "NotifyAPI 서비스 > http 서버를 구성하는 중에 치명적인 오류가 발생하였습니다."

			applog.WithComponentAndFields("api.service", log.Fields{
				"port":  s.appConfig.NotifyAPI.WS.ListenPort,
				"error": err,
			}).Error(m)

			s.notificationSender.NotifyWithErrorToDefault(fmt.Sprintf("%s\r\n\r\n%s", m, err))
		}
	}(s.appConfig.NotifyAPI.WS.ListenPort)

	select {
	case <-serviceStopCtx.Done():
		applog.WithComponent("api.service").Info("NotifyAPI 서비스 중지중...")

		// 웹서버를 종료한다.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := e.Shutdown(ctx); err != nil {
			applog.WithComponentAndFields("api.service", log.Fields{
				"error": err,
			}).Error(err)
		}

		<-httpServerDone

		s.runningMu.Lock()
		s.running = false
		s.notificationSender = nil
		s.runningMu.Unlock()

		applog.WithComponent("api.service").Info("NotifyAPI 서비스 중지됨")
	}
}
