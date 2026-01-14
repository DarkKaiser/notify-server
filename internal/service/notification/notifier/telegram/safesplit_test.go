package telegram

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestSafeSplit(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		limit         int
		expectedChunk string
		expectedRem   string
	}{
		{
			name:          "ASCII within limit",
			input:         "Hello",
			limit:         10,
			expectedChunk: "Hello",
			expectedRem:   "",
		},
		{
			name:          "ASCII exact limit",
			input:         "Hello",
			limit:         5,
			expectedChunk: "Hello",
			expectedRem:   "",
		},
		{
			name:          "ASCII exceed limit",
			input:         "HelloWorld",
			limit:         5,
			expectedChunk: "Hello",
			expectedRem:   "World",
		},
		{
			name:          "Korean exact limit (Each hangul is 3 bytes)",
			input:         "가나다", // 9 bytes
			limit:         9,
			expectedChunk: "가나다",
			expectedRem:   "",
		},
		{
			name:          "Korean within limit",
			input:         "가나다",
			limit:         10,
			expectedChunk: "가나다",
			expectedRem:   "",
		},
		{
			name:          "Korean split at boundary",
			input:         "가나다",
			limit:         6,
			expectedChunk: "가나",
			expectedRem:   "다",
		},
		{
			name:          "Korean split mid-character (1 byte)",
			input:         "가나다",
			limit:         4, // '가'(3) + 1 byte of '나' -> Should cut at '가'
			expectedChunk: "가",
			expectedRem:   "나다",
		},
		{
			name:          "Korean split mid-character (2 bytes)",
			input:         "가나다",
			limit:         5, // '가'(3) + 2 bytes of '나' -> Should cut at '가'
			expectedChunk: "가",
			expectedRem:   "나다",
		},
		{
			name:          "Mixed Content",
			input:         "A가B나C", // 1 + 3 + 1 + 3 + 1 = 9 bytes
			limit:         6,       // A(1) + 가(3) + B(1) + 1 byte of 나 -> Should cut at B
			expectedChunk: "A가B",
			expectedRem:   "나C",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, rem := safeSplit(tt.input, tt.limit)
			assert.Equal(t, tt.expectedChunk, chunk, "Chunk mismatch")
			assert.Equal(t, tt.expectedRem, rem, "Remainder mismatch")
			assert.True(t, utf8.ValidString(chunk), "Chunk should be valid UTF8")
			assert.True(t, utf8.ValidString(rem), "Remainder should be valid UTF8")
		})
	}
}
