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
	telegramBotCommandHelp   = "help"
	telegramBotCommandCancel = "cancel"

	telegramBotCommandSeparator        = "_"
	telegramBotCommandInitialCharacter = "/"
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

			notifier.botCommands = append(notifier.botCommands,
				telegramBotCommand{
					command:            fmt.Sprintf("%s_%s", utils.ToSnakeCase(t.ID), utils.ToSnakeCase(c.ID)),
					commandTitle:       fmt.Sprintf("%s > %s", t.Title, c.Title),
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

					if _, err := n.bot.Send(tgbotapi.NewMessage(n.chatID, m)); err != nil {
						log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
					}

					continue
				} else if strings.HasPrefix(command, fmt.Sprintf("%s%s", telegramBotCommandCancel, telegramBotCommandSeparator)) == true {
					// 취소명령 형식 : /cancel_nnnn
					commandSplit := strings.Split(command, telegramBotCommandSeparator)
					if len(commandSplit) == 2 {
						if taskInstanceID, err := strconv.ParseUint(commandSplit[1], 10, 64); err == nil {
							if taskRunner.TaskCancel(task.TaskInstanceID(taskInstanceID)) == false {
								m := fmt.Sprintf("작업취소 요청이 실패하였습니다.(ID:%d)", taskInstanceID)

								log.Error(m)
								if _, err := n.bot.Send(tgbotapi.NewMessage(n.chatID, m)); err != nil {
									log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
								}
							}

							continue
						}
					}
				}

				for _, botCommand := range n.botCommands {
					if command == botCommand.command {
						if taskRunner.TaskRunWithContext(botCommand.taskID, botCommand.taskCommandID, nil, string(n.ID()), true) == false {
							log.Errorf("사용자가 요청한 작업('%s')의 실행 요청이 실패하였습니다.", botCommand.commandTitle)

							n.notificationSendC <- &notificationSendData{
								message: "사용자가 요청한 작업의 실행 요청이 실패하였습니다.",
								taskCtx: task.NewContext().WithTask(botCommand.taskID, botCommand.taskCommandID).WithError(),
							}
						}

						goto LOOP
					}
				}
			}

			m := fmt.Sprintf("'%s'는 등록되지 않은 명령어입니다.\n명령어를 모르시면 '%s%s'을 입력하세요.", update.Message.Text, telegramBotCommandInitialCharacter, telegramBotCommandHelp)
			if _, err := n.bot.Send(tgbotapi.NewMessage(n.chatID, m)); err != nil {
				log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
			}

		case notificationSendData := <-n.notificationSendC:
			if notificationSendData.taskCtx == nil {
				if _, err := n.bot.Send(tgbotapi.NewMessage(n.chatID, notificationSendData.message)); err != nil {
					log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
				}
			} else {
				m := notificationSendData.message

				taskID, ok1 := notificationSendData.taskCtx.Value(task.TaskCtxKeyTaskID).(task.TaskID)
				taskCommandID, ok2 := notificationSendData.taskCtx.Value(task.TaskCtxKeyTaskCommandID).(task.TaskCommandID)
				if ok1 == true && ok2 == true {
					for _, botCommand := range n.botCommands {
						if botCommand.taskID == taskID && botCommand.taskCommandID == taskCommandID {
							m = fmt.Sprintf("<b>[ %s ]</b>\n\n%s", botCommand.commandTitle, m)
							break
						}
					}
				}

				// TaskInstanceID가 존재하는 경우 취소 명령어를 붙인다.
				if taskInstanceID, ok := notificationSendData.taskCtx.Value(task.TaskCtxKeyTaskInstanceID).(task.TaskInstanceID); ok == true {
					m += fmt.Sprintf("\n%s%s%s%d", telegramBotCommandInitialCharacter, telegramBotCommandCancel, telegramBotCommandSeparator, taskInstanceID)
				}

				if errorOccured, ok := notificationSendData.taskCtx.Value(task.TaskCtxKeyErrorOccurred).(bool); ok == true && errorOccured == true {
					m = fmt.Sprintf("%s\n\n*** 오류가 발생하였습니다. ***", m)
				}

				messageConfig := tgbotapi.NewMessage(n.chatID, m)
				messageConfig.ParseMode = tgbotapi.ModeHTML

				if _, err := n.bot.Send(messageConfig); err != nil {
					log.Errorf("알림메시지 발송이 실패하였습니다.(error:%s)", err)
				}
			}

		case <-notificationStopCtx.Done():
			n.bot.StopReceivingUpdates()

			close(n.notificationSendC)

			n.bot = nil
			n.notificationSendC = nil

			log.Debugf("'%s' Telegram Notifier의 작업이 중지됨", n.ID())

			return
		}
	}
}
