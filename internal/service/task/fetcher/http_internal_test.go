package fetcher

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultTransport_Settings(t *testing.T) {
	assert.Equal(t, DefaultMaxIdleConns, defaultTransport.MaxIdleConns)
	assert.Equal(t, DefaultMaxIdleConns, defaultTransport.MaxIdleConnsPerHost)
	assert.Equal(t, DefaultIdleConnTimeout, defaultTransport.IdleConnTimeout)
	assert.Equal(t, DefaultTLSHandshakeTimeout, defaultTransport.TLSHandshakeTimeout)
}

func TestHTTPFetcher_InternalTransport(t *testing.T) {
	f := NewHTTPFetcher()
	assert.NotNil(t, f.client)

	transport, ok := f.client.Transport.(*http.Transport)
	assert.True(t, ok, "Transport should be *http.Transport")
	assert.Equal(t, DefaultMaxIdleConns, transport.MaxIdleConnsPerHost)
}

func TestTransportCache_Limit(t *testing.T) {
	transportMu.Lock()
	transportCache = make(map[transportKey]*list.Element)
	transportList.Init()
	transportMu.Unlock()

	for i := 0; i < maxTransportCacheSize; i++ {
		proxy := fmt.Sprintf("http://proxy-%d.com", i)
		_, err := getSharedTransport(proxy, 0, DefaultMaxIdleConns, DefaultIdleConnTimeout, DefaultTLSHandshakeTimeout, 0)
		assert.NoError(t, err)
	}

	transportMu.Lock()
	cacheSizeBefore := len(transportCache)
	transportMu.Unlock()
	assert.Equal(t, maxTransportCacheSize, cacheSizeBefore)

	newProxy := "http://overflow.com"
	_, err := getSharedTransport(newProxy, 0, DefaultMaxIdleConns, DefaultIdleConnTimeout, DefaultTLSHandshakeTimeout, 0)
	assert.NoError(t, err)

	transportMu.Lock()
	cacheSizeAfter := len(transportCache)
	_, exists := transportCache[transportKey{
		proxyURL:            newProxy,
		headerTimeout:       0,
		maxIdleConns:        DefaultMaxIdleConns,
		idleConnTimeout:     DefaultIdleConnTimeout,
		tlsHandshakeTimeout: DefaultTLSHandshakeTimeout,
		maxConnsPerHost:     0,
	}]
	transportMu.Unlock()

	assert.Equal(t, maxTransportCacheSize, cacheSizeAfter)
	assert.True(t, exists)
}

func TestTransportCache_LRU(t *testing.T) {
	transportMu.Lock()
	transportCache = make(map[transportKey]*list.Element)
	transportList.Init()
	oldLimit := maxTransportCacheSize
	transportMu.Unlock()

	for i := 0; i < oldLimit; i++ {
		proxy := fmt.Sprintf("http://proxy-%d.com", i)
		_, _ = getSharedTransport(proxy, 0, 0, 0, 0, 0)
	}

	for i := 0; i < 15; i++ {
		_, _ = getSharedTransport("http://proxy-0.com", 0, 0, 0, 0, 0)
	}

	_, _ = getSharedTransport("http://proxy-overflow.com", 0, 0, 0, 0, 0)

	transportMu.Lock()
	defer transportMu.Unlock()

	_, exists0 := transportCache[transportKey{proxyURL: "http://proxy-0.com"}]
	_, exists1 := transportCache[transportKey{proxyURL: "http://proxy-1.com"}]
	_, existsOverflow := transportCache[transportKey{proxyURL: "http://proxy-overflow.com"}]

	assert.True(t, exists0)
	assert.False(t, exists1)
	assert.True(t, existsOverflow)
}

func TestCreateTransport_CustomSettings(t *testing.T) {
	maxIdle := 50
	idleTimeout := 45 * time.Second
	tlsTimeout := 5 * time.Second
	headerTimeout := 3 * time.Second
	maxConns := 10

	tr, err := createTransport(nil, "", headerTimeout, maxIdle, idleTimeout, tlsTimeout, maxConns)
	assert.NoError(t, err)
	assert.Equal(t, maxIdle, tr.MaxIdleConns)
	assert.Equal(t, maxIdle, tr.MaxIdleConnsPerHost)
	assert.Equal(t, idleTimeout, tr.IdleConnTimeout)
	assert.Equal(t, tlsTimeout, tr.TLSHandshakeTimeout)
	assert.Equal(t, headerTimeout, tr.ResponseHeaderTimeout)
	assert.Equal(t, maxConns, tr.MaxConnsPerHost)
}

