package constants

import "time"

// TODO 미완료
const (
	// DefaultRetryDelay 알림 발송 실패 시 재시도 대기 시간의 기본값입니다.
	DefaultRetryDelay = 1 * time.Second

	// DefaultRateLimit 텔레그램 API Rate Limit 기본값 (초당 허용 요청 수)
	// 공식 문서는 채팅방당 초당 1회, 전역 초당 30회를 권장합니다.
	DefaultRateLimit = 1

	// DefaultRateBurst 텔레그램 API Rate Limit 버스트 기본값 (순간 최대 허용 요청 수)
	DefaultRateBurst = 5

	// DefaultHTTPClientTimeout 텔레그램 API 클라이언트의 HTTP 요청 타임아웃 기본값
	// Long Polling 타임아웃(60s)보다 넉넉하게 설정하여, 클라이언트가 먼저 연결을 끊는 문제를 방지합니다.
	DefaultHTTPClientTimeout = 70 * time.Second

	// DefaultNotifyTimeout 알림 발송 요청 시 대기열이 가득 찼을 때 대기하는 최대 시간 (Backpressure)
	DefaultNotifyTimeout = 5 * time.Second

	// TelegramNotifierBufferSize 텔레그램 Notifier의 내부 버퍼 크기
	// Rate Limit(1 TPS)와 Shutdown Timeout(60s)을 고려하여, 종료 시 유실 없이 모두 처리할 수 있는 크기(30)로 설정합니다.
	TelegramNotifierBufferSize = 30

	// TelegramShutdownTimeout 텔레그램 봇 종료 시 잔여 메시지 처리를 위해 대기하는 최대 시간입니다.
	// 이 시간이 지나면 남은 메시지는 처리되지 않고 버려질 수 있습니다.
	// TelegramNotifierBufferSize * (1 / DefaultRateLimit) 보다 충분히 커야 합니다.
	TelegramShutdownTimeout = 60 * time.Second

	// TelegramCommandConcurrency 텔레그램 봇 명령어의 최대 동시 처리 개수입니다.
	// 너무 높으면 시스템 리소스에 부담을 줄 수 있고, 너무 낮으면 사용자 반응이 지연될 수 있습니다.
	TelegramCommandConcurrency = 100

	// TelegramCommandTimeout 텔레그램 봇 명령어 처리 시 최대 허용 시간입니다.
	// 외부 API 호출 지연 등으로 인한 고루틴 무한 대기(Leak)를 방지하기 위해 사용됩니다.
	TelegramCommandTimeout = 10 * time.Second

	// TelegramSendTimeout 텔레그램 API로 메시지를 실제 전송할 때 사용하는 타임아웃입니다.
	// Rate Limit 대기 시간 등을 고려하여 충분히 길게(30s) 설정합니다.
	// 기존 DefaultNotifyTimeout(5s)은 큐잉 대기 시간으로만 사용합니다.
	TelegramSendTimeout = 30 * time.Second
)
