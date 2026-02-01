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
			name:     "Nil header returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "Empty header returns empty",
			input:    http.Header{},
			expected: http.Header{},
		},
		{
			name: "No sensitive headers are preserved",
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
			name: "Sensitive headers are redacted",
			input: http.Header{
				"Authorization":       []string{"Bearer secret-token"},
				"Proxy-Authorization": []string{"Basic user:pass"},
				"Cookie":              []string{"session=abc"},
				"Set-Cookie":          []string{"id=123"},
				"X-Custom-Header":     []string{"value"},
			},
			expected: http.Header{
				"Authorization":       []string{"***"},
				"Proxy-Authorization": []string{"***"},
				"Cookie":              []string{"***"},
				"Set-Cookie":          []string{"***"},
				"X-Custom-Header":     []string{"value"},
			},
		},
		{
			name: "Headers are case-insensitive (canonicalization)",
			input: func() http.Header {
				h := http.Header{}
				h.Set("authorization", "Bearer lower") // Set() canonicalizes keys
				h.Set("COOKIE", "session=upper")
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
			assert.Equal(t, tt.expected, result)

			if tt.input != nil {
				// Immutability check: modifying result should not affect input
				result.Set("New-Header", "value")
				assert.Empty(t, tt.input.Get("New-Header"), "Original header should not be modified")
			}
		})
	}
}

func Test_redactURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string // String input for convenience, parsed in test
		expected string
	}{
		{
			name:     "Nil URL returns empty string",
			input:    "", // handled specially
			expected: "",
		},
		{
			name:     "Simple URL without secrets",
			input:    "https://example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "URL with user info (password)",
			input:    "https://user:password@example.com/path",
			expected: "https://user:xxxxx@example.com/path",
		},
		{
			name:     "URL with user info (no password)",
			input:    "https://user@example.com/path",
			expected: "https://user@example.com/path", // url.Redacted behavior (no password to redact)
		},
		{
			name:     "URL with query parameters",
			input:    "https://example.com/path?token=secret&id=123",
			expected: "https://example.com/path?id=xxxxx&token=xxxxx", // Sorted by key
		},
		{
			name:     "URL with user info and query parameters",
			input:    "https://user:pass@example.com/path?key=value",
			expected: "https://user:xxxxx@example.com/path?key=xxxxx",
		},
		{
			name:     "URL with fragment preserved",
			input:    "https://example.com/path?q=v#fragment",
			expected: "https://example.com/path?q=xxxxx#fragment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Nil URL returns empty string" {
				assert.Equal(t, "", redactURL(nil))
				return
			}

			u, err := url.Parse(tt.input)
			require.NoError(t, err)

			result := redactURL(u)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_redactRawURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// 1. Valid URLs (Delegates to redactURL)
		{
			name:     "Valid URL with sensitive info",
			input:    "https://user:pass@example.com/resource?token=secret",
			expected: "https://user:xxxxx@example.com/resource?token=xxxxx",
		},
		{
			name:     "Valid URL plain",
			input:    "https://example.com",
			expected: "https://example.com",
		},

		// 2. Fallback Logic: Invalid/Special URLs
		{
			name:     "Scheme-less proxy URL (parse fails or treated as path)",
			input:    "user:pass@proxy.example.com:8080",
			expected: "xxxxx:xxxxx@proxy.example.com:8080",
		},
		{
			name:     "Scheme-less proxy URL with no port",
			input:    "user:pass@proxy.example.com",
			expected: "xxxxx:xxxxx@proxy.example.com",
		},
		{
			name:     "Invalid control characters (Parser failure)",
			input:    "http://user:pass@exam\nple.com",
			expected: "http://xxxxx:xxxxx@exam\nple.com", // Fallback logic kicks in
		},
		{
			name:     "Multiple @ signs (Greedy match - Parsed as valid URL)",
			input:    "https://user:p@ss@example.com",
			expected: "https://user:xxxxx@example.com", // url.Parse handles this, masking only password
		},

		// 3. Fallback Logic: No sensitive info patterns
		{
			name:     "String without @ or scheme",
			input:    "just-a-string",
			expected: "just-a-string",
		},
		{
			name:     "String with scheme but no @",
			input:    "http://example.com",
			expected: "http://example.com",
		},
		{
			name:     "String with @ but no scheme (treated as auth)",
			input:    "no-scheme@domain.com",
			expected: "xxxxx:xxxxx@domain.com",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactRawURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
