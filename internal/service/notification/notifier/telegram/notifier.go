package telegram

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/darkkaiser/notify-server/internal/service/notification/constants"
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

	// handlerSemaphore 봇 명령어 처리 핸들러의 동시 실행 수를 제한하기 위한 세마포어
	handlerSemaphore chan struct{}

	botCommands []telegramBotCommand

	// botCommandsByCommand command 문자열로 빠르게 조회하기 위한 Map (O(1) 조회)
	botCommandsByCommand map[string]telegramBotCommand

	// botCommandsByTaskAndCommand "taskID" -> "commandID" -> command 구조로 조회 (키 충돌 방지)
	botCommandsByTaskAndCommand map[string]map[string]telegramBotCommand
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
		case update, ok := <-updateC:
			if !ok {
				applog.WithComponentAndFields("notification.telegram", applog.Fields{
					"notifier_id": n.ID(),
					"chat_id":     n.chatID,
				}).Error("텔레그램 업데이트 채널이 닫혔습니다. 수신 루프를 종료합니다.")
				return
			}

			// 메시지가 없는 업데이트는 무시
			if update.Message == nil {
				continue
			}

			// 등록되지 않은 ChatID인 경우는 무시한다.
			if update.Message.Chat.ID != n.chatID {
				continue
			}

			// 수신된 명령어를 처리 핸들러로 위임
			// 명령어 처리 중 네트워크 지연(예: 메시지 발송)이 발생해도
			// Receiver 루프가 차단되지 않도록 고루틴으로 실행합니다.
			//
			// Goroutine Leak 방지: 세마포어를 통해 동시 실행 고루틴 수를 제한합니다.
			select {
			case n.handlerSemaphore <- struct{}{}:
				go func(msg *tgbotapi.Message) {
					defer func() { <-n.handlerSemaphore }()
					n.handleCommand(n.executor, msg)
				}(update.Message)
			case <-notificationStopCtx.Done():
				// 컨텍스트 종료 시 루프 탈출
				return
			default:
				// 세마포어가 가득 찬 경우 (Backpressure)
				// 과도한 부하 상황이므로 로그를 남기고 메시지를 처리하지 않음 (Drop)
				// 사용자가 다시 시도하도록 유도하거나, 단순히 무시하여 서버 안정성을 확보함.
				applog.WithComponentAndFields("notification.telegram", applog.Fields{
					"notifier_id": n.ID(),
					"chat_id":     n.chatID,
				}).Warn("봇 명령어 처리량이 한계에 도달하여 요청을 처리할 수 없습니다 (Drop)")
			}

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

			// 안전하게 메시지 처리 (Panic Recovery)
			func() {
				defer func() {
					if r := recover(); r != nil {
						applog.WithComponentAndFields("notification.telegram", applog.Fields{
							"notifier_id": n.ID(),
							"panic":       r,
						}).Error("알림 메시지 발송 중 패닉 발생 (Recovered)")
					}
				}()

				// 메시지 전송 시 독립적인 컨텍스트 사용 (Graceful Shutdown 시 In-Flight 메시지 유실 방지)
				// 메인 ctx가 취소되더라도, 이미 꺼낸 메시지는 끝까지 전송을 시도해야 합니다.
				sendCtx, cancel := context.WithTimeout(context.Background(), constants.DefaultNotifyTimeout)
				defer cancel()

				n.handleNotifyRequest(sendCtx, notifyRequest)
			}()

			// 서비스 종료 시그널 수신
		case <-ctx.Done():
			// BaseNotifier가 완전히 닫혀서 더 이상 새로운 요청을 받지 않을 때까지 대기
			// 이를 통해 runSender가 종료된 후에 Notify가 호출되어 메시지가 유실되는 경쟁 상태를 방지합니다.
			<-n.Done()

			// 컨텍스트 종료 시 남은 요청 처리 (Drain)
			// Drain 시에는 이미 취소된(Expired) Context를 사용하면 안 되므로,
			// 새로운 Background Context나 Drain 전용 타임아웃 컨텍스트를 사용해야 합니다.
			// 무한 대기(Hang)를 방지하기 위해 60초의 타임아웃을 설정합니다.
			// (버퍼 1000개, 초당 20~30개 처리 가정 시 최소 30~50초 소요됨)
			drainCtx, cancel := context.WithTimeout(context.Background(), constants.TelegramShutdownTimeout)
			defer cancel()

			// 큐에 남은 미처리 요청을 비동기적으로 처리 (Non-blocking Drain)
			// RequestC가 닫히지 않으므로 range를 사용할 수 없음.
		Loop:
			for {
				select {
				case notifyRequest := <-n.RequestC:
					// 이미 타임아웃이 발생했다면 더 이상 시도하지 않고 루프 탈출
					if drainCtx.Err() != nil {
						applog.WithComponentAndFields("notification.telegram", applog.Fields{
							"notifier_id": n.ID(),
						}).Warn("Shutdown Drain 타임아웃 발생, 잔여 메시지 발송 중단")
						break Loop
					}
					// Drain 중에도 Panic Recovery가 필요합니다.
					func() {
						defer func() {
							if r := recover(); r != nil {
								applog.WithComponentAndFields("notification.telegram", applog.Fields{
									"notifier_id": n.ID(),
									"panic":       r,
								}).Error("Shutdown Drain 중 패닉 발생 (Recovered)")
							}
						}()
						n.handleNotifyRequest(drainCtx, notifyRequest)
					}()
				default:
					// 채널이 비었으면 루프 탈출
					break Loop
				}
			}
			return
		}
	}
}
