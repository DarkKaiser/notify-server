package fetcher

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_redactHeaders(t *testing.T) {
	tests := []struct {
		name     string
		input    http.Header
		expected http.Header
	}{
		{
			name:     "Nil header",
			input:    nil,
			expected: nil,
		},
		{
			name:     "Empty header",
			input:    http.Header{},
			expected: http.Header{},
		},
		{
			name: "No sensitive headers",
			input: http.Header{
				"Content-Type": []string{"application/json"},
				"Accept":       []string{"*/*"},
			},
			expected: http.Header{
				"Content-Type": []string{"application/json"},
				"Accept":       []string{"*/*"},
			},
		},
		{
			name: "With sensitive headers",
			input: http.Header{
				"Authorization":       []string{"Bearer secret-token"},
				"Proxy-Authorization": []string{"Basic user:pass"},
				"Cookie":              []string{"session=abc"},
				"Set-Cookie":          []string{"id=123"},
				"Content-Type":        []string{"text/html"},
			},
			expected: http.Header{
				"Authorization":       []string{"***"},
				"Proxy-Authorization": []string{"***"},
				"Cookie":              []string{"***"},
				"Set-Cookie":          []string{"***"},
				"Content-Type":        []string{"text/html"},
			},
		},
		{
			name: "Case sensitivity check",
			input: func() http.Header {
				h := http.Header{}
				h.Set("authorization", "Bearer lower") // Set() canonicalizes the key
				h.Set("COOKIE", "session=upper")       // Set() canonicalizes the key
				return h
			}(),
			expected: http.Header{
				"Authorization": []string{"***"},
				"Cookie":        []string{"***"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactHeaders(tt.input)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}

			// Immutability check
			if tt.input != nil {
				// Modify result and check input is not affected
				result.Set("New-Header", "value")
				assert.Empty(t, tt.input.Get("New-Header"), "Original header should not be modified")
			}
		})
	}
}

func Test_redactURL(t *testing.T) {
	tests := []struct {
		name     string
		urlStr   string
		expected string
	}{
		{
			name:     "Nil URL",
			urlStr:   "", // handled by code logic, though method takes *url.URL
			expected: "",
		},
		{
			name:     "Simple URL",
			urlStr:   "https://example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "URL with UserPass",
			urlStr:   "https://user:pass@example.com/path",
			expected: "https://user:xxxxx@example.com/path",
		},
		{
			name:     "URL with Query",
			urlStr:   "https://example.com/path?token=secret123&user=admin",
			expected: "https://example.com/path?token=xxxxx&user=xxxxx",
		},
		{
			name:     "URL with UserPass and Query",
			urlStr:   "https://user:pass@example.com/path?key=val",
			expected: "https://user:xxxxx@example.com/path?key=xxxxx",
		},
		{
			name:     "Fragment should be preserved",
			urlStr:   "https://example.com/path?k=v#fragment",
			expected: "https://example.com/path?k=xxxxx#fragment",
		},
		{
			// Query parameters are sorted by key during encoding
			name:     "Complex Query",
			urlStr:   "https://example.com/search?q=hello&lang=en&page=1",
			expected: "https://example.com/search?lang=xxxxx&page=xxxxx&q=xxxxx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Nil URL" {
				assert.Equal(t, "", redactURL(nil))
				return
			}

			u, err := url.Parse(tt.urlStr)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, redactURL(u))
		})
	}
}

func Test_redactURL_ParseFailureFallback(t *testing.T) {
	// url.Parse handles most strings, but we want to verify fallback logic.
	// However, since url.Redacted() returns a valid string usually, parse failure is rare.
	// But if Redacted() returns a string that url.Parse fails on, we should return Redacted() string.
	// Since redactURL is internal and relies on stdlib, it's hard to force parse failure on valid inputs.
	// This test simply ensures no panic on typical complex inputs.

	// Example of a URL that might be valid initially but problematic after redaction?
	// Actually, url.Redacted() typically returns valid URL strings.
	// We'll trust the main tests for coverage, but explicitly test a case where Redacted() is valid.
	u, _ := url.Parse("https://user:pass@example.com")
	result := redactURL(u)
	assert.Equal(t, "https://user:xxxxx@example.com", result)
}
