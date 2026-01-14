package constants

import "time"

// 서버 설정 기본값 상수입니다.
const (
	// ------------------------------------------------------------------------------------------------
	// HTTP 연결 타임아웃 설정 (시간 제한)
	// ------------------------------------------------------------------------------------------------

	// DefaultReadTimeout 요청 본문 읽기 최대 시간 (30초)
	DefaultReadTimeout = 30 * time.Second

	// DefaultWriteTimeout 응답 쓰기 최대 시간 (65초)
	DefaultWriteTimeout = 65 * time.Second

	// DefaultIdleTimeout Keep-Alive 연결 유휴 최대 시간 (120초)
	DefaultIdleTimeout = 120 * time.Second

	// ------------------------------------------------------------------------------------------------
	// 트래픽 제한 정책 (Rate Limiting)
	// ------------------------------------------------------------------------------------------------

	// DefaultRateLimitPerSecond IP별 초당 허용 요청 수 (기본값: 20)
	DefaultRateLimitPerSecond = 20

	// DefaultRateLimitBurst IP별 버스트 허용량 (기본값: 40)
	DefaultRateLimitBurst = 40
)
