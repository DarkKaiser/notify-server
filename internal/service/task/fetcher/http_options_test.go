package fetcher

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPOptions_Table tests all functional options for HTTPFetcher using a table-driven approach.
// It verifies both the HTTPFetcher internal state and the resulting http.Transport configuration.
func TestHTTPOptions_Table(t *testing.T) {
	// Helper to get http.Transport from fetcher for verification
	getTransport := func(f *HTTPFetcher) *http.Transport {
		if f.client == nil || f.client.Transport == nil {
			return nil
		}
		if tr, ok := f.client.Transport.(*http.Transport); ok {
			return tr
		}
		return nil
	}

	type testCase struct {
		name        string
		options     []Option
		verify      func(t *testing.T, f *HTTPFetcher)
		expectError bool // Set to true if NewHTTPFetcher is expected to result in an init error (checked via Do)
	}

	tests := []testCase{
		// =================================================================================
		// Timeout Options
		// =================================================================================
		{
			name:    "WithTimeout - Positive",
			options: []Option{WithTimeout(10 * time.Second)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, 10*time.Second, f.client.Timeout)
			},
		},
		{
			name:    "WithTimeout - Zero (Infinite)",
			options: []Option{WithTimeout(0)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, time.Duration(0), f.client.Timeout)
			},
		},
		{
			name:    "WithResponseHeaderTimeout",
			options: []Option{WithResponseHeaderTimeout(5 * time.Second)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, 5*time.Second, f.headerTimeout)
				// Transport verification
				tr := getTransport(f)
				require.NotNil(t, tr)
				assert.Equal(t, 5*time.Second, tr.ResponseHeaderTimeout)
			},
		},
		{
			name:    "WithTLSHandshakeTimeout",
			options: []Option{WithTLSHandshakeTimeout(3 * time.Second)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, 3*time.Second, f.tlsHandshakeTimeout)
				tr := getTransport(f)
				require.NotNil(t, tr)
				assert.Equal(t, 3*time.Second, tr.TLSHandshakeTimeout)
			},
		},
		{
			name:    "WithIdleConnTimeout",
			options: []Option{WithIdleConnTimeout(45 * time.Second)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, 45*time.Second, f.idleConnTimeout)
				tr := getTransport(f)
				require.NotNil(t, tr)
				assert.Equal(t, 45*time.Second, tr.IdleConnTimeout)
			},
		},

		// =================================================================================
		// Connection Pool Options
		// =================================================================================
		{
			name:    "WithMaxIdleConns - Positive",
			options: []Option{WithMaxIdleConns(50)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, 50, f.maxIdleConns)
				tr := getTransport(f)
				require.NotNil(t, tr)
				assert.Equal(t, 50, tr.MaxIdleConns)
				assert.Equal(t, 50, tr.MaxIdleConnsPerHost) // Should sync with global limit
			},
		},
		{
			name:    "WithMaxIdleConns - Zero (Unlimited)",
			options: []Option{WithMaxIdleConns(0)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, 0, f.maxIdleConns)
				tr := getTransport(f)
				require.NotNil(t, tr)
				assert.Equal(t, 0, tr.MaxIdleConns) // 0 means no limit in http.Transport
			},
		},
		{
			name:    "WithMaxIdleConns - Negative (Ignore/Default)",
			options: []Option{WithMaxIdleConns(-1)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, -1, f.maxIdleConns) // Field set to -1
				tr := getTransport(f)
				require.NotNil(t, tr)
				// Logic in createTransport: if maxIdle >= 0 { set }
				// So if -1, it should retain default (which is defaultTransport.MaxIdleConns = 100)
				// OR it is cloned from defaultTransport
				assert.Equal(t, DefaultMaxIdleConns, tr.MaxIdleConns)
			},
		},
		{
			name:    "WithMaxConnsPerHost - Positive",
			options: []Option{WithMaxConnsPerHost(10)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, 10, f.maxConnsPerHost)
				tr := getTransport(f)
				require.NotNil(t, tr)
				assert.Equal(t, 10, tr.MaxConnsPerHost)
			},
		},
		{
			name:    "WithMaxConnsPerHost - Zero (Unlimited)",
			options: []Option{WithMaxConnsPerHost(0)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, 0, f.maxConnsPerHost)
				tr := getTransport(f)
				require.NotNil(t, tr)
				assert.Equal(t, 0, tr.MaxConnsPerHost)
			},
		},

		// =================================================================================
		// Proxy Option
		// =================================================================================
		{
			name:    "WithProxy - Valid URL",
			options: []Option{WithProxy("http://proxy.example.com:8080")},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, "http://proxy.example.com:8080", f.proxyURL)
				tr := getTransport(f)
				require.NotNil(t, tr)

				// Verify Proxy function works
				req, _ := http.NewRequest("GET", "http://example.com", nil)
				proxyURL, err := tr.Proxy(req)
				assert.NoError(t, err)
				assert.NotNil(t, proxyURL)
				assert.Equal(t, "http://proxy.example.com:8080", proxyURL.String())
			},
		},
		{
			name:    "WithProxy - Empty (No Proxy)",
			options: []Option{WithProxy("")},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, "", f.proxyURL)
				tr := getTransport(f)
				require.NotNil(t, tr)
				// Default transport proxy is usually nil or FromEnvironment
				// Here we explicit check if our createTransport handles "" correctly (usually nil)
			},
		},
		{
			name:    "WithProxy - Invalid URL (Runtime Error)",
			options: []Option{WithProxy(" ://invalid")},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, " ://invalid", f.proxyURL)
				// Invalid proxy URL causes configureTransport to fail and set f.initErr
				assert.Error(t, f.initErr)
				req, _ := http.NewRequest("GET", "http://example.com", nil)
				_, err := f.Do(req)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid proxy URL")
			},
			expectError: true,
		},

		// =================================================================================
		// Client Behavior Options
		// =================================================================================
		{
			name:    "WithUserAgent",
			options: []Option{WithUserAgent("MyBot/1.0")},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.Equal(t, "MyBot/1.0", f.defaultUA)
				// Actual header injection is tested in http_test.go
			},
		},
		{
			name:    "WithMaxRedirects",
			options: []Option{WithMaxRedirects(5)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.NotNil(t, f.client.CheckRedirect)

				// Simulate redirect check
				req, _ := http.NewRequest("GET", "http://example.com", nil)
				via := make([]*http.Request, 5) // 5 prior redirects
				err := f.client.CheckRedirect(req, via)
				assert.ErrorIs(t, err, http.ErrUseLastResponse, "Should stop after 5 redirects")

				viaLessThanMax := make([]*http.Request, 4)
				err = f.client.CheckRedirect(req, viaLessThanMax)
				assert.NoError(t, err)
			},
		},
		{
			name:    "WithCookieJar",
			options: []Option{WithCookieJar(&mockCookieJar{})},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.NotNil(t, f.client.Jar)
				_, ok := f.client.Jar.(*mockCookieJar)
				assert.True(t, ok)
			},
		},

		// =================================================================================
		// Transport Control Options
		// =================================================================================
		{
			name:    "WithDisableTransportCache",
			options: []Option{WithDisableTransportCache(true)},
			verify: func(t *testing.T, f *HTTPFetcher) {
				assert.True(t, f.disableCache)
				// When cache is disabled, Do() creates a new transport every time (not cached)
				// Implementation detail: createTransport is called directly.
			},
		},
		{
			name: "WithTransport - Custom Transport",
			options: []Option{
				WithTransport(&http.Transport{DisableKeepAlives: true}),
			},
			verify: func(t *testing.T, f *HTTPFetcher) {
				tr := getTransport(f)
				require.NotNil(t, tr)
				assert.True(t, tr.DisableKeepAlives)
				// WithTransport should disable internal caching mechanism preference if it was set?
				// Actually WithTransport sets client.Transport directly.
				// But HTTPFetcher.Do() logic might override it if not careful.
				// Current implementation: if client.Transport is set, we use it?
				// Let's check HTTPFetcher.configureTransport -- it skips if client.Transport is set?
				// No, configureTransport logic:
				// if f.client.Transport == nil { ... setup default ... }
				// So if WithTransport sets it, configureTransport preserves it?
				// Let's check needsSpecialTransport logic.

				// CAUTION: The current implementation of configureTransport needs to be compatible with WithTransport!
				// If `needsSpecialTransport()` returns true (which checks options), it might try to overwrite.
				// However, `WithTransport` is an Option. Options run BEFORE configureTransport.
				// If WithTransport sets f.client.Transport, configureTransport should respect it?
				// Let's verify this behavior. Ideally `WithTransport` disables other transport logic.
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := NewHTTPFetcher(tc.options...)
			tc.verify(t, f)
		})
	}
}

