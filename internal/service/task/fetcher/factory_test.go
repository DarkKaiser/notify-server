package fetcher

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// Helper Functions for Pointers
// =========================================================================

func intPtr(v int) *int {
	return &v
}

func durationPtr(v time.Duration) *time.Duration {
	return &v
}

func stringPtr(v string) *string {
	return &v
}

// =========================================================================
// Tests for Generics Helper Functions
// =========================================================================

func TestNormalizePtr(t *testing.T) {
	// 정규화 로직: 10보다 작으면 10으로, 100보다 크면 100으로 보정
	normalizer := func(v int) int {
		if v < 10 {
			return 10
		}
		if v > 100 {
			return 100
		}
		return v
	}

	tests := []struct {
		name     string
		input    *int
		defValue int
		expected int
	}{
		{
			name:     "Nil input should use default value",
			input:    nil,
			defValue: 50,
			expected: 50, // 50은 범위 내이므로 그대로 반환
		},
		{
			name:     "Nil input with out-of-range default should be normalized",
			input:    nil,
			defValue: 5,
			expected: 10, // 5 -> 10 보정
		},
		{
			name:     "Value too small should be normalized",
			input:    intPtr(5),
			defValue: 50,
			expected: 10,
		},
		{
			name:     "Value too large should be normalized",
			input:    intPtr(150),
			defValue: 50,
			expected: 100,
		},
		{
			name:     "Valid value should be kept",
			input:    intPtr(75),
			defValue: 50,
			expected: 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ptr := tt.input
			normalizePtr(&ptr, tt.defValue, normalizer)
			assert.NotNil(t, ptr)
			assert.Equal(t, tt.expected, *ptr)
		})
	}
}

func TestNormalizePtrPair(t *testing.T) {
	// 정규화 로직: min이 max보다 크면 max를 min에 맞춤
	normalizer := func(min, max int) (int, int) {
		if min > max {
			return min, min
		}
		return min, max
	}

	tests := []struct {
		name      string
		inputMin  *int
		inputMax  *int
		defMin    int
		defMax    int
		expectMin int
		expectMax int
	}{
		{
			name:      "Nil inputs should use defaults",
			inputMin:  nil,
			inputMax:  nil,
			defMin:    10,
			defMax:    20,
			expectMin: 10,
			expectMax: 20,
		},
		{
			name:      "Invalid default relationship should be normalized",
			inputMin:  nil,
			inputMax:  nil,
			defMin:    30,
			defMax:    20, // min > max
			expectMin: 30,
			expectMax: 30,
		},
		{
			name:      "Explicit values respecting logic",
			inputMin:  intPtr(5),
			inputMax:  intPtr(10),
			defMin:    1,
			defMax:    100,
			expectMin: 5,
			expectMax: 10,
		},
		{
			name:      "Explicit values violating logic should be normalized",
			inputMin:  intPtr(50),
			inputMax:  intPtr(30),
			defMin:    1,
			defMax:    100,
			expectMin: 50,
			expectMax: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p1 := tt.inputMin
			p2 := tt.inputMax
			normalizePtrPair(&p1, &p2, tt.defMin, tt.defMax, normalizer)
			assert.Equal(t, tt.expectMin, *p1)
			assert.Equal(t, tt.expectMax, *p2)
		})
	}
}

// =========================================================================
// Tests for Config Normalization
// =========================================================================

