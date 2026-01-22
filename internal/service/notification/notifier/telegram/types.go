package telegram

import (
	"time"

	"golang.org/x/time/rate"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// component Notification 서비스의 텔레그램 Notifier 로깅용 컴포넌트 이름
const component = "notification.notifier.telegram"

const (
	// messageMaxLength 텔레그램 메시지 전송 시 허용되는 최대 문자 길이입니다.
	//
	// 텔레그램 Bot API 공식 제한은 4096자이지만, HTML 태그 및 메타데이터 오버헤드를 고려하여
	// 안전 마진을 두고 3900자로 설정했습니다. 이를 초과하는 메시지는 자동으로 분할 전송됩니다.
	messageMaxLength = 3900

	// shutdownTimeout 텔레그램 Notifier 종료 시 큐에 남은 메시지를 처리하기 위해 대기하는 최대 시간입니다.
	//
	// 이 시간 동안 Drain 로직이 실행되어 버퍼에 쌓인 미전송 메시지를 최대한 처리합니다.
	// 타임아웃이 경과하면 남은 메시지는 손실될 수 있으므로, 버퍼 크기와 전송 속도를 고려하여
	// 충분히 여유있게 설정해야 합니다 (권장: 버퍼크기 / 초당전송속도 * 2 이상).
	shutdownTimeout = 60 * time.Second
)

// client 텔레그램 봇 API와의 통신을 추상화한 인터페이스입니다.
type client interface {
	// 봇 정보 조회
	GetSelf() tgbotapi.User

	// 메시지 송수신
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)

	// 리소스 정리
	StopReceivingUpdates()
}

// tgClient tgbotapi.BotAPI를 래핑하여 client 인터페이스를 구현하는 구조체입니다.
//
// 이 구조체는 임베딩(Embedding)을 통해 tgbotapi.BotAPI의 모든 메서드를 상속받으며,
// client 인터페이스에 정의되지 않은 추가 메서드(예: GetSelf)를 구현합니다.
type tgClient struct {
	*tgbotapi.BotAPI
}

// GetSelf 현재 봇의 사용자 정보를 반환합니다.
func (c *tgClient) GetSelf() tgbotapi.User {
	return c.Self
}

// telegramNotifier 텔레그램을 통한 알림 발송 및 봇 명령어 처리를 담당하는 Notifier 구현체입니다.
type telegramNotifier struct {
	notifier.Base

	// === 메시지 전송 관련 ===

	// chatID 메시지를 전송할 텔레그램 채팅방의 고유 식별자입니다.
	chatID int64

	// client 텔레그램 봇 API와의 통신을 담당하는 클라이언트입니다.
	client client

	// retryDelay API 호출 실패 시 재시도 전에 대기하는 시간입니다.
	// 일시적인 네트워크 장애나 서버 부하 상황에서 즉시 재시도하지 않고 백오프(Backoff)를 적용합니다.
	retryDelay time.Duration

	// rateLimiter 텔레그램 API 호출 속도를 제어하는 Rate Limiter입니다.
	// API 정책(채팅방당 초당 1회)을 준수하여 봇이 차단되는 것을 방지합니다.
	rateLimiter *rate.Limiter

	// === 명령어 처리 관련 ===

	// executor 봇 명령어 실행 시 작업(Task)을 제출하고 취소하는 역할을 담당합니다.
	executor contract.TaskExecutor

	// commandSemaphore 봇 명령어를 처리하는 고루틴의 동시 실행 수를 제한하는 세마포어입니다.
	// 과도한 명령어 요청으로 인한 리소스 고갈(Goroutine Leak)을 방지합니다.
	commandSemaphore chan struct{}

	// botCommands 등록된 모든 봇 명령어 목록입니다.
	// 텔레그램 봇 메뉴 설정 및 도움말 표시에 사용됩니다.
	botCommands []botCommand

	// botCommandsByName 명령어 이름(문자열)을 키로 하여 명령어를 빠르게 조회하는 인덱스입니다.
	botCommandsByName map[string]botCommand

	// botCommandsByTask TaskID와 CommandID의 조합으로 명령어를 조회하는 2단계 인덱스입니다.
	// 작업 취소 시 특정 Task의 특정 Command를 찾기 위해 사용되며, 키 충돌을 방지합니다.
	botCommandsByTask map[contract.TaskID]map[contract.TaskCommandID]botCommand
}

// 인터페이스 준수 확인
var _ notifier.Notifier = (*telegramNotifier)(nil)
