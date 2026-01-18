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

// params 텔레그램 Notifier 인스턴스를 생성하기 위해 필요한 설정 값들을 담고 있는 구조체입니다.
type params struct {
	BotToken  string
	ChatID    int64
	AppConfig *config.AppConfig
}

// NewCreator 텔레그램 Notifier를 생성하는 팩토리 함수(CreatorFunc)를 반환합니다.
func NewCreator() notifier.CreatorFunc {
	return buildCreator(newNotifier)
}

// constructor 텔레그램 Notifier 생성 로직을 추상화한 함수 타입입니다.
type constructor func(id contract.NotifierID, executor contract.TaskExecutor, p params) (notifier.Notifier, error)

// buildCreator 주입된 생성자 함수(create)를 기반으로 텔레그램 Notifier 팩토리를 생성하여 반환합니다.
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

// newNotifier 텔레그램 봇 API 클라이언트를 초기화하여 Notifier 인스턴스를 생성합니다.
func newNotifier(id contract.NotifierID, executor contract.TaskExecutor, p params) (notifier.Notifier, error) {
	applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
		"notifier_id": id,
		"bot_token":   strutil.Mask(p.BotToken),
		"chat_id":     p.ChatID,
	}).Debug(constants.LogMsgTelegramInitClient)

	// 1. 텔레그램 봇 API 통신을 위한 커스텀 HTTP 클라이언트를 생성합니다.
	// Go의 기본 http.DefaultClient는 타임아웃이 설정되어 있지 않아, 네트워크 장애 발생 시
	// 요청이 무한히 대기하는(Hang) 심각한 리소스 누수(Goroutine Leak)가 발생할 수 있습니다.
	// 이를 방지하기 위해 반드시 명시적인 타임아웃을 설정해야 합니다.
	client := &http.Client{
		Timeout: constants.DefaultTelegramHTTPClientTimeout,
	}

	// 2. 봇 API 클라이언트 인스턴스를 초기화합니다.
	// 앞서 생성한 안전한 HTTP 클라이언트를 주입하여 API와의 모든 통신을 처리합니다.
	botAPI, err := tgbotapi.NewBotAPIWithClient(p.BotToken, tgbotapi.APIEndpoint, client)
	if err != nil {
		// @@@@@
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, "텔레그램 봇 API 클라이언트 초기화에 실패했습니다. 유효한 봇 토큰인지 확인해주세요.")
	}

	// 3. 디버그 모드 설정
	// 앱 설정에 따라 봇 API의 상세 로그 출력 여부를 결정합니다.
	botAPI.Debug = p.AppConfig.Debug

	return newNotifierWithBot(id, &defaultBotClient{BotAPI: botAPI}, executor, p)
}

// newNotifierWithBot 외부에서 주입된 텔레그램 봇 API 클라이언트(botClient)를 사용하여 Notifier 인스턴스를 생성합니다.
func newNotifierWithBot(id contract.NotifierID, botClient botClient, executor contract.TaskExecutor, p params) (notifier.Notifier, error) {
	// @@@@@
	// 1. Notifier 기본 구조체 초기화
	// 재시도 정책, 속도 제한(Rate Limiter), 동시성 제어 등 핵심 기능을 설정합니다.
	notifier := &telegramNotifier{
		Base: notifier.NewBase(id, true, constants.TelegramNotifierBufferSize, constants.DefaultTelegramNotifyTimeout),

		chatID: p.ChatID,

		botClient: botClient,

		executor: executor,

		// 재시도 정책 및 속도 제한 설정 (Telegram API 정책 준수)
		retryDelay: constants.DefaultTelegramRetryDelay,
		limiter:    rate.NewLimiter(rate.Limit(constants.DefaultTelegramRateLimit), constants.DefaultTelegramRateBurst),

		// 명령어 처리 동시성 제한 (Semaphore Pattern)
		// 과도한 요청으로 인한 리소스 고갈을 방지하기 위해 버퍼 채널을 사용합니다.
		concurrencyLimit: make(chan struct{}, constants.TelegramCommandConcurrency),
	}

	// 명령어 중복 검사를 위한 임시 맵 (설정 오류 조기 발견용)
	registeredCommands := make(map[string]botCommand)

	// 2. 봇 명령어 등록 및 검증
	// 설정(AppConfig)에 정의된 작업(Task)들을 순회하며 실행 가능한 명령어를 생성합니다.
	for _, t := range p.AppConfig.Tasks {
		for _, c := range t.Commands {
			// 해당 커맨드가 Notifier 사용이 불가능하게 설정된 경우 건너뜁니다.
			if !c.Notifier.Usable {
				continue
			}

			// 필수 설정 값 검증 (Fail-Fast)
			// 불완전한 설정으로 서버가 실행되는 것을 방지합니다.
			if t.ID == "" || c.ID == "" {
				return nil, apperrors.New(apperrors.InvalidInput, fmt.Sprintf(
					"텔레그램 명령어 생성 실패: TaskID 또는 CommandID는 비어있을 수 없습니다. (Task:'%s', Command:'%s')",
					t.ID, c.ID,
				))
			}

			// 명령어 이름 생성: TaskID와 CommandID를 조합하여 유니크한 명령어 이름을 만듭니다.
			// 예: TaskID="MyTask", CommandID="Run" -> "/my_task_run"
			commandName := fmt.Sprintf("%s_%s", strcase.ToSnake(t.ID), strcase.ToSnake(c.ID))

			// 중복 명령어 충돌 검사
			// 서로 다른 Task가 우연히 같은 명령어 이름을 가지게 되는 경우를 방지합니다.
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
	// 기본 도움말 명령어 추가
	notifier.botCommands = append(notifier.botCommands,
		botCommand{
			name:        botCommandHelp,
			title:       "도움말",
			description: "도움말을 표시합니다.",
		},
	)

	// 3. 빠른 검색을 위한 인덱싱 (Lookup Map 구축)
	// O(n) 선형 탐색 대신 O(1) 해시 맵 검색을 사용하여 성능을 최적화합니다.
	notifier.botCommandsByCommand = make(map[string]botCommand, len(notifier.botCommands))
	// 복합 키 검색 지원: TaskID -> CommandID -> Command
	notifier.botCommandsByTaskAndCommand = make(map[string]map[string]botCommand)

	for _, cmd := range notifier.botCommands {
		// 1) 명령어 이름으로 조회 ("/run_task" -> Command)
		notifier.botCommandsByCommand[cmd.name] = cmd

		// 2) TaskID와 CommandID 조합으로 조회 (내부 로직용)
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
