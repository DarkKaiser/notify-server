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
	NidTelegramDarkKaiserNotifyBot NotifierId = "darkkaiser_notify_bot"
)

type notifier struct {
	id NotifierId

	notifyStopWaiter *sync.WaitGroup

	notifyServiceStopCtx context.Context
}

func (n *notifier) Id() NotifierId {
	return n.id
}

type notifierHandler interface {
	Id() NotifierId
	Notify(m string) bool //@@@@@
}

// @@@@@
//type NotifyRequester interface {
//	Notify(id NotifierId, m string) (succeeded bool)
//}

type notifyService struct {
	config *global.AppConfig

	serviceStopCtx    context.Context
	serviceStopWaiter *sync.WaitGroup

	notifyStopWaiter *sync.WaitGroup

	running   bool
	runningMu sync.Mutex

	notifierHandlers []notifierHandler
}

func NewNotifyService(config *global.AppConfig, serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) service.Service {
	return &notifyService{
		config: config,

		serviceStopCtx:    serviceStopCtx,
		serviceStopWaiter: serviceStopWaiter,

		notifyStopWaiter: &sync.WaitGroup{},

		running:   false,
		runningMu: sync.Mutex{},
	}
}

func (s *notifyService) Run() {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Debug("Notify 서비스 시작중...")

	if s.running == true {
		defer s.serviceStopWaiter.Done()

		log.Warn("Notify 서비스가 이미 시작됨!!!")

		return
	}

	// Telegram Notifier를 실행한다.
	for _, telegram := range s.config.Notifiers.Telegrams {
		switch NotifierId(telegram.Id) {
		case NidTelegramDarkKaiserNotifyBot:
			s.notifyStopWaiter.Add(1)
			h := newTelegramNotifier(NidTelegramDarkKaiserNotifyBot, telegram.Token, telegram.ChatId, s.notifyStopWaiter, s.serviceStopCtx)
			s.notifierHandlers = append(s.notifierHandlers, h)

			log.Debugf("'%s' Telegram Notifier가 Notify 서비스에 등록되었습니다.", NidTelegramDarkKaiserNotifyBot)

		default:
			log.Panicf("알 수 없는 Notifier ID가 입력되었습니다.(Notifier:Telegram, NotifierId:%s)", telegram.Id)
		}
	}

	// Notify 서비스를 시작한다.
	go func() {
		defer s.serviceStopWaiter.Done()

		select {
		case <-s.serviceStopCtx.Done():
			log.Debug("Notify 서비스 중지중...")

			// 등록된 모든 Notifier의 알림활동이 중지될때까지 대기한다.
			s.notifyStopWaiter.Wait()

			s.runningMu.Lock()
			s.running = false
			s.notifierHandlers = nil
			s.runningMu.Unlock()

			log.Debug("Notify 서비스 중지됨")
		}
	}()

	s.running = true

	log.Debug("Notify 서비스 시작됨")
}

//@@@@@
func (s *notifyService) Notify(id NotifierId, m string) (succeeded bool) {
	succeeded = false

	for _, notifier := range s.notifierHandlers {
		if notifier.Id() == id {
			// 채널을 이용해서 메시지를 넘겨주는걸로 변경
			notifier.Notify(m)
			succeeded = true
			break
		}
	}

	return
}
