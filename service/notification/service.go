package notification

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/service/task"
	log "github.com/sirupsen/logrus"
)

// NotifierID 알림 채널(Notifier)를 식별하는 고유 ID입니다.
type NotifierID string

// notifier NotifierHandler의 기본 구현체입니다.
// 공통적인 알림 채널 처리 로직을 제공하며, 구체적인 구현체에 임베딩되어 사용됩니다.
type notifier struct {
	id NotifierID

	supportsHTMLMessage bool

	notificationSendC chan *notificationSendData
}

// NotifierHandler 알림 채널(예: Telegram, Slack)의 공통 인터페이스입니다.
// 실제 알림 발송 로직은 이 인터페이스를 구현하여 정의합니다.
type NotifierHandler interface {
	ID() NotifierID

	// Notify 알림 발송 요청을 처리합니다.
	// 실제 발송은 비동기 큐를 통해 처리될 수 있습니다.
	//
	// 반환값:
	//   - succeeded: 요청이 정상적으로 접수되었는지 여부
	Notify(message string, taskCtx task.TaskContext) (succeeded bool)

	// Run Notifier의 메인 루프를 실행합니다.
	// 메시지 큐를 소비하여 실제 발송 작업을 수행합니다.
	Run(taskRunner task.TaskRunner, notificationStopCtx context.Context, notificationStopWaiter *sync.WaitGroup)

	SupportsHTMLMessage() bool
}

func (n *notifier) ID() NotifierID {
	return n.id
}

