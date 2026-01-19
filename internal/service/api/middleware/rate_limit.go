package middleware

import (
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

const (
	// maxIPRateLimiters 추적 가능한 최대 IP 개수 (메모리 보호 및 DoS 방지)
	maxIPRateLimiters = 10000
)

// ipRateLimiter IP 주소별 Rate Limiter를 관리하는 구조체입니다.
//
// Token Bucket 알고리즘을 사용하여 IP별로 독립적인 요청 제한을 적용합니다.
//
// 동시성 안전성:
//   - sync.RWMutex로 여러 고루틴에서 안전하게 접근 가능
//   - 읽기 작업은 RLock, 쓰기 작업은 Lock으로 최적화
//
// 메모리 관리:
//   - 최대 10,000개 IP 추적 (maxIPRateLimiters)
//   - 제한 초과 시 맵에서 랜덤하게 하나 제거 (Go Map 순회 특성 활용)
type ipRateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	rate     rate.Limit // 초당 허용 요청 수
	burst    int        // 버스트 허용량
}

// newIPRateLimiter 새로운 IP 기반 Rate Limiter를 생성합니다.
//
// Parameters:
//   - requestsPerSecond: 초당 허용 요청 수 (예: 20)
//   - burst: 버스트 허용량 (예: 40)
func newIPRateLimiter(requestsPerSecond int, burst int) *ipRateLimiter {
	return &ipRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
	}
}

// getLimiter 특정 IP의 Rate Limiter를 반환합니다. 없으면 새로 생성합니다.
//
// 동시성 안전하며, Double-Checked Locking 패턴을 사용하여 성능을 최적화합니다.
func (i *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	// 1. 읽기 락으로 먼저 확인 (성능 최적화)
	i.mu.RLock()
	limiter, exists := i.limiters[ip]
	i.mu.RUnlock()

	if exists {
		return limiter
	}

	// 2. 쓰기 락으로 생성
	i.mu.Lock()
	defer i.mu.Unlock()

	// Double-check: 다른 고루틴이 이미 생성했을 수 있음
	limiter, exists = i.limiters[ip]
	if exists {
		return limiter
	}

	// 3. 메모리 보호: 최대 개수 초과 시 하나 제거
	if len(i.limiters) >= maxIPRateLimiters {
		// Go Map 순회는 랜덤이므로 간이 LRU 효과
		for oldIP := range i.limiters {
			delete(i.limiters, oldIP)
			break
		}
	}

	// 4. 새 Limiter 생성 및 저장
	limiter = rate.NewLimiter(i.rate, i.burst)
	i.limiters[ip] = limiter

	return limiter
}

// RateLimit IP 기반 Rate Limiting 미들웨어를 반환합니다.
//
// Token Bucket 알고리즘을 사용하여 IP별로 요청 속도를 제한합니다.
// 제한 초과 시 HTTP 429 (Too Many Requests)를 반환하고 Retry-After 헤더를 포함합니다.
//
// Parameters:
//   - requestsPerSecond: 초당 허용 요청 수 (양수, 예: 20)
//   - burst: 버스트 허용량 (양수, 예: 40)
//
// Token Bucket 알고리즘:
//   - Rate: 초당 토큰 생성 속도 (requestsPerSecond)
//   - Burst: 버킷 크기 (burst), 최대 저장 가능한 토큰 수
//   - 요청마다 토큰 1개 소비, 부족 시 요청 거부
//
// 사용 예시:
//
//	e := echo.New()
//	e.Use(middleware.RateLimit(20, 40)) // 초당 20 요청, 버스트 40
//
// 주의사항:
//   - 메모리 기반 저장소 (서버 재시작 시 초기화)
//   - 다중 서버 환경에서는 서버별로 독립적인 제한 적용
//
// Panics:
//   - requestsPerSecond 또는 burst가 0 이하인 경우
func RateLimit(requestsPerSecond int, burst int) echo.MiddlewareFunc {
	if requestsPerSecond <= 0 {
		panic(fmt.Sprintf(constants.PanicMsgRateLimitRequestsPerSecondInvalid, requestsPerSecond))
	}
	if burst <= 0 {
		panic(fmt.Sprintf(constants.PanicMsgRateLimitBurstInvalid, burst))
	}

	limiter := newIPRateLimiter(requestsPerSecond, burst)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 1. 클라이언트 IP 추출
			ip := c.RealIP()

			// 2. IP별 Limiter 가져오기
			ipLimiter := limiter.getLimiter(ip)

			// 3. Rate Limit 확인
			if !ipLimiter.Allow() {
				// 제한 초과 로깅
				applog.WithComponentAndFields(constants.MiddlewareRateLimit, applog.Fields{
					"remote_ip": ip,
					"path":      c.Request().URL.Path,
					"method":    c.Request().Method,
				}).Warn(constants.LogMsgRateLimitExceeded)

				// Retry-After 헤더 설정 (1초 후 재시도 권장)
				c.Response().Header().Set(constants.RetryAfter, constants.RetryAfterSeconds)

				// HTTP 429 응답
				return httputil.NewTooManyRequestsError(constants.ErrMsgTooManyRequests)
			}

			// 4. 다음 핸들러 실행
			return next(c)
		}
	}
}
