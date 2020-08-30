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
}

func (n *notifier) Id() NotifierId {
	return n.id
}

type notifierHandler interface {
	Id() NotifierId
	Notify(m string) bool //@@@@@
}

type notifyService struct {
	config *global.AppConfig

	running   bool
	runningMu sync.Mutex

	notifierHandlers []notifierHandler

	notifyStopWaiter *sync.WaitGroup
}

func NewNotifyService(config *global.AppConfig) service.Service {
	return &notifyService{
		config: config,

		running:   false,
		runningMu: sync.Mutex{},

		notifyStopWaiter: &sync.WaitGroup{},
	}
}

func (s *notifyService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Debug("Notify 서비스 시작중...")

	if s.running == true {
		defer serviceStopWaiter.Done()

		log.Warn("Notify 서비스가 이미 시작됨!!!")

		return
	}

	// Telegram Notifier의 알림활동을 시작한다.
	for _, telegram := range s.config.Notifiers.Telegrams {
		switch NotifierId(telegram.Id) {
		case NidTelegramDarkKaiserNotifyBot:
			s.notifyStopWaiter.Add(1)
			h := newTelegramNotifier(NidTelegramDarkKaiserNotifyBot, telegram.Token, telegram.ChatId, serviceStopCtx, s.notifyStopWaiter)
			s.notifierHandlers = append(s.notifierHandlers, h)

			log.Debugf("'%s' Telegram Notifier가 Notify 서비스에 등록되었습니다.", NidTelegramDarkKaiserNotifyBot)

		default:
			log.Panicf("알 수 없는 Notifier ID가 입력되었습니다.(Notifier:Telegram, NotifierId:%s)", telegram.Id)
		}
	}

	// Notify 서비스를 시작한다.
	go func() {
		defer serviceStopWaiter.Done()

		select {
		case <-serviceStopCtx.Done():
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

	// runningMu lock???
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
