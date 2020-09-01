package notify

import (
	"context"
	"github.com/darkkaiser/notify-server/global"
	"github.com/darkkaiser/notify-server/service"
	"github.com/darkkaiser/notify-server/service/task"
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

	Run(r task.TaskRunRequester, notifyStopCtx context.Context, notifyStopWaiter *sync.WaitGroup)

	//@@@@@
	Notify(m string) bool
}

type NotifyRequester interface {
	Notify(id NotifierId, message string, ctx context.Context) (succeeded bool)
}

type notifyService struct {
	config *global.AppConfig

	running   bool
	runningMu sync.Mutex

	notifierHandlers []notifierHandler

	notifyStopWaiter *sync.WaitGroup

	taskRunRequester task.TaskRunRequester
}

func NewNotifyService(config *global.AppConfig) service.Service {
	return &notifyService{
		config: config,

		running:   false,
		runningMu: sync.Mutex{},

		notifyStopWaiter: &sync.WaitGroup{},

		taskRunRequester: nil,
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

	// TaskRunRequester 객체를 구한다.
	if o := valueCtx.Value("TaskRunRequester"); o != nil {
		r, ok := o.(task.TaskRunRequester)
		if ok == false {
			log.Panicf("TaskRunRequester 객체를 구할 수 없습니다.")
		}
		s.taskRunRequester = r
	} else {
		log.Panicf("TaskRunRequester 객체를 구할 수 없습니다.")
	}

	// Telegram Notifier의 작업을 시작한다.
	for _, telegram := range s.config.Notifiers.Telegrams {
		switch NotifierId(telegram.Id) {
		case NidTelegramDarkKaiserNotifyBot:
			h := newTelegramNotifier(NidTelegramDarkKaiserNotifyBot, telegram.Token, telegram.ChatId)
			s.notifierHandlers = append(s.notifierHandlers, h)

			s.notifyStopWaiter.Add(1)
			go h.Run(s.taskRunRequester, serviceStopCtx, s.notifyStopWaiter)

			log.Debugf("'%s' Telegram Notifier가 Notify 서비스에 등록되었습니다.", NidTelegramDarkKaiserNotifyBot)

		default:
			log.Panicf("알 수 없는 Notifier ID가 입력되었습니다.(Notifier:Telegram, NotifierId:%s)", telegram.Id)
		}
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
		s.runningMu.Unlock()

		log.Debug("Notify 서비스 중지됨")
	}
}

//@@@@@
func (s *notifyService) Notify(id NotifierId, message string, ctx context.Context) (succeeded bool) {
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
