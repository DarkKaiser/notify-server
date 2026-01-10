package middleware

import (
	"sync"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// ipRateLimiter IP 주소별로 Rate Limiter를 관리하는 구조체입니다.
//
// 이 구조체는 다음과 같은 역할을 수행합니다:
//   - IP 주소별로 독립적인 rate.Limiter 인스턴스 관리
//   - 동시성 안전한 Limiter 접근 (sync.RWMutex 사용)
//   - Token Bucket 알고리즘 기반 Rate Limiting
//
// 동시성 안전성:
//   - sync.RWMutex를 사용하여 여러 고루틴에서 안전하게 접근 가능
//   - 읽기 작업(getLimiter)은 RLock으로 최적화
//   - 쓰기 작업(새 Limiter 생성)은 Lock으로 보호
//
// 메모리 관리:
//   - IP 주소는 한 번 추가되면 서버 재시작 전까지 메모리에 유지됨
//   - 현재 프로젝트 규모에서는 문제없으나, 대규모 트래픽 환경에서는 다음 개선 고려:
//   - 최대 IP 개수 제한 (예: 10,000개)
//   - LRU 캐시 사용으로 오래된 IP 자동 제거
//   - 주기적인 정리 작업 (예: 1시간 미사용 IP 제거)
type ipRateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	rate     rate.Limit // 초당 허용 요청 수
	burst    int        // 버스트 허용량
}

// newIPRateLimiter 새로운 IP 기반 Rate Limiter를 생성합니다.
//
// Parameters:
//   - requestsPerSecond: 초당 허용할 요청 수 (예: 20 = 초당 20개 요청)
//   - burst: 버스트 허용량 (예: 40 = 최대 40개 토큰 저장)
//
// Returns:
//   - 초기화된 ipRateLimiter 인스턴스
func newIPRateLimiter(requestsPerSecond int, burst int) *ipRateLimiter {
	return &ipRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
	}
}

// getLimiter 특정 IP 주소에 대한 Rate Limiter를 반환합니다.
// IP에 대한 Limiter가 없으면 새로 생성합니다.
//
// 이 메서드는 동시성 안전하며, 여러 고루틴에서 동시에 호출 가능합니다.
func (i *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	// 먼저 읽기 락으로 확인 (성능 최적화)
	i.mu.RLock()
	limiter, exists := i.limiters[ip]
	i.mu.RUnlock()

	if exists {
		return limiter
	}

	// 없으면 쓰기 락으로 생성
	i.mu.Lock()
	defer i.mu.Unlock()

	// Double-check: 다른 고루틴이 이미 생성했을 수 있음
	limiter, exists = i.limiters[ip]
	if exists {
		return limiter
	}

	// 새 Limiter 생성
	limiter = rate.NewLimiter(i.rate, i.burst)
	i.limiters[ip] = limiter

	return limiter
}

// RateLimiting IP 기반 Rate Limiting 미들웨어를 반환합니다.
//
// 이 미들웨어는 다음과 같은 기능을 제공합니다:
//   - IP 주소별로 독립적인 요청 제한
//   - Token Bucket 알고리즘 사용 (golang.org/x/time/rate)
//   - 제한 초과 시 429 Too Many Requests 응답
//   - Retry-After 헤더 제공 (1초 권장)
//
// Parameters:
//   - requestsPerSecond: 초당 허용할 요청 수 (예: 20, 양수여야 함)
//   - burst: 버스트 허용량 (예: 40, 양수여야 함)
//
// Token Bucket 알고리즘:
//   - Rate: 초당 토큰 생성 속도 (requestsPerSecond)
//   - Burst: 버킷 크기 (burst), 최대 저장 가능한 토큰 수
//   - 요청마다 토큰 1개 소비
//   - 토큰 부족 시 요청 거부 (429 응답)
//
// 사용 예시:
//
//	e := echo.New()
//	e.Use(middleware.RateLimiting(20, 40)) // 초당 20 요청, 버스트 40
//
// 주의사항:
//   - 메모리 기반 저장소 사용 (서버 재시작 시 초기화)
//   - 다중 서버 환경에서는 서버별로 독립적인 제한 적용
//   - 장기 실행 시 IP 개수에 비례하여 메모리 사용량 증가 가능
//
// Panics:
//   - requestsPerSecond가 0 이하인 경우
//   - burst가 0 이하인 경우
func RateLimiting(requestsPerSecond int, burst int) echo.MiddlewareFunc {
	// 입력 검증
	if requestsPerSecond <= 0 {
		panic("[RateLimiting] requestsPerSecond는 양수여야 합니다")
	}
	if burst <= 0 {
		panic("[RateLimiting] burst는 양수여야 합니다")
	}

	limiter := newIPRateLimiter(requestsPerSecond, burst)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 클라이언트 IP 추출
			ip := c.RealIP()

			// IP별 Limiter 가져오기
			ipLimiter := limiter.getLimiter(ip)

			// Rate Limit 확인
			if !ipLimiter.Allow() {
				// 제한 초과 로깅
				applog.WithComponentAndFields(constants.ComponentMiddleware, applog.Fields{
					"remote_ip": ip,
					"path":      c.Request().URL.Path,
					"method":    c.Request().Method,
				}).Warn("Rate limit 초과")

				// Retry-After 헤더 설정 (1초 후 재시도 권장)
				c.Response().Header().Set("Retry-After", "1")

				// 429 Too Many Requests 응답
				return httputil.NewTooManyRequestsError(constants.ErrMsgTooManyRequests)
			}

			// 다음 핸들러 실행
			return next(c)
		}
	}
}
