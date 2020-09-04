package service

import (
	"context"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
	"sync"
)

type telegramNotifier struct {
	notifier

	chatId int64

	bot *tgbotapi.BotAPI

	botCommands []telegramBotCommand
}

type telegramBotCommand struct {
	command     string
	description string
}

const (
	botCommandHelp                      string = "help"
	botCommandCancel                    string = "cancel"
	botCommandAlganicmallWatchNewEvents string = "alganicmall_watch_new_events"
)

func newTelegramNotifier(id NotifierId, token string, chatId int64) notifierHandler {
	notifier := &telegramNotifier{
		notifier: notifier{
			id: id,

			notifySendC: make(chan *notifySendData, 10),
		},

		chatId: chatId,
	}

	notifier.botCommands = append(notifier.botCommands, telegramBotCommand{
		command:     botCommandAlganicmallWatchNewEvents,
		description: "엘가닉몰에 신규 이벤트가 발생될 때 알림 메시지를 보냅니다.",
	}, telegramBotCommand{
		command:     botCommandHelp,
		description: "도움말을 표시합니다.",
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

func (n *telegramNotifier) Run(runner TaskRunner, notifyStopCtx context.Context, notifyStopWaiter *sync.WaitGroup) {
	defer notifyStopWaiter.Done()

	config := tgbotapi.NewUpdate(0)
	config.Timeout = 60

	updateC, _ := n.bot.GetUpdatesChan(config)

	log.Debugf("'%s' Telegram Notifier의 작업이 시작됨(Authorized on account %s)", n.id, n.bot.Self.UserName)

LOOP:
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

			command := update.Message.Text[1:]
			switch command {
			case botCommandHelp:
				m := fmt.Sprintf("입력 가능한 명령어는 아래와 같습니다:\n\n")
				for i, botCommand := range n.botCommands {
					if i != 0 {
						m += "\n\n"
					}
					m += fmt.Sprintf("/%s\n%s", botCommand.command, botCommand.description)
				}

				_, err := n.bot.Send(tgbotapi.NewMessage(n.chatId, m))
				if err != nil {
					log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
				}

				continue

			case botCommandCancel:
				// 취소명령/cancel_xxx@@@@@
				continue
			}

			for _, botCommand := range n.botCommands {
				if command == botCommand.command {
					// @@@@@
					//////////////////
					ctx := context.Background()
					ctx = context.WithValue(ctx, "taskId", TidAlganicMall)
					ctx = context.WithValue(ctx, "taskCommandId", TcidAlganicMallWatchNewEvents)
					ctx = context.WithValue(ctx, "messageId", update.Message.MessageID)

					if runner.TaskRunWithContext(TidAlganicMall, TcidAlganicMallWatchNewEvents, n.Id(), ctx) == true {
					}
					//////////////////

					goto LOOP
				}
			}

			m := fmt.Sprintf("'%s'는 등록되지 않은 명령어입니다.\n명령어를 모르시면 '/help'을 입력하세요.", update.Message.Text)
			_, err := n.bot.Send(tgbotapi.NewMessage(n.chatId, m))
			if err != nil {
				log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
			}

		case notifySendData := <-n.notifySendC:
			// @@@@@
			msg := tgbotapi.NewMessage(n.chatId, notifySendData.message)
			//msg.ReplyToMessageID = update.Message.MessageID
			_, err := n.bot.Send(msg)
			if err != nil {
				log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
			}

		case <-notifyStopCtx.Done():
			n.bot.StopReceivingUpdates()

			////////////////////////////////
			// @@@@@
			n.bot = nil
			n.chatId = 0
			n.botCommands = nil
			close(n.notifySendC)
			////////////////////////////////

			log.Debugf("'%s' Telegram Notifier의 작업이 중지됨", n.id)

			return
		}
	}
}
