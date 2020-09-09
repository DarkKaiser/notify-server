package notify

import (
	"context"
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service"
	log "github.com/sirupsen/logrus"
	"sync"
)

type NotifierID string

// @@@@@
type NotifierContextKey string

// @@@@@
const (
	NotifierContextKeyTaskID     NotifierContextKey = "taskId"
	NotifierContextKeyCommandID  NotifierContextKey = "taskCommandId"
	NotifierContextKeyInstanceID NotifierContextKey = "taskInstanceId"
)

type notifier struct {
	id NotifierID

	notificationSendC chan *notificationSendData
}

func (n *notifier) ID() NotifierID {
	return n.id
}

func (n *notifier) Notify(ctx context.Context, message string) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("알림메시지 발송중에 panic이 발생하였습니다.(NotifierID:%s, Message:%s, panic:%s", n.ID(), message, r)
		}
	}()

	n.notificationSendC <- &notificationSendData{
		ctx:     ctx,
		message: message,
	}

	return true
}

type notifierHandler interface {
	ID() NotifierID
	Notify(ctx context.Context, message string) (succeeded bool)
	Run(notifierActionProcessor NotifierActionProcessor, notificationStopCtx context.Context, notificationStopWaiter *sync.WaitGroup)
}

type notificationSendData struct {
	ctx     context.Context
	message string
}

type NotificationSender interface {
	Notify(id NotifierID, ctx context.Context, message string) bool
	NotifyWithDefault(message string) bool
}

type NotifierActionProcessor interface {
	TaskRun(id string, commandID string, notifierID NotifierID, notifyResultOfTaskRunRequest bool) (succeeded bool)
	TaskRunWithContext(id string, commandID string, notifierID NotifierID, notifierCtx context.Context, notifyResultOfTaskRunRequest bool) (succeeded bool)
	TaskCancel(instanceID uint64) (succeeded bool)
}

type notificationService struct {
	config *g.AppConfig

	running   bool
	runningMu sync.Mutex

	defaultNotifierHandler  notifierHandler
	notifierHandlers        []notifierHandler
	notifierActionProcessor NotifierActionProcessor

	notificationStopWaiter *sync.WaitGroup
}

func NewService(config *g.AppConfig) service.Service {
	return &notificationService{
		config: config,

		running:   false,
		runningMu: sync.Mutex{},

		defaultNotifierHandler:  nil,
		notifierActionProcessor: nil,

		notificationStopWaiter: &sync.WaitGroup{},
	}
}

func (s *notificationService) Run(valueCtx context.Context, serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Debug("Notification 서비스 시작중...")

	if s.running == true {
		defer serviceStopWaiter.Done()

		log.Warn("Notification 서비스가 이미 시작됨!!!")

		return
	}

	// NotifierActionProcessor 객체를 구한다.
	if o := valueCtx.Value("notify.notifier_action_processor"); o != nil {
		r, ok := o.(NotifierActionProcessor)
		if ok == false {
			log.Panicf("NotifierActionProcessor 객체를 구할 수 없습니다.")
		}
		s.notifierActionProcessor = r
	} else {
		log.Panicf("NotifierActionProcessor 객체를 구할 수 없습니다.")
	}

	// Telegram Notifier의 작업을 시작한다.
	for _, telegram := range s.config.Notifiers.Telegrams {
		notifierID := NotifierID(telegram.ID)
		h := newTelegramNotifier(notifierID, telegram.Token, telegram.ChatID, s.config)
		s.notifierHandlers = append(s.notifierHandlers, h)

		s.notificationStopWaiter.Add(1)
		go h.Run(s.notifierActionProcessor, serviceStopCtx, s.notificationStopWaiter)

		log.Debugf("'%s' Telegram Notifier가 Notification 서비스에 등록되었습니다.", notifierID)
	}

	// 기본 Notifier를 구한다.
	for _, h := range s.notifierHandlers {
		if h.ID() == NotifierID(s.config.Notifiers.DefaultNotifierID) {
			s.defaultNotifierHandler = h
			break
		}
	}
	if s.defaultNotifierHandler == nil {
		log.Panicf("기본 NotifierID('%s')를 찾을 수 없습니다.", s.config.Notifiers.DefaultNotifierID)
	}

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	log.Debug("Notification 서비스 시작됨")
}

func (s *notificationService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	select {
	case <-serviceStopCtx.Done():
		log.Debug("Notification 서비스 중지중...")

		// 등록된 모든 Notifier의 작업이 중지될때까지 대기한다.
		s.notificationStopWaiter.Wait()

		// @@@@@
		///////////////////////////////////
		s.runningMu.Lock()
		s.running = false
		s.notifierHandlers = nil
		s.defaultNotifierHandler = nil
		s.notifierActionProcessor = nil
		s.runningMu.Unlock()
		///////////////////////////////////

		log.Debug("Notification 서비스 중지됨")
	}
}

func (s *notificationService) Notify(id NotifierID, ctx context.Context, message string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	for _, h := range s.notifierHandlers {
		if h.ID() == id {
			return h.Notify(ctx, message)
		}
	}

	m := fmt.Sprintf("존재하지 않는 NotifierID('%s')입니다. 알림메시지 발송이 실패하였습니다.(Message:%s)", id, message)

	log.Error(m)
	s.defaultNotifierHandler.Notify(nil, m)

	return false
}

func (s *notificationService) NotifyWithDefault(message string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	return s.defaultNotifierHandler.Notify(nil, message)
}
