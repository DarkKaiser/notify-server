package fetcher

import (
	"container/list"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultTransport_Settings verifies that the shared default transport matches expected defaults.
func TestDefaultTransport_Settings(t *testing.T) {
	assert.Equal(t, DefaultMaxIdleConns, defaultTransport.MaxIdleConns)
	assert.Equal(t, DefaultMaxIdleConns, defaultTransport.MaxIdleConnsPerHost)
	assert.Equal(t, DefaultIdleConnTimeout, defaultTransport.IdleConnTimeout)
	assert.Equal(t, DefaultTLSHandshakeTimeout, defaultTransport.TLSHandshakeTimeout)
}

// TestHTTPFetcher_Close verifies that Close correctly cleans up isolated transports
// while leaving shared/default transports untouched.
func TestHTTPFetcher_Close(t *testing.T) {
	t.Run("Default Transport - No Op", func(t *testing.T) {
		f := NewHTTPFetcher() // uses defaultTransport
		err := f.Close()
		assert.NoError(t, err)
		// defaultTransport should remain active (cannot easily check "closed" state, but no panic)
	})

	t.Run("Shared Transport - No Op", func(t *testing.T) {
		// Uses shared cache
		f := NewHTTPFetcher(WithProxy("http://proxy.local:8080")) // creates/shares transport
		err := f.Close()
		assert.NoError(t, err)
	})

	t.Run("Isolated Transport - Closes Idle Connections", func(t *testing.T) {
		// Use DisableTransportCache to force isolated transport
		f := NewHTTPFetcher(WithDisableTransportCache(true))
		err := f.Close()
		assert.NoError(t, err)
		// Internal logic calls CloseIdleConnections.
		// We verify it doesn't panic.
	})
}

// TestTransportCache_Internal verifies the LRU and caching logic directly.
func TestTransportCache_Internal(t *testing.T) {
	// Reset cache for testing
	transportCacheMu.Lock()
	transportCache = make(map[transportCacheKey]*list.Element)
	transportCacheList.Init()
	transportCacheMu.Unlock()

	limit := DefaultMaxTransportCacheSize

	t.Run("LRU Eviction", func(t *testing.T) {
		transportCacheMu.Lock()
		transportCache = make(map[transportCacheKey]*list.Element)
		transportCacheList.Init()
		transportCacheMu.Unlock()

		// Fill cache to limit
		for i := 0; i < limit; i++ {
			key := transportCacheKey{maxIdleConns: i}
			_, err := getSharedTransport(key)
			require.NoError(t, err)
		}

		require.Equal(t, limit, transportCacheList.Len())

		// Add one more -> Should evict the oldest (index 0)
		key := transportCacheKey{maxIdleConns: limit + 1}
		_, err := getSharedTransport(key)
		require.NoError(t, err)

		transportCacheMu.RLock()
		_, ok := transportCache[transportCacheKey{maxIdleConns: 0}]
		assert.False(t, ok, "Oldest item should be evicted")
		transportCacheMu.RUnlock()
	})

	t.Run("Smart Eviction - Prefer Proxy", func(t *testing.T) {
		transportCacheMu.Lock()
		transportCache = make(map[transportCacheKey]*list.Element)
		transportCacheList.Init()
		transportCacheMu.Unlock()

		// Scenario:
		// 1. Fill cache with mostly direct connections (important).
		// 2. Add a few proxy connections (eviction candidates) at the END (recently used).
		// 3. Trigger eviction -> Should evict proxy even if it's recent, to protect direct connections.

		// 1. Fill with Direct connections
		for i := 0; i < limit-2; i++ {
			key := transportCacheKey{maxIdleConns: i} // Direct (no proxy)
			_, err := getSharedTransport(key)
			require.NoError(t, err)
		}

		// 2. Add Proxy connections (Recently used)
		proxyKey1 := transportCacheKey{proxyURL: "http://proxy1.local", maxIdleConns: 9991}
		proxyKey2 := transportCacheKey{proxyURL: "http://proxy2.local", maxIdleConns: 9992}

		_, err := getSharedTransport(proxyKey1)
		require.NoError(t, err)
		_, err = getSharedTransport(proxyKey2)
		require.NoError(t, err)

		// Assert conditions
		require.Equal(t, limit, transportCacheList.Len())
		// proxy2 is at Front (Most Recently Used)
		// proxy1 is next
		// Direct connections are at Back

		// 3. Add one more item to trigger eviction
		newKey := transportCacheKey{maxIdleConns: 8888}
		_, err = getSharedTransport(newKey)
		require.NoError(t, err)

		// Verification:
		// Smart Eviction searches from Back (Oldest) for 10 items.
		// Wait, our proxies are at Front (Newest).
		// The logic searches: `curr := transportCacheList.Back(); for i < 10 ...`
		// So it looks at the OLDEST 10 items.
		// If our proxies are Newest, they won't be found by the search loop.
		// So it should fall back to evicting the absolute oldest (Direct).

		// Let's adjust the test to match the logic's intent:
		// Put proxies in the "Oldest 10" zone.

		// Reset and retry logic match
		transportCacheMu.Lock()
		transportCache = make(map[transportCacheKey]*list.Element)
		transportCacheList.Init()
		transportCacheMu.Unlock()

		// A. Add Proxy connections FIRST (So they become Oldest)
		pk1 := transportCacheKey{proxyURL: "http://p1", maxIdleConns: 1}
		pk2 := transportCacheKey{proxyURL: "http://p2", maxIdleConns: 2}
		_, _ = getSharedTransport(pk1)
		_, _ = getSharedTransport(pk2)

		// B. Add Direct connections to fill the rest (Newest)
		for i := 0; i < limit-2; i++ {
			k := transportCacheKey{maxIdleConns: 100 + i}
			_, _ = getSharedTransport(k)
		}

		// Now:
		// Back (Oldest) -> pk1, pk2
		// Front (Newest) -> Direct...

		// C. Trigger eviction
		kNew := transportCacheKey{maxIdleConns: 9999}
		_, _ = getSharedTransport(kNew)

		// D. Verify: pk1 (Oldest Proxy) should be evicted.
		// Actually, pk1 is the absolute oldest AND a proxy.
		// So it would be evicted anyway by standard LRU.
		// To prove "Smart Eviction", we need:
		// Oldest = Direct
		// 2nd Oldest = Proxy.
		// If standard LRU -> Oldest (Direct) dies.
		// If Smart Eviction -> Proxy dies (even if 2nd oldest).

		// Let's try "Smart Eviction" proof scenario:
		transportCacheMu.Lock()
		transportCache = make(map[transportCacheKey]*list.Element)
		transportCacheList.Init()
		transportCacheMu.Unlock()

		// 1. Add Direct (Will be Absolute Oldest)
		directOld := transportCacheKey{maxIdleConns: 1000}
		_, _ = getSharedTransport(directOld)

		// 2. Add Proxy (Will be 2nd Oldest)
		proxyTarget := transportCacheKey{proxyURL: "http://target", maxIdleConns: 2000}
		_, _ = getSharedTransport(proxyTarget)

		// 3. Fill the rest with Direct
		for i := 0; i < limit-2; i++ {
			k := transportCacheKey{maxIdleConns: 3000 + i}
			_, _ = getSharedTransport(k)
		}

		// Current State:
		// Back -> [DirectOld] -> [ProxyTarget] -> ... -> Front

		// 4. Trigger Eviction
		_, err = getSharedTransport(transportCacheKey{maxIdleConns: 9999})
		require.NoError(t, err)

		// 5. Verify
		transportCacheMu.RLock()
		_, hasDirect := transportCache[directOld]
		_, hasProxy := transportCache[proxyTarget]
		transportCacheMu.RUnlock()

		assert.True(t, hasDirect, "Direct connection (Absolute Oldest) should be SPARED by smart eviction")
		assert.False(t, hasProxy, "Proxy connection (2nd Oldest) should be EVICTED by smart eviction")
	})

	t.Run("Concurrency & Double-Check", func(t *testing.T) {
		// Reset
		transportCacheMu.Lock()
		transportCache = make(map[transportCacheKey]*list.Element)
		transportCacheList.Init()
		transportCacheMu.Unlock()

		const goroutines = 20
		const keyCount = 5
		done := make(chan bool)

		for i := 0; i < goroutines; i++ {
			go func(id int) {
				// Use a mix of keys to cause collisions and creation
				key := transportCacheKey{maxIdleConns: id % keyCount}
				_, err := getSharedTransport(key)
				assert.NoError(t, err)

				// High concurrency read/write
				for j := 0; j < 100; j++ {
					k := transportCacheKey{maxIdleConns: j % keyCount}
					_, _ = getSharedTransport(k)
				}
				done <- true
			}(i)
		}

		for i := 0; i < goroutines; i++ {
			<-done
		}

		transportCacheMu.RLock()
		assert.LessOrEqual(t, len(transportCache), keyCount, "Should not exceed unique keys")
		transportCacheMu.RUnlock()
	})
}

func TestParameters_Application(t *testing.T) {
	key := transportCacheKey{
		proxyURL:              "http://user:pass@proxy.local:8080",
		maxIdleConns:          123,
		maxConnsPerHost:       45,
		idleConnTimeout:       5 * time.Second,
		tlsHandshakeTimeout:   2 * time.Second,
		responseHeaderTimeout: 3 * time.Second,
	}

	tr, err := newTransport(nil, key)
	require.NoError(t, err)

	// Verify Proxy
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	proxyURL, err := tr.Proxy(req)
	require.NoError(t, err)
	assert.Equal(t, "proxy.local:8080", proxyURL.Host)
	u := proxyURL.User.Username()
	assert.Equal(t, "user", u)

	// Verify Pooling
	assert.Equal(t, 123, tr.MaxIdleConns)
	assert.Equal(t, 123, tr.MaxIdleConnsPerHost) // Logic sets this same as MaxIdleConns
	assert.Equal(t, 45, tr.MaxConnsPerHost)

	// Verify Timeouts
	assert.Equal(t, 5*time.Second, tr.IdleConnTimeout)
	assert.Equal(t, 2*time.Second, tr.TLSHandshakeTimeout)
	assert.Equal(t, 3*time.Second, tr.ResponseHeaderTimeout)
}

func TestTransport_CopyOnWrite(t *testing.T) {
	baseTr := &http.Transport{
		MaxIdleConns: 10,
	}

	f := NewHTTPFetcher(WithMaxIdleConns(20)) // Requesting 20
	// Need to access internal method via interface or test hook?
	// configureTransportFromProvided is private.
	// However, setupTransport calls it if client.Transport is set.

	// Inject base transport
	f.client.Transport = baseTr

	// This looks tricky to call directly since setupTransport is private and called by Do().
	// But we are in the same package `fetcher`, so we can call methods on `f` if they are defined in `http.go`.
	// Wait, the file under test is `http_transport.go`, but these methods belong to `HTTPFetcher`.
	// Let's use `setupTransport` (private).

	err := f.setupTransport()
	require.NoError(t, err)

	// Result
	finalTr := f.client.Transport.(*http.Transport)

	// Should be the SAME object (no cloning)
	assert.Equal(t, baseTr, finalTr, "Spec: WithTransport ignores other options, so transport object should be reused")

	// Should have ORIGINAL settings (options ignored)
	assert.Equal(t, 10, finalTr.MaxIdleConns)

	// Base object should be untouched
	assert.Equal(t, 10, baseTr.MaxIdleConns)
}

// TestCreateTransport_Internal verifies internal helper logic.
func TestCreateTransport_Internal(t *testing.T) {
	t.Run("Proxy Redaction", func(t *testing.T) {
		// Verify that invalid proxy URL in key returns a safe error
		key := transportCacheKey{proxyURL: "http://user:secret@:invalid-port"}
		_, err := newTransport(nil, key)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "프록시 URL")
		assert.NotContains(t, err.Error(), "secret") // Password should be redacted
	})
}

