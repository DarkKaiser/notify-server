package service

import (
	"context"
	"github.com/darkkaiser/notify-server/global"
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

	Run(runner TaskRunner, notifyStopCtx context.Context, notifyStopWaiter *sync.WaitGroup)

	//@@@@@
	Notify(ctx context.Context, message string) bool
}

type NotifySender interface {
	Notify(id NotifierId, ctx context.Context, message string) (succeeded bool)
	NotifyWithDefaultNotifier(message string) (succeeded bool)
}

type notifyService struct {
	config *global.AppConfig

	running   bool
	runningMu sync.Mutex

	notifierHandlers       []notifierHandler
	defaultNotifierHandler notifierHandler

	notifyStopWaiter *sync.WaitGroup

	taskRunner TaskRunner
}

func NewNotifyService(config *global.AppConfig) Service {
	return &notifyService{
		config: config,

		running:   false,
		runningMu: sync.Mutex{},

		defaultNotifierHandler: nil,

		notifyStopWaiter: &sync.WaitGroup{},

		taskRunner: nil,
	}
}

func (s *notifyService) Run(valueCtx context.Context, serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Debug("Notify 서비스 시작중...")

	if s.running == true {
		defer serviceStopWaiter.Done()

		log.Warn("Notify 서비스가 이미 시작됨!!!")

		return
	}

	// TaskRunner 객체를 구한다.
	if o := valueCtx.Value("taskrunner"); o != nil {
		r, ok := o.(TaskRunner)
		if ok == false {
			log.Panicf("TaskRunner 객체를 구할 수 없습니다.")
		}
		s.taskRunner = r
	} else {
		log.Panicf("TaskRunner 객체를 구할 수 없습니다.")
	}

	// Telegram Notifier의 작업을 시작한다.
	for _, telegram := range s.config.Notifiers.Telegrams {
		switch NotifierId(telegram.Id) {
		case NidTelegramDarkKaiserNotifyBot:
			h := newTelegramNotifier(NidTelegramDarkKaiserNotifyBot, telegram.Token, telegram.ChatId)
			s.notifierHandlers = append(s.notifierHandlers, h)

			s.notifyStopWaiter.Add(1)
			go h.Run(s.taskRunner, serviceStopCtx, s.notifyStopWaiter)

			log.Debugf("'%s' Telegram Notifier가 Notify 서비스에 등록되었습니다.", NidTelegramDarkKaiserNotifyBot)

		default:
			log.Panicf("알 수 없는 Notifier ID가 입력되었습니다.(Notifier:Telegram, NotifierId:%s)", telegram.Id)
		}
	}

	// 기본 Notifier를 구한다.
	for _, h := range s.notifierHandlers {
		if h.Id() == NotifierId(s.config.Notifiers.Default) {
			s.defaultNotifierHandler = h
			break
		}
	}
	if s.defaultNotifierHandler == nil {
		log.Panicf("알 수 없는 기본 Notifier ID가 입력되었습니다.(NotifierId:%s)", s.config.Notifiers.Default)
	}

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	log.Debug("Notify 서비스 시작됨")
}

func (s *notifyService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	select {
	case <-serviceStopCtx.Done():
		log.Debug("Notify 서비스 중지중...")

		// 등록된 모든 Notifier의 작업이 중지될때까지 대기한다.
		s.notifyStopWaiter.Wait()

		s.runningMu.Lock()
		s.running = false
		s.notifierHandlers = nil
		s.defaultNotifierHandler = nil
		s.taskRunner = nil //@@@@@
		s.runningMu.Unlock()

		log.Debug("Notify 서비스 중지됨")
	}
}

func (s *notifyService) Notify(id NotifierId, ctx context.Context, message string) (succeeded bool) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	for _, h := range s.notifierHandlers {
		if h.Id() == id {
			//@@@@@ 함수구현, 내부에서 바로 notifier를 호출할지 아니면 채널을 통해서 보내고 나서 호출할지는 더 고민
			// 채널을 이용해서 메시지를 넘겨주는걸로 변경
			//notifier.NotifyC() <- struct {
			//	ctx,
			//	message
			//}
			h.Notify(ctx, message)

			return true
		}
	}

	// @@@@@ log.error+notify(???)

	return false
}

func (s *notifyService) NotifyWithDefaultNotifier(message string) (succeeded bool) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	// @@@@@
	s.defaultNotifierHandler.Notify(nil, message)

	return true
}
