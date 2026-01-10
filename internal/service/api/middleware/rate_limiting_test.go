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
)

// TestRateLimiting_InputValidation 입력 검증을 테스트합니다.
func TestRateLimiting_InputValidation(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerSecond int
		burst             int
		expectPanic       bool
		expectedMessage   string
	}{
		{
			name:              "Valid Positive Values",
			requestsPerSecond: 10,
			burst:             20,
			expectPanic:       false,
		},
		{
			name:              "Zero RequestsPerSecond",
			requestsPerSecond: 0,
			burst:             20,
			expectPanic:       true,
			expectedMessage:   "[RateLimiting] requestsPerSecond는 양수여야 합니다",
		},
		{
			name:              "Negative RequestsPerSecond",
			requestsPerSecond: -10,
			burst:             20,
			expectPanic:       true,
			expectedMessage:   "[RateLimiting] requestsPerSecond는 양수여야 합니다",
		},
		{
			name:              "Zero Burst",
			requestsPerSecond: 10,
			burst:             0,
			expectPanic:       true,
			expectedMessage:   "[RateLimiting] burst는 양수여야 합니다",
		},
		{
			name:              "Negative Burst",
			requestsPerSecond: 10,
			burst:             -20,
			expectPanic:       true,
			expectedMessage:   "[RateLimiting] burst는 양수여야 합니다",
		},
		{
			name:              "Both Zero",
			requestsPerSecond: 0,
			burst:             0,
			expectPanic:       true,
			expectedMessage:   "[RateLimiting] requestsPerSecond는 양수여야 합니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

// TestRateLimiting_BasicOperation Rate Limiting 미들웨어의 기본 동작을 검증합니다.
func TestRateLimiting_BasicOperation(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerSecond int
		burst             int
		requestCount      int
		expectedAllowed   int
		expectedBlocked   int
	}{
		{
			name:              "Within Limit",
			requestsPerSecond: 10,
			burst:             20,
			requestCount:      15,
			expectedAllowed:   15,
			expectedBlocked:   0,
		},
		{
			name:              "Exceed Limit",
			requestsPerSecond: 5,
			burst:             10,
			requestCount:      20,
			expectedAllowed:   10, // 버스트만큼 허용
			expectedBlocked:   10,
		},
		{
			name:              "Exact Burst",
			requestsPerSecond: 10,
			burst:             5,
			requestCount:      5,
			expectedAllowed:   5,
			expectedBlocked:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			middleware := RateLimiting(tt.requestsPerSecond, tt.burst)

			handler := func(c echo.Context) error {
				return c.String(http.StatusOK, "success")
			}

			h := middleware(handler)

			allowed := 0
			blocked := 0

			// Execute
			for i := 0; i < tt.requestCount; i++ {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("X-Real-IP", "192.168.1.1")
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)

				err := h(c)

				if err != nil {
					blocked++
					// 429 에러 확인
					httpErr, ok := err.(*echo.HTTPError)
					assert.True(t, ok)
					assert.Equal(t, http.StatusTooManyRequests, httpErr.Code)
				} else {
					allowed++
					assert.Equal(t, http.StatusOK, rec.Code)
				}
			}

			// Verify
			assert.Equal(t, tt.expectedAllowed, allowed, "Allowed requests count mismatch")
			assert.Equal(t, tt.expectedBlocked, blocked, "Blocked requests count mismatch")
		})
	}
}

// TestRateLimiting_IPIsolation 서로 다른 IP는 독립적인 Rate Limit을 가지는지 검증합니다.
func TestRateLimiting_IPIsolation(t *testing.T) {
	// Setup
	e := echo.New()
	middleware := RateLimiting(5, 10)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	}

	h := middleware(handler)

	// IP1에서 10개 요청 (버스트 소진)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Real-IP", "192.168.1.1")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h(c)
		assert.NoError(t, err)
	}

	// IP1에서 추가 요청 (차단되어야 함)
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("X-Real-IP", "192.168.1.1")
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)

	err1 := h(c1)
	assert.Error(t, err1)
	httpErr1, ok := err1.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusTooManyRequests, httpErr1.Code)

	// IP2에서 요청 (허용되어야 함 - 독립적인 제한)
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-Real-IP", "192.168.1.2")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)

	err2 := h(c2)
	assert.NoError(t, err2)
	assert.Equal(t, http.StatusOK, rec2.Code)
}

