package telegram

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// 텔레그램 봇 명령어 상수
	// 봇과 사용자 간의 상호작용에 사용됩니다.
	telegramBotCommandHelp   = "help"   // 도움말
	telegramBotCommandCancel = "cancel" // 작업 취소

	telegramBotCommandSeparator        = "_" // 명령어와 인자(예: InstanceID)를 구분하는 구분자
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

	taskID    task.ID        // 이 명령어와 연결된 작업(Task) ID
	commandID task.CommandID // 이 명령어와 연결된 작업 커맨드 ID
}

// telegramBotAPI 텔레그램 봇 API 인터페이스
type telegramBotAPI interface {
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	StopReceivingUpdates()
	GetSelf() tgbotapi.User
}

// telegramBotAPIClient tgbotapi.BotAPI 구현체를 래핑한 구조체 (telegramBotAPI 인터페이스 구현)
type telegramBotAPIClient struct {
	*tgbotapi.BotAPI
}

// GetSelf 텔레그램 봇의 정보를 반환합니다.
func (w *telegramBotAPIClient) GetSelf() tgbotapi.User {
	return w.Self
}

// telegramNotifier 텔레그램 알림 발송 및 봇 상호작용을 처리하는 Notifier
type telegramNotifier struct {
	notifier.BaseNotifier

	chatID int64

	botAPI telegramBotAPI

	executor task.Executor

	retryDelay time.Duration
	limiter    *rate.Limiter

	botCommands []telegramBotCommand
}

// Run 메시지 폴링 및 알림 처리 메인 루프
func (n *telegramNotifier) Run(notificationStopCtx context.Context) {
	// 텔레그램 메시지 수신 설정
	config := tgbotapi.NewUpdate(0)
	config.Timeout = 60 // Long Polling 타임아웃 60초 설정

	// 메시지 수신 채널 획득
	updateC := n.botAPI.GetUpdatesChan(config)

	applog.WithComponentAndFields("notification.telegram", applog.Fields{
		"notifier_id":  n.ID(),
		"bot_username": n.botAPI.GetSelf().UserName,
		"chat_id":      n.chatID,
	}).Debug("Telegram Notifier의 작업이 시작됨")

	var wg sync.WaitGroup

	// 1. 알림 발송을 전담하는 고루틴 시작 (Sender)
	// 이를 통해 알림 전송 지연이 발생하더라도 봇 명령어 수신(Receiver)은 영향을 받지 않습니다.
	wg.Add(1)
	go func() {
		defer wg.Done()
		n.runSender(notificationStopCtx)
	}()

	// 2. 텔레그램 메시지 수신 및 명령어 처리 (Receiver)
	// 메인 루프에서는 봇 업데이트만 처리합니다.
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

		case <-notificationStopCtx.Done():
			// 텔레그램 메시지 수신을 중지하고 관련 리소스를 정리합니다.
			n.botAPI.StopReceivingUpdates()
			n.Close()

			// Sender 고루틴이 종료될 때까지 대기
			wg.Wait()

			n.botAPI = nil

			applog.WithComponentAndFields("notification.telegram", applog.Fields{
				"notifier_id": n.ID(),
				"chat_id":     n.chatID,
			}).Debug("Telegram Notifier의 작업이 중지됨")

			return
		}
	}
}

// runSender 알림 발송 요청을 처리하는 작업 루프 (Worker)
func (n *telegramNotifier) runSender(ctx context.Context) {
	for {
		select {
		// 내부 시스템으로부터 발송할 알림 요청 수신
		case notifyRequest, ok := <-n.RequestC:
			if !ok {
				return // 채널이 닫히면 종료
			}
			n.handleNotifyRequest(notifyRequest)

			// 서비스 종료 시그널 수신
		case <-ctx.Done():
			// 컨텍스트 종료 시 남은 요청 처리 (Drain)
			for notifyRequest := range n.RequestC {
				n.handleNotifyRequest(notifyRequest)
			}
			return
		}
	}
}
