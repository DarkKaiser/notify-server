package notification

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/task"
	log "github.com/sirupsen/logrus"
)

type NotifierID string

// notifier
type notifier struct {
	id NotifierID

	supportHTMLMessage bool

	notificationSendC chan *notificationSendData
}

type NotifierHandler interface {
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

			log.WithFields(log.Fields{
				"notifier_id":    n.ID(),
				"message_length": len(message),
				"panic":          r,
			}).Error("알림메시지 발송중에 panic 발생")
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

// notificationSendData
type notificationSendData struct {
	message string
	taskCtx task.TaskContext
}

// NotificationSender
type NotificationSender interface {
	Notify(notifierID string, title string, message string, errorOccurred bool) bool
	NotifyToDefault(message string) bool
	NotifyWithErrorToDefault(message string) bool
}

// NotificationService
type NotificationService struct {
	appConfig *g.AppConfig

	running   bool
	runningMu sync.Mutex

	defaultNotifierHandler NotifierHandler
	notifierHandlers       []NotifierHandler

	taskRunner task.TaskRunner

	notificationStopWaiter *sync.WaitGroup

	newNotifier func(id NotifierID, botToken string, chatID int64, appConfig *g.AppConfig) NotifierHandler
}

func NewService(appConfig *g.AppConfig, taskRunner task.TaskRunner) *NotificationService {
	return &NotificationService{
		appConfig: appConfig,

		running:   false,
		runningMu: sync.Mutex{},

		defaultNotifierHandler: nil,

		taskRunner: taskRunner,

		notificationStopWaiter: &sync.WaitGroup{},

		newNotifier: newTelegramNotifier,
	}
}

func (s *NotificationService) SetNewNotifier(newNotifierFn func(id NotifierID, botToken string, chatID int64, appConfig *g.AppConfig) NotifierHandler) {
	s.newNotifier = newNotifierFn
}

func (s *NotificationService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Info("Notification 서비스 시작중...")

	if s.taskRunner == nil {
		defer serviceStopWaiter.Done()

		return fmt.Errorf("TaskRunner 객체가 초기화되지 않았습니다")
	}

	if s.running == true {
		defer serviceStopWaiter.Done()

		log.WithFields(log.Fields{
			"component": "notification.service",
		}).Warn("Notification 서비스가 이미 시작됨!!!")

		return nil
	}

	// Telegram Notifier의 작업을 시작한다.
	for _, telegram := range s.appConfig.Notifiers.Telegrams {
		h := s.newNotifier(NotifierID(telegram.ID), telegram.BotToken, telegram.ChatID, s.appConfig)
		s.notifierHandlers = append(s.notifierHandlers, h)

		s.notificationStopWaiter.Add(1)
		go h.Run(s.taskRunner, serviceStopCtx, s.notificationStopWaiter)

		log.WithFields(log.Fields{
			"component":   "notification.service",
			"notifier_id": telegram.ID,
		}).Debug("Telegram Notifier가 Notification 서비스에 등록됨")
	}

	// 기본 Notifier를 구한다.
	for _, h := range s.notifierHandlers {
		if h.ID() == NotifierID(s.appConfig.Notifiers.DefaultNotifierID) {
			s.defaultNotifierHandler = h
			break
		}
	}
	if s.defaultNotifierHandler == nil {
		defer serviceStopWaiter.Done()

		return fmt.Errorf("기본 NotifierID('%s')를 찾을 수 없습니다", s.appConfig.Notifiers.DefaultNotifierID)
	}

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	log.Info("Notification 서비스 시작됨")

	return nil
}

func (s *NotificationService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	select {
	case <-serviceStopCtx.Done():
		log.Info("Notification 서비스 중지중...")

		// 등록된 모든 Notifier의 작업이 중지될때까지 대기한다.
		s.notificationStopWaiter.Wait()

		s.runningMu.Lock()
		s.running = false
		s.taskRunner = nil
		s.notifierHandlers = nil
		s.defaultNotifierHandler = nil
		s.runningMu.Unlock()

		log.Info("Notification 서비스 중지됨")
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

	log.WithFields(log.Fields{
		"component":   "notification.service",
		"notifier_id": notifierID,
	}).Error(m)

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