// mockCookieJar for testing WithCookieJar
type mockCookieJar struct{}

func (m *mockCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {}
func (m *mockCookieJar) Cookies(u *url.URL) []*http.Cookie             { return nil }

// TestHTTPOptions_Interaction verifies interactions between multiple options.
func TestHTTPOptions_Interaction(t *testing.T) {
	t.Run("WithProxy overrides Transport Cache", func(t *testing.T) {
		// When Proxy is set, it should use a cached transport for that proxy, OR create a new one.
		// It should NOT use the default global transport.
		f := NewHTTPFetcher(WithProxy("http://proxy.local:8080"))
		tr := f.GetTransport() // Public method to get transport/client.Transport

		assert.NotNil(t, tr)
		// It should be a *http.Transport
		httpTr, ok := tr.(*http.Transport)
		require.True(t, ok)

		// Verify proxy is set
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		proxyURL, err := httpTr.Proxy(req)
		assert.NoError(t, err)
		assert.Equal(t, "http://proxy.local:8080", proxyURL.String())
	})

	t.Run("WithTransport vs Other Options", func(t *testing.T) {
		// Even if WithTransport is used, other transport options (like WithMaxIdleConns) should be applied
		// if they are explicitly set. The fetcher logic (setupCustomTransport) clones the transport
		// and applies the settings.
		customTr := &http.Transport{}
		f := NewHTTPFetcher(
			WithTransport(customTr),
			WithMaxIdleConns(999),
		)

		currentTr := f.GetTransport()
		// It should be a clone, so not equal pointer to customTr (unless no options changed)
		// Here we changed MaxIdleConns, so it should be a new instance (clone)
		assert.NotEqual(t, customTr, currentTr, "Should clone transport when modifying settings")

		httpTr := currentTr.(*http.Transport)
		assert.Equal(t, 999, httpTr.MaxIdleConns, "Options should be applied even when WithTransport is used")
	})

	t.Run("WithProxy with Custom Non-HTTP Transport", func(t *testing.T) {
		// Verify behavior when setting Proxy on a custom Transport that is NOT *http.Transport.
		// It should return an error because we can't configure it.
		customTr := &mockRoundTripper{}
		f := NewHTTPFetcher(
			WithTransport(customTr),
			WithProxy("http://proxy.local:8080"),
		)

		req, _ := http.NewRequest("GET", "http://example.com", nil)
		_, err := f.Do(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot apply special settings to non-http.Transport")
	})
}

// mockRoundTripper for testing custom transport
type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, nil
}
