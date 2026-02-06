package fetcher

import (
	"reflect"
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
	// 내부 상수는 export되지 않았으므로 값으로 직접 검증
	const (
		defaultRetryDelay          = 1 * time.Second
		defaultMaxRetryDelay       = 30 * time.Second
		defaultMaxBytes            = 10 * 1024 * 1024
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
		{
			name:  "Zero values should be replaced with defaults",
			input: Config{},
			expected: Config{
				MaxRetries:            0,
				MinRetryDelay:         defaultRetryDelay,
				MaxRetryDelay:         defaultMaxRetryDelay,
				MaxBytes:              defaultMaxBytes,
				Timeout:               nil, // Optional fields remain nil if not set
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
		{
			name: "MaxRetries clamping",
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
			name: "MaxRetries upper bound clamping",
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
				Timeout:               durationPtr(defaultTimeout),
				TLSHandshakeTimeout:   durationPtr(defaultTLSHandshakeTimeout),
				IdleConnTimeout:       durationPtr(defaultIdleConnTimeout),
				ResponseHeaderTimeout: durationPtr(0), // No default for header timeout, just 0
			},
		},
		{
			name: "Connection limits negative values (use default or zero)",
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
				MaxIdleConnsPerHost: intPtr(0), // Default is 0 (net/http default)
				MaxConnsPerHost:     intPtr(0), // Unlimited
				MaxRedirects:        intPtr(defaultMaxRedirects),
			},
		},
		{
			name: "RetryDelay logic",
			input: Config{
				MinRetryDelay: 500 * time.Millisecond, // Should bump to 1s
				MaxRetryDelay: 2 * time.Second,
			},
			expected: Config{
				MaxRetries:    0,
				MinRetryDelay: 1 * time.Second,
				MaxRetryDelay: 2 * time.Second,
				MaxBytes:      defaultMaxBytes,
			},
		},
		{
			name: "RetryDelay logic - Max less than Min",
			input: Config{
				MinRetryDelay: 5 * time.Second,
				MaxRetryDelay: 2 * time.Second, // Should bump to Min
			},
			expected: Config{
				MaxRetries:    0,
				MinRetryDelay: 5 * time.Second,
				MaxRetryDelay: 5 * time.Second,
				MaxBytes:      defaultMaxBytes,
			},
		},
		{
			name: "MaxBytes NoLimit",
			input: Config{
				MaxBytes: -1,
			},
			expected: Config{
				MaxRetries:    0,
				MinRetryDelay: defaultRetryDelay,
				MaxRetryDelay: defaultMaxRetryDelay,
				MaxBytes:      -1,
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
// Tests for Fetcher Chain Construction
// =========================================================================

// getFieldViaReflection 은 비공개 필드 값을 리플렉션으로 읽어옵니다.
// 테스트 목적의 검증을 위해서만 사용합니다.
func getFieldViaReflection(obj interface{}, fieldName string) interface{} {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	field := val.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}
	// reflect.Value 포장 없이 원본 값 반환을 위해 Interface() 사용 시도
	// unexported field는 Interface() 호출 시 panic 발생하므로 unsafe Pointer로 접근하거나
	// 여기서는 단순히 타입 확인 및 nil 여부만 체크하는 것으로 타협할 수도 있으나,
	// test 패키지 내부이므로 검증의 정확성을 위해 reflect2 등을 쓰지 않는 한 값 접근은 제한적.
	//
	// Go 1.0+ 표준 reflect로는 unexported field의 값을 읽을 수 없음.
	// 따라서 값 검증 대신 타입 체인 구조 검증에 집중하고,
	// 꼭 필요한 경우 해당 구조체에 Test hook을 두거나, 구조체 생성을 신뢰함.
	//
	// 하지만 여기서는 "Expert Level" 요청이므로,
	// 각 Fetcher 구현체가 올바르게 감싸졌는지 '구조'를 확인하는 것에 집중한다.
	return field
}

func TestNewFromConfig_FullChain(t *testing.T) {
	cfg := Config{
		MaxRetries:                   3,
		MinRetryDelay:                2 * time.Second,
		MaxBytes:                     500,
		EnableUserAgentRandomization: true,
		UserAgents:                   []string{"Agent-A", "Agent-B"},
		AllowedStatusCodes:           []int{200, 201},
		AllowedMimeTypes:             []string{"application/json"},
		DisableLogging:               false,
	}
	// Apply defaults not needed if NewFromConfig calls it, but calling here helps predict values
	// NewFromConfig calls applyDefaults internally.

	f := NewFromConfig(cfg)

	// Chain Order Expected:
	// 1. LoggingFetcher
	// 2. UserAgentFetcher
	// 3. RetryFetcher
	// 4. MimeTypeFetcher
	// 5. StatusCodeFetcher
	// 6. MaxBytesFetcher
	// 7. HTTPFetcher

	// 1. LoggingFetcher
	logF, ok := f.(*LoggingFetcher)
	require.True(t, ok, "Outermost should be LoggingFetcher")
	require.NotNil(t, logF.delegate)

	// 2. UserAgentFetcher
	uaF, ok := logF.delegate.(*UserAgentFetcher)
	require.True(t, ok, "Second should be UserAgentFetcher")
	require.NotNil(t, uaF.delegate)

	// 3. RetryFetcher
	retryF, ok := uaF.delegate.(*RetryFetcher)
	require.True(t, ok, "Third should be RetryFetcher")
	require.NotNil(t, retryF.delegate)

	// 4. MimeTypeFetcher
	mimeF, ok := retryF.delegate.(*MimeTypeFetcher)
	require.True(t, ok, "Fourth should be MimeTypeFetcher")
	require.NotNil(t, mimeF.delegate)

	// 5. StatusCodeFetcher
	codeF, ok := mimeF.delegate.(*StatusCodeFetcher)
	require.True(t, ok, "Fifth should be StatusCodeFetcher")
	require.NotNil(t, codeF.delegate)

	// 6. MaxBytesFetcher
	bytesF, ok := codeF.delegate.(*MaxBytesFetcher)
	require.True(t, ok, "Sixth should be MaxBytesFetcher")
	require.NotNil(t, bytesF.delegate)

	// 7. HTTPFetcher
	httpF, ok := bytesF.delegate.(*HTTPFetcher)
	require.True(t, ok, "Innermost should be HTTPFetcher")
	require.NotNil(t, httpF)
}

func TestNewFromConfig_MinimalChain(t *testing.T) {
	cfg := Config{
		DisableLogging:               true,
		EnableUserAgentRandomization: false,
		AllowedMimeTypes:             nil, // Skip MimeType check
		AllowedStatusCodes:           nil, // Default check will be added? No, check logic.
		// Logic says: if !DisableStatusCodeValidation { if len > 0 { WithOptions } else { Default } }
		// So StatusCodeFetcher is ALWAYS added unless DisableStatusCodeValidation is true.
	}

	f := NewFromConfig(cfg)

	// Chain Order Expected:
	// (No Logging)
	// (No UserAgent)
	// 1. RetryFetcher
	// (No MimeType)
	// 2. StatusCodeFetcher (Default)
	// 3. MaxBytesFetcher
	// 4. HTTPFetcher

	// 1. RetryFetcher
	retryF, ok := f.(*RetryFetcher)
	require.True(t, ok, "Outermost should be RetryFetcher")

	// 2. StatusCodeFetcher
	codeF, ok := retryF.delegate.(*StatusCodeFetcher)
	require.True(t, ok, "Second should be StatusCodeFetcher")

	// 3. MaxBytesFetcher
	bytesF, ok := codeF.delegate.(*MaxBytesFetcher)
	require.True(t, ok, "Third should be MaxBytesFetcher")

	// 4. HTTPFetcher
	httpF, ok := bytesF.delegate.(*HTTPFetcher)
	require.True(t, ok, "Innermost should be HTTPFetcher")
	require.NotNil(t, httpF)
}

func TestNewFromConfig_DisableValidation(t *testing.T) {
	cfg := Config{
		DisableLogging:              true,
		DisableStatusCodeValidation: true,
	}

	f := NewFromConfig(cfg)

	// Chain Order Expected:
	// 1. RetryFetcher
	// (No StatusCodeFetcher)
	// 2. MaxBytesFetcher
	// 3. HTTPFetcher

	retryF, ok := f.(*RetryFetcher)
	require.True(t, ok, "Outermost should be RetryFetcher")

	// Should skip StatusCodeFetcher and go straight to MaxBytesFetcher
	bytesF, ok := retryF.delegate.(*MaxBytesFetcher)
	require.True(t, ok, "Should skip StatusCodeFetcher and go to MaxBytesFetcher")

	httpF, ok := bytesF.delegate.(*HTTPFetcher)
	require.True(t, ok, "Innermost should be HTTPFetcher")
	require.NotNil(t, httpF)
}

func TestNew(t *testing.T) {
	// New() convenience function test
	f := New(5, 2*time.Second, 1024)

	// Should have similar structure to minimal chain but with Retry settings set
	// Default chain includes Logging -> Retry ...
	logF, ok := f.(*LoggingFetcher)
	require.True(t, ok, "Should include LoggingFetcher by default")

	retryF, ok := logF.delegate.(*RetryFetcher)
	require.True(t, ok, "Should include RetryFetcher")

	// We can't verify values of retryF.maxRetries without reflection unsafe,
	// but the structure confirms the factory logic runs.
	assert.NotNil(t, retryF)
}

func TestNewFromConfig_Proxy(t *testing.T) {
	// Proxy settings verify
	cfg := Config{
		ProxyURL: stringPtr("http://proxy.local"),
	}
	f := NewFromConfig(cfg)
	require.NotNil(t, f)
	// This mainly verifies that setting ProxyURL doesn't panic and constructs a valid chain
}

func TestNewFromConfig_DisableCaching(t *testing.T) {
	cfg := Config{
		DisableTransportCaching: true,
	}
	f := NewFromConfig(cfg)
	require.NotNil(t, f)
	// Ensures construction with caching disabled works
}
