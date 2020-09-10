package notification

import (
	"context"
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"sync"
)

const (
	// @@@@@
	NotifierContextKeyBotCommand NotifierContextKey = "botCommand"
	NotifierContextKeyMessageID  NotifierContextKey = "messageId"
)

const (
	telegramBotCommandHelp   string = "help"
	telegramBotCommandCancel string = "cancel"

	telegramBotCommandSeparator        string = "_"
	telegramBotCommandInitialCharacter string = "/"
)

type telegramBotCommand struct {
	command            string
	commandTitle       string
	commandDescription string

	taskID        task.TaskID
	taskCommandID task.TaskCommandID
}

type telegramNotifier struct {
	notifier

	chatID int64

	bot *tgbotapi.BotAPI

	botCommands []telegramBotCommand
}

func newTelegramNotifier(id NotifierID, token string, chatID int64, config *g.AppConfig) notifierHandler {
	notifier := &telegramNotifier{
		notifier: notifier{
			id: id,

			notificationSendC: make(chan *notificationSendData, 10),
		},

		chatID: chatID,
	}

	// Bot Command를 초기화합니다.
	for _, t := range config.Tasks {
		for _, c := range t.Commands {
			if c.Notifier.Usable == false {
				continue
			}

			command := fmt.Sprintf("%s%s%s", utils.ToSnakeCase(t.ID, telegramBotCommandSeparator), telegramBotCommandSeparator, utils.ToSnakeCase(c.ID, telegramBotCommandSeparator))

			notifier.botCommands = append(notifier.botCommands,
				telegramBotCommand{
					command:            command,
					commandTitle:       t.Title,
					commandDescription: c.Description,

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

	// 텔레그램 봇을 생성한다.
	var err error
	notifier.bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	notifier.bot.Debug = true

	return notifier
}

func (n *telegramNotifier) Run(taskRunner task.TaskRunner, notificationStopCtx context.Context, notificationStopWaiter *sync.WaitGroup) {
	defer notificationStopWaiter.Done()

	config := tgbotapi.NewUpdate(0)
	config.Timeout = 60

	updateC, _ := n.bot.GetUpdatesChan(config)

	log.Debugf("'%s' Telegram Notifier의 작업이 시작됨(Authorized on account %s)", n.ID(), n.bot.Self.UserName)

LOOP:
	for {
		select {
		case update := <-updateC:
			// ignore any non-Message Updates
			if update.Message == nil {
				continue
			}

			// 등록되지 않은 ChatID인 경우는 무시한다.
			if update.Message.Chat.ID != n.chatID {
				continue
			}

			if update.Message.Text[:1] == telegramBotCommandInitialCharacter {
				command := update.Message.Text[1:]

				if command == telegramBotCommandHelp {
					m := fmt.Sprintf("입력 가능한 명령어는 아래와 같습니다:\n\n")
					for i, botCommand := range n.botCommands {
						if i != 0 {
							m += "\n\n"
						}
						m += fmt.Sprintf("%s%s\n%s", telegramBotCommandInitialCharacter, botCommand.command, botCommand.commandDescription)
					}

					_, err := n.bot.Send(tgbotapi.NewMessage(n.chatID, m))
					if err != nil {
						log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
					}

					continue
				} else if strings.HasPrefix(command, fmt.Sprintf("%s%s", telegramBotCommandCancel, telegramBotCommandSeparator)) == true {
					// 취소명령 형식 : /cancel_nnn
					commandSplit := strings.Split(command, telegramBotCommandSeparator)
					if len(commandSplit) == 2 {
						// @@@@@
						//////////////////
						taskInstanceID, _ := strconv.ParseUint(commandSplit[1], 10, 32)
						taskRunner.TaskCancel(task.TaskInstanceID(taskInstanceID))
						//////////////////
						continue
					}
				}

				for _, botCommand := range n.botCommands {
					if command == botCommand.command {
						// @@@@@
						//////////////////
						// filldefaultcontext()
						ctx := context.Background()
						ctx = context.WithValue(ctx, NotifierContextTaskID, botCommand.taskID)
						ctx = context.WithValue(ctx, NotifierContextTaskCommandID, botCommand.taskCommandID)
						// ctx = context.WithValue(ctx, NotifierContextKeyInstanceID, 0) // @@@@@ add 하지 않음

						// telegram notifier에 종속적인 값들
						ctx = context.WithValue(ctx, NotifierContextKeyBotCommand, command)
						ctx = context.WithValue(ctx, NotifierContextKeyMessageID, update.Message.MessageID)

						if taskRunner.TaskRunWithContext(task.TaskID(botCommand.taskID), task.TaskCommandID(botCommand.taskCommandID), string(n.ID()), ctx, true) == false {
							log.Errorf("Task 실행요청이 실패하였습니다.(%s)", botCommand)
							// bot.send
						}
						//////////////////

						goto LOOP
					}
				}
			}

			m := fmt.Sprintf("'%s'는 등록되지 않은 명령어입니다.\n명령어를 모르시면 '/%s'을 입력하세요.", update.Message.Text, telegramBotCommandHelp)
			_, err := n.bot.Send(tgbotapi.NewMessage(n.chatID, m))
			if err != nil {
				log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
			}

		case notificationSendData := <-n.notificationSendC:
			//@@@@@
			////////////////////////////////
			if notificationSendData.ctx == nil {
				m := tgbotapi.NewMessage(n.chatID, notificationSendData.message)
				_, err := n.bot.Send(m)
				if err != nil {
					log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
				}
			} else {
				// @@@@@
				m := notificationSendData.message
				v, ok := notificationSendData.ctx.Value(NotifierContextTaskInstanceID).(uint64)
				if ok == true {
					m += fmt.Sprintf("\n/%s%s%d", telegramBotCommandCancel, telegramBotCommandSeparator, v)
				}
				msg := tgbotapi.NewMessage(n.chatID, m)
				//msg.ReplyToMessageID = update.Message.MessageID
				_, err := n.bot.Send(msg)
				if err != nil {
					log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
				}
			}
			////////////////////////////////

		case <-notificationStopCtx.Done():
			n.bot.StopReceivingUpdates()

			////////////////////////////////
			// @@@@@
			n.bot = nil
			n.chatID = 0
			n.botCommands = nil
			close(n.notificationSendC)
			////////////////////////////////

			log.Debugf("'%s' Telegram Notifier의 작업이 중지됨", n.ID())

			return
		}
	}
}
