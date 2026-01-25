package telegram

import (
	"fmt"
	"net/http"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/iancoleman/strcase"
	"golang.org/x/time/rate"
)

// 네트워크 설정
const (
	// defaultHTTPClientTimeout 텔레그램 API 요청의 HTTP 타임아웃 설정입니다.
	// Long Polling 대기 시간(60s)보다 넉넉하게 잡아, 의도치 않은 연결 끊김(Premature Timeout)을 방지합니다.
	defaultHTTPClientTimeout = 70 * time.Second
)

// 버퍼 및 대기열 제어
const (
	// notifierBufferSize 텔레그램 메시지 발송을 위한 내부 버퍼 채널의 크기입니다.
	// 발송 속도(1 TPS)와 종료 대기 시간(60s)을 계산하여, 안정적인 종료를 보장하는 최적값(30)으로 설정했습니다.
	notifierBufferSize = 30

	// defaultEnqueueTimeout 대기열이 가득 찼을 때, 요청을 바로 버리지 않고 기다려주는 최대 시간입니다.
	// 시스템 과부하 상황에서 전체 서비스의 응답성을 보호하기 위한 안전장치입니다.
	defaultEnqueueTimeout = 5 * time.Second
)

// 속도 제한 및 재시도 정책
const (
	// defaultRateLimit 텔레그램 API 호출 속도를 제어하는 초당 허용 요청 수(Rate Limit)입니다.
	// 채팅방당 초당 1회라는 엄격한 정책을 준수하여, 봇이 API 차단을 당하지 않도록 보호합니다.
	defaultRateLimit = 1

	// defaultRateBurst 일시적으로 허용되는 최대 API 요청 버스트(Burst) 크기입니다.
	// 짧은 순간에 몰리는 트래픽을 유연하게 처리하면서도, 평균적인 전송 속도를 일정하게 유지합니다.
	defaultRateBurst = 5

	// defaultRetryDelay 알림 발송 실패 시, 즉시 재시도하지 않고 잠시 대기하는 기본 시간입니다.
	// 일시적인 장애 상황에서 불필요한 API 호출을 줄여 시스템 회복을 돕습니다.
	defaultRetryDelay = 1 * time.Second
)

// 명령어 처리
const (
	// commandExecutionLimit 동시에 실행 가능한 봇 명령어 처리 고루틴의 최대 개수입니다.
	// 리소스 고갈 공격(DoS)을 방지하면서도, 일반적인 사용자 요청은 지연 없이 처리할 수 있는 값입니다.
	commandExecutionLimit = 100
)

// creationArgs 텔레그램 Notifier 인스턴스를 생성하기 위해 필요한 설정 값들을 담고 있는 구조체입니다.
type creationArgs struct {
	BotToken   string
	ChatID     int64
	AppConfig  *config.AppConfig
	HTTPClient *http.Client // 테스트 시, 외부에서 정의한 HTTP 클라이언트(예: Mock Transport)를 주입하여 네트워크 요청을 제어하기 위한 선택적 필드입니다.
}

// NewCreator 텔레그램 Notifier를 생성하는 팩토리 함수(CreatorFunc)를 반환합니다.
func NewCreator() notifier.CreatorFunc {
	return buildCreator(newNotifier)
}

// notifierCtor 텔레그램 Notifier 생성 로직을 추상화한 함수 타입입니다.
type notifierCtor func(id contract.NotifierID, executor contract.TaskExecutor, args creationArgs) (notifier.Notifier, error)

// buildCreator 주입된 생성자 함수(create)를 기반으로 텔레그램 Notifier 팩토리를 생성하여 반환합니다.
func buildCreator(create notifierCtor) notifier.CreatorFunc {
	return func(appConfig *config.AppConfig, executor contract.TaskExecutor) ([]notifier.Notifier, error) {
		var notifiers []notifier.Notifier

		for _, telegram := range appConfig.Notifier.Telegrams {
			args := creationArgs{
				BotToken:  telegram.BotToken,
				ChatID:    telegram.ChatID,
				AppConfig: appConfig,
			}
			n, err := create(contract.NotifierID(telegram.ID), executor, args)
			if err != nil {
				return nil, err
			}
			notifiers = append(notifiers, n)
		}

		return notifiers, nil
	}
}

