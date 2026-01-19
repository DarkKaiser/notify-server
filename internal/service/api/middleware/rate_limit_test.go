package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// =============================================================================
// Rate Limiting 미들웨어 테스트
// =============================================================================

// TestNewIPRateLimiter_WhiteBox는 ipRateLimiter 구조체의 초기화 로직을 검증합니다.
//
// 검증 항목:
//   - limiters 맵 초기화 여부
//   - rate 및 burst 설정값 정확성
func TestNewIPRateLimiter_WhiteBox(t *testing.T) {
	t.Parallel()

	rps := 10
	burst := 20
	limiter := newIPRateLimiter(rps, burst)

	assert.NotNil(t, limiter.limiters, "limiters 맵은 초기화되어야 합니다")
	assert.Equal(t, rate.Limit(rps), limiter.rate, "rate 값이 일치해야 합니다")
	assert.Equal(t, burst, limiter.burst, "burst 값이 일치해야 합니다")
	assert.Equal(t, 0, len(limiter.limiters), "초기 limiters 맵은 비어있어야 합니다")
}

// TestRateLimit_InputValidation_Table은 미들웨어 생성 시 입력값 검증 로직을 테스트합니다.
// 잘못된 입력값(음수, 0)에 대해 패닉이 발생하는지 확인합니다.
func TestRateLimit_InputValidation_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		requestsPerSecond int
		burst             int
		expectPanic       bool
		expectedMessage   string
	}{
		{"성공: 정상값 입력", 10, 20, false, ""},
		{"실패: RPS 0 입력", 0, 20, true, "requestsPerSecond는 양수여야 합니다"},
		{"실패: RPS 음수 입력", -10, 20, true, "requestsPerSecond는 양수여야 합니다"},
		{"실패: Burst 0 입력", 10, 0, true, "burst는 양수여야 합니다"},
		{"실패: Burst 음수 입력", 10, -20, true, "burst는 양수여야 합니다"},
		{"실패: 둘 다 0 입력", 0, 0, true, "requestsPerSecond는 양수여야 합니다"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.expectPanic {
				assert.Panics(t, func() {
					RateLimit(tt.requestsPerSecond, tt.burst)
				}, "잘못된 입력값에 대해 패닉이 발생해야 합니다")
			} else {
				assert.NotPanics(t, func() {
					RateLimit(tt.requestsPerSecond, tt.burst)
				}, "정상 입력값에 대해 패닉이 발생하지 않아야 합니다")
			}
		})
	}
}

// TestRateLimit_Scenarios_Table은 다양한 사용 시나리오에 대한 통합 테스트를 수행합니다.
//
// 시나리오 포함:
//   - 기본 허용 및 차단 (Basic Allowance and Blocking)
//   - IP 분리 (IP Isolation)
//   - 경로 간 제한 공유 (Shared Limit across Paths)
//   - 응답 헤더 및 바디 검증 (Response Headers and Body)
func TestRateLimit_Scenarios_Table(t *testing.T) {
	// 로그 캡처 설정이 필요하지 않은 병렬 테스트
	t.Parallel()

	tests := []struct {
		name       string
		rps        int
		burst      int
		operations func(*testing.T, echo.HandlerFunc)
	}{
		{
			name:  "시나리오: 기본 허용 및 차단",
			rps:   10,
			burst: 20,
			operations: func(t *testing.T, h echo.HandlerFunc) {
				// 1. 버스트 내 요청 허용 (20개)
				for i := 0; i < 20; i++ {
					assertRequest(t, h, "1.1.1.1", http.StatusOK)
				}
				// 2. 버스트 초과 요청 차단
				assertRequest(t, h, "1.1.1.1", http.StatusTooManyRequests)
			},
		},
		{
			name:  "시나리오: IP 독립성 보장",
			rps:   1,
			burst: 1,
			operations: func(t *testing.T, h echo.HandlerFunc) {
				// IP A 소진
				assertRequest(t, h, "1.1.1.1", http.StatusOK)
				assertRequest(t, h, "1.1.1.1", http.StatusTooManyRequests)

				// IP B는 IP A의 영향을 받지 않고 성공해야 함
				assertRequest(t, h, "2.2.2.2", http.StatusOK)
				assertRequest(t, h, "2.2.2.2", http.StatusTooManyRequests)
			},
		},
		{
			name:  "시나리오: 동일 IP 내 경로 간 제한 공유",
			rps:   1,
			burst: 1,
			operations: func(t *testing.T, h echo.HandlerFunc) {
				// 경로 A 요청으로 제한 소진
				assertRequestPath(t, h, "1.1.1.1", "/path/a", http.StatusOK)

				// 경로 B 요청도 차단되어야 함 (IP 단위 제한이므로)
				assertRequestPath(t, h, "1.1.1.1", "/path/b", http.StatusTooManyRequests)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 목 핸들러: 항상 200 OK 반환
			mockHandler := func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			}

			middleware := RateLimit(tt.rps, tt.burst)
			h := middleware(mockHandler)

			tt.operations(t, h)
		})
	}
}

