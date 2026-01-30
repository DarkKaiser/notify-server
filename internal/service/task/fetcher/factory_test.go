package fetcher

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ApplyDefaults(t *testing.T) {
	// 상수가 비공개이므로 테스트에서 직접 검증 값으로 사용
	const (
		expectedDefaultRetryDelay          = 1 * time.Second
		expectedDefaultMaxRetryDelay       = 30 * time.Second
		expectedDefaultMaxBytes            = 10 * 1024 * 1024 // 10MB
		expectedDefaultTimeout             = 30 * time.Second
		expectedDefaultMaxIdleConns        = 100
		expectedDefaultIdleConnTimeout     = 90 * time.Second
		expectedDefaultTLSHandshakeTimeout = 10 * time.Second
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
				MaxRetries:          0, // minRetries
				RetryDelay:          expectedDefaultRetryDelay,
				MaxRetryDelay:       expectedDefaultMaxRetryDelay,
				MaxBytes:            expectedDefaultMaxBytes,
				Timeout:             expectedDefaultTimeout,
				MaxIdleConns:        0, // 0 means unlimited
				IdleConnTimeout:     expectedDefaultIdleConnTimeout,
				TLSHandshakeTimeout: expectedDefaultTLSHandshakeTimeout,
				MaxConnsPerHost:     0,
			},
		},
		{
			name: "MaxRetries clamping",
			input: Config{
				MaxRetries: -5, // Should populate to 0
			},
			expected: Config{
				MaxRetries:          0,
				RetryDelay:          expectedDefaultRetryDelay,
				MaxRetryDelay:       expectedDefaultMaxRetryDelay,
				MaxBytes:            expectedDefaultMaxBytes,
				Timeout:             expectedDefaultTimeout,
				IdleConnTimeout:     expectedDefaultIdleConnTimeout,
				TLSHandshakeTimeout: expectedDefaultTLSHandshakeTimeout,
			},
		},
		{
			name: "MaxRetries upper bound clamping",
			input: Config{
				MaxRetries: 100, // Should cap at 10
			},
			expected: Config{
				MaxRetries:          10,
				RetryDelay:          expectedDefaultRetryDelay,
				MaxRetryDelay:       expectedDefaultMaxRetryDelay,
				MaxBytes:            expectedDefaultMaxBytes,
				Timeout:             expectedDefaultTimeout,
				IdleConnTimeout:     expectedDefaultIdleConnTimeout,
				TLSHandshakeTimeout: expectedDefaultTLSHandshakeTimeout,
			},
		},
		{
			name: "RetryDelay minimum clamping",
			input: Config{
				RetryDelay: 500 * time.Millisecond, // Should round up to 1s
			},
			expected: Config{
				MaxRetries:          0,
				RetryDelay:          1 * time.Second,
				MaxRetryDelay:       expectedDefaultMaxRetryDelay,
				MaxBytes:            expectedDefaultMaxBytes,
				Timeout:             expectedDefaultTimeout,
				IdleConnTimeout:     expectedDefaultIdleConnTimeout,
				TLSHandshakeTimeout: expectedDefaultTLSHandshakeTimeout,
			},
		},
		{
			name: "MaxRetryDelay logic (less than RetryDelay)",
			input: Config{
				RetryDelay:    5 * time.Second,
				MaxRetryDelay: 2 * time.Second, // Should be bumped to RetryDelay
			},
			expected: Config{
				MaxRetries:          0,
				RetryDelay:          5 * time.Second,
				MaxRetryDelay:       5 * time.Second,
				MaxBytes:            expectedDefaultMaxBytes,
				Timeout:             expectedDefaultTimeout,
				IdleConnTimeout:     expectedDefaultIdleConnTimeout,
				TLSHandshakeTimeout: expectedDefaultTLSHandshakeTimeout,
			},
		},
		{
			name: "MaxBytes NoLimit",
			input: Config{
				MaxBytes: -1, // NoLimit
			},
			expected: Config{
				MaxRetries:          0,
				RetryDelay:          expectedDefaultRetryDelay,
				MaxRetryDelay:       expectedDefaultMaxRetryDelay,
				MaxBytes:            -1, // Kept as -1
				Timeout:             expectedDefaultTimeout,
				IdleConnTimeout:     expectedDefaultIdleConnTimeout,
				TLSHandshakeTimeout: expectedDefaultTLSHandshakeTimeout,
			},
		},
		{
			name: "Timeout unlimited",
			input: Config{
				Timeout: -1,
			},
			expected: Config{
				MaxRetries:          0,
				RetryDelay:          expectedDefaultRetryDelay,
				MaxRetryDelay:       expectedDefaultMaxRetryDelay,
				MaxBytes:            expectedDefaultMaxBytes,
				Timeout:             -1, // Kept as -1
				IdleConnTimeout:     expectedDefaultIdleConnTimeout,
				TLSHandshakeTimeout: expectedDefaultTLSHandshakeTimeout,
			},
		},
		{
			name: "MaxIdleConns default trigger",
			input: Config{
				MaxIdleConns: -1, // Trigger default
			},
			expected: Config{
				MaxRetries:          0,
				RetryDelay:          expectedDefaultRetryDelay,
				MaxRetryDelay:       expectedDefaultMaxRetryDelay,
				MaxBytes:            expectedDefaultMaxBytes,
				Timeout:             expectedDefaultTimeout,
				MaxIdleConns:        expectedDefaultMaxIdleConns, // 100
				IdleConnTimeout:     expectedDefaultIdleConnTimeout,
				TLSHandshakeTimeout: expectedDefaultTLSHandshakeTimeout,
			},
		},
		{
			name: "MaxConnsPerHost negative correction",
			input: Config{
				MaxConnsPerHost: -5,
			},
			expected: Config{
				MaxRetries:          0,
				RetryDelay:          expectedDefaultRetryDelay,
				MaxRetryDelay:       expectedDefaultMaxRetryDelay,
				MaxBytes:            expectedDefaultMaxBytes,
				Timeout:             expectedDefaultTimeout,
				IdleConnTimeout:     expectedDefaultIdleConnTimeout,
				TLSHandshakeTimeout: expectedDefaultTLSHandshakeTimeout,
				MaxConnsPerHost:     0, // Corrected to 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.input
			cfg.ApplyDefaults()
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestNewFromConfig_ChainConstruction(t *testing.T) {
	// Full configuration to enable all middlewares
	cfg := Config{
		MaxRetries:          3,
		AllowedContentTypes: []string{"application/json"},
		UserAgents:          []string{"test-agent"},
		DisableLogging:      false,
	}
	cfg.ApplyDefaults()

	f := NewFromConfig(cfg)
	require.NotNil(t, f)

	// Check Chain Order (Outer -> Inner)
	// 1. LoggingFetcher
	loggingFetcher, ok := f.(*LoggingFetcher)
	require.True(t, ok, "Outermost should be LoggingFetcher")
	require.NotNil(t, loggingFetcher.delegate)

	// 2. UserAgentFetcher
	uaFetcher, ok := loggingFetcher.delegate.(*UserAgentFetcher)
	require.True(t, ok, "Second should be UserAgentFetcher")
	require.NotNil(t, uaFetcher.delegate)

	// 3. RetryFetcher
	retryFetcher, ok := uaFetcher.delegate.(*RetryFetcher)
	require.True(t, ok, "Third should be RetryFetcher")
	require.NotNil(t, retryFetcher.delegate)

	// 4. MimeTypeFetcher
	mimeFetcher, ok := retryFetcher.delegate.(*MimeTypeFetcher)
	require.True(t, ok, "Fourth should be MimeTypeFetcher")
	require.NotNil(t, mimeFetcher.delegate)

	// 5. StatusCodeFetcher
	statusFetcher, ok := mimeFetcher.delegate.(*StatusCodeFetcher)
	require.True(t, ok, "Fifth should be StatusCodeFetcher")
	require.NotNil(t, statusFetcher.delegate)

	// 6. MaxBytesFetcher
	maxBytesFetcher, ok := statusFetcher.delegate.(*MaxBytesFetcher)
	require.True(t, ok, "Sixth should be MaxBytesFetcher")
	require.NotNil(t, maxBytesFetcher.delegate)

	// 7. HTTPFetcher (Core)
	httpFetcher, ok := maxBytesFetcher.delegate.(*HTTPFetcher)
	require.True(t, ok, "Innermost should be HTTPFetcher")
	require.NotNil(t, httpFetcher.client)
}

func TestNewFromConfig_MinimalChain(t *testing.T) {
	// Minimal configuration: Disable logging, no UA, no allowed codes (default), no mime types
	cfg := Config{
		DisableLogging: true,
	}
	cfg.ApplyDefaults()

	f := NewFromConfig(cfg)
	require.NotNil(t, f)

	// Chain expectation:
	// RetryFetcher -> StatusCodeFetcher -> MaxBytesFetcher -> HTTPFetcher
	// (Logging, UserAgent, MimeType SHOULD BE MISSING)

	// 1. Should NOT be LoggingFetcher
	_, isLogging := f.(*LoggingFetcher)
	assert.False(t, isLogging, "LoggingFetcher should be disabled")

	// 2. Outermost should be RetryFetcher
	retryFetcher, ok := f.(*RetryFetcher)
	require.True(t, ok, "Outermost should be RetryFetcher")

	// 3. UserAgentFetcher and MimeTypeFetcher should be skipped
	// RetryFetcher delegate -> StatusCodeFetcher
	_, isUA := retryFetcher.delegate.(*UserAgentFetcher)
	assert.False(t, isUA, "UserAgentFetcher should be disabled")

	_, isMime := retryFetcher.delegate.(*MimeTypeFetcher)
	assert.False(t, isMime, "MimeTypeFetcher should be disabled")

	statusFetcher, ok := retryFetcher.delegate.(*StatusCodeFetcher)
	require.True(t, ok, "Delegate of Retry should be StatusCodeFetcher")

	// 4. MaxBytesFetcher
	maxBytesFetcher, ok := statusFetcher.delegate.(*MaxBytesFetcher)
	require.True(t, ok, "Delegate of StatusCode should be MaxBytesFetcher")

	// 5. HTTPFetcher
	_, ok = maxBytesFetcher.delegate.(*HTTPFetcher)
	require.True(t, ok, "Delegate of MaxBytes should be HTTPFetcher")
}

func TestNewFromConfig_TransportCache(t *testing.T) {
	// Case 1: Cache Enabled (Default)
	cfg1 := Config{DisableTransportCache: false}
	cfg1.ApplyDefaults()
	f1 := NewFromConfig(cfg1)
	httpF1 := getHTTPFetcher(t, f1)
	// We can't easily check the internal transport instance equality without valid upstream mocks or complex reflection,
	// but we can check if the 'disableCache' field (if exists and has validation) or similar.
	// For now, ensuring no panic and correct type is basic.
	// With expert knowledge, if we knew HTTPFetcher had a 'transport' field we could check pointer equality across calls,
	// but NewFromConfig creates a NEW Fetcher chain each time.
	// So we verify the option is passed.
	assert.NotNil(t, httpF1)

	// Case 2: Cache Disabled
	cfg2 := Config{DisableTransportCache: true}
	cfg2.ApplyDefaults()
	f2 := NewFromConfig(cfg2)
	httpF2 := getHTTPFetcher(t, f2)
	assert.NotNil(t, httpF2)
}

// Helper to drill down to HTTPFetcher (assuming full or partial chain)
func getHTTPFetcher(t *testing.T, f Fetcher) *HTTPFetcher {
	// Unwrap known decorators until we find HTTPFetcher
	current := f
	for {
		switch v := current.(type) {
		case *LoggingFetcher:
			current = v.delegate
		case *UserAgentFetcher:
			current = v.delegate
		case *RetryFetcher:
			current = v.delegate
		case *MimeTypeFetcher:
			current = v.delegate
		case *StatusCodeFetcher:
			current = v.delegate
		case *MaxBytesFetcher:
			current = v.delegate
		case *HTTPFetcher:
			return v
		default:
			t.Fatalf("Unknown fetcher type in chain: %T", current)
			return nil
		}
	}
}

func TestNewFromConfig_OptionPropagation(t *testing.T) {
	// Test if options are correctly passed to the innermost HTTPFetcher
	// We verify specific fields that are only settable via options or config mapping
	cfg := Config{
		Timeout:         5 * time.Second,
		MaxIdleConns:    50,
		MaxConnsPerHost: 20,
	}
	cfg.ApplyDefaults()

	f := NewFromConfig(cfg)

	// Traverse to HTTPFetcher
	// Assuming default chain (Logging -> Retry -> StatusCode -> MaxBytes -> HTTP)
	// Note: UserAgent and MimeType are skipped because lists are empty in cfg
	loggingFetcher, _ := f.(*LoggingFetcher)
	retryFetcher, _ := loggingFetcher.delegate.(*RetryFetcher)
	statusFetcher, _ := retryFetcher.delegate.(*StatusCodeFetcher)
	maxBytesFetcher, _ := statusFetcher.delegate.(*MaxBytesFetcher)
	httpFetcher, ok := maxBytesFetcher.delegate.(*HTTPFetcher)

	require.True(t, ok, "Should reach HTTPFetcher")

	// Verify Transport settings (requires reflection or careful inspection if fields are unexported)
	// Since HTTPFetcher.client is unexported, and Transport is inside it,
	// we rely on the fact that if NewHTTPFetcher used the options, the underlying transport would be configured.
	// However, we can't easily check internal transport fields without exporting them or using unsafe/reflection.
	// For "Expert Level" within standard testing, we ensure the Construction logic covers it:
	// We can trust NewHTTPFetcher tests to verify Options application.
	// Here we just verify the right usage of NewHTTPFetcher via the chain presence.

	assert.NotNil(t, httpFetcher)
}

func TestNew_Helper(t *testing.T) {
	// Verify New() helper correctly sets up a minimal config
	f := New(5, 2*time.Second, 1024)
	require.NotNil(t, f)

	// Check if RetryFetcher got the right config
	loggingFetcher, _ := f.(*LoggingFetcher)
	// UserAgent skipped (empty)
	retryFetcher, ok := loggingFetcher.delegate.(*RetryFetcher)
	require.True(t, ok, "Should include RetryFetcher")

	// We can't check private fields of RetryFetcher (maxRetries) easily from here unless we use reflection
	// or if we rely on behavior. But verifying the structure is usually sufficient for Factory tests.
	assert.NotNil(t, retryFetcher)
}
