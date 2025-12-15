package notification

import (
	"fmt"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
)

type telegramNotifierCreatorFunc func(id NotifierID, botToken string, chatID int64, appConfig *config.AppConfig, executor task.Executor) (NotifierHandler, error)

// NewTelegramConfigProcessor 텔레그램 Notifier 설정을 처리하는 NotifierConfigProcessor를 생성하여 반환합니다.
// 이 처리기는 애플리케이션 설정에 따라 텔레그램 Notifier 인스턴스들을 초기화합니다.
func NewTelegramConfigProcessor(creator telegramNotifierCreatorFunc) NotifierConfigProcessor {
	return func(appConfig *config.AppConfig, executor task.Executor) ([]NotifierHandler, error) {
		var handlers []NotifierHandler

		for _, telegram := range appConfig.Notifiers.Telegrams {
			h, err := creator(NotifierID(telegram.ID), telegram.BotToken, telegram.ChatID, appConfig, executor)
			if err != nil {
				return nil, err
			}
			handlers = append(handlers, h)
		}

		return handlers, nil
	}
}

// newTelegramNotifier 실제 텔레그램 봇 API를 이용하여 Notifier 인스턴스를 생성합니다.
func newTelegramNotifier(id NotifierID, botToken string, chatID int64, appConfig *config.AppConfig, executor task.Executor) (NotifierHandler, error) {
	applog.WithComponentAndFields("notification.telegram", log.Fields{
		"bot_token": strutil.MaskSensitiveData(botToken),
	}).Debug("텔레그램 봇 초기화 시도")

	botAPI, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "텔레그램 봇 초기화 실패 (토큰을 확인해주세요)")
	}
	botAPI.Debug = appConfig.Debug

	return newTelegramNotifierWithBot(id, &telegramBotAPIClient{BotAPI: botAPI}, chatID, appConfig, executor), nil
}

// newTelegramNotifierWithBot telegramBotAPI 구현체를 이용하여 Notifier 인스턴스를 생성합니다.
func newTelegramNotifierWithBot(id NotifierID, botAPI telegramBotAPI, chatID int64, appConfig *config.AppConfig, executor task.Executor) NotifierHandler {
	notifier := &telegramNotifier{
		notifier: NewNotifier(id, true, 100),

		chatID: chatID,

		botAPI: botAPI,

		executor: executor,
	}

	// 봇 명령어 목록을 초기화합니다.
	for _, t := range appConfig.Tasks {
		for _, c := range t.Commands {
			// 해당 커맨드가 Notifier 사용이 불가능하게 설정된 경우 건너뜁니다.
			if !c.Notifier.Usable {
				continue
			}

			// 명령어 문자열 생성: taskID와 commandID를 SnakeCase로 변환하여 조합 (예: myTask, run -> my_task_run)
			notifier.botCommands = append(notifier.botCommands,
				telegramBotCommand{
					command:            fmt.Sprintf("%s_%s", strutil.ToSnakeCase(t.ID), strutil.ToSnakeCase(c.ID)),
					commandTitle:       fmt.Sprintf("%s > %s", t.Title, c.Title), // 제목: 작업명 > 커맨드명
					commandDescription: c.Description,                            // 설명: 커맨드 설명

					taskID:    task.ID(t.ID),
					commandID: task.CommandID(c.ID),
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
