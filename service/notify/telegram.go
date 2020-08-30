package notify

import (
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
	"sync"
)

type telegramNotifier struct {
	notifier

	chatId int64

	bot *tgbotapi.BotAPI
}

func newTelegramNotifier(id NotifierId, token string, chatId int64, notifyStopCtx context.Context, notifyStopWaiter *sync.WaitGroup) notifierHandler {
	notifier := &telegramNotifier{
		notifier: notifier{
			id: id,
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

	go notifier.run0(updateC, notifyStopCtx, notifyStopWaiter)

	log.Debugf("'%s' Telegram Notifier의 알림활동이 시작됨(Authorized on account %s)", notifier.id, notifier.bot.Self.UserName)

	return notifier
}

func (n *telegramNotifier) run0(updateC tgbotapi.UpdatesChannel, notifyStopCtx context.Context, notifyStopWaiter *sync.WaitGroup) {
	defer notifyStopWaiter.Done()

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
				// @@@@@ 텔레그램으로 취소할수있는 메시지를 보내야함
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

		case <-notifyStopCtx.Done():
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
