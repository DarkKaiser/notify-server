package constants

import "time"

const (
	// DefaultRetryDelay 알림 발송 실패 시 재시도 대기 시간의 기본값입니다.
	DefaultRetryDelay = 1 * time.Second

	// DefaultRateLimit 텔레그램 API Rate Limit 기본값 (초당 허용 요청 수)
	// 공식 문서는 채팅방당 초당 1회, 전역 초당 30회를 권장합니다.
	DefaultRateLimit = 1

	// DefaultRateBurst 텔레그램 API Rate Limit 버스트 기본값 (순간 최대 허용 요청 수)
	DefaultRateBurst = 5

	// DefaultHTTPClientTimeout 텔레그램 API 클라이언트의 HTTP 요청 타임아웃 기본값
	DefaultHTTPClientTimeout = 30 * time.Second

	// DefaultNotifyTimeout 알림 발송 요청 시 대기열이 가득 찼을 때 대기하는 최대 시간 (Backpressure)
	DefaultNotifyTimeout = 5 * time.Second

	// TelegramNotifierBufferSize 텔레그램 Notifier의 내부 버퍼 크기
	// Rate Limit(1 TPS)와 Shutdown Timeout(60s)을 고려하여, 종료 시 유실 없이 모두 처리할 수 있는 크기(50)로 설정합니다.
	TelegramNotifierBufferSize = 50
)
