package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// TestNewIPRateLimiter_WhiteBox 내부 구조체가 올바르게 초기화되는지 검증합니다.
func TestNewIPRateLimiter_WhiteBox(t *testing.T) {
	t.Parallel()

	rps := 10
	burst := 20
	limiter := newIPRateLimiter(rps, burst)

	assert.NotNil(t, limiter.limiters)
	assert.Equal(t, rate.Limit(rps), limiter.rate)
	assert.Equal(t, burst, limiter.burst)
	assert.Equal(t, 0, len(limiter.limiters))
}

// TestRateLimiting_InputValidation 입력 검증을 테스트합니다.
func TestRateLimiting_InputValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		requestsPerSecond int
		burst             int
		expectPanic       bool
		expectedMessage   string
	}{
		{"Valid Positive Values", 10, 20, false, ""},
		{"Zero RequestsPerSecond", 0, 20, true, "[RateLimiting] requestsPerSecond는 양수여야 합니다"},
		{"Negative RequestsPerSecond", -10, 20, true, "[RateLimiting] requestsPerSecond는 양수여야 합니다"},
		{"Zero Burst", 10, 0, true, "[RateLimiting] burst는 양수여야 합니다"},
		{"Negative Burst", 10, -20, true, "[RateLimiting] burst는 양수여야 합니다"},
		{"Both Zero", 0, 0, true, "[RateLimiting] requestsPerSecond는 양수여야 합니다"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.expectPanic {
				assert.PanicsWithValue(t, tt.expectedMessage, func() {
					RateLimiting(tt.requestsPerSecond, tt.burst)
				})
			} else {
				assert.NotPanics(t, func() {
					RateLimiting(tt.requestsPerSecond, tt.burst)
				})
			}
		})
	}
}

// TestRateLimiting_Scenarios 다양한 시나리오에 대한 통합 테스트입니다.
func TestRateLimiting_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rps        int
		burst      int
		operations func(*testing.T, echo.HandlerFunc)
	}{
		{
			name:  "Basic Allowance and Blocking",
			rps:   10,
			burst: 20,
			operations: func(t *testing.T, h echo.HandlerFunc) {
				// 1. 버스트 내 요청 허용
				for i := 0; i < 20; i++ {
					assertRequest(t, h, "1.1.1.1", http.StatusOK)
				}
				// 2. 버스트 초과 요청 차단
				assertRequest(t, h, "1.1.1.1", http.StatusTooManyRequests)
			},
		},
		{
			name:  "IP Isolation",
			rps:   1,
			burst: 1,
			operations: func(t *testing.T, h echo.HandlerFunc) {
				// IP A 소진
				assertRequest(t, h, "1.1.1.1", http.StatusOK)
				assertRequest(t, h, "1.1.1.1", http.StatusTooManyRequests)

				// IP B는 영향 없어야 함
				assertRequest(t, h, "2.2.2.2", http.StatusOK)
				assertRequest(t, h, "2.2.2.2", http.StatusTooManyRequests)
			},
		},
		{
			name:  "Different Paths Share Limit per IP",
			rps:   1,
			burst: 1,
			operations: func(t *testing.T, h echo.HandlerFunc) {
				// 경로 A 요청으로 제한 소진
				assertRequestpath(t, h, "1.1.1.1", "/path/a", http.StatusOK)

				// 경로 B 요청도 차단되어야 함 (IP 기준이므로)
				assertRequestpath(t, h, "1.1.1.1", "/path/b", http.StatusTooManyRequests)
			},
		},
		{
			name:  "Response Headers and Body",
			rps:   1,
			burst: 1,
			operations: func(t *testing.T, h echo.HandlerFunc) {
				// 정상 응답
				assertRequest(t, h, "1.1.1.1", http.StatusOK)

				// 차단 응답 검증
				e := echo.New()
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Real-IP", "1.1.1.1")
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)

				err := h(c)

				// 에러 타입 및 코드 검증
				require.Error(t, err)
				httpErr, ok := err.(*echo.HTTPError)
				require.True(t, ok)
				assert.Equal(t, http.StatusTooManyRequests, httpErr.Code)
				assert.Contains(t, fmt.Sprintf("%v", httpErr.Message), constants.ErrMsgTooManyRequests)

				// 헤더 검증
				assert.Equal(t, "1", rec.Header().Get("Retry-After"))
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 목 핸들러
			mockHandler := func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			}

			middleware := RateLimiting(tt.rps, tt.burst)
			h := middleware(mockHandler)

			tt.operations(t, h)
		})
	}
}

