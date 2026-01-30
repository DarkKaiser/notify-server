package fetcher

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusCodeFetcher_Do(t *testing.T) {
	tests := []struct {
		name            string
		status          int
		responseBody    string
		expectError     bool
		expectedErrType apperrors.ErrorType
		containsSnippet bool
	}{
		{
			name:         "Success 200 OK",
			status:       http.StatusOK,
			responseBody: "success content",
			expectError:  false,
		},
		{
			name:            "Error 404 Not Found",
			status:          http.StatusNotFound,
			responseBody:    "page not found",
			expectError:     true,
			expectedErrType: apperrors.NotFound,
			containsSnippet: true,
		},
		{
			name:            "Error 500 Internal Server Error",
			status:          http.StatusInternalServerError,
			responseBody:    "server error details",
			expectError:     true,
			expectedErrType: apperrors.Unavailable,
			containsSnippet: true,
		},
		{
			name:            "Error 429 Too Many Requests",
			status:          http.StatusTooManyRequests,
			responseBody:    "rate limit exceeded",
			expectError:     true,
			expectedErrType: apperrors.Unavailable,
			containsSnippet: true,
		},
		{
			name:            "Error 403 Forbidden",
			status:          http.StatusForbidden,
			responseBody:    "access denied",
			expectError:     true,
			expectedErrType: apperrors.Forbidden,
			containsSnippet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock delegate
			delegate := &mockFetcher{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					resp := httptest.NewRecorder()
					resp.WriteHeader(tt.status)
					_, _ = resp.WriteString(tt.responseBody)
					result := resp.Result()
					result.Request = req // CheckResponseStatus uses resp.Request
					return result, nil
				},
			}

			f := NewStatusCodeFetcher(delegate)
			resp, err := f.Do(httptest.NewRequest(http.MethodGet, "http://example.com", nil))

			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, apperrors.Is(err, tt.expectedErrType))
				if tt.containsSnippet {
					assert.Contains(t, err.Error(), tt.responseBody)
				}
				assert.Nil(t, resp, "Response should be nil on status error to prevent resource leaks")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				body, _ := io.ReadAll(resp.Body)
				assert.Equal(t, tt.responseBody, string(body))
				resp.Body.Close()
			}
		})
	}
}

// mockFetcher is a helper for testing
type mockFetcher struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockFetcher) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

func TestStatusCodeFetcher_WithAllowedStatuses(t *testing.T) {
	delegate := &mockFetcher{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			resp := httptest.NewRecorder()
			resp.WriteHeader(http.StatusNotFound)
			return resp.Result(), nil
		},
	}

	// 1. Default (only 200 OK)
	f1 := NewStatusCodeFetcher(delegate)
	_, err := f1.Do(httptest.NewRequest(http.MethodGet, "http://example.com", nil))
	assert.Error(t, err)

	// 2. With 404 Allowed
	f2 := NewStatusCodeFetcherWithOptions(delegate, http.StatusNotFound)
	resp, err := f2.Do(httptest.NewRequest(http.MethodGet, "http://example.com", nil))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

type trackCloseBody struct {
	io.ReadCloser
	closed bool
}

func (b *trackCloseBody) Close() error {
	b.closed = true
	return b.ReadCloser.Close()
}

// TestStatusCodeFetcher_Leak_Fix verifies that StatusCheckFetcher closes the body on error.
func TestStatusCodeFetcher_Leak_Fix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	// Track Body Close
	hf := NewHTTPFetcher()
	sf := NewStatusCodeFetcher(hf)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := sf.Do(req)

	// 1. Should return nil response and error
	assert.Error(t, err)
	assert.Nil(t, resp)

	// 2. Verify it's a StatusError with snippet
	var statusErr *StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, http.StatusInternalServerError, statusErr.StatusCode)
	assert.Contains(t, statusErr.BodySnippet, "internal server error")
}

// TestStatusCodeFetcher_ManualTrack verifies close call via mock
func TestStatusCodeFetcher_ManualTrack(t *testing.T) {
	body := &trackCloseBody{
		ReadCloser: io.NopCloser(bytes.NewReader(nil)),
		closed:     false,
	}
	mock := &mockFetcher{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       body,
			}, nil
		},
	}
	sf := NewStatusCodeFetcher(mock)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, err := sf.Do(req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, body.closed, "Body should be closed on status error")
}