func TestConfig_applyDefaults(t *testing.T) {
	// 내부 상수는 export되지 않았으므로 값으로 직접 검증하거나 export_test.go 활용
	const (
		defaultRetryDelay          = 1 * time.Second
		defaultMaxRetryDelay       = 30 * time.Second
		defaultMaxBytes            = 10 * 1024 * 1024 // 10MB
		defaultTimeout             = 30 * time.Second
		defaultMaxIdleConns        = 100
		defaultIdleConnTimeout     = 90 * time.Second
		defaultTLSHandshakeTimeout = 10 * time.Second
		defaultMaxRedirects        = 10
	)

	tests := []struct {
		name     string
		input    Config
		expected Config
	}{
		// 1. Zero Value Test
		{
			name:  "Zero values should be replaced with safer defaults",
			input: Config{},
			expected: Config{
				MaxRetries:            0, // 0 means disabled
				MinRetryDelay:         defaultRetryDelay,
				MaxRetryDelay:         defaultMaxRetryDelay,
				MaxBytes:              defaultMaxBytes,
				Timeout:               nil, // Optionals remain nil
				TLSHandshakeTimeout:   nil,
				ResponseHeaderTimeout: nil,
				IdleConnTimeout:       nil,
				MaxIdleConns:          nil,
				MaxIdleConnsPerHost:   nil,
				MaxConnsPerHost:       nil,
				ProxyURL:              nil,
				MaxRedirects:          nil,
			},
		},

		// 2. Retry Logic Tests
		{
			name: "MaxRetries clamping (negative -> 0)",
			input: Config{
				MaxRetries: -5,
			},
			expected: Config{
				MaxRetries:    0,
				MinRetryDelay: defaultRetryDelay,
				MaxRetryDelay: defaultMaxRetryDelay,
				MaxBytes:      defaultMaxBytes,
			},
		},
		{
			name: "MaxRetries upper bound clamping (>10 -> 10)",
			input: Config{
				MaxRetries: 100,
			},
			expected: Config{
				MaxRetries:    10, // MaxAllowedRetries
				MinRetryDelay: defaultRetryDelay,
				MaxRetryDelay: defaultMaxRetryDelay,
				MaxBytes:      defaultMaxBytes,
			},
		},
		{
			name: "RetryDelay logic (Min < 1s -> 1s)",
			input: Config{
				MinRetryDelay: 500 * time.Millisecond,
			},
			expected: Config{
				MaxRetries:    0,
				MinRetryDelay: 1 * time.Second, // Bumped to 1s
				MaxRetryDelay: 30 * time.Second,
				MaxBytes:      defaultMaxBytes,
			},
		},
		{
			name: "RetryDelay logic (Max < Min -> Max = Min)",
			input: Config{
				MinRetryDelay: 5 * time.Second,
				MaxRetryDelay: 2 * time.Second,
			},
			expected: Config{
				MaxRetries:    0,
				MinRetryDelay: 5 * time.Second,
				MaxRetryDelay: 5 * time.Second, // Bumped to Min
				MaxBytes:      defaultMaxBytes,
			},
		},

		// 3. Timeout Logic Tests (Negative -> Special Handling)
		{
			name: "Timeouts negative values (use default or zero)",
			input: Config{
				Timeout:               durationPtr(-1),
				TLSHandshakeTimeout:   durationPtr(-1),
				IdleConnTimeout:       durationPtr(-1),
				ResponseHeaderTimeout: durationPtr(-1),
			},
			expected: Config{
				MaxRetries:            0,
				MinRetryDelay:         defaultRetryDelay,
				MaxRetryDelay:         defaultMaxRetryDelay,
				MaxBytes:              defaultMaxBytes,
				Timeout:               durationPtr(defaultTimeout),             // Default
				TLSHandshakeTimeout:   durationPtr(defaultTLSHandshakeTimeout), // Default
				IdleConnTimeout:       durationPtr(defaultIdleConnTimeout),     // Default
				ResponseHeaderTimeout: durationPtr(0),                          // 0 (No Timeout)
			},
		},

		// 4. Connection Limits Tests (Negative -> Special Handling)
		{
			name: "Connection limits negative values",
			input: Config{
				MaxIdleConns:        intPtr(-1),
				MaxIdleConnsPerHost: intPtr(-1),
				MaxConnsPerHost:     intPtr(-1),
				MaxRedirects:        intPtr(-1),
			},
			expected: Config{
				MaxRetries:          0,
				MinRetryDelay:       defaultRetryDelay,
				MaxRetryDelay:       defaultMaxRetryDelay,
				MaxBytes:            defaultMaxBytes,
				MaxIdleConns:        intPtr(defaultMaxIdleConns),
				MaxIdleConnsPerHost: intPtr(0), // Default 0 (uses net/http default 2)
				MaxConnsPerHost:     intPtr(0), // 0 (Unlimited)
				MaxRedirects:        intPtr(defaultMaxRedirects),
			},
		},

		// 5. MaxBytes Tests
		{
			name: "MaxBytes NoLimit (-1)",
			input: Config{
				MaxBytes: -1,
			},
			expected: Config{
				MaxRetries:    0,
				MinRetryDelay: defaultRetryDelay,
				MaxRetryDelay: defaultMaxRetryDelay,
				MaxBytes:      -1, // Should be kept as -1
			},
		},
		{
			name: "MaxBytes Negative but not -1 (Invalid -> Default)",
			input: Config{
				MaxBytes: -500,
			},
			expected: Config{
				MaxRetries:    0,
				MinRetryDelay: defaultRetryDelay,
				MaxRetryDelay: defaultMaxRetryDelay,
				MaxBytes:      defaultMaxBytes,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.input
			cfg.applyDefaults()
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// =========================================================================
// Tests for Fetcher Chain Construction (Value Propagation)
// =========================================================================

func TestNewFromConfig_ValuePropagation(t *testing.T) {
	// 1. 모든 값이 명시적으로 설정된 Config 준비
	explicitTimeout := 5 * time.Second
	explicitProxy := "http://user:pass@proxy.example.com:8080"
	explicitMaxIdle := 50
	explicitMaxBytes := int64(2048)

	cfg := Config{
		// Network
		Timeout:               durationPtr(explicitTimeout),
		TLSHandshakeTimeout:   durationPtr(2 * time.Second),
		ResponseHeaderTimeout: durationPtr(3 * time.Second),
		IdleConnTimeout:       durationPtr(4 * time.Second),
		ProxyURL:              stringPtr(explicitProxy),
		MaxIdleConns:          intPtr(explicitMaxIdle),
		MaxIdleConnsPerHost:   intPtr(10),
		MaxConnsPerHost:       intPtr(20),

		// Retry
		MaxRetries:    5,
		MinRetryDelay: 100 * time.Millisecond, // Should normalize to 1s
		MaxRetryDelay: 5 * time.Second,

		// Validation
		DisableStatusCodeValidation: false,
		AllowedStatusCodes:          []int{200, 201, 204},
		AllowedMimeTypes:            []string{"application/json", "text/xml"},
		MaxBytes:                    explicitMaxBytes,
		MaxRedirects:                intPtr(3),

		// Middleware
		EnableUserAgentRandomization: true,
		UserAgents:                   []string{"TestAgent/1.0", "TestAgent/2.0"},
		DisableLogging:               false,
		DisableTransportCaching:      true,
	}

	// 2. Fetcher 생성
	f := NewFromConfig(cfg)
	require.NotNil(t, f)

	// 3. 체인 순서 및 값 검증 (Outer -> Inner)

	// Layer 1: LoggingFetcher
	// LoggingFetcher는 단순히 delegate만 가짐
	curr := f
	delegate := UnwrapLoggingFetcher(curr)
	require.NotNil(t, delegate, "Outermost should be LoggingFetcher")
	curr = delegate

	// Layer 2: UserAgentFetcher
	var uaList []string
	curr, uaList = InspectUserAgentFetcher(curr)
	require.NotNil(t, curr, "Second layer should be UserAgentFetcher")
	assert.ElementsMatch(t, []string{"TestAgent/1.0", "TestAgent/2.0"}, uaList, "UserAgents passed correctly")

	// Layer 3: RetryFetcher
	var maxRetries int
	var minDelay, maxDelay time.Duration
	curr, maxRetries, minDelay, maxDelay = InspectRetryFetcher(curr)
	require.NotNil(t, curr, "Third layer should be RetryFetcher")
	assert.Equal(t, 5, maxRetries, "MaxRetries passed correctly")
	assert.Equal(t, 1*time.Second, minDelay, "MinRetryDelay normalized correctly (min 1s)")
	assert.Equal(t, 5*time.Second, maxDelay, "MaxRetryDelay passes correctly")

	// Layer 4: MimeTypeFetcher
	var allowedMimes []string
	curr, allowedMimes, _ = InspectMimeTypeFetcher(curr)
	require.NotNil(t, curr, "Fourth layer should be MimeTypeFetcher")
	assert.ElementsMatch(t, []string{"application/json", "text/xml"}, allowedMimes, "AllowedMimeTypes passed correctly")

	// Layer 5: StatusCodeFetcher
	var allowedCodes []int
	curr, allowedCodes = InspectStatusCodeFetcher(curr)
	require.NotNil(t, curr, "Fifth layer should be StatusCodeFetcher")
	assert.ElementsMatch(t, []int{200, 201, 204}, allowedCodes, "AllowedStatusCodes passed correctly")

	// Layer 6: MaxBytesFetcher
	var maxBytes int64
	curr, maxBytes = InspectMaxBytesFetcher(curr)
	require.NotNil(t, curr, "Sixth layer should be MaxBytesFetcher")
	assert.Equal(t, explicitMaxBytes, maxBytes, "MaxBytes passed correctly")

	// Layer 7: HTTPFetcher (Innermost)
	httpOpts := InspectHTTPFetcher(curr)
	require.NotNil(t, httpOpts, "Innermost should be HTTPFetcher")

	// HTTPFetcher Configuration Check
	assert.Equal(t, explicitProxy, httpOpts.ProxyURL, "ProxyURL passed correctly")
	assert.Equal(t, explicitMaxIdle, httpOpts.MaxIdleConns, "MaxIdleConns passed correctly")
	assert.Equal(t, 10, httpOpts.MaxIdleConnsPerHost, "MaxIdleConnsPerHost passed correctly")
	assert.Equal(t, 20, httpOpts.MaxConnsPerHost, "MaxConnsPerHost passed correctly")
	assert.Equal(t, 4*time.Second, httpOpts.IdleConnTimeout, "IdleConnTimeout passed correctly")
	assert.Equal(t, 2*time.Second, httpOpts.TLSHandshakeTimeout, "TLSHandshakeTimeout passed correctly")
	assert.Equal(t, 3*time.Second, httpOpts.ResponseHeaderTimeout, "ResponseHeaderTimeout passed correctly")
	assert.True(t, httpOpts.DisableCaching, "DisableTransportCaching passed correctly")
}

func TestNewFromConfig_MiddlewareToggling(t *testing.T) {
	tests := []struct {
		name       string
		cfg        Config
		checkChain func(*testing.T, Fetcher)
	}{
		{
			name: "Disable Logging",
			cfg: Config{
				DisableLogging: true,
			},
			checkChain: func(t *testing.T, f Fetcher) {
				// LoggingFetcher should NOT be present
				// Default chain without Logging: Retry -> StatusCode -> MaxBytes -> HTTP
				// (Assuming default empty config mostly)
				_, ok := f.(*LoggingFetcher)
				assert.False(t, ok, "LoggingFetcher should be absent when DisableLogging is true")
			},
		},
		{
			name: "Disable StatusCode Validation",
			cfg: Config{
				DisableLogging:              true, // to simplify chain
				DisableStatusCodeValidation: true,
			},
			checkChain: func(t *testing.T, f Fetcher) {
				// Should skip StatusCodeFetcher
				// Retry -> MaxBytes
				delegate, _, _, _ := InspectRetryFetcher(f)
				require.NotNil(t, delegate)
				_, ok := delegate.(*StatusCodeFetcher)
				assert.False(t, ok, "StatusCodeFetcher should be absent when verification disabled")
				_, ok = delegate.(*MaxBytesFetcher)
				assert.True(t, ok, "Should go directly to MaxBytesFetcher")
			},
		},
		{
			name: "Disable UserAgent Randomization",
			cfg: Config{
				EnableUserAgentRandomization: false,
			},
			checkChain: func(t *testing.T, f Fetcher) {
				// If logging enabled: Logging -> Retry (Skip UA)
				delegate := UnwrapLoggingFetcher(f)
				_, ok := delegate.(*UserAgentFetcher)
				assert.False(t, ok, "UserAgentFetcher should be absent is disabled")
				_, ok = delegate.(*RetryFetcher)
				assert.True(t, ok, "Should go directly to RetryFetcher")
			},
		},
		{
			name: "Disable MimeType Validation (Empty List)",
			cfg: Config{
				AllowedMimeTypes: nil, // Empty
				DisableLogging:   true,
			},
			checkChain: func(t *testing.T, f Fetcher) {
				// Retry -> StatusCode (Skip Mime)
				delegate, _, _, _ := InspectRetryFetcher(f)
				_, ok := delegate.(*MimeTypeFetcher)
				assert.False(t, ok, "MimeTypeFetcher should be absent if allowed list empty")
				_, ok = delegate.(*StatusCodeFetcher)
				assert.True(t, ok, "Should go directly to StatusCodeFetcher")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFromConfig(tt.cfg)
			tt.checkChain(t, f)
		})
	}
}

func TestNew(t *testing.T) {
	// New() 편의 함수 검증
	// New(maxRetries, minDelay, maxBytes, opts...)

	f := New(5, 500*time.Millisecond, 1024)

	// 1. Defaults applied?
	// MinDelay 500ms -> 1s normalized
	// Includes Logging by default
	delegate := UnwrapLoggingFetcher(f)
	require.NotNil(t, delegate)

	// Retry Fetcher Check
	_, maxRetries, minDelay, _ := InspectRetryFetcher(delegate)
	assert.Equal(t, 5, maxRetries)
	assert.Equal(t, 1*time.Second, minDelay, "Should apply normalization through applyDefaults")

	// MaxBytes Check (Drill down)
	// Retry -> StatusCode -> MaxBytes
	retryDelegate, _, _, _ := InspectRetryFetcher(delegate)
	statusDelegate, _ := InspectStatusCodeFetcher(retryDelegate)
	_, maxBytes := InspectMaxBytesFetcher(statusDelegate)

	assert.Equal(t, int64(1024), maxBytes)
}

func TestNewFromConfig_Proxy(t *testing.T) {
	// 간단히 프록시 설정이 에러 없이 통과되는지, 그리고 설정이 적용되는지 확인
	cfg := Config{
		DisableLogging: true,
		ProxyURL:       stringPtr("http://127.0.0.1:8080"),
	}
	f := NewFromConfig(cfg)

	// Drill down to HTTPFetcher
	// Retry -> StatusCode -> MaxBytes -> HTTP
	d1, _, _, _ := InspectRetryFetcher(f)
	d2, _ := InspectStatusCodeFetcher(d1)
	d3, _ := InspectMaxBytesFetcher(d2)
	opts := InspectHTTPFetcher(d3)

	assert.Equal(t, "http://127.0.0.1:8080", opts.ProxyURL)
}
