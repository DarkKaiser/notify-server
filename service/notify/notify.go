package notify

import (
	"context"
	"github.com/darkkaiser/notify-server/global"
	"github.com/darkkaiser/notify-server/service"
	log "github.com/sirupsen/logrus"
	"sync"
)

type NotifierId string

const (
	NidTelegramNotifyBot NotifierId = "darkkaiser_notify_bot"
)

//@@@@@
type NotifierService interface {
	Id() NotifierId
	Notify(m string) bool
}

// @@@@@
type NotifyRequester interface {
	Notify(id NotifierId, m string) (succeeded bool)
}

type notifyServiceGroup struct {
	config *global.AppConfig

	serviceCtx        context.Context
	serviceStopWaiter *sync.WaitGroup

	running   bool
	runningMu sync.Mutex

	//@@@@@
	notifierServiceStopWaiter *sync.WaitGroup //@@@@@
	notifyServiceList         []NotifierService
}

func NewNotifyServiceGroup(config *global.AppConfig, serviceCtx context.Context, serviceStopWaiter *sync.WaitGroup) service.Service {
	return &notifyServiceGroup{
		config: config,

		serviceCtx:        serviceCtx,
		serviceStopWaiter: serviceStopWaiter,

		running:   false,
		runningMu: sync.Mutex{},

		//@@@@@
		notifierServiceStopWaiter: &sync.WaitGroup{},
	}
}

func (sg *notifyServiceGroup) Run() {
	sg.runningMu.Lock()
	defer sg.runningMu.Unlock()

	if sg.running == true {
		return
	}

	// Telegram Notify 서비스를 시작한다.
	for _, telegram := range sg.config.Notifiers.Telegrams {
		switch NotifierId(telegram.Id) {
		case NidTelegramNotifyBot:
			// @@@@@
			sg.notifierServiceStopWaiter.Add(1)
			t := newTelegramNotifyService(sg.serviceCtx, sg.notifierServiceStopWaiter, NidTelegramNotifyBot, telegram.Token, telegram.ChatId)
			t.Run(sg.config)
			//sg.notifyServiceList = append(sg.notifyServiceList, &t)

		default:
			log.Panicf("지원하지 않는 Telegram Id('%s')가 입력되었습니다.", telegram.Id)
		}
	}

	sg.serviceStopWaiter.Add(1)
	go func() {
		defer sg.serviceStopWaiter.Done()

		select {
		case <-sg.serviceCtx.Done():
			log.Debug("Notify 서비스를 중지합니다.")

			//@@@@@
			///////////////////////////////////
			sg.notifierServiceStopWaiter.Wait()
			// notifyservice.wait
			//sg.notifyServiceList.clear

			sg.runningMu.Lock()
			sg.running = false
			sg.runningMu.Unlock()
			///////////////////////////////////

			log.Debug("Notify 서비스가 중지되었습니다.")
		}
	}()

	sg.running = true
}

//@@@@@
func (sg *notifyServiceGroup) Notify(id NotifierId, m string) (succeeded bool) {
	succeeded = false

	for _, notifier := range sg.notifyServiceList {
		if notifier.Id() == id {
			// 채널을 이용해서 메시지를 넘겨주는걸로 변경
			notifier.Notify(m)
			succeeded = true
			break
		}
	}

	return
}