func TestCreateTransport_ProxyRedaction(t *testing.T) {
	invalidProxy := "http://user:pass@host:8080:extra"
	_, err := createTransport(nil, invalidProxy, 0, 0, 0, 0, 0)
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "pass")
}

func TestTransportCache_PriorityEviction(t *testing.T) {
	transportMu.Lock()
	transportCache = make(map[transportKey]*list.Element)
	transportList.Init()
	transportMu.Unlock()

	for i := 0; i < maxTransportCacheSize-1; i++ {
		_, _ = getSharedTransport("", time.Duration(i+1), 0, 0, 0, 0)
	}
	_, _ = getSharedTransport("http://proxy-to-keep.com", 0, 0, 0, 0, 0)

	transportMu.Lock()
	transportCache = make(map[transportKey]*list.Element)
	transportList.Init()
	transportMu.Unlock()

	for i := 0; i < 50; i++ {
		_, _ = getSharedTransport("", time.Duration(i+1), 0, 0, 0, 0)
	}
	proxyKey := transportKey{proxyURL: "http://target-proxy.com"}
	_, _ = getSharedTransport(proxyKey.proxyURL, 0, 0, 0, 0, 0)
	for i := 50; i < maxTransportCacheSize-1; i++ {
		_, _ = getSharedTransport("", time.Duration(i+1), 0, 0, 0, 0)
	}

	_, _ = getSharedTransport("http://new-one.com", 0, 0, 0, 0, 0)

	transportMu.Lock()
	_, basic0Exists := transportCache[transportKey{proxyURL: "", headerTimeout: time.Duration(1)}]
	_, proxyExists := transportCache[proxyKey]
	transportMu.Unlock()

	assert.False(t, basic0Exists)
	assert.True(t, proxyExists)
}

func TestTransportCache_LRU_Fix(t *testing.T) {
	transportMu.Lock()
	transportCache = make(map[transportKey]*list.Element)
	transportList.Init()
	transportMu.Unlock()

	configs := []struct {
		proxy string
		id    int
	}{
		{proxy: "http://proxy1:8080", id: 1},
		{proxy: "http://proxy2:8080", id: 2},
		{proxy: "http://proxy3:8080", id: 3},
	}

	for _, cfg := range configs {
		_, err := getSharedTransport(cfg.proxy, 0, 0, 0, 0, 0)
		require.NoError(t, err)
	}

	for i := 0; i < 15; i++ {
		_, err := getSharedTransport("http://proxy1:8080", 0, 0, 0, 0, 0)
		require.NoError(t, err)
	}

	transportMu.Lock()
	require.Equal(t, 3, transportList.Len())
	require.Equal(t, "http://proxy1:8080", transportList.Front().Value.(*transportCacheEntry).key.proxyURL)
	transportMu.Unlock()
}

func TestTransportCache_Concurrent(t *testing.T) {
	transportMu.Lock()
	transportCache = make(map[transportKey]*list.Element)
	transportList = list.New()
	transportMu.Unlock()

	const numGoroutines = 100
	const numRequests = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numRequests; j++ {
				tr, err := getSharedTransport("", 0, 100, 90*time.Second, 10*time.Second, 0)
				assert.NoError(t, err)
				assert.NotNil(t, tr)
			}
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 1, len(transportCache))
}

func TestHTTPFetcher_TransportIsolation_Internal(t *testing.T) {
	baseTr := &http.Transport{}
	proxyURL := "http://localhost:8080"
	f1 := NewHTTPFetcher(WithTransport(baseTr), WithProxy(proxyURL))
	f2 := NewHTTPFetcher(WithTransport(baseTr))

	tr1, ok := f1.client.Transport.(*http.Transport)
	assert.True(t, ok)
	assert.NotNil(t, tr1.Proxy)

	assert.Equal(t, baseTr, f2.client.Transport)
	assert.Nil(t, baseTr.Proxy)
}

