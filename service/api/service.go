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

const (
	// shutdownTimeout 서버 종료 시 대기 시간
	shutdownTimeout = 5 * time.Second
)

// NotifyAPIService Notify API 서버의 생명주기를 관리하는 서비스입니다.
//
// 이 서비스는 다음과 같은 역할을 수행합니다:
//   - Echo 기반 HTTP/HTTPS 서버 시작 및 종료
//   - API 엔드포인트 라우팅 설정 (Health Check, Version, 알림 메시지 전송 등)
//   - Swagger UI 제공
//   - 서비스 상태 관리 (시작/중지)
//   - Graceful Shutdown 지원 (5초 타임아웃)
//
// 서비스는 고루틴으로 실행되며, context를 통해 종료 신호를 받습니다.
// Start() 메서드로 시작하고, context 취소로 종료됩니다.
type NotifyAPIService struct {
	appConfig *config.AppConfig

	running   bool
	runningMu sync.Mutex

	notificationService notification.Service

	buildInfo common.BuildInfo
}

// NewNotifyAPIService NotifyAPIService 인스턴스를 생성합니다.
//
// Returns:
//   - 초기화된 NotifyAPIService 인스턴스
func NewNotifyAPIService(appConfig *config.AppConfig, notificationService notification.Service, buildInfo common.BuildInfo) *NotifyAPIService {
	return &NotifyAPIService{
		appConfig: appConfig,

		running:   false,
		runningMu: sync.Mutex{},

		notificationService: notificationService,

		buildInfo: buildInfo,
	}
}

// Start API 서비스를 시작합니다.
//
// 서비스는 별도의 고루틴에서 실행되며, 다음 작업을 수행합니다:
//  1. Echo 서버 설정 (미들웨어, 라우트)
//  2. HTTP/HTTPS 서버 시작
//  3. Shutdown 신호 대기
//  4. Graceful Shutdown 처리
//
// Parameters:
//   - serviceStopCtx: 종료 신호를 받기 위한 Context (cancel 호출 시 종료)
//   - serviceStopWaiter: 서비스 종료 대기를 위한 WaitGroup
//
// Returns:
//   - error: notificationService가 nil이거나 이미 실행 중인 경우 에러 반환
//
// Note: 이 함수는 즉시 반환되며, 실제 서버는 고루틴에서 실행됩니다.
func (s *NotifyAPIService) Start(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent("api.service").Info("NotifyAPI 서비스 시작중...")

	if s.notificationService == nil {
		defer serviceStopWaiter.Done()
		return apperrors.New(apperrors.ErrInternal, "notificationService 객체가 초기화되지 않았습니다")
	}

	if s.running {
		defer serviceStopWaiter.Done()
		applog.WithComponent("api.service").Warn("NotifyAPI 서비스가 이미 시작됨!!!")
		return nil
	}

	go s.runServiceLoop(serviceStopCtx, serviceStopWaiter)

	s.running = true

	applog.WithComponent("api.service").Info("NotifyAPI 서비스 시작됨")

	return nil
}

// runServiceLoop 서비스의 메인 실행 루프입니다.
// 서버 설정, HTTP 서버 시작, Shutdown 대기를 순차적으로 수행합니다.
func (s *NotifyAPIService) runServiceLoop(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
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
//
// 다음 순서로 서버를 구성합니다:
//  1. ApplicationManager 생성 (API 인증 관리)
//  2. Handler 생성 (System, v1 API)
//  3. Echo 서버 생성 (미들웨어 포함)
//  4. 라우트 등록 (전역, v1)
//
// Returns:
//   - 설정이 완료된 Echo 인스턴스
func (s *NotifyAPIService) setupServer() *echo.Echo {
	// ApplicationManager 생성
	applicationManager := apiauth.NewApplicationManager(s.appConfig)

	// Handler 생성
	systemHandler := handler.NewSystemHandler(s.notificationService, s.buildInfo)
	v1Handler := v1handler.NewHandler(applicationManager, s.notificationService)

	// Echo 서버 생성
	e := NewHTTPServer(HTTPServerConfig{
		Debug:        s.appConfig.Debug,
		AllowOrigins: s.appConfig.NotifyAPI.CORS.AllowOrigins,
	})

	// 라우트 설정
	SetupRoutes(e, systemHandler)
	v1.SetupRoutes(e, v1Handler)

	return e
}

// startHTTPServer HTTP 서버를 시작합니다.
//
// 설정에 따라 HTTP 또는 HTTPS 서버를 시작합니다.
// 서버가 종료되면 done 채널을 닫아 대기 중인 고루틴에 신호를 보냅니다.
//
// Parameters:
//   - e: Echo 인스턴스
//   - done: 서버 종료 신호를 보내기 위한 채널
//
// Note: 이 함수는 블로킹되며, 서버가 종료될 때까지 반환되지 않습니다.
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
// 정상 종료(http.ErrServerClosed)는 Info 레벨로 로깅하고,
// 그 외의 에러는 Error 레벨로 로깅하며 텔레그램 알림을 전송합니다.
func (s *NotifyAPIService) handleServerError(err error) {
	// 에러가 없으면 처리하지 않음 (정상 종료)
	if err == nil {
		return
	}

	// 정상적인 서버 종료
	if errors.Is(err, http.ErrServerClosed) {
		applog.WithComponent("api.service").Info("NotifyAPI 서비스 > http 서버 중지됨")
		return
	}

	// 예상치 못한 에러 발생
	msg := "NotifyAPI 서비스 > http 서버를 구성하는 중에 치명적인 오류가 발생하였습니다."
	applog.WithComponentAndFields("api.service", log.Fields{
		"port":  s.appConfig.NotifyAPI.WS.ListenPort,
		"error": err,
	}).Error(msg)

	s.notificationService.NotifyWithErrorToDefault(fmt.Sprintf("%s\r\n\r\n%s", msg, err))
}

// waitForShutdown Shutdown 신호를 대기하고 Graceful Shutdown을 처리합니다.
//
// 다음 순서로 종료를 처리합니다:
//  1. Context 취소 신호 대기 (블로킹)
//  2. Echo 서버에 Shutdown 신호 전송 (5초 타임아웃)
//  3. HTTP 서버 완전 종료 대기
//  4. 서비스 상태 정리 (running = false)
//
// Parameters:
//   - serviceStopCtx: 종료 신호를 받기 위한 Context
//   - e: Echo 인스턴스
//   - httpServerDone: HTTP 서버 종료 완료 신호를 받기 위한 채널
//
// Note: 이 함수는 서비스가 완전히 종료될 때까지 블로킹됩니다.
func (s *NotifyAPIService) waitForShutdown(serviceStopCtx context.Context, e *echo.Echo, httpServerDone chan struct{}) {
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
	s.notificationService = nil
	s.runningMu.Unlock()

	applog.WithComponent("api.service").Info("NotifyAPI 서비스 중지됨")
}
