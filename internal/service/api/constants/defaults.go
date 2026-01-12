package constants

import "time"

// 서버 설정 기본값 상수입니다.
const (
	// DefaultRequestTimeout HTTP 요청 처리의 최대 허용 시간 (60초)
	DefaultRequestTimeout = 60 * time.Second

	// DefaultRateLimitPerSecond IP별 초당 허용 요청 수 (기본값: 20)
	DefaultRateLimitPerSecond = 20

	// DefaultRateLimitBurst IP별 버스트 허용량 (기본값: 40)
	DefaultRateLimitBurst = 40
)
