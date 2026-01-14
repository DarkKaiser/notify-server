package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/darkkaiser/notify-server/internal/pkg/version"

	_ "github.com/darkkaiser/notify-server/docs"
	"github.com/darkkaiser/notify-server/internal/config"
	apiauth "github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/handler/system"
	v1 "github.com/darkkaiser/notify-server/internal/service/api/v1"
	v1handler "github.com/darkkaiser/notify-server/internal/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
)

const (
	// shutdownTimeout Graceful Shutdown 시 최대 대기 시간 (5초)
	shutdownTimeout = 5 * time.Second
)

// Service Notify API 서버의 생명주기를 관리하는 서비스입니다.
//
// 이 서비스는 다음과 같은 역할을 수행합니다:
//   - Echo 기반 HTTP/HTTPS 서버 시작 및 종료
//   - 미들웨어 체인 설정 (PanicRecovery, RequestID, RateLimiting, HTTPLogger, CORS, Secure)
//   - 인증 관리 (Authenticator 생성 및 API 엔드포인트 보호)
//   - API 엔드포인트 라우팅 설정 (Health Check, Version, 알림 메시지 전송 등)
//   - Swagger UI 제공
//   - 커스텀 HTTP 에러 핸들러 설정
//   - 서비스 상태 관리 (시작/중지)
//   - Graceful Shutdown 지원 (5초 타임아웃)
//   - 서버 에러 처리 및 알림 전송 (예상치 못한 에러 발생 시)
//
// 서비스는 고루틴으로 실행되며, context를 통해 종료 신호를 받습니다.
// Start() 메서드로 시작하고, context 취소로 종료됩니다.
type Service struct {
	appConfig *config.AppConfig

	notificationSender notifier.Sender

	buildInfo version.Info

	running   bool
	runningMu sync.Mutex
}

// NewService Service 인스턴스를 생성합니다.
func NewService(appConfig *config.AppConfig, notificationSender notifier.Sender, buildInfo version.Info) *Service {
	if appConfig == nil {
		panic(constants.PanicMsgAppConfigRequired)
	}
	if notificationSender == nil {
		panic(constants.PanicMsgNotificationSenderRequired)
	}

	return &Service{
		appConfig: appConfig,

		notificationSender: notificationSender,

		buildInfo: buildInfo,

		running:   false,
		runningMu: sync.Mutex{},
	}
}

// Start API 서비스를 시작합니다.
//
// 서비스는 별도의 고루틴에서 실행되며, 다음 작업을 수행합니다:
//  1. 서비스 상태 검증 (notificationSender nil 체크, 중복 실행 방지)
//  2. Echo 서버 설정 (Authenticator, Handler, 미들웨어, 라우트)
//  3. HTTP/HTTPS 서버 시작 (별도 고루틴)
//  4. Shutdown 신호 대기
//  5. Graceful Shutdown 처리 (5초 타임아웃)
//  6. 서버 에러 처리 및 알림 전송 (예상치 못한 에러 발생 시)
//  7. 서비스 상태 정리 (running 플래그 초기화)
//
// Parameters:
//   - serviceStopCtx: 서비스 종료 신호를 받기 위한 Context
//   - serviceStopWG: 서비스 종료 완료를 알리기 위한 WaitGroup
//
// Returns:
//   - error: notificationSender가 nil이거나 서비스가 이미 실행 중인 경우
//
// Note: 이 함수는 즉시 반환되며, 실제 서버는 고루틴에서 실행됩니다.
func (s *Service) Start(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent(constants.ComponentService).Info(constants.LogMsgServiceStarting)

	if s.running {
		defer serviceStopWG.Done()
		applog.WithComponent(constants.ComponentService).Warn(constants.LogMsgServiceAlreadyStarted)
		return nil
	}

	s.running = true

	go s.runServiceLoop(serviceStopCtx, serviceStopWG)

	applog.WithComponent(constants.ComponentService).Info(constants.LogMsgServiceStarted)

	return nil
}

// runServiceLoop 서비스의 메인 실행 루프입니다.
// 서버 설정, HTTP 서버 시작, Shutdown 대기를 순차적으로 수행합니다.
func (s *Service) runServiceLoop(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) {
	defer serviceStopWG.Done()

	// 서버 설정
	e := s.setupServer()

	// HTTP 서버 시작
	httpServerDone := make(chan struct{})
	go s.startHTTPServer(e, httpServerDone)

	// Shutdown 대기
	s.waitForShutdown(serviceStopCtx, e, httpServerDone)
}