func TestHTTPFetcher_RedirectReferer_Internal(t *testing.T) {
	var capturedReferer string
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReferer = r.Header.Get("Referer")
		w.WriteHeader(http.StatusOK)
	}))
	defer finalServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusFound)
	}))
	defer redirectServer.Close()

	f := NewHTTPFetcher()
	req, _ := http.NewRequest(http.MethodGet, redirectServer.URL, nil)
	resp, err := f.Do(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, redirectServer.URL, capturedReferer)
	if resp != nil {
		resp.Body.Close()
	}
}

func TestFetcherImprovements_MaxIdleConns(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"Default (-1)", -1, DefaultMaxIdleConns},
		{"Unlimited (0)", 0, 0},
		{"Custom (50)", 50, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{MaxIdleConns: tt.input, DisableLogging: true}
			cfg.ApplyDefaults()
			assert.Equal(t, tt.expected, cfg.MaxIdleConns)

			f := NewFromConfig(cfg)

			// Chain unwarpping
			curr := f
			var hf *HTTPFetcher
			for curr != nil {
				if v, ok := curr.(*HTTPFetcher); ok {
					hf = v
					break
				}
				switch v := curr.(type) {
				case *LoggingFetcher:
					curr = v.delegate
				case *RetryFetcher:
					curr = v.delegate
				case *StatusCodeFetcher:
					curr = v.delegate
				case *MaxBytesFetcher:
					curr = v.delegate
				case *MimeTypeFetcher:
					curr = v.delegate
				case *UserAgentFetcher:
					curr = v.delegate
				default:
					curr = nil
				}
			}
			assert.NotNil(t, hf)
			tr := hf.client.Transport.(*http.Transport)
			assert.Equal(t, tt.expected, tr.MaxIdleConns)
		})
	}
}

func TestRetryFetcher_NonRetriableStatuses_Internal(t *testing.T) {
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
			resp, err := rf.Do(req)

			if tt.retriable {
				assert.Equal(t, 3, callCount)
			} else {
				assert.Equal(t, 1, callCount)
			}
			if resp != nil {
				resp.Body.Close()
			}
			_ = err
		})
	}
}

func TestRetryFetcher_NoGetBody_Internal(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewBufferString("test body"))
	req.GetBody = nil

	f := NewRetryFetcher(&HTTPFetcher{}, 3, 10*time.Millisecond, 100*time.Millisecond)
	resp, err := f.Do(req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestHTTPStatusError_StructuredInfo_Internal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("custom error message"))
	}))
	defer ts.Close()

	f := NewHTTPFetcher()
	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := f.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	err = CheckResponseStatus(resp)
	var statusErr *HTTPStatusError
	require.True(t, errors.As(err, &statusErr))
	assert.Equal(t, http.StatusNotFound, statusErr.StatusCode)
	assert.Equal(t, "custom error message", statusErr.BodySnippet)
}

func TestRetryFetcher_GetBodyError_Internal(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Body = io.NopCloser(bytes.NewBufferString("test"))
	req.GetBody = func() (io.ReadCloser, error) {
		return nil, errors.New("get body failed")
	}

	mockFetcher := &mockFetcherInternal{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(bytes.NewBufferString("error")),
			}, nil
		},
	}

	f := NewRetryFetcher(mockFetcher, 3, 1*time.Millisecond, 10*time.Millisecond)
	resp, err := f.Do(req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "GetBody 함수 실행 중 오류가 발생하여 재시도를 위한 요청 본문 재생성에 실패했습니다")
}

type mockFetcherInternal struct {
	DoFunc func(*http.Request) (*http.Response, error)
}

func (m *mockFetcherInternal) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

func TestRetryFetcher_RetryAfterCap_Internal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	maxDelay := 100 * time.Millisecond
	f := NewRetryFetcher(NewHTTPFetcher(), 1, 10*time.Millisecond, maxDelay)

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	start := time.Now()
	_, _ = f.Do(req)
	duration := time.Since(start)
	// 시스템 오버헤드를 고려하여 1.5초로 여유있게 설정
	// maxDelay가 100ms이므로 정상적으로는 200ms 이내에 완료되어야 함
	assert.Less(t, duration, 1500*time.Millisecond)
}
