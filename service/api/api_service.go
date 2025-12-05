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
	apiauth "github.com/darkkaiser/notify-server/service/api/auth"
	"github.com/darkkaiser/notify-server/service/api/handler"
	v1 "github.com/darkkaiser/notify-server/service/api/v1"
	v1handler "github.com/darkkaiser/notify-server/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/labstack/echo/v4"
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

const (
	// shutdownTimeout 서버 종료 시 대기 시간
	shutdownTimeout = 5 * time.Second
)

func (s *NotifyAPIService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	// 서버 설정
	e := s.setupServer()

	// HTTP 서버 시작
	httpServerDone := make(chan struct{})
	go s.startHTTPServer(e, httpServerDone)

	// Shutdown 대기
	s.waitForShutdown(serviceStopCtx, e, httpServerDone)
}

// setupServer Echo 서버 및 라우트를 설정합니다.
func (s *NotifyAPIService) setupServer() *echo.Echo {
	// ApplicationManager 생성
	applicationManager := apiauth.NewApplicationManager(s.appConfig)

	// Handler 생성
	v1Handler := v1handler.NewHandler(applicationManager, s.notificationSender)
	systemHandler := handler.NewSystemHandler(s.notificationSender, s.buildInfo)

	// Echo 서버 생성
	e := NewServer(ServerConfig{
		Debug:        s.appConfig.Debug,
		AllowOrigins: s.appConfig.NotifyAPI.CORS.AllowOrigins,
	})

	// 라우트 설정
	SetupRoutes(e, systemHandler)
	v1.SetupRoutes(e, v1Handler)

	return e
}

// startHTTPServer HTTP 서버를 시작합니다.
func (s *NotifyAPIService) startHTTPServer(e *echo.Echo, done chan struct{}) {
	defer close(done)

	port := s.appConfig.NotifyAPI.WS.ListenPort
	applog.WithComponentAndFields("api.service", log.Fields{
		"port": port,
	}).Debug("NotifyAPI 서비스 > http 서버 시작")

	var err error
	if s.appConfig.NotifyAPI.WS.TLSServer {
		err = e.StartTLS(
			fmt.Sprintf(":%d", port),
			s.appConfig.NotifyAPI.WS.TLSCertFile,
			s.appConfig.NotifyAPI.WS.TLSKeyFile,
		)
	} else {
		err = e.Start(fmt.Sprintf(":%d", port))
	}

	s.handleServerError(err)
}

// handleServerError 서버 에러를 처리합니다.
func (s *NotifyAPIService) handleServerError(err error) {
	if errors.Is(err, http.ErrServerClosed) {
		applog.WithComponent("api.service").Info("NotifyAPI 서비스 > http 서버 중지됨")
		return
	}

	msg := "NotifyAPI 서비스 > http 서버를 구성하는 중에 치명적인 오류가 발생하였습니다."
	applog.WithComponentAndFields("api.service", log.Fields{
		"port":  s.appConfig.NotifyAPI.WS.ListenPort,
		"error": err,
	}).Error(msg)

	s.notificationSender.NotifyWithErrorToDefault(fmt.Sprintf("%s\r\n\r\n%s", msg, err))
}

// waitForShutdown Shutdown 신호를 대기하고 처리합니다.
func (s *NotifyAPIService) waitForShutdown(
	serviceStopCtx context.Context,
	e *echo.Echo,
	httpServerDone chan struct{},
) {
	<-serviceStopCtx.Done()

	applog.WithComponent("api.service").Info("NotifyAPI 서비스 중지중...")

	// 웹서버 종료
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		applog.WithComponentAndFields("api.service", log.Fields{
			"error": err,
		}).Error("서버 종료 중 오류 발생")
	}

	<-httpServerDone

	// 상태 정리
	s.runningMu.Lock()
	s.running = false
	s.notificationSender = nil
	s.runningMu.Unlock()

	applog.WithComponent("api.service").Info("NotifyAPI 서비스 중지됨")
}