// TestRateLimit_ResponseHeadersAndLogs는 차단 시 응답 헤더와 로그를 심층 검증합니다.
func TestRateLimit_ResponseHeadersAndLogs(t *testing.T) {
	// 로그 캡처를 위해 직렬 실행 (t.Parallel() 제거 권장하거나 로그 캡처 함수 내부에서 처리)
	// 여기서는 로그 캡처 때문에 직렬 실행
	var buf bytes.Buffer
	applog.SetOutput(&buf)
	applog.SetFormatter(&applog.JSONFormatter{})
	defer applog.SetOutput(applog.StandardLogger().Out)

	middleware := RateLimit(1, 1)
	mockHandler := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }
	h := middleware(mockHandler)

	// 1. 첫 요청 성공
	assertRequest(t, h, "3.3.3.3", http.StatusOK)
	buf.Reset() // 로그 초기화

	// 2. 두 번째 요청 차단 및 검증
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "3.3.3.3")
	rec := httptest.NewRecorder()

	// echo.Context 생성
	c := e.NewContext(req, rec)

	// 핸들러 실행
	err := h(c)

	// 2.1 에러 응답 검증
	require.Error(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusTooManyRequests, httpErr.Code)
	assert.Contains(t, fmt.Sprintf("%v", httpErr.Message), constants.ErrMsgTooManyRequests)

	// 2.2 Retry-After 헤더 검증
	assert.Equal(t, constants.RetryAfterSeconds, rec.Header().Get(constants.RetryAfter))

	// 2.3 로그 검증
	require.Greater(t, buf.Len(), 0, "로그가 기록되어야 합니다")

	var logEntry map[string]interface{}
	parseErr := json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, parseErr)

	assert.Equal(t, "warning", logEntry["level"])
	assert.Equal(t, "API 요청 속도 제한 초과 (차단됨)", logEntry["msg"])
	assert.Equal(t, "3.3.3.3", logEntry["remote_ip"])
	assert.Equal(t, "/test", logEntry["path"])
}

// TestRateLimit_Recovery는 시간 경과 후 제한이 복구되는지 검증합니다.
func TestRateLimit_Recovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Short 모드에서는 시간 의존 테스트(TestRateLimit_Recovery) 스킵")
	}
	t.Parallel()

	rps := 10
	burst := 5
	middleware := RateLimit(rps, burst)
	h := middleware(func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	// 1. 버스트 완전히 소진
	for i := 0; i < burst; i++ {
		assertRequest(t, h, "1.1.1.1", http.StatusOK)
	}

	// 2. 차단 확인 (확실히 차단됨을 보장하기 위해)
	assertRequest(t, h, "1.1.1.1", http.StatusTooManyRequests)

	// 3. 1초 대기 (토큰 재충전)
	// rate 패키지는 시간 기반이므로 sleep이 필요함
	time.Sleep(1100 * time.Millisecond) // 약간의 여유를 둠

	// 4. 복구 확인 (최소 1개의 요청은 성공해야 함)
	assertRequest(t, h, "1.1.1.1", http.StatusOK)
}

// TestRateLimit_Concurrency_StressTest는 고동시성 상황에서 교착 상태(deadlock)나
// 데이터 경합(race condition)이 발생하지 않는지 검증합니다.
func TestRateLimit_Concurrency_StressTest(t *testing.T) {
	t.Parallel()

	e := echo.New()
	// 충분한 용량으로 설정하여 429 에러가 발생하지 않도록 함 (동시성 안전성 검증이 목적)
	middleware := RateLimit(500, 1000)
	h := middleware(func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	var wg sync.WaitGroup
	workers := 20   // 고루틴 수 증가
	requests := 100 // 각 고루틴 당 요청 수

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			ip := fmt.Sprintf("192.168.0.%d", id) // 각 워커는 고유 IP 사용
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

// TestRateLimit_MaxIPLimit은 최대 IP 추적 수 제한 및 오래된 항목 제거 로직을 검증합니다.
func TestRateLimit_MaxIPLimit(t *testing.T) {
	t.Parallel()

	// 테스트 효율성을 위해 작은 단위로 검증할 수 없으므로(상수가 const임),
	// 통합 테스트 레벨에서 대량의 IP를 생성하여 검증합니다.
	// newIPRateLimiter는 private 함수이므로 공개된 RateLimit을 통해 간접 검증하거나,
	// 화이트박스 테스트(내부 헬퍼)를 활용합니다.

	// internal 패키지 테스트이므로 newIPRateLimiter에 접근 가능
	limiter := newIPRateLimiter(1, 1)

	// maxIPRateLimiters보다 많은 수의 고유 IP로 요청 시도
	const overloadCount = 10100 // maxIPRateLimiters(10000) + 100

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
				ip := fmt.Sprintf("stress-ip-%d", i)
				_ = limiter.getLimiter(ip)
			}
		}(w)
	}
	wg.Wait()

	// 검증
	limiter.mu.RLock()
	size := len(limiter.limiters)
	limiter.mu.RUnlock()

	// 맵 크기는 maxIPRateLimiters 상수를 초과하지 않아야 함
	// 참고: rate_limiting.go의 maxIPRateLimiters 상수는 exported가 아니므로 직접 참조 불가
	// 하지만 같은 패키지 내 테스트이므로 상수를 알 수 있음.
	// rate_limiting.go 상단 const 참조 (10000)
	const expectedMax = 10000

	assert.LessOrEqual(t, size, expectedMax,
		"메모리 누수 감지: 맵 크기(%d)가 최대 허용치(%d)를 초과했습니다", size, expectedMax)
}

// --- Helpers ---

func assertRequest(t *testing.T, h echo.HandlerFunc, ip string, expectedStatus int) {
	t.Helper()
	assertRequestPath(t, h, ip, "/", expectedStatus)
}

func assertRequestPath(t *testing.T, h echo.HandlerFunc, ip string, path string, expectedStatus int) {
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
				// 에러가 HTTPError가 아닌 경우 실패 처리
				t.Errorf("expected echo.HTTPError but got %T: %v", err, err)
			}
		} else {
			// 미들웨어가 에러를 리턴하지 않고 직접 응답을 쓴 경우 (RateLimit 미들웨어는 error 리턴함)
			assert.Equal(t, expectedStatus, rec.Code)
		}
	} else {
		assert.NoError(t, err)
		assert.Equal(t, expectedStatus, rec.Code)
	}
}