// newNotifier 텔레그램 봇 API 클라이언트를 초기화하여 Notifier 인스턴스를 생성합니다.
func newNotifier(id contract.NotifierID, executor contract.TaskExecutor, args creationArgs) (notifier.Notifier, error) {
	applog.WithComponentAndFields(component, applog.Fields{
		"notifier_id": id,
		"bot_token":   strutil.Mask(args.BotToken),
		"chat_id":     args.ChatID,
	}).Debug("텔레그램 Notifier 초기화: 봇 API 클라이언트 생성을 시작합니다")

	// 1. 텔레그램 봇 API 통신을 위한 HTTP 클라이언트를 설정합니다.
	// 테스트 등의 경우 외부에서 주입된 HTTP 클라이언트가 있다면 우선 사용합니다.
	httpClient := args.HTTPClient
	if httpClient == nil {
		// Go의 기본 http.DefaultClient는 타임아웃이 설정되어 있지 않아, 네트워크 장애 발생 시
		// 요청이 무한히 대기하는(Hang) 심각한 리소스 누수(Goroutine Leak)가 발생할 수 있습니다.
		// 이를 방지하기 위해 반드시 명시적인 타임아웃을 설정해야 합니다.
		httpClient = &http.Client{
			Timeout: defaultHTTPClientTimeout,
		}
	}

	// 2. 봇 API 클라이언트 인스턴스를 초기화합니다.
	// 앞서 생성한 안전한 HTTP 클라이언트를 주입하여 API와의 모든 통신을 처리합니다.
	botAPI, err := tgbotapi.NewBotAPIWithClient(args.BotToken, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		return nil, NewErrInvalidBotToken(err)
	}

	// 3. 디버그 모드 설정
	// 앱 설정에 따라 봇 API의 상세 로그 출력 여부를 결정합니다.
	botAPI.Debug = args.AppConfig.Debug

	return newNotifierWithClient(id, &tgClient{BotAPI: botAPI}, executor, args)
}

// newNotifierWithClient 외부에서 주입된 텔레그램 봇 API 클라이언트(client)를 사용하여 Notifier 인스턴스를 생성합니다.
func newNotifierWithClient(id contract.NotifierID, client client, executor contract.TaskExecutor, args creationArgs) (notifier.Notifier, error) {
	// 1. Notifier 기본 구조체 초기화
	// 재시도 정책, 속도 제한(Rate Limiter), 동시성 제어 등 핵심 기능을 설정합니다.
	notifier := &telegramNotifier{
		Base: notifier.NewBase(id, true, notifierBufferSize, defaultEnqueueTimeout),

		chatID: args.ChatID,

		client: client,

		executor: executor,

		// 재시도 정책(Retry Policy): API 호출 실패 시 즉시 재시도하지 않고 일정 시간 대기합니다.
		// 이를 통해 일시적인 네트워크 장애나 서버 부하 상황에서 불필요한 요청 폭주를 막습니다.
		retryDelay: defaultRetryDelay,

		// 속도 제한(Rate Limiting): 텔레그램 API 정책을 준수하기 위해 발송 속도를 제어합니다.
		//   * Rate: 초당 허용 요청 수
		//   * Burst: 순간 최대 허용 요청 수 (짧은 시간 내 연속 요청 허용)
		rateLimiter: rate.NewLimiter(rate.Limit(defaultRateLimit), defaultRateBurst),

		// 명령어 처리 동시성 제한
		// 과도한 요청으로 인한 리소스 고갈을 방지하기 위해 버퍼 채널을 사용합니다.
		commandSemaphore: make(chan struct{}, commandExecutionLimit),
	}

	// 2. 봇 명령어 등록 및 검증

	// 봇 명령어 중복 검사를 위한 임시 맵
	registeredCommands := make(map[string]botCommand)

	for _, t := range args.AppConfig.Tasks {
		for _, c := range t.Commands {
			// 해당 명령이 알림 사용이 불가능하게 설정된 경우 건너뜁니다.
			if !c.Notifier.Usable {
				continue
			}

			// 필수 설정 값 검증
			if t.ID == "" || c.ID == "" {
				return nil, NewErrInvalidCommandIDs(t.ID, c.ID)
			}

			// 명령어 이름 생성: TaskID와 CommandID를 조합하여 유니크한 명령어 이름을 만듭니다.
			// 예: TaskID="MyTask", CommandID="Run" -> "/my_task_run"
			commandName := fmt.Sprintf("%s_%s", strcase.ToSnake(t.ID), strcase.ToSnake(c.ID))

			// 중복 명령어 충돌 검사: 서로 다른 Task가 우연히 같은 명령어 이름을 가지게 되는 경우를 방지합니다.
			if existing, exists := registeredCommands[commandName]; exists {
				return nil, NewErrDuplicateCommandName(commandName, existing.taskID.String(), existing.commandID.String(), t.ID, c.ID)
			}

			newCommand := botCommand{
				name:        commandName,
				title:       fmt.Sprintf("%s > %s", t.Title, c.Title), // 제목: 작업명 > 커맨드명
				description: c.Description,

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

	// 3. 빠른 검색을 위한 인덱싱
	notifier.botCommandsByName = make(map[string]botCommand, len(notifier.botCommands))
	notifier.botCommandsByTask = make(map[contract.TaskID]map[contract.TaskCommandID]botCommand) // 복합 키 검색 지원: TaskID -> CommandID -> Command

	for _, command := range notifier.botCommands {
		// 1) 명령어 이름으로 조회
		notifier.botCommandsByName[command.name] = command

		// 2) TaskID와 CommandID 조합으로 조회
		if !command.taskID.IsEmpty() && !command.commandID.IsEmpty() {
			tID := command.taskID
			cID := command.commandID

			if _, exists := notifier.botCommandsByTask[tID]; !exists {
				notifier.botCommandsByTask[tID] = make(map[contract.TaskCommandID]botCommand)
			}
			notifier.botCommandsByTask[tID][cID] = command
		}
	}

	return notifier, nil
}