// TestRateLimiting_RetryAfterHeader Retry-After 헤더가 올바르게 설정되는지 검증합니다.
func TestRateLimiting_RetryAfterHeader(t *testing.T) {
	// Setup
	e := echo.New()
	middleware := RateLimiting(1, 2)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	}

	h := middleware(handler)

	// 버스트 소진
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Real-IP", "192.168.1.1")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		h(c)
	}

	// 제한 초과 요청
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h(c)

	// Verify
	assert.Error(t, err)
	assert.Equal(t, "1", rec.Header().Get("Retry-After"), "Retry-After header should be set to 1")
}

// TestRateLimiting_Concurrency 동시성 안전성을 검증합니다.
func TestRateLimiting_Concurrency(t *testing.T) {
	// Setup
	e := echo.New()
	middleware := RateLimiting(100, 200)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	}

	h := middleware(handler)

	// 여러 고루틴에서 동시에 요청
	const goroutines = 10
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()

			ip := fmt.Sprintf("192.168.1.%d", goroutineID)

			for i := 0; i < requestsPerGoroutine; i++ {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("X-Real-IP", ip)
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)

				h(c) // 에러 무시 (동시성 안전성만 확인)
			}
		}(g)
	}

	wg.Wait()
	// Race detector로 실행 시 문제 없으면 통과
}

// TestRateLimiting_BurstAllowance 버스트 허용량이 올바르게 동작하는지 검증합니다.
func TestRateLimiting_BurstAllowance(t *testing.T) {
	// Setup
	e := echo.New()
	requestsPerSecond := 10
	burst := 30
	middleware := RateLimiting(requestsPerSecond, burst)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	}

	h := middleware(handler)

	// 버스트만큼 즉시 요청 가능
	for i := 0; i < burst; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Real-IP", "192.168.1.1")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h(c)
		assert.NoError(t, err, "Request %d should be allowed (within burst)", i+1)
	}

	// 버스트 초과 시 차단
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h(c)
	assert.Error(t, err, "Request beyond burst should be blocked")
}

// TestRateLimiting_Recovery 시간 경과 후 Rate Limit이 복구되는지 검증합니다.
func TestRateLimiting_Recovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-dependent test in short mode")
	}

	// Setup
	e := echo.New()
	requestsPerSecond := 10
	burst := 5
	middleware := RateLimiting(requestsPerSecond, burst)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	}

	h := middleware(handler)

	// 버스트 소진
	for i := 0; i < burst; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Real-IP", "192.168.1.1")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		h(c)
	}

	// 제한 초과 확인
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("X-Real-IP", "192.168.1.1")
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)

	err1 := h(c1)
	assert.Error(t, err1, "Should be blocked immediately after burst")

	// 1초 대기 (10 requests/sec이므로 10개 토큰 충전)
	time.Sleep(1 * time.Second)

	// 다시 요청 가능
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-Real-IP", "192.168.1.1")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)

	err2 := h(c2)
	assert.NoError(t, err2, "Should be allowed after recovery period")
}

// TestRateLimiting_ErrorMessage 에러 메시지가 올바른지 검증합니다.
func TestRateLimiting_ErrorMessage(t *testing.T) {
	// Setup
	e := echo.New()
	middleware := RateLimiting(1, 1)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	}

	h := middleware(handler)

	// 버스트 소진
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("X-Real-IP", "192.168.1.1")
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	h(c1)

	// 제한 초과 요청
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-Real-IP", "192.168.1.1")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)

	err := h(c2)

	// Verify
	require.Error(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)

	assert.Equal(t, http.StatusTooManyRequests, httpErr.Code)
	assert.Contains(t, fmt.Sprintf("%v", httpErr.Message), constants.ErrMsgTooManyRequests)
}

// TestRateLimiting_DifferentPaths 다양한 경로에서 동일한 IP는 동일한 제한을 받는지 검증합니다.
func TestRateLimiting_DifferentPaths(t *testing.T) {
	// Setup
	e := echo.New()
	middleware := RateLimiting(5, 10)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	}

	h := middleware(handler)

	// 다양한 경로로 요청 (동일 IP)
	paths := []string{"/api/v1/test", "/api/v1/users", "/health", "/version"}

	totalRequests := 0
	for _, path := range paths {
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("X-Real-IP", "192.168.1.1")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			h(c)
			totalRequests++
		}
	}

	// 버스트 초과 확인 (12개 요청, 버스트 10)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h(c)
	assert.Error(t, err, "Should be blocked after exceeding burst across different paths")
}
