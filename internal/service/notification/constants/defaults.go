package constants

import "time"

// TODO 미완료
const (
	// TelegramShutdownTimeout 텔레그램 봇 종료 시 잔여 메시지 처리를 위해 대기하는 최대 시간입니다.
	// 이 시간이 지나면 남은 메시지는 처리되지 않고 버려질 수 있습니다.
	// TelegramNotifierBufferSize * (1 / DefaultRateLimit) 보다 충분히 커야 합니다.
	TelegramShutdownTimeout = 60 * time.Second

	// TelegramCommandTimeout 텔레그램 봇 명령어 처리 시 최대 허용 시간입니다.
	// 외부 API 호출 지연 등으로 인한 고루틴 무한 대기(Leak)를 방지하기 위해 사용됩니다.
	TelegramCommandTimeout = 10 * time.Second

	// TelegramSendTimeout 텔레그램 API로 메시지를 실제 전송할 때 사용하는 타임아웃입니다.
	// Rate Limit 대기 시간 등을 고려하여 충분히 길게(30s) 설정합니다.
	// 기존 DefaultTelegramEnqueueTimeout(5s)은 큐잉 대기 시간으로만 사용합니다.
	TelegramSendTimeout = 30 * time.Second
)
