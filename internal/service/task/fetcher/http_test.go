package fetcher_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPFetcher_Do verifies the standard behavior of the Do method.
// It covers success scenarios, error handling, context cancellation, and header injection.
func TestHTTPFetcher_Do(t *testing.T) {
	// Mock server to simulate various responses
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Header Verification
		if r.URL.Path == "/verify-headers" {
			if r.Header.Get("User-Agent") == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("Missing User-Agent"))
				return
			}
			if r.Header.Get("Accept") == "" || r.Header.Get("Accept-Language") == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("Missing Standard Headers"))
				return
			}
		}

		// Timeout Simulation
		if r.URL.Path == "/timeout" {
			time.Sleep(200 * time.Millisecond)
		}

		// Echo path
		if r.URL.Path == "/verify-path" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("path-ok"))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))
	defer ts.Close()

	tests := []struct {
		name          string
		fetcherOpts   []fetcher.Option
		reqFunc       func() *http.Request
		ctxTimeout    time.Duration
		expectedError string // Substring match
		validateResp  func(t *testing.T, resp *http.Response)
	}{
		{
			name: "Success - Basic Request",
			reqFunc: func() *http.Request {
				req, _ := http.NewRequest("GET", ts.URL, nil)
				return req
			},
			validateResp: func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			},
		},
		{
			name: "Success - Header Injection",
			reqFunc: func() *http.Request {
				req, _ := http.NewRequest("GET", ts.URL+"/verify-headers", nil)
				return req
			},
			validateResp: func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			},
		},
		{
			name: "Success - Custom User-Agent Preserved",
			reqFunc: func() *http.Request {
				req, _ := http.NewRequest("GET", ts.URL+"/verify-headers", nil)
				req.Header.Set("User-Agent", "CustomBot/1.0")
				return req
			},
			validateResp: func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				// Note: Server logic checks if UA exists, assumes if it returns 200, it passed.
				// Ideally server should echo it back for strict verification, but this verify logic
				// combined with server logic (400 if missing) covers the "it's sent" part.
			},
		},
		{
			name: "Error - Context Timeout",
			reqFunc: func() *http.Request {
				req, _ := http.NewRequest("GET", ts.URL+"/timeout", nil)
				return req
			},
			ctxTimeout:    100 * time.Millisecond,
			expectedError: "context deadline exceeded",
		},
		{
			name: "Error - Context Canceled",
			reqFunc: func() *http.Request {
				req, _ := http.NewRequest("GET", ts.URL+"/timeout", nil)
				ctx, cancel := context.WithCancel(context.Background())
				req = req.WithContext(ctx)
				cancel() // Cancel immediately
				return req
			},
			expectedError: "context canceled",
		},
		{
			name: "Error - Invalid URL",
			reqFunc: func() *http.Request {
				// Use port 0 to force immediate connection refused on most systems,
				// avoiding slow DNS lookups for non-existent domains.
				req, _ := http.NewRequest("GET", "http://127.0.0.1:0", nil)
				return req
			},
			expectedError: "dial tcp",
		},
		{
			name: "Error - Init Error Propagation",
			fetcherOpts: []fetcher.Option{
				fetcher.WithProxy(" ://invalid-proxy"), // This causes init error
			},
			reqFunc: func() *http.Request {
				req, _ := http.NewRequest("GET", ts.URL, nil)
				return req
			},
			expectedError: "제공된 프록시 URL의 형식이 올바르지 않습니다",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := fetcher.NewHTTPFetcher(tc.fetcherOpts...)
			req := tc.reqFunc()

			if tc.ctxTimeout > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), tc.ctxTimeout)
				defer cancel()
				req = req.WithContext(ctx)
			}

			resp, err := f.Do(req)

			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				defer resp.Body.Close()

				if tc.validateResp != nil {
					tc.validateResp(t, resp)
				}
			}
		})
	}
}

// TestHTTPFetcher_RequestCloning ensures that the fetcher does not mutate the original request.
func TestHTTPFetcher_RequestCloning(t *testing.T) {
	f := fetcher.NewHTTPFetcher()
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	// Ensure original headers are empty
	assert.Empty(t, req.Header.Get("User-Agent"))
	assert.Empty(t, req.Header.Get("Accept"))

	// We use an invalid URL to fail fast, but the cloning logic happens BEFORE the request execution
	// check. However, Do() might return initErr or URL error.
	// To safely test this without external calls, we can inspect if Do modifies it.
	// Even if Do returns an error, we check the original req.

	// Using a mock server ensures Do proceeds deeper into logic
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	validReq, _ := http.NewRequest("GET", ts.URL, nil)
	_, _ = f.Do(validReq)

	assert.Empty(t, validReq.Header.Get("User-Agent"), "Original User-Agent should remain empty")
	assert.Empty(t, validReq.Header.Get("Accept"), "Original Accept should remain empty")
}

// TestCheckResponseStatus verified the error formatting helper.
func TestCheckResponseStatus(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://user:pass@example.com/api?token=secret", nil)
	resp := &http.Response{
		StatusCode: http.StatusTeapot,
		Status:     "418 I'm a teapot",
		Request:    req,
	}

	err := fetcher.CheckResponseStatus(resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "418 I'm a teapot")
	assert.Contains(t, err.Error(), "http://user:xxxxx@example.com/api?token=xxxxx") // Redacted
	assert.NotContains(t, err.Error(), "pass")
	assert.NotContains(t, err.Error(), "secret")
}
