package telegram

import (
	"fmt"
	"net/http"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/notification/constants"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/notification/types"
	"github.com/darkkaiser/notify-server/internal/service/task"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/iancoleman/strcase"
	"golang.org/x/time/rate"
)

type telegramNotifierCreatorFunc func(id types.NotifierID, botToken string, chatID int64, appConfig *config.AppConfig, executor task.Executor) (notifier.NotifierHandler, error)

// NewConfigProcessor 텔레그램 Notifier 설정을 처리하는 NotifierConfigProcessor를 생성하여 반환합니다.
// 이 처리기는 애플리케이션 설정에 따라 텔레그램 Notifier 인스턴스들을 초기화합니다.
func NewConfigProcessor(creator telegramNotifierCreatorFunc) notifier.NotifierConfigProcessor {
	return func(appConfig *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
		var handlers []notifier.NotifierHandler

		for _, telegram := range appConfig.Notifier.Telegrams {
			h, err := creator(types.NotifierID(telegram.ID), telegram.BotToken, telegram.ChatID, appConfig, executor)
			if err != nil {
				return nil, err
			}
			handlers = append(handlers, h)
		}

		return handlers, nil
	}
}

// NewNotifier 실제 텔레그램 봇 API를 이용하여 Notifier 인스턴스를 생성합니다.
func NewNotifier(id types.NotifierID, botToken string, chatID int64, appConfig *config.AppConfig, executor task.Executor) (notifier.NotifierHandler, error) {
	applog.WithComponentAndFields("notification.telegram", applog.Fields{
		"notifier_id": id,
		"bot_token":   strutil.Mask(botToken),
		"chat_id":     chatID,
	}).Debug("텔레그램 봇 초기화 시도")

	// 텔레그램 봇 API 클라이언트 초기화 (Timeout 설정 포함)
	// 기본 http.Client는 Timeout이 없어 네트워크 지연 시 고루틴이 무한 대기할 수 있습니다.
	client := &http.Client{
		Timeout: constants.DefaultHTTPClientTimeout,
	}

	botAPI, err := tgbotapi.NewBotAPIWithClient(botToken, tgbotapi.APIEndpoint, client)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, "텔레그램 봇 초기화 실패 (토큰을 확인해주세요)")
	}
	botAPI.Debug = appConfig.Debug

	return newTelegramNotifierWithBot(id, &telegramBotAPIClient{BotAPI: botAPI}, chatID, appConfig, executor), nil
}

// newTelegramNotifierWithBot telegramBotAPI 구현체를 이용하여 Notifier 인스턴스를 생성합니다.
func newTelegramNotifierWithBot(id types.NotifierID, botAPI telegramBotAPI, chatID int64, appConfig *config.AppConfig, executor task.Executor) notifier.NotifierHandler {
	notifier := &telegramNotifier{
		BaseNotifier: notifier.NewBaseNotifier(id, true, constants.TelegramNotifierBufferSize, constants.DefaultNotifyTimeout),

		chatID: chatID,

		botAPI: botAPI,

		retryDelay: constants.DefaultRetryDelay,
		limiter:    rate.NewLimiter(rate.Limit(constants.DefaultRateLimit), constants.DefaultRateBurst),

		executor: executor,

		// 최대 100개의 동시 명령어를 처리할 수 있도록 설정
		handlerSemaphore: make(chan struct{}, 100),
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
					command:            fmt.Sprintf("%s_%s", strcase.ToSnake(t.ID), strcase.ToSnake(c.ID)),
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

	// botCommands 슬라이스를 기반으로 빠른 조회를 위한 Map 초기화
	notifier.botCommandsByCommand = make(map[string]telegramBotCommand, len(notifier.botCommands))
	notifier.botCommandsByTaskAndCommand = make(map[string]telegramBotCommand, len(notifier.botCommands))

	for _, botCommand := range notifier.botCommands {
		// command 문자열로 조회 가능하도록 Map에 추가
		notifier.botCommandsByCommand[botCommand.command] = botCommand

		// taskID와 commandID가 있는 경우에만 "taskID_commandID" 키로 Map에 추가
		if !botCommand.taskID.IsEmpty() && !botCommand.commandID.IsEmpty() {
			key := fmt.Sprintf("%s_%s", botCommand.taskID, botCommand.commandID)
			notifier.botCommandsByTaskAndCommand[key] = botCommand
		}
	}

	return notifier
}
