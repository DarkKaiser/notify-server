package notify

import (
	"context"
	"fmt"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
	"sync"
)

type telegramNotifier struct {
	notifier

	chatId int64

	bot         *tgbotapi.BotAPI
	botCommands []telegramBotCommand     //@@@@@
	r           task.TaskHandleRequester // @@@@@
}

// @@@@@
type telegramBotCommand struct {
	command            string
	commandKor         string
	commandSyntax      string
	commandDescription string
}

func newTelegramNotifier(id NotifierId, token string, chatId int64) notifierHandler {
	notifier := &telegramNotifier{
		notifier: notifier{
			id: id,
		},

		chatId: chatId,
	}

	// @@@@@
	notifier.botCommands = append(notifier.botCommands, telegramBotCommand{
		command:            "new_alganicmall_event",
		commandKor:         "엘가닉몰 이벤트",
		commandSyntax:      "/new_alganicmall_event (도움말)",
		commandDescription: "엘가닉몰 이벤트를 표시합니다.",
	}, telegramBotCommand{
		command:            "help",
		commandKor:         "도움말",
		commandSyntax:      "/help (도움말)",
		commandDescription: "도움말을 표시합니다.",
	})

	// 텔레그램 봇을 생성한다.
	var err error
	notifier.bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	notifier.bot.Debug = true

	return notifier
}

func (n *telegramNotifier) Run(notifyStopCtx context.Context, notifyStopWaiter *sync.WaitGroup) {
	defer notifyStopWaiter.Done()

	// @@@@@ notifyservice run()함수에서 구하기, 못 구할때는 panic 발생???
	v := notifyStopCtx.Value("TaskHandleRequester")
	u, ok := v.(task.TaskHandleRequester)
	println(ok)
	taskService := u
	//if v := ctx.Value("current_user"); v != nil {
	//	// 타입 확인(type assertion)
	//	u, ok := v.(User)
	//	if !ok {
	//		return errors.New("Not authorized")
	//	}
	//	currentUser = u
	//} else {
	//	return errors.New("Not authorized")
	//}

	config := tgbotapi.NewUpdate(0)
	config.Timeout = 60

	updateC, _ := n.bot.GetUpdatesChan(config)

	log.Debugf("'%s' Telegram Notifier의 작업이 시작됨(Authorized on account %s)", n.id, n.bot.Self.UserName)

	for {
		select {
		case update := <-updateC:
			// ignore any non-Message Updates
			if update.Message == nil {
				continue
			}

			// 등록되지 않은 ChatID인 경우는 무시한다.
			if update.Message.Chat.ID != n.chatId {
				continue
			}

			// @@@@@
			command := update.Message.Text
			command = command[1:]

			if command == "help" {
				var m = fmt.Sprintf("입력 가능한 명령어는 아래와 같습니다:\n\n")
				for i, botCommand := range n.botCommands {
					if i != 0 {
						m += "\n\n"
					}
					m += fmt.Sprintf("%s\n%s", botCommand.commandSyntax, botCommand.commandDescription)
				}
				msg := tgbotapi.NewMessage(n.chatId, string(m))
				n.bot.Send(msg)

				continue
			}

			for _, botCommand := range n.botCommands {
				if command == botCommand.command {
					ctx := context.Background()
					ctx = context.WithValue(ctx, "chatId", update.Message.Chat.ID)
					ctx = context.WithValue(ctx, "messageId", update.Message.MessageID)

					taskService.TaskRunWithContext(task.TidAlganicMall, task.TcidAlganicMallWatchNewEvents, ctx)

					continue
				}
			}

			// 취소명령/cancel_xxx

			m := fmt.Sprintf("'%s'는 등록되지 않은 명령어입니다.\n명령어를 모르시면 '/help'을 입력하세요.", update.Message.Text)
			msg := tgbotapi.NewMessage(n.chatId, string(m))
			n.bot.Send(msg)

			//					add <- struct {
			//						taskId : TI_ALGANICMALL,
			//						commandId : TCI_ALGANICMALL_CRAWING,
			//						ctx : ctx,
			//					}
			//} else if update.Message.Text == "/help" {
			//	//@@@@@
			//	m := fmt.Sprintf("입력 가능한 명령어는 아래와 같습니다:\n")
			//	msg := tgbotapi.NewMessage(n.chatId, string(m))
			//	n.bot.Send(msg)
			//} else {
			//}
			//			case receive<-notifymessage:받을때 context를 그대로 받는다.
			//				msg := tgbotapi.NewMessage(297396697, m)
			//				//msg.ReplyToMessageID = update.Message.MessageID
			//				s.bot.Send(msg)

		case <-notifyStopCtx.Done():
			n.bot.StopReceivingUpdates()

			log.Debugf("'%s' Telegram Notifier의 작업이 중지됨", n.id)

			return
		}
	}
}

//@@@@@ XXXXX channel로 수신
func (n *telegramNotifier) Notify(m string) bool {
	msg := tgbotapi.NewMessage(n.chatId, m)
	//msg.ReplyToMessageID = update.Message.MessageID
	n.bot.Send(msg)

	return true
}
