package task

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHTTPFetcher_Methods_Table consolidates generic HTTPFetcher method tests (Do, Get, User-Agent behavior)
func TestHTTPFetcher_Methods_Table(t *testing.T) {
	// Setup a test server that validates User-Agent
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		if userAgent == "" || !strings.Contains(userAgent, "Mozilla/5.0") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	fetcher := NewHTTPFetcher()

	tests := []struct {
		name        string
		action      func() (*http.Response, error)
		expectError bool
	}{
		{
			name: "Do Request (Automatic User-Agent)",
			action: func() (*http.Response, error) {
				req, _ := http.NewRequest("GET", ts.URL, nil)
				return fetcher.Do(req)
			},
			expectError: false,
		},
		{
			name: "Get Request (Automatic User-Agent)",
			action: func() (*http.Response, error) {
				return fetcher.Get(ts.URL)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.action()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if resp != nil {
					assert.Equal(t, http.StatusOK, resp.StatusCode)
				}
			}
		})
	}
}

// TestHTTPFetcher_Timeout checks initialization (Basic check)
func TestHTTPFetcher_Timeout(t *testing.T) {
	fetcher := NewHTTPFetcher()
	assert.NotNil(t, fetcher)
}
