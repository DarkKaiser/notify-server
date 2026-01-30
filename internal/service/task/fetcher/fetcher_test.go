package fetcher_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		ctx           context.Context
		mockSetup     func(*mockFetcher)
		checkResponse func(*testing.T, *http.Response)
		expectedError string
	}{
		{
			name: "Success (Status OK)",
			url:  "https://example.com/ok",
			ctx:  context.Background(),
			mockSetup: func(m *mockFetcher) {
				m.doFunc = func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, http.MethodGet, req.Method)
					assert.Equal(t, "https://example.com/ok", req.URL.String())
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("ok body")),
					}, nil
				}
			},
			checkResponse: func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				body, _ := io.ReadAll(resp.Body)
				assert.Equal(t, "ok body", string(body))
			},
		},
		{
			name: "Invalid URL",
			url:  "://invalid-url",
			ctx:  context.Background(),
			mockSetup: func(m *mockFetcher) {
				m.doFunc = func(req *http.Request) (*http.Response, error) {
					t.Fatal("Do should not be called")
					return nil, nil
				}
			},
			expectedError: "missing protocol scheme",
		},
		{
			name: "Context Deadline Exceeded",
			url:  "https://example.com/timeout",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Already canceled
				return ctx
			}(),
			mockSetup: func(m *mockFetcher) {
				// Request creation might succeed, but Do might fail or not be called depending on implementation细节.
				// userFetch.Get uses NewRequestWithContext -> Do.
				// NewRequestWithContext checks context? generic implementation doesn't necessarily fail immediately.
				// However, Get calls f.Do(req). Implementation of Do usually checks context.
				m.doFunc = func(req *http.Request) (*http.Response, error) {
					// Simulate context check in Do
					return nil, req.Context().Err()
				}
			},
			expectedError: "context canceled",
		},
		{
			name: "Fetcher Error (Network)",
			url:  "https://example.com/error",
			ctx:  context.Background(),
			mockSetup: func(m *mockFetcher) {
				m.doFunc = func(req *http.Request) (*http.Response, error) {
					return nil, errors.New("network failure")
				}
			},
			expectedError: "network failure",
		},
		{
			name: "Fetcher Error with Response (Drain Body)",
			url:  "https://example.com/500",
			ctx:  context.Background(),
			mockSetup: func(m *mockFetcher) {
				m.doFunc = func(req *http.Request) (*http.Response, error) {
					// Simulate a response body that needs draining
					mockBody := &mockReadCloser{
						readFunc: func(p []byte) (n int, err error) {
							return 0, io.EOF
						},
						closeFunc: func() error {
							return nil
						},
					}
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       mockBody,
					}, errors.New("request failed")
				}
			},
			expectedError: "request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockFetcher{}
			if tt.mockSetup != nil {
				tt.mockSetup(m)
			}

			resp, err := fetcher.Get(tt.ctx, m, tt.url)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				}
				// Cleanup
				if resp.Body != nil {
					resp.Body.Close()
				}
			}
		})
	}
}

func TestGet_BodyDrainLogic(t *testing.T) {
	// Specialized test to verify drainAndCloseBody is actually called
	m := &mockFetcher{}
	closed := false
	read := false

	m.doFunc = func(req *http.Request) (*http.Response, error) {
		mockBody := &mockReadCloser{
			readFunc: func(p []byte) (n int, err error) {
				read = true
				return 0, io.EOF
			},
			closeFunc: func() error {
				closed = true
				return nil
			},
		}
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       mockBody,
		}, errors.New("upstream error")
	}

	_, err := fetcher.Get(context.Background(), m, "https://example.com")
	assert.Error(t, err)
	assert.True(t, read, "Body should be read (drained)")
	assert.True(t, closed, "Body should be closed")
}

// Ensure context values are propagated
func TestGet_ContextPropagation(t *testing.T) {
	key := "my-key"
	val := "my-value"
	ctx := context.WithValue(context.Background(), key, val)

	m := &mockFetcher{
		doFunc: func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, val, req.Context().Value(key))
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}

	resp, err := fetcher.Get(ctx, m, "https://example.com")
	require.NoError(t, err)
	resp.Body.Close()
}

// =============================================================================
// Mocks
// =============================================================================

type mockFetcher struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockFetcher) Do(req *http.Request) (*http.Response, error) {
	if m.doFunc != nil {
		return m.doFunc(req)
	}
	return nil, errors.New("mockFetcher: Do not implemented")
}

type mockReadCloser struct {
	readFunc  func(p []byte) (n int, err error)
	closeFunc func() error
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.readFunc != nil {
		return m.readFunc(p)
	}
	return 0, io.EOF
}

func (m *mockReadCloser) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}
