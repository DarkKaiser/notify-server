package telegram

import (
	"fmt"
	"net/http"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/constants"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/iancoleman/strcase"
	"golang.org/x/time/rate"
)

// TODO 미완료

// params 텔레그램 Notifier 생성에 필요한 설정 정보
type params struct {
	BotToken  string
	ChatID    int64
	AppConfig *config.AppConfig
}

// NewCreator 텔레그램 Notifier 설정을 처리하는 CreatorFunc를 생성하여 반환합니다.
func NewCreator() notifier.CreatorFunc {
	return buildCreator(newNotifier)
}

// constructor는 의존성 주입을 위해 Notifier 생성을 추상화한 함수 타입입니다.
type constructor func(id contract.NotifierID, executor contract.TaskExecutor, p params) (notifier.Notifier, error)

// buildCreator 텔레그램 Notifier 설정을 처리하는 CreatorFunc를 생성하여 반환합니다.
// 의존성 주입을 위해 생성자 함수를 인자로 받습니다.
func buildCreator(create constructor) notifier.CreatorFunc {
	return func(appConfig *config.AppConfig, executor contract.TaskExecutor) ([]notifier.Notifier, error) {
		var notifiers []notifier.Notifier

		for _, telegram := range appConfig.Notifier.Telegrams {
			p := params{
				BotToken:  telegram.BotToken,
				ChatID:    telegram.ChatID,
				AppConfig: appConfig,
			}
			n, err := create(contract.NotifierID(telegram.ID), executor, p)
			if err != nil {
				return nil, err
			}
			notifiers = append(notifiers, n)
		}

		return notifiers, nil
	}
}

// newNotifier 실제 텔레그램 봇 API를 이용하여 Notifier 인스턴스를 생성합니다.
func newNotifier(id contract.NotifierID, executor contract.TaskExecutor, p params) (notifier.Notifier, error) {
	applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
		"notifier_id": id,
		"bot_token":   strutil.Mask(p.BotToken),
		"chat_id":     p.ChatID,
	}).Debug("텔레그램 봇 초기화 시도")

	// 텔레그램 봇 API 클라이언트 초기화 (Timeout 설정 포함)
	// 기본 http.Client는 Timeout이 없어 네트워크 지연 시 고루틴이 무한 대기할 수 있습니다.
	client := &http.Client{
		Timeout: constants.DefaultHTTPClientTimeout,
	}

	botAPI, err := tgbotapi.NewBotAPIWithClient(p.BotToken, tgbotapi.APIEndpoint, client)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, "텔레그램 봇 초기화 실패 (토큰을 확인해주세요)")
	}
	botAPI.Debug = p.AppConfig.Debug

	return newWithBot(id, &defaultBotClient{BotAPI: botAPI}, executor, p)
}

// newWithBot botClient 구현체를 이용하여 Notifier 인스턴스를 생성합니다.
func newWithBot(id contract.NotifierID, botClient botClient, executor contract.TaskExecutor, p params) (notifier.Notifier, error) {
	notifier := &telegramNotifier{
		Base: notifier.NewBase(id, true, constants.TelegramNotifierBufferSize, constants.DefaultNotifyTimeout),

		chatID: p.ChatID,

		botClient: botClient,

		retryDelay: constants.DefaultRetryDelay,
		limiter:    rate.NewLimiter(rate.Limit(constants.DefaultRateLimit), constants.DefaultRateBurst),

		executor: executor,

		// 최대 100개의 동시 명령어를 처리할 수 있도록 설정
		concurrencyLimit: make(chan struct{}, constants.TelegramCommandConcurrency),
	}

	// 명령어 중복 검사를 위한 임시 맵
	registeredCommands := make(map[string]botCommand)

	// 봇 명령어 목록을 초기화합니다.
	for _, t := range p.AppConfig.Tasks {
		for _, c := range t.Commands {
			// 해당 커맨드가 Notifier 사용이 불가능하게 설정된 경우 건너뜁니다.
			if !c.Notifier.Usable {
				continue
			}

			// TaskID나 CommandID가 비어있으면 유효하지 않은 명령어가 생성되므로 에러 처리
			if t.ID == "" || c.ID == "" {
				return nil, apperrors.New(apperrors.InvalidInput, fmt.Sprintf(
					"텔레그램 명령어 생성 실패: TaskID 또는 CommandID는 비어있을 수 없습니다. (Task:'%s', Command:'%s')",
					t.ID, c.ID,
				))
			}

			// 명령어 문자열 생성: taskID와 commandID를 SnakeCase로 변환하여 조합 (예: myTask, run -> my_task_run)
			commandName := fmt.Sprintf("%s_%s", strcase.ToSnake(t.ID), strcase.ToSnake(c.ID))

			// 중복 명령어 검사 (Fail-Fast)
			if existing, exists := registeredCommands[commandName]; exists {
				return nil, apperrors.New(apperrors.InvalidInput, fmt.Sprintf(
					"텔레그램 명령어 충돌이 감지되었습니다. 명령어: '/%s' (Task:'%s', Command:'%s')가 이미 등록된 (Task:'%s', Command:'%s')와 충돌합니다. TaskID 또는 CommandID를 변경해주세요.",
					commandName, t.ID, c.ID, existing.taskID, existing.commandID,
				))
			}

			newCommand := botCommand{
				name:        commandName,
				title:       fmt.Sprintf("%s > %s", t.Title, c.Title), // 제목: 작업명 > 커맨드명
				description: c.Description,                            // 설명: 커맨드 설명

				taskID:    contract.TaskID(t.ID),
				commandID: contract.TaskCommandID(c.ID),
			}

			notifier.botCommands = append(notifier.botCommands, newCommand)
			registeredCommands[commandName] = newCommand
		}
	}
	notifier.botCommands = append(notifier.botCommands,
		botCommand{
			name:        botCommandHelp,
			title:       "도움말",
			description: "도움말을 표시합니다.",
		},
	)

	// botCommands 슬라이스를 기반으로 빠른 조회를 위한 Map 초기화
	notifier.botCommandsByCommand = make(map[string]botCommand, len(notifier.botCommands))
	// botCommandsByTaskAndCommand "taskID" -> "commandID" -> command 구조로 조회 (키 충돌 방지)
	notifier.botCommandsByTaskAndCommand = make(map[string]map[string]botCommand)

	for _, cmd := range notifier.botCommands {
		// command 문자열로 조회 가능하도록 Map에 추가
		notifier.botCommandsByCommand[cmd.name] = cmd

		// taskID와 commandID가 있는 경우에만 "taskID" -> "commandID" 구조로 Map에 추가
		if !cmd.taskID.IsEmpty() && !cmd.commandID.IsEmpty() {
			tID := string(cmd.taskID)
			cID := string(cmd.commandID)

			if _, exists := notifier.botCommandsByTaskAndCommand[tID]; !exists {
				notifier.botCommandsByTaskAndCommand[tID] = make(map[string]botCommand)
			}
			notifier.botCommandsByTaskAndCommand[tID][cID] = cmd
		}
	}

	return notifier, nil
}
