package notify

import (
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
	"sync"
)

type telegramNotifier struct {
	notifier

	bot *tgbotapi.BotAPI

	chatId int64
}

func newTelegramNotifier(id NotifierId, token string, chatId int64, notifyStopWaiter *sync.WaitGroup, notifyServiceStopCtx context.Context) notifierHandler {
	notifier := &telegramNotifier{
		notifier: notifier{
			id:                   id,
			notifyStopWaiter:     notifyStopWaiter,
			notifyServiceStopCtx: notifyServiceStopCtx,
		},

		chatId: chatId,
	}

	// 텔레그램 봇을 생성한다.
	var err error
	notifier.bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	notifier.bot.Debug = true

	config := tgbotapi.NewUpdate(0)
	config.Timeout = 60

	updateC, err := notifier.bot.GetUpdatesChan(config)

	go notifier._running_(updateC)

	log.Debugf("'%s' Telegram Notifier의 알림활동이 시작됨(Authorized on account %s)", notifier.id, notifier.bot.Self.UserName)

	return notifier
}

func (n *telegramNotifier) _running_(updateC tgbotapi.UpdatesChannel) {
	defer n.notifyStopWaiter.Done()

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
			log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

			if update.Message.Text == "/help" {
				n.Notify("도움말은 아직이예요")
			} else if update.Message.Text == "/new_alganicmall_event" {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "chatId", update.Message.Chat.ID)
				ctx = context.WithValue(ctx, "messageId", update.Message.MessageID)

				//					add <- struct {
				//						taskId : TI_ALGANICMALL,
				//						commandId : TCI_ALGANICMALL_CRAWING,
				//						ctx : ctx,
				//					}
			}
			//			case receive<-notifymessage:받을때 context를 그대로 받는다.
			//				msg := tgbotapi.NewMessage(297396697, m)
			//				//msg.ReplyToMessageID = update.Message.MessageID
			//				s.bot.Send(msg)

		case <-n.notifyServiceStopCtx.Done():
			n.bot.StopReceivingUpdates()

			log.Debugf("'%s' Telegram Notifier의 알림활동이 중지됨", n.id)

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
