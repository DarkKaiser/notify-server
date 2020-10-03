package api

import (
	"context"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/notification"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sync"
	"time"
)

//
// application
//
type application struct {
	id                string
	title             string
	description       string
	apiKey            string
	defaultNotifierID string
}

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
		log.Panicf("NotificationSender 객체가 초기화되지 않았습니다.")
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

	// 허용된 Application 목록을 구한다.
	var applications []*application
	for _, app := range s.config.NotifyAPI.Applications {
		applications = append(applications, &application{
			id:                app.ID,
			title:             app.Title,
			description:       app.Description,
			apiKey:            app.APIKey,
			defaultNotifierID: app.DefaultNotifierID,
		})
	}

	// @@@@@
	e := echo.New()

	// logging and panic recovery middleware
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}\n",
	}))
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Initialize handler
	h := &Handler{
		app_list:           applications,
		notificationSender: s.notificationSender,
	}

	e.GET("/request/:message", h.messageHandler)

	go func() {
		e.HideBanner = true
		e.Logger.Fatal(e.Start(":8080"))
	}()

	for {
		select {
		case <-serviceStopCtx.Done():
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			e.Shutdown(ctx)
			return
		}
	}
}

//
// Handler
//
// @@@@@
type Handler struct {
	app_list           []*application
	notificationSender notification.NotificationSender
}

// @@@@@
func (h *Handler) messageHandler(c echo.Context) error {
	for _, a := range h.app_list {
		if a.id == "lottoPrediction" {
			h.notificationSender.Notify(a.defaultNotifierID, "title", c.Param("message"), false)
			// http://notify-api.darkkaiser.com/api/notify/

			// 허용가능한 ID목록+인증키를 읽어서 메시지를 보내면 체크한다.
			// 등록되지 않은 id이면 거부한다.
			// 등록된 id이면 webNotificationSender.Notify(id, notifierid, message, isError)
			// commandTitle:       fmt.Sprintf("%s > %s", t.Title, c.Title), => json 파일에 저장되어 있는값을 notifier에서 알아서 읽어온다.(notifier.webSenderTitle 등의 이름으로 따로 관리)
			// => 이렇게 되면 notifier가 추가될때마다 항상 같이 관리가 되어져야 됨, title을 직접 넘김
			// => webNotificationSender.Notify(notifierid, commandTitle, message, isError)
			// web -> task -> notifier 순으로 가는건????

			break
		}
	}
	return c.String(http.StatusOK, "Hello, World!  "+c.Param("message"))
}