// Notify 메시지를 큐에 등록하여 비동기 발송을 요청합니다.
// 전송 중 패닉이 발생해도 recover하여 서비스 안정성을 유지합니다.
func (n *notifier) Notify(message string, taskCtx task.TaskContext) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			applog.WithComponentAndFields("notification.service", log.Fields{
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

func (n *notifier) SupportsHTMLMessage() bool {
	return n.supportsHTMLMessage
}

// notificationSendData 내부 채널을 통해 전달되는 알림 데이터입니다.
type notificationSendData struct {
	message string
	taskCtx task.TaskContext
}

// NotificationSender 알림 발송 기능을 제공하는 인터페이스입니다.
// 외부 컴포넌트(API, 스케줄러 등)는 이 인터페이스를 통해 알림 서비스를 사용합니다.
type NotificationSender interface {
	Notify(notifierID string, title string, message string, errorOccurred bool) bool
	NotifyToDefault(message string) bool
	NotifyWithErrorToDefault(message string) bool
}

type NotificationService struct {
	appConfig *config.AppConfig

	running   bool
	runningMu sync.Mutex

	defaultNotifierHandler NotifierHandler
	notifierHandlers       []NotifierHandler

	taskRunner task.TaskRunner

	// notificationStopWaiter 모든 하위 Notifier의 종료를 대기하는 WaitGroup
	notificationStopWaiter *sync.WaitGroup

	// newNotifier Notifier 생성 팩토리
	newNotifier func(id NotifierID, botToken string, chatID int64, appConfig *config.AppConfig) NotifierHandler
}

func NewService(appConfig *config.AppConfig, taskRunner task.TaskRunner) *NotificationService {
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

func (s *NotificationService) SetNewNotifier(newNotifierFn func(id NotifierID, botToken string, chatID int64, appConfig *config.AppConfig) NotifierHandler) {
	s.newNotifier = newNotifierFn
}

// Run 알림 서비스를 시작하여 등록된 Notifier들을 활성화합니다.
func (s *NotificationService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent("notification.service").Info("Notification 서비스 시작중...")

	if s.taskRunner == nil {
		defer serviceStopWaiter.Done()
		return apperrors.New(apperrors.ErrInternal, "TaskRunner 객체가 초기화되지 않았습니다")
	}

	if s.running {
		defer serviceStopWaiter.Done()
		applog.WithComponent("notification.service").Warn("Notification 서비스가 이미 시작됨!!!")
		return nil
	}

	// 1. Telegram Notifier들을 초기화 및 실행
	for _, telegram := range s.appConfig.Notifiers.Telegrams {
		h := s.newNotifier(NotifierID(telegram.ID), telegram.BotToken, telegram.ChatID, s.appConfig)
		s.notifierHandlers = append(s.notifierHandlers, h)

		s.notificationStopWaiter.Add(1)

		go h.Run(s.taskRunner, serviceStopCtx, s.notificationStopWaiter)

		applog.WithComponentAndFields("notification.service", log.Fields{
			"notifier_id": telegram.ID,
		}).Debug("Telegram Notifier가 Notification 서비스에 등록됨")
	}

	// 2. 기본 Notifier 설정 (존재 여부 확인)
	for _, h := range s.notifierHandlers {
		if h.ID() == NotifierID(s.appConfig.Notifiers.DefaultNotifierID) {
			s.defaultNotifierHandler = h
			break
		}
	}
	if s.defaultNotifierHandler == nil {
		defer serviceStopWaiter.Done()
		return apperrors.New(apperrors.ErrNotFound, fmt.Sprintf("기본 NotifierID('%s')를 찾을 수 없습니다", s.appConfig.Notifiers.DefaultNotifierID))
	}

	// 3. 서비스 종료 감시 루틴 실행
	go s.waitForShutdown(serviceStopCtx, serviceStopWaiter)

	s.running = true

	applog.WithComponent("notification.service").Info("Notification 서비스 시작됨")

	return nil
}

// waitForShutdown 서비스의 종료 신호를 감지하고 리소스를 안전하게 정리합니다.
func (s *NotificationService) waitForShutdown(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	<-serviceStopCtx.Done()

	applog.WithComponent("notification.service").Info("Notification 서비스 중지중...")

	// 등록된 모든 Notifier의 고루틴 작업이 완료(종료)될 때까지 대기합니다.
	// 각 Notifier의 Run 메서드에서 defer s.notificationStopWaiter.Done()이 호출되어야 합니다.
	s.notificationStopWaiter.Wait()

	s.runningMu.Lock()
	s.running = false
	s.taskRunner = nil
	s.notifierHandlers = nil
	s.defaultNotifierHandler = nil
	s.runningMu.Unlock()

	applog.WithComponent("notification.service").Info("Notification 서비스 중지됨")
}

// Notify 지정된 Notifier를 통해 알림 메시지를 발송합니다.
// API 핸들러 등 외부에서 특정 채널을 통하여 알림을 보내고 싶을 때 사용합니다.
//
// 파라미터:
//   - notifierID: 알림 채널 ID
//   - title: 알림 메시지의 제목 (TaskContext에 저장됨)
//   - message: 전송할 메시지 내용
//   - errorOccurred: 에러 발생 여부
//
// 반환값:
//   - bool: 발송 요청이 성공적으로 큐에 등록되었는지 여부 (실제 전송 성공 여부는 아님)
func (s *NotificationService) Notify(notifierID string, title string, message string, errorOccurred bool) bool {
	taskCtx := task.NewContext().With(task.TaskCtxKeyTitle, title)
	if errorOccurred {
		taskCtx.WithError()
	}

	return s.NotifyWithTaskContext(notifierID, message, taskCtx)
}

// NotifyToDefault 시스템 기본 알림 채널로 알림 메시지를 발송합니다.
//
// 파라미터:
//   - message: 전송할 메시지 내용
//
// 반환값:
//   - bool: 발송 요청이 성공적으로 큐에 등록되었는지 여부 (실제 전송 성공 여부는 아님)
func (s *NotificationService) NotifyToDefault(message string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	return s.defaultNotifierHandler.Notify(message, nil)
}

// NotifyWithErrorToDefault 시스템 기본 알림 채널로 "에러" 알림 메시지를 발송합니다.
// 시스템 오류, 작업 실패 등 관리자의 주의가 필요한 상황에서 사용합니다.
// 내부적으로 TaskContext에 Error 속성을 추가하여 Notifier가 이를 인지할 수 있게 합니다.
//
// 파라미터:
//   - message: 전송할 메시지 내용
//
// 반환값:
//   - bool: 발송 요청이 성공적으로 큐에 등록되었는지 여부 (실제 전송 성공 여부는 아님)
func (s *NotificationService) NotifyWithErrorToDefault(message string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	return s.defaultNotifierHandler.Notify(message, task.NewContext().WithError())
}

// NotifyWithTaskContext 가장 저수준의 알림 발송 메서드입니다.
// 구체적인 TaskContext를 직접 주입하여 세밀한 제어가 필요할 때 사용됩니다.
//
// 파라미터:
//   - notifierID: 알림 채널 ID
//   - message: 전송할 메시지 내용
//   - taskCtx: Task 컨텍스트 (nil 가능)
//
// 반환값:
//   - bool: 발송 요청이 성공적으로 큐에 등록되었는지 여부, ID를 못 찾은 경우 false (실제 전송 성공 여부는 아님)
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

	applog.WithComponentAndFields("notification.service", log.Fields{
		"notifier_id": notifierID,
	}).Error(m)

	s.defaultNotifierHandler.Notify(m, task.NewContext().WithError())

	return false
}

// SupportsHTMLMessage 해당 Notifier가 HTML 포맷을 지원하는지 확인합니다.
func (s *NotificationService) SupportsHTMLMessage(notifierID string) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	id := NotifierID(notifierID)
	for _, h := range s.notifierHandlers {
		if h.ID() == id {
			return h.SupportsHTMLMessage()
		}
	}

	return false
}
