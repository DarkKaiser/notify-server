package notification

import (
	"context"
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/task"
	log "github.com/sirupsen/logrus"
	"sync"
)

type NotifierID string

//
// notifier
//
type notifier struct {
	id NotifierID

	supportHTMLMessage bool

	notificationSendC chan *notificationSendData
}

type notifierHandler interface {
	ID() NotifierID

	Notify(message string, taskCtx task.TaskContext) (succeeded bool)

	Run(taskRunner task.TaskRunner, notificationStopCtx context.Context, notificationStopWaiter *sync.WaitGroup)

	SupportHTMLMessage() bool
}

func (n *notifier) ID() NotifierID {
	return n.id
}

func (n *notifier) Notify(message string, taskCtx task.TaskContext) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			log.Errorf("알림메시지 발송중에 panic이 발생하였습니다.(NotifierID:%s, Message:%s, panic:%s", n.ID(), message, r)
		}
	}()

	n.notificationSendC <- &notificationSendData{
		message: message,
		taskCtx: taskCtx,
	}

	return true
}

func (n *notifier) SupportHTMLMessage() bool {
	return n.supportHTMLMessage
}

//
// notificationSendData
//
type notificationSendData struct {
	message string
	taskCtx task.TaskContext
}

//
// NotificationSender
//
type NotificationSender interface {
	Notify(notifierID string, title string, message string, errorOccurred bool) bool
	NotifyToDefault(message string) bool
	NotifyWithErrorToDefault(message string) bool
}

//
// NotificationService
//
type NotificationService struct {
	config *g.AppConfig

	running   bool
	runningMu sync.Mutex

	defaultNotifierHandler notifierHandler
	notifierHandlers       []notifierHandler

	taskRunner task.TaskRunner

	notificationStopWaiter *sync.WaitGroup
}

func NewService(config *g.AppConfig, taskRunner task.TaskRunner) *NotificationService {
	return &NotificationService{
		config: config,

		running:   false,
		runningMu: sync.Mutex{},

		defaultNotifierHandler: nil,

		taskRunner: taskRunner,

		notificationStopWaiter: &sync.WaitGroup{},
	}
}

func (s *NotificationService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Debug("Notification 서비스 시작중...")

	if s.taskRunner == nil {
		log.Panic("TaskRunner 객체가 초기화되지 않았습니다.")
	}

	if s.running == true {
		defer serviceStopWaiter.Done()

		log.Warn("Notification 서비스가 이미 시작됨!!!")

		return
	}

	// Telegram Notifier의 작업을 시작한다.
	for _, telegram := range s.config.Notifiers.Telegrams {
		h := newTelegramNotifier(NotifierID(telegram.ID), telegram.BotToken, telegram.ChatID, s.config)
		s.notifierHandlers = append(s.notifierHandlers, h)

		s.notificationStopWaiter.Add(1)
		go h.Run(s.taskRunner, serviceStopCtx, s.notificationStopWaiter)

		log.Debugf("'%s' Telegram Notifier가 Notification 서비스에 등록되었습니다.", telegram.ID)
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

func (s *NotificationService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	select {
	case <-serviceStopCtx.Done():
		log.Debug("Notification 서비스 중지중...")

		// 등록된 모든 Notifier의 작업이 중지될때까지 대기한다.
		s.notificationStopWaiter.Wait()

		s.runningMu.Lock()
		s.running = false
		s.taskRunner = nil
		s.notifierHandlers = nil
		s.defaultNotifierHandler = nil
		s.runningMu.Unlock()

		log.Debug("Notification 서비스 중지됨")
	}
}

func (s *NotificationService) Notify(notifierID string, title string, message string, errorOccurred bool) bool {
	taskCtx := task.NewContext().With(task.TaskCtxKeyTitle, title)
	if errorOccurred == true {
		taskCtx.WithError()
	}

	return s.NotifyWithTaskContext(notifierID, message, taskCtx)
}

func (s *NotificationService) NotifyToDefault(message string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	return s.defaultNotifierHandler.Notify(message, nil)
}

func (s *NotificationService) NotifyWithErrorToDefault(message string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	return s.defaultNotifierHandler.Notify(message, task.NewContext().WithError())
}

func (s *NotificationService) NotifyWithTaskContext(notifierID string, message string, taskCtx task.TaskContext) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	id := NotifierID(notifierID)
	for _, h := range s.notifierHandlers {
		if h.ID() == id {
			return h.Notify(message, taskCtx)
		}
	}

	m := fmt.Sprintf("알 수 없는 Notifier('%s')입니다. 알림메시지 발송이 실패하였습니다.(Message:%s)", notifierID, message)

	log.Error(m)

	s.defaultNotifierHandler.Notify(m, task.NewContext().WithError())

	return false
}

func (s *NotificationService) SupportHTMLMessage(notifierID string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	id := NotifierID(notifierID)
	for _, h := range s.notifierHandlers {
		if h.ID() == id {
			return h.SupportHTMLMessage()
		}
	}

	return false
}