// setupServer Echo 서버 인스턴스를 생성하고 모든 설정을 완료합니다.
//
// 다음 순서로 서버를 구성합니다:
//  1. Authenticator 생성 (애플리케이션 인증 관리, App Key 해시 저장)
//  2. Handler 생성 (System 핸들러, v1 API 핸들러)
//  3. Echo 서버 생성 (미들웨어 체인, CORS 설정 포함)
//  4. 라우트 등록 (전역 라우트, v1 API 라우트)
func (s *Service) setupServer() *echo.Echo {
	// 1. Authenticator 생성
	authenticator := apiauth.NewAuthenticator(s.appConfig)

	// 2. Handler 생성
	systemHandler := system.NewHandler(s.notificationSender, s.buildInfo)
	v1Handler := v1handler.NewHandler(s.notificationSender)

	// 3. Echo 서버 생성 (미들웨어 체인 포함)
	e := NewHTTPServer(HTTPServerConfig{
		Debug:        s.appConfig.Debug,
		EnableHSTS:   s.appConfig.NotifyAPI.WS.TLSServer,
		AllowOrigins: s.appConfig.NotifyAPI.CORS.AllowOrigins,
	})

	// 4. 라우트 등록
	SetupRoutes(e, systemHandler)
	v1.SetupRoutes(e, v1Handler, authenticator)

	return e
}

// startHTTPServer HTTP/HTTPS 서버를 시작합니다.
//
// 설정에 따라 TLS 활성화 여부를 결정하며, 서버가 종료되면 done 채널을 닫아
// 대기 중인 고루틴에 신호를 보냅니다.
//
// Parameters:
//   - e: Echo 서버 인스턴스
//   - done: 서버 종료 완료 신호 채널
//
// Note: 이 함수는 블로킹되며, 서버가 종료될 때까지 반환되지 않습니다.
func (s *Service) startHTTPServer(e *echo.Echo, done chan struct{}) {
	defer close(done)

	port := s.appConfig.NotifyAPI.WS.ListenPort
	applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
		"port": port,
	}).Debug(constants.LogMsgServiceHTTPServerStarting)

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

// handleServerError HTTP 서버 시작 중 발생한 에러를 처리합니다.
//
// 에러 처리 방식:
//   - nil: 처리하지 않음 (정상 종료)
//   - http.ErrServerClosed: Info 레벨 로깅 (Graceful Shutdown)
//   - 그 외: Error 레벨 로깅 + 텔레그램 알림 전송 (예상치 못한 에러)
func (s *Service) handleServerError(err error) {
	// nil: 정상 종료, 처리 불필요
	if err == nil {
		return
	}

	// http.ErrServerClosed: Graceful Shutdown 완료
	if errors.Is(err, http.ErrServerClosed) {
		applog.WithComponent(constants.ComponentService).Info(constants.LogMsgServiceHTTPServerStopped)
		return
	}

	// 예상치 못한 에러: 로깅 및 알림 전송
	message := constants.LogMsgServiceHTTPServerFatalError
	applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
		"port":  s.appConfig.NotifyAPI.WS.ListenPort,
		"error": err,
	}).Error(message)

	s.notificationSender.NotifyDefaultWithError(fmt.Sprintf("%s\r\n\r\n%s", message, err))
}

// waitForShutdown 종료 신호를 대기하고 Graceful Shutdown을 수행합니다.
//
// 종료 처리 순서:
//  1. 종료 신호 대기 (정상 종료 또는 서버 조기 종료)
//  2. Echo 서버 Shutdown 호출 (5초 타임아웃)
//  3. HTTP 서버 완전 종료 대기
//  4. 서비스 상태 정리 (running 플래그 초기화)
//
// Parameters:
//   - serviceStopCtx: 종료 신호를 받기 위한 Context
//   - e: Echo 서버 인스턴스
//   - httpServerDone: HTTP 서버 종료 완료 신호 채널
//
// Note: 이 함수는 서비스가 완전히 종료될 때까지 블로킹됩니다.
func (s *Service) waitForShutdown(serviceStopCtx context.Context, e *echo.Echo, httpServerDone chan struct{}) {
	select {
	case <-serviceStopCtx.Done():
		// 정상적인 종료 신호 수신
		applog.WithComponent(constants.ComponentService).Info(constants.LogMsgServiceStopping)
	case <-httpServerDone:
		// HTTP 서버가 예기치 않게 종료됨 (포트 바인딩 실패, 패닉 등)
		// 이미 종료되었으므로 Shutdown 호출 없이 상태만 정리
		applog.WithComponent(constants.ComponentService).Error(constants.LogMsgServiceUnexpectedExit)

		s.cleanup()

		return
	}

	// Graceful Shutdown 시작 (5초 타임아웃)
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		applog.WithComponentAndFields(constants.ComponentService, applog.Fields{
			"error": err,
		}).Error(constants.LogMsgServiceHTTPServerShutdownError)
	}

	<-httpServerDone

	s.cleanup()
}

// cleanup 서비스 종료 후 상태를 정리합니다.
func (s *Service) cleanup() {
	s.runningMu.Lock()
	s.running = false
	// 주의: notificationSender는 의도적으로 nil로 설정하지 않음
	// - 종료 중에도 다른 고루틴(Health Check 등)이 접근 가능
	// - nil 설정 시 동시 접근으로 인한 panic 위험
	// - 메모리는 GC가 Service 객체 해제 시 자동 정리
	s.runningMu.Unlock()

	applog.WithComponent(constants.ComponentService).Info(constants.LogMsgServiceStopped)
}
