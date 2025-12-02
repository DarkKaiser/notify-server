package notification

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
)

const (
	telegramBotCommandHelp   = "help"
	telegramBotCommandCancel = "cancel"

	telegramBotCommandSeparator        = "_"
	telegramBotCommandInitialCharacter = "/"

	// 한번에 보낼 수 있는 텔레그램 메시지의 최대 길이
	telegramMessageMaxLength = 3900
)

// TelegramBot defines the interface for interacting with the Telegram Bot API.
// This allows for mocking in tests.
type TelegramBot interface {
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	StopReceivingUpdates()
	GetSelf() tgbotapi.User
}

// telegramBotWrapper wraps the actual tgbotapi.BotAPI to implement the TelegramBot interface.
type telegramBotWrapper struct {
	*tgbotapi.BotAPI
}

func (w *telegramBotWrapper) GetSelf() tgbotapi.User {
	return w.Self
}

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

	bot TelegramBot

	botCommands []telegramBotCommand
}

func newTelegramNotifier(id NotifierID, botToken string, chatID int64, config *g.AppConfig) NotifierHandler {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true

	return newTelegramNotifierWithBot(id, &telegramBotWrapper{BotAPI: bot}, chatID, config)
}

// newTelegramNotifierWithBot is an internal constructor that accepts a TelegramBot interface.
// This is useful for testing.
func newTelegramNotifierWithBot(id NotifierID, bot TelegramBot, chatID int64, config *g.AppConfig) NotifierHandler {
	notifier := &telegramNotifier{
		notifier: notifier{
			id: id,

			supportHTMLMessage: true,

			notificationSendC: make(chan *notificationSendData, 10),
		},

		chatID: chatID,
		bot:    bot,
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

	return notifier
}

func (n *telegramNotifier) Run(taskRunner task.TaskRunner, notificationStopCtx context.Context, notificationStopWaiter *sync.WaitGroup) {
	defer notificationStopWaiter.Done()

	config := tgbotapi.NewUpdate(0)
	config.Timeout = 60

	updateC := n.bot.GetUpdatesChan(config)

	log.WithFields(log.Fields{
		"component":    "notification.telegram",
		"notifier_id":  n.ID(),
		"bot_username": n.bot.GetSelf().UserName,
	}).Debug("Telegram Notifier의 작업이 시작됨")

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
					m := "입력 가능한 명령어는 아래와 같습니다:\n\n"
					for i, botCommand := range n.botCommands {
						if i != 0 {
							m += "\n\n"
						}
						m += fmt.Sprintf("%s%s\n%s", telegramBotCommandInitialCharacter, botCommand.command, botCommand.commandDescription)
					}

					if _, err := n.bot.Send(tgbotapi.NewMessage(n.chatID, m)); err != nil {
						log.WithFields(log.Fields{
							"notifier_id": n.ID(),
							"error":       err,
						}).Error("알림메시지 발송 실패")
					}

					continue
				} else if strings.HasPrefix(command, fmt.Sprintf("%s%s", telegramBotCommandCancel, telegramBotCommandSeparator)) == true {
					// 취소명령 형식 : /cancel_nnnn
					commandSplit := strings.Split(command, telegramBotCommandSeparator)
					if len(commandSplit) == 2 {
						taskInstanceID := commandSplit[1]
						if taskRunner.TaskCancel(task.TaskInstanceID(taskInstanceID)) == false {
							n.notificationSendC <- &notificationSendData{
								message: fmt.Sprintf("작업취소 요청이 실패하였습니다.(ID:%s)", taskInstanceID),
								taskCtx: task.NewContext().WithError(),
							}
						}

						continue
					}
				}

				for _, botCommand := range n.botCommands {
					if command == botCommand.command {
						if taskRunner.TaskRun(botCommand.taskID, botCommand.taskCommandID, string(n.ID()), true, task.TaskRunByUser) == false {
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
				log.WithFields(log.Fields{
					"notifier_id": n.ID(),
					"error":       err,
				}).Error("알림메시지 발송 실패")
			}

		case notificationSendData := <-n.notificationSendC:
			m := notificationSendData.message

			if notificationSendData.taskCtx == nil {
				if _, err := n.bot.Send(tgbotapi.NewMessage(n.chatID, m)); err != nil {
					log.WithFields(log.Fields{
						"component":   "notification.telegram",
						"notifier_id": n.ID(),
						"error":       err,
					}).Error("알림메시지 발송이 실패하였습니다")
				}
			} else {
				title, ok := notificationSendData.taskCtx.Value(task.TaskCtxKeyTitle).(string)
				if ok == true && len(title) > 0 {
					m = fmt.Sprintf("<b>【 %s 】</b>\n\n%s", title, m)
				} else {
					taskID, ok1 := notificationSendData.taskCtx.Value(task.TaskCtxKeyTaskID).(task.TaskID)
					taskCommandID, ok2 := notificationSendData.taskCtx.Value(task.TaskCtxKeyTaskCommandID).(task.TaskCommandID)
					if ok1 == true && ok2 == true {
						for _, botCommand := range n.botCommands {
							if botCommand.taskID == taskID && botCommand.taskCommandID == taskCommandID {
								m = fmt.Sprintf("<b>【 %s 】</b>\n\n%s", botCommand.commandTitle, m)
								break
							}
						}
					}
				}

				// TaskInstanceID가 존재하는 경우 취소 명령어를 붙인다.
				if taskInstanceID, ok := notificationSendData.taskCtx.Value(task.TaskCtxKeyTaskInstanceID).(task.TaskInstanceID); ok == true {
					m += fmt.Sprintf("\n%s%s%s%s", telegramBotCommandInitialCharacter, telegramBotCommandCancel, telegramBotCommandSeparator, taskInstanceID)

					// 작업 실행 후 경과시간(단위 : 초)
					if elapsedTimeAfterRun, ok := notificationSendData.taskCtx.Value(task.TaskCtxKeyElapsedTimeAfterRun).(int64); ok == true && elapsedTimeAfterRun > 0 {
						seconds := elapsedTimeAfterRun % 60
						elapsedTimeAfterRun = elapsedTimeAfterRun / 60
						minutes := elapsedTimeAfterRun % 60
						hours := elapsedTimeAfterRun / 60

						var elapsedTimeString string
						if hours > 0 {
							elapsedTimeString = fmt.Sprintf("%d시간 ", hours)
						}
						if minutes > 0 {
							elapsedTimeString += fmt.Sprintf("%d분 ", minutes)
						}
						if seconds > 0 {
							elapsedTimeString += fmt.Sprintf("%d초 ", seconds)
						}

						if len(elapsedTimeString) > 0 {
							m += fmt.Sprintf(" (%s지남)", elapsedTimeString)
						}
					}
				}

				if errorOccurred, ok := notificationSendData.taskCtx.Value(task.TaskCtxKeyErrorOccurred).(bool); ok == true && errorOccurred == true {
					m = fmt.Sprintf("%s\n\n*** 오류가 발생하였습니다. ***", m)
				}

				if len(m) <= telegramMessageMaxLength {
					messageConfig := tgbotapi.NewMessage(n.chatID, m)
					messageConfig.ParseMode = tgbotapi.ModeHTML

					if _, err := n.bot.Send(messageConfig); err != nil {
						log.WithFields(log.Fields{
							"component":   "notification.telegram",
							"notifier_id": n.ID(),
							"error":       err,
						}).Error("알림메시지 발송 실패")
					} else {
						log.WithFields(log.Fields{
							"component":   "notification.telegram",
							"notifier_id": n.ID(),
						}).Info("알림메시지 발송 성공")
					}
				} else {
					// 메시지를 줄 단위로 분할한다.
					lines := strings.Split(m, "\n")

					var messageChunk string
					for _, line := range lines {
						// 보낼 메시지 길이와 새로 추가될 메시지의 길이를 합산하여 최대 길이를 초과하는지 확인한다.
						if len(messageChunk)+len(line)+1 > telegramMessageMaxLength {
							messageConfig := tgbotapi.NewMessage(n.chatID, messageChunk)
							messageConfig.ParseMode = tgbotapi.ModeHTML

							if _, err := n.bot.Send(messageConfig); err != nil {
								log.WithFields(log.Fields{
									"notifier_id": n.ID(),
									"error":       err,
								}).Error("알림메시지 발송 실패")
							}

							messageChunk = line
						} else {
							if len(messageChunk) > 0 {
								messageChunk += "\n"
							}
							messageChunk += line
						}
					}

					if len(messageChunk) > 0 {
						messageConfig := tgbotapi.NewMessage(n.chatID, messageChunk)
						messageConfig.ParseMode = tgbotapi.ModeHTML

						if _, err := n.bot.Send(messageConfig); err != nil {
							log.WithFields(log.Fields{
								"notifier_id": n.ID(),
								"error":       err,
							}).Error("알림메시지 발송 실패")
						}
					}
				}
			}

		case <-notificationStopCtx.Done():
			n.bot.StopReceivingUpdates()

			close(n.notificationSendC)

			n.bot = nil
			n.notificationSendC = nil

			log.WithFields(log.Fields{
				"component":   "notification.telegram",
				"notifier_id": n.ID(),
			}).Debug("Telegram Notifier의 작업이 중지됨")

			return
		}
	}
}
