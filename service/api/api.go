package api

import (
	"context"
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/api/router"
	"github.com/darkkaiser/notify-server/service/notification"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sync"
	"time"
)

//
// NotifyAPIService
//
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

func (s *NotifyAPIService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Debug("NotifyAPI 서비스 시작중...")

	if s.notificationSender == nil {
		log.Panic("NotificationSender 객체가 초기화되지 않았습니다.")
	}

	if s.running == true {
		defer serviceStopWaiter.Done()

		log.Warn("NotifyAPI 서비스가 이미 시작됨!!!")

		return
	}

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	log.Debug("NotifyAPI 서비스 시작됨")
}

func (s *NotifyAPIService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	e := router.New(s.config, s.notificationSender)

	go func(listenPort int) {
		log.Debug("NotifyAPI 서비스 > http 서버 시작")
		if err := e.Start(fmt.Sprintf(":%d", listenPort)); err != nil {
			if err == http.ErrServerClosed {
				log.Debug("NotifyAPI 서비스 > http 서버 중지")
			} else {
				m := fmt.Sprintf("NotifyAPI RESTful 서비스를 구성하는 중에 치명적인 오류가 발생하였습니다.\r\n\r\n%s", err)

				log.Error(m)
				s.notificationSender.NotifyWithErrorToDefault(m)
			}
		}
	}(s.config.NotifyAPI.ListenPort)

	select {
	case <-serviceStopCtx.Done():
		log.Debug("NotifyAPI 서비스 중지중...")

		// 웹서버를 종료한다.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := e.Shutdown(ctx); err != nil {
			log.Error(err)
		}

		s.runningMu.Lock()
		s.running = false
		s.notificationSender = nil
		s.runningMu.Unlock()

		log.Debug("NotifyAPI 서비스 중지됨")
	}
}
