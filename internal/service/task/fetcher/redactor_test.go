package fetcher

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_isSensitiveKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		// 1. Exact Match (정확히 일치)
		{name: "Exact match: token", key: "token", expected: true},
		{name: "Exact match: secret", key: "secret", expected: true},
		{name: "Exact match: password", key: "password", expected: true},
		{name: "Exact match: api_key", key: "api_key", expected: true},
		{name: "Exact match: Case Insensitive", key: "ToKeN", expected: true},

		// 2. Suffix Match (접미사 일치)
		{name: "Suffix match: _token", key: "access_token", expected: true}, // both exact list and suffix match
		{name: "Suffix match: custom_token", key: "custom_token", expected: true},
		{name: "Suffix match: _secret", key: "app_secret", expected: true},
		{name: "Suffix match: _password", key: "db_password", expected: true},

		// 3. False Positives (오탐 방지) - Partial Match
		{name: "Partial match: monkey (contains key)", key: "monkey", expected: false},           // "key" exact match list
		{name: "Partial match: broken (contains token)", key: "broken", expected: false},         // "token" exact match list
		{name: "Partial match: passage (contains pass)", key: "passage", expected: false},        // "pass" exact match list
		{name: "Partial match: compass (contains pass)", key: "compass", expected: false},        // "pass" exact match list
		{name: "Partial match: keyword (contains key)", key: "keyword", expected: false},         // "key" exact match list
		{name: "Partial match: oss_signature (not _sig)", key: "oss_signature", expected: false}, // "signature" is exact match, "oss_signature" doesn't match suffix

		// 4. False Positives - Suffix Mismatch
		{name: "Suffix mismatch: _key (too aggressive suffix excluded)", key: "my_key", expected: false}, // "_key" is NOT in sensitiveSuffixes
		{name: "Suffix mismatch: token_id (prefix match)", key: "token_id", expected: false},
		{name: "Suffix mismatch: secret_agent", key: "secret_agent", expected: false},

		// 5. Non-sensitive keys
		{name: "Common key: id", key: "id", expected: false},
		{name: "Common key: page", key: "page", expected: false},
		{name: "Common key: sort", key: "sort", expected: false},
		{name: "Common key: view", key: "view", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSensitiveKey(tt.key)
			assert.Equal(t, tt.expected, result, "key: %s", tt.key)
		})
	}
}

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
				"Host":         []string{"example.com"},
			},
			expected: http.Header{
				"Content-Type": []string{"application/json"},
				"Accept":       []string{"*/*"},
				"Host":         []string{"example.com"},
			},
		},
		{
			name: "Sensitive headers are redacted",
			input: http.Header{
				"Authorization":       []string{"Bearer secret-token"},
				"Proxy-Authorization": []string{"Basic user:pass"},
				"Cookie":              []string{"session=abc"},
				"Set-Cookie":          []string{"id=123"},
				"X-Custom-Header":     []string{"value"}, // Should be preserved
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
			name: "Headers are case-insensitive",
			input: func() http.Header {
				h := http.Header{}
				h.Set("authorization", "Bearer lower") // Canonicalizes to "Authorization"
				h.Set("COOKIE", "session=upper")       // Canonicalizes to "Cookie"
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
				// Ensure deep copy behavior for map values isn't strictly required by spec (Clone does shallow copy of values slicing),
				// but here we just check if setting a key in result affects input.
				result.Set("New-Header", "value")
				assert.Empty(t, tt.input.Get("New-Header"), "Original header should not be modified")
			}
		})
	}
}

