package notification

import (
	"context"

	applog "github.com/darkkaiser/notify-server/pkg/log"
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

	taskID        task.ID        // 이 명령어와 연결된 작업(Task) ID
	taskCommandID task.CommandID // 이 명령어와 연결된 작업 커맨드 ID
}

// TelegramBotAPI 텔레그램 봇 API 인터페이스
type TelegramBotAPI interface {
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	StopReceivingUpdates()
	GetSelf() tgbotapi.User
}

// telegramNotifier 텔레그램 알림 발송 및 봇 상호작용을 처리하는 Notifier
type telegramNotifier struct {
	notifier

	chatID int64

	botAPI TelegramBotAPI

	botCommands []telegramBotCommand

	executor task.Executor
}

// Run 메시지 폴링 및 알림 처리 메인 루프
func (n *telegramNotifier) Run(notificationStopCtx context.Context) {
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
			n.handleCommand(n.executor, update.Message)

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
