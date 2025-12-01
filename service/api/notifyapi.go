package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	_ "github.com/darkkaiser/notify-server/docs"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/api/handler"
	"github.com/darkkaiser/notify-server/service/api/router"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// NotifyAPIService
type NotifyAPIService struct {
	config *g.AppConfig

	running   bool
	runningMu sync.Mutex

	notificationSender notification.NotificationSender
}

func NewNotifyAPIService(config *g.AppConfig, notificationSender notification.NotificationSender) *NotifyAPIService {
	return &NotifyAPIService{
		config: config,

		running:   false,
		runningMu: sync.Mutex{},

		notificationSender: notificationSender,
	}
}

func (s *NotifyAPIService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Debug("NotifyAPI 서비스 시작중...")

	if s.notificationSender == nil {
		defer serviceStopWaiter.Done()

		return errors.New("NotificationSender 객체가 초기화되지 않았습니다")
	}

	if s.running == true {
		defer serviceStopWaiter.Done()

		log.Warn("NotifyAPI 서비스가 이미 시작됨!!!")

		return nil
	}

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	log.Debug("NotifyAPI 서비스 시작됨")

	return nil
}

func (s *NotifyAPIService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	h := handler.NewHandler(s.config, s.notificationSender)

	e := router.New()
	grp := e.Group("/api/v1")
	{
		grp.POST("/notice/message", h.NotifyMessageSendHandler)
	}

	e.GET("/swagger/*", echoSwagger.WrapHandler)

	echo.NotFoundHandler = func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "페이지를 찾을 수 없습니다.")
	}

	// httpServerDone은 HTTP 서버 고루틴이 종료될 때까지 대기하기 위한 채널이다.
	// 서비스 종료 시 s.notificationSender를 nil로 설정하기 전에 HTTP 서버가 완전히 종료되었음을 보장하여
	// 경쟁 상태(Race Condition)를 방지한다.
	httpServerDone := make(chan struct{})

	go func(listenPort int) {
		defer close(httpServerDone)

		log.Debugf("NotifyAPI 서비스 > http 서버(:%d) 시작", listenPort)

		var err error
		if s.config.NotifyAPI.WS.TLSServer == true {
			err = e.StartTLS(fmt.Sprintf(":%d", listenPort), s.config.NotifyAPI.WS.TLSCertFile, s.config.NotifyAPI.WS.TLSKeyFile)
		} else {
			err = e.Start(fmt.Sprintf(":%d", listenPort))
		}

		// Start(), StartTLS() 함수는 항상 nil이 아닌 error를 반환한다.
		if errors.Is(err, http.ErrServerClosed) == true {
			log.Debug("NotifyAPI 서비스 > http 서버 중지됨")
		} else {
			m := "NotifyAPI 서비스 > http 서버를 구성하는 중에 치명적인 오류가 발생하였습니다."

			log.WithFields(log.Fields{
				"component": "api",
				"port":      s.config.NotifyAPI.WS.ListenPort,
				"error":     err,
			}).Error(m)

			s.notificationSender.NotifyWithErrorToDefault(fmt.Sprintf("%s\r\n\r\n%s", m, err))
		}
	}(s.config.NotifyAPI.WS.ListenPort)

	select {
	case <-serviceStopCtx.Done():
		log.Debug("NotifyAPI 서비스 중지중...")

		// 웹서버를 종료한다.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := e.Shutdown(ctx); err != nil {
			log.Error(err)
		}

		<-httpServerDone

		s.runningMu.Lock()
		s.running = false
		s.notificationSender = nil
		s.runningMu.Unlock()

		log.Debug("NotifyAPI 서비스 중지됨")
	}
}