func Test_redactURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string // String input for convenience
		expected string
	}{
		{
			name:     "Nil URL returns empty string",
			input:    "", // Handled specially in test loop
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
			name:     "URL with user info (no password, treated as token)",
			input:    "https://token@example.com/path",
			expected: "https://xxxxx@example.com/path",
		},
		// Selective Redaction Tests
		{
			name:     "Selective redaction: specific keys masked",
			input:    "https://example.com/path?token=secret&api_key=12345",
			expected: "https://example.com/path?api_key=xxxxx&token=xxxxx", // Sorted
		},
		{
			name:     "Selective redaction: non-sensitive keys preserved",
			input:    "https://example.com/path?id=123&page=1&sort=desc",
			expected: "https://example.com/path?id=123&page=1&sort=desc",
		},
		{
			name:     "Selective redaction: Mixed sensitive and non-sensitive",
			input:    "https://example.com/path?token=secret&id=123&mode=view",
			expected: "https://example.com/path?id=123&mode=view&token=xxxxx",
		},
		// Edge Cases for Matching
		{
			name:     "False positive check: broken (ends with oken, contains token)",
			input:    "https://example.com?broken=value",
			expected: "https://example.com?broken=value", // Should NOT be masked
		},
		{
			name:     "False positive check: monkey (contains key)",
			input:    "https://example.com?monkey=banana",
			expected: "https://example.com?monkey=banana", // Should NOT be masked
		},
		{
			name:     "Suffix match check: my_token",
			input:    "https://example.com?my_token=secret",
			expected: "https://example.com?my_token=xxxxx",
		},
		{
			name:     "Suffix match check: client_secret",
			input:    "https://example.com?client_secret=hidden",
			expected: "https://example.com?client_secret=xxxxx",
		},
		// Complex URLs
		{
			name:     "Complex: User auth + Query params + Fragment",
			input:    "https://admin:pass@example.com:8443/api?q=search&token=jwt#fragment",
			expected: "https://admin:xxxxx@example.com:8443/api?q=search&token=xxxxx#fragment",
		},
		{
			name:     "URL with multiple values for same key",
			input:    "https://example.com?id=1&token=a&token=b",
			expected: "https://example.com?id=1&token=xxxxx", // Values are replaced by single "xxxxx" or multiple?
			// url.Values.Set() replaces existing values.
			// Logic: query.Set(key, "xxxxx") -> replaces all values with single "xxxxx".
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input == "" {
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
			name:     "Valid URL standard",
			input:    "https://user:pass@example.com/path?token=secret",
			expected: "https://user:xxxxx@example.com/path?token=xxxxx",
		},
		{
			name:     "Valid URL no auth",
			input:    "https://example.com/path?id=123",
			expected: "https://example.com/path?id=123",
		},

		// 2. Fallback Logic: Behaving like a Proxy or specific non-standard formats
		{
			name:     "Scheme-less proxy URL (user:pass@host:port)",
			input:    "user:pass@proxy.example.com:8080",
			expected: "xxxxx:xxxxx@proxy.example.com:8080",
		},
		{
			name:     "Scheme-less auth (user:pass@host)",
			input:    "admin:1234@internal-service",
			expected: "xxxxx:xxxxx@internal-service",
		},
		{
			name: "@ in query param (Should utilize redactURL logic if parsable, but if scheme missing?)",
			// Note: "example.com/s?q=me@test.com" parses as path "example.com/s", rawquery "q=me@test.com" if scheme missing?
			// Actually url.Parse("example.com/...") usually fails or parses weirdly without scheme.
			// But redactRawURL checks (!strings.Contains(rawURL, "://") && strings.Contains(rawURL, "@"))
			// Here "://" is missing and "@" is present. Fallback logic triggers.
			// Fallback logic limits search to before '?' or '#'.
			// So @ in "q=me@test.com" comes AFTER '?'.
			// authSearchLimit will be index of '?'.
			// LastIndex("@") in "example.com/s" is -1.
			// Fallback logic returns original string. Correct.
			input:    "example.com/search?email=user@test.com",
			expected: "example.com/search?email=user@test.com",
		},
		{
			name: "@ in path (Should NOT be redacted by fallback if no scheme)",
			// "no-scheme/user@home" -> "://" missing, "@" present.
			// Fallback triggers.
			// authSearchLimit = len.
			// LastIndex("@") found.
			// "xxxxx:xxxxx" + "@home" -> "xxxxx:xxxxx@home"
			// Это aggressive fallback. It assumes if no scheme and @ exists, it looks like auth.
			// "user@host" is ambiguous. Could be email, could be "user@host".
			// Our redactor assumes auth to be safe.
			input:    "user@host-without-scheme",
			expected: "xxxxx:xxxxx@host-without-scheme",
		},
		{
			name:     "Malformed URL with @ (Fallback triggers)",
			input:    "http://user:pass@invalid\nnewline.com",
			expected: "http://xxxxx:xxxxx@invalid\nnewline.com",
		},
		{
			name:     "Double @ signs (Fallback handles last one)",
			input:    "u:p@ss@host-no-scheme", // first @ part of password?
			expected: "xxxxx:xxxxx@host-no-scheme",
		},

		// 3. No change scenarios
		{
			name:     "Simple string",
			input:    "simple-string",
			expected: "simple-string",
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
