package notification

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/config"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutils"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
)

const (
	// 텔레그램 봇 명령어 상수
	// 봇과 사용자 간의 상호작용에 사용됩니다.
	telegramBotCommandHelp   = "help"   // 도움말
	telegramBotCommandCancel = "cancel" // 작업 취소

	telegramBotCommandSeparator        = "_" // 명령어와 인자(예: TaskInstanceID)를 구분하는 구분자
	telegramBotCommandInitialCharacter = "/" // 텔레그램 명령어가 시작됨을 알리는 문자

	// 텔레그램 메시지 최대 길이 제한 (API Spec)
	// 한 번에 전송 가능한 최대 4096자 중 메타데이터 여분을 고려하여 3900자로 제한합니다.
	telegramMessageMaxLength = 3900
)

// telegramBotCommand 봇에서 실행 가능한 명령어 메타데이터
type telegramBotCommand struct {
	command            string
	commandTitle       string
	commandDescription string

	taskID        task.TaskID        // 이 명령어와 연결된 작업(Task) ID
	taskCommandID task.TaskCommandID // 이 명령어와 연결된 작업 커맨드 ID
}

// TelegramBotAPI 텔레그램 봇 API 인터페이스
type TelegramBotAPI interface {
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	StopReceivingUpdates()
	GetSelf() tgbotapi.User
}

// telegramBotAPIClient tgbotapi.BotAPI 구현체를 래핑한 구조체
type telegramBotAPIClient struct {
	*tgbotapi.BotAPI
}

// GetSelf 텔레그램 봇의 정보를 반환합니다.
func (w *telegramBotAPIClient) GetSelf() tgbotapi.User {
	return w.Self
}

// telegramNotifier 텔레그램 알림 발송 및 봇 상호작용을 처리하는 Notifier
type telegramNotifier struct {
	notifier

	chatID int64

	botAPI TelegramBotAPI

	botCommands []telegramBotCommand
}

type telegramNotifierCreatorFunc func(id NotifierID, botToken string, chatID int64, appConfig *config.AppConfig) (NotifierHandler, error)

// NewTelegramConfigProcessor 텔레그램 Notifier 설정을 처리하는 NotifierConfigProcessor를 생성하여 반환합니다.
// 이 처리기는 애플리케이션 설정에 따라 텔레그램 Notifier 인스턴스들을 초기화합니다.
func NewTelegramConfigProcessor(creatorFn telegramNotifierCreatorFunc) NotifierConfigProcessor {
	return func(appConfig *config.AppConfig) ([]NotifierHandler, error) {
		var handlers []NotifierHandler

		for _, telegram := range appConfig.Notifiers.Telegrams {
			h, err := creatorFn(NotifierID(telegram.ID), telegram.BotToken, telegram.ChatID, appConfig)
			if err != nil {
				return nil, err
			}
			handlers = append(handlers, h)
		}

		return handlers, nil
	}
}

// newTelegramNotifier 실제 텔레그램 봇 API를 이용하여 Notifier 인스턴스를 생성합니다.
func newTelegramNotifier(id NotifierID, botToken string, chatID int64, appConfig *config.AppConfig) (NotifierHandler, error) {
	applog.WithComponentAndFields("notification.telegram", log.Fields{
		"bot_token": applog.MaskSensitiveData(botToken),
	}).Debug("Telegram Bot 초기화 시도")

	botAPI, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, fmt.Errorf("telegram bot 초기화 실패: %w", err)
	}
	botAPI.Debug = true

	return newTelegramNotifierWithBot(id, &telegramBotAPIClient{BotAPI: botAPI}, chatID, appConfig), nil
}

// newTelegramNotifierWithBot TelegramBotAPI 구현체를 이용하여 Notifier 인스턴스를 생성합니다.
func newTelegramNotifierWithBot(id NotifierID, botAPI TelegramBotAPI, chatID int64, appConfig *config.AppConfig) NotifierHandler {
	notifier := &telegramNotifier{
		notifier: NewNotifier(id, true, 100),
		chatID:   chatID,
		botAPI:   botAPI,
	}

	// 봇 명령어 목록을 초기화합니다.
	for _, t := range appConfig.Tasks {
		for _, c := range t.Commands {
			// 해당 커맨드가 Notifier 사용이 불가능하게 설정된 경우 건너뜁니다.
			if !c.Notifier.Usable {
				continue
			}

			// 명령어 문자열 생성: taskID와 commandID를 SnakeCase로 변환하여 조합 (예: myTask, run -> my_task_run)
			notifier.botCommands = append(notifier.botCommands,
				telegramBotCommand{
					command:            fmt.Sprintf("%s_%s", strutils.ToSnakeCase(t.ID), strutils.ToSnakeCase(c.ID)),
					commandTitle:       fmt.Sprintf("%s > %s", t.Title, c.Title), // 제목: 작업명 > 커맨드명
					commandDescription: c.Description,                            // 설명: 커맨드 설명

					taskID:        task.TaskID(t.ID),
					taskCommandID: task.TaskCommandID(c.ID),
				},
			)
		}
	}
	notifier.botCommands = append(notifier.botCommands,
		telegramBotCommand{
			command:            telegramBotCommandHelp,
			commandTitle:       "도움말",
			commandDescription: "도움말을 표시합니다.",
		},
	)

	return notifier
}

// Run 메시지 폴링 및 알림 처리 메인 루프
func (n *telegramNotifier) Run(taskRunner task.TaskRunner, notificationStopCtx context.Context, notificationStopWaiter *sync.WaitGroup) {
	defer notificationStopWaiter.Done()

	// 텔레그램 메시지 수신 설정
	config := tgbotapi.NewUpdate(0)
	config.Timeout = 60 // Long Polling 타임아웃 60초 설정

	// 메시지 수신 채널 획득
	updateC := n.botAPI.GetUpdatesChan(config)

	applog.WithComponentAndFields("notification.telegram", log.Fields{
		"notifier_id":  n.ID(),
		"bot_username": n.botAPI.GetSelf().UserName,
	}).Debug("Telegram Notifier의 작업이 시작됨")

	for {
		select {
		// 1. 텔레그램 봇 서버로부터 새로운 메시지 수신
		case update := <-updateC:
			// 메시지가 없는 업데이트는 무시
			if update.Message == nil {
				continue
			}

			// 등록되지 않은 ChatID인 경우는 무시한다.
			if update.Message.Chat.ID != n.chatID {
				continue
			}

			// 수신된 명령어를 처리 핸들러로 위임
			n.handleCommand(taskRunner, update.Message)

		// 2. 내부 시스템으로부터 발송할 알림 요청 수신
		case notifyRequest := <-n.requestC:
			n.handleNotifyRequest(notifyRequest)

		// 3. 서비스 종료 시그널 수신
		case <-notificationStopCtx.Done():
			// 텔레그램 메시지 수신을 중지하고 관련 리소스를 정리합니다.
			n.botAPI.StopReceivingUpdates()
			n.Close()
			n.botAPI = nil

			applog.WithComponentAndFields("notification.telegram", log.Fields{
				"notifier_id": n.ID(),
			}).Debug("Telegram Notifier의 작업이 중지됨")

			return
		}
	}
}