// TestRateLimiting_Recovery 시간 경과 후 복구 테스트
func TestRateLimiting_Recovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Short 모드에서는 시간 의존 테스트 스킵")
	}
	t.Parallel()

	rps := 10
	burst := 5
	middleware := RateLimiting(rps, burst)
	h := middleware(func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	// 버스트 소진
	for i := 0; i < burst; i++ {
		assertRequest(t, h, "1.1.1.1", http.StatusOK)
	}

	// 차단 확인
	assertRequest(t, h, "1.1.1.1", http.StatusTooManyRequests)

	// 1초 대기 (충전)
	time.Sleep(1 * time.Second)

	// 복구 확인
	assertRequest(t, h, "1.1.1.1", http.StatusOK)
}

// TestRateLimiting_Concurrency 동시성 테스트
func TestRateLimiting_Concurrency(t *testing.T) {
	t.Parallel()

	e := echo.New()
	middleware := RateLimiting(100, 200)
	h := middleware(func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	var wg sync.WaitGroup
	workers := 10
	requests := 50

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			ip := fmt.Sprintf("192.168.0.%d", id)
			for j := 0; j < requests; j++ {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("X-Real-IP", ip)
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)
				_ = h(c)
			}
		}(i)
	}
	wg.Wait()
}

// TestRateLimiting_MaxIPLimit 최대 IP 추적 수 제한 검증
func TestRateLimiting_MaxIPLimit(t *testing.T) {
	t.Parallel()

	// 테스트용으로 작은 limiter 생성
	limiter := newIPRateLimiter(1, 1)

	// 최대 개수 + @ 만큼 IP 생성하여 접근
	// 실제 상수는 10,000이지만 테스트에서는 해당 로직을 트리거하기 위해
	// 화이트박스 테스트로 직접 limiters 맵에 접근을 할 순 없으므로
	// 실제 10,000개를 채우거나, 코드를 수정해야 함.
	// 하지만 여기선 통합 테스트 레벨이므로 10,001개를 루프 돌리는 것이 맞음 (Go 테스트는 빠름)

	const overloadCount = maxIPRateLimiters + 100

	// 병렬로 채우기 (속도 개선)
	var wg sync.WaitGroup
	workerCount := 10
	ipsPerWorker := overloadCount / workerCount

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			start := workerID * ipsPerWorker
			end := start + ipsPerWorker
			for i := start; i < end; i++ {
				ip := fmt.Sprintf("ip-%d", i)
				_ = limiter.getLimiter(ip)
			}
		}(w)
	}
	wg.Wait()

	// 검증
	limiter.mu.RLock()
	size := len(limiter.limiters)
	limiter.mu.RUnlock()

	// 사이즈는 maxIPRateLimiters 이하이어야 함
	assert.LessOrEqual(t, size, maxIPRateLimiters,
		"Memory leak detected: map size %d exceeds max %d", size, maxIPRateLimiters)
}

// --- Helpers ---

func assertRequest(t *testing.T, h echo.HandlerFunc, ip string, expectedStatus int) {
	t.Helper()
	assertRequestpath(t, h, ip, "/", expectedStatus)
}

func assertRequestpath(t *testing.T, h echo.HandlerFunc, ip string, path string, expectedStatus int) {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Real-IP", ip)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h(c)

	if expectedStatus >= 400 {
		if err != nil {
			he, ok := err.(*echo.HTTPError)
			if ok {
				assert.Equal(t, expectedStatus, he.Code)
			} else {
				assert.Fail(t, "expected echo.HTTPError")
			}
		} else {
			// 핸들러가 에러를 리턴하지 않고 직접 write한 경우 (미들웨어 특성에 따라 다름)
			// 여기 구현에서는 429 시 error를 리턴함.
			assert.Equal(t, expectedStatus, rec.Code)
		}
	} else {
		assert.NoError(t, err)
		assert.Equal(t, expectedStatus, rec.Code)
	}
}