// TestHTTPFetcher_TransportSelection verifies that correct transport (Default vs Shared vs Isolated) is selected.
func TestHTTPFetcher_TransportSelection(t *testing.T) {
	t.Run("Selects Default Transport", func(t *testing.T) {
		f := NewHTTPFetcher()
		assert.Equal(t, defaultTransport, f.client.Transport)
	})

	t.Run("Selects Shared Transport", func(t *testing.T) {
		// Using options that trigger customization -> shared cache
		f := NewHTTPFetcher(WithMaxIdleConns(50))
		tr, ok := f.client.Transport.(*http.Transport)
		require.True(t, ok)
		assert.NotEqual(t, defaultTransport, tr)
		assert.Equal(t, 50, tr.MaxIdleConns)
	})

	t.Run("Selects Isolated Transport", func(t *testing.T) {
		f := NewHTTPFetcher(WithDisableTransportCache(true))
		tr, ok := f.client.Transport.(*http.Transport)
		require.True(t, ok)

		// Verify isolation by mutation
		originalMaxIdle := defaultTransport.MaxIdleConns

		// Modify the isolated transport
		tr.MaxIdleConns = originalMaxIdle + 1

		// Assert that defaultTransport remains unchanged
		assert.Equal(t, originalMaxIdle, defaultTransport.MaxIdleConns, "defaultTransport should not be modified")
		assert.NotEqual(t, defaultTransport.MaxIdleConns, tr.MaxIdleConns, "Isolated transport should be modified")
	})
}

// TestRetryFetcher_Internal_Helpers tests internal helper behavior for RetryFetcher.
// Since we are in the same package, we can test internal methods/state if needed.
// (Moved from previous http_internal_test.go to preserve coverage)
func TestRetryFetcher_NonRetriableStatuses_Internal(t *testing.T) {
	// ... (Same logic as before, just consolidated)
	// This test essentially verifies IsRetriable logic via integration.

	tests := []struct {
		status    int
		retriable bool
	}{
		{http.StatusInternalServerError, true},
		{http.StatusNotImplemented, false},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				w.WriteHeader(tt.status)
			}))
			defer server.Close()

			f := NewHTTPFetcher()
			rf := NewRetryFetcher(f, 2, 1*time.Millisecond, 10*time.Millisecond)

			req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
			_, _ = rf.Do(req)

			if tt.retriable {
				assert.Equal(t, 3, callCount)
			} else {
				assert.Equal(t, 1, callCount)
			}
		})
	}
}
