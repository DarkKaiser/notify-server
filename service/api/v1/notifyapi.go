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
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/service/api/v1/handler"
	"github.com/darkkaiser/notify-server/service/api/v1/router"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// NotifyAPIService
type NotifyAPIService struct {
	appConfig *config.AppConfig

	running   bool
	runningMu sync.Mutex

	notificationSender notification.NotificationSender

	// 빌드 정보
	version     string
	buildDate   string
	buildNumber string
}

func NewNotifyAPIService(appConfig *config.AppConfig, notificationSender notification.NotificationSender, version, buildDate, buildNumber string) *NotifyAPIService {
	return &NotifyAPIService{
		appConfig: appConfig,

		running:   false,
		runningMu: sync.Mutex{},

		notificationSender: notificationSender,

		version:     version,
		buildDate:   buildDate,
		buildNumber: buildNumber,
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

	if s.running == true {
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
	h := handler.NewHandler(s.appConfig, s.notificationSender, s.version, s.buildDate, s.buildNumber)

	e := router.New()

	// System 엔드포인트 (인증 불필요)
	e.GET("/health", h.HealthCheckHandler)
	e.GET("/version", h.VersionHandler)

	// API v1 엔드포인트
	grp := e.Group("/api/v1")
	{
		grp.POST("/notice/message", h.SendNotifyMessageHandler)
	}

	// Swagger UI 설정
	e.GET("/swagger/*", echoSwagger.EchoWrapHandler(
		// Swagger 문서 JSON 파일 위치 지정
		echoSwagger.URL("/swagger/doc.json"),
		// 딥 링크 활성화 (특정 API로 바로 이동 가능한 URL 지원)
		echoSwagger.DeepLinking(true),
		// 문서 로드 시 태그(Tag) 목록만 펼침 상태로 표시 ("list", "full", "none")
		echoSwagger.DocExpansion("list"),
	))

	echo.NotFoundHandler = func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "페이지를 찾을 수 없습니다.")
	}

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
		if s.appConfig.NotifyAPI.WS.TLSServer == true {
			err = e.StartTLS(fmt.Sprintf(":%d", listenPort), s.appConfig.NotifyAPI.WS.TLSCertFile, s.appConfig.NotifyAPI.WS.TLSKeyFile)
		} else {
			err = e.Start(fmt.Sprintf(":%d", listenPort))
		}

		// Start(), StartTLS() 함수는 항상 nil이 아닌 error를 반환한다.
		if errors.Is(err, http.ErrServerClosed) == true {
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
