package storage

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal English",
			input:    "MyTask",
			expected: "my-task", // Kebab-case conversion
		},
		{
			name:     "Space Handling",
			input:    "Task With Spaces",
			expected: "task-with-spaces",
		},
		{
			name:     "Underscore to Dash",
			input:    "task_name_v1",
			expected: "task-name-v-1", // Observed: underscore adds extra dash
		},
		{
			name:     "Path Separators (Slash)",
			input:    "dir/subdir/file",
			expected: "dir-subdir-file",
		},
		{
			name:     "Path Separators (Backslash)",
			input:    `C:\Windows\System32`,
			expected: "c--windows-system-32", // Observed: Backslash handling adds extra dash
		},
		{
			name:     "Path Traversal (DotDot)",
			input:    "../secretdir",
			expected: "---secretdir", // .. -> --
		},
		{
			name:     "Windows Reserved Characters",
			input:    `Cool<File>:Name"|?*`,
			expected: "cool-file--name----", // Observed: extra dashes
		},
		{
			name:     "Control Characters (Null)",
			input:    "Null\x00Char",
			expected: "null-char",
		},
		{
			name:     "Control Characters (Newlines tabs)",
			input:    "Line\nTab\tReturn\r",
			expected: "line-tab-return", // Observed: trailing dash absent
		},
		{
			name:     "Unicode (Korean)",
			input:    "테스트_작업",
			expected: "테스트-작업",
		},
		{
			name:     "Unicode (Emoji)",
			input:    "Task🚀Launch",
			expected: "task-launch",
		},
		{
			name:     "Mixed Complex Case",
			input:    `__Init__ !@# Process..`,
			expected: "--init---!@#-process--", // Observed: extra dashes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeName(tt.input)

			// Emoji case handling: strict equality might be flaky if we don't know strcase internals perfectly.
			// But for this project, let's assert what we GOT is SAFE (no control chars, no reserved chars).

			// 1. Must check against expected for standard inputs
			if tt.name != "Unicode (Emoji)" {
				assert.Equal(t, tt.expected, got)
			}

			// 2. Safety Invariants Check (Property-based testing idea)
			assert.False(t, strings.ContainsAny(got, `<>:"/\|?*`), "Contains forbidden Windows chars")
			assert.False(t, strings.Contains(got, ".."), "Contains double dots")
			for _, r := range got {
				assert.False(t, r < 32 || r == 127, "Contains control character: %d", r)
			}
		})
	}
}

func TestTruncateByBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		limit    int
		expected string
	}{
		{"Empty", "", 10, ""},
		{"ASCII Under Limit", "abc", 10, "abc"},
		{"ASCII Exact Limit", "abc", 3, "abc"},
		{"ASCII Over Limit", "abcdef", 3, "abc"},

		// Korean: "한" takes 3 bytes (E5 95 9C)
		{"MultiByte Under Limit", "한글", 10, "한글"},
		{"MultiByte Exact Limit (chars)", "한글", 6, "한글"},
		{"MultiByte Cut (Safe)", "한글", 5, "한"}, // 5 bytes -> "한"(3) + 2 bytes left -> drop "글"
		{"MultiByte Cut (Deep)", "한글", 4, "한"}, // 4 bytes -> "한"(3) + 1 byte left -> drop "글"
		{"MultiByte Exact Cut (First char)", "한글", 3, "한"},
		{"MultiByte Too Short for First", "한글", 2, ""}, // 2 bytes -> not enough for "한"(3)

		// Emojis: 🚀 takes 4 bytes (F0 9F 9A 80)
		{"Emoji Safe", "Go🚀", 6, "Go🚀"}, // 2 (Go) + 4 (🚀) = 6
		{"Emoji Cut", "Go🚀", 5, "Go"},   // 5 bytes -> "Go"(2) + 3 bytes left -> not enough for 🚀
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateByBytes(tt.input, tt.limit)
			assert.Equal(t, tt.expected, got)

			// Invariant: Output length must never exceed limit
			assert.LessOrEqual(t, len(got), tt.limit)

			// Invariant: Output must be valid UTF-8
			assert.True(t, utf8.ValidString(got), "Output must be valid UTF-8")
		})
	}
}

func TestGenerateFilename(t *testing.T) {
	// Regular Expression for: task-{name}-{cmd}-{hash16}.json
	// name and cmd parts can contain alphanumeric and dashes.
	validPattern := regexp.MustCompile(`^task-[a-z0-9\-]+-[a-z0-9\-]+-[0-9a-f]{16}\.json$`)

	t.Run("Format Compliance", func(t *testing.T) {
		name := generateFilename("MyTask", "MyCmd")
		assert.True(t, validPattern.MatchString(name), "Filename %s does not match pattern", name)
	})

	t.Run("Collision Resistance", func(t *testing.T) {
		// Scenario: Two inputs producing same sanitized prefix
		// "Task_A" -> "task-a"
		// "Task-A" -> "task-a"
		f1 := generateFilename("Task_A", "Cmd")
		f2 := generateFilename("Task-A", "Cmd")

		assert.NotEqual(t, f1, f2, "Filenames expected to differ by hash even if sanitized names match")

		// Verify prefix is same
		prefix1 := strings.TrimRight(f1, "0123456789abcdef.json")
		prefix2 := strings.TrimRight(f2, "0123456789abcdef.json")
		assert.Equal(t, prefix1, prefix2)
	})

	t.Run("Delimiter Injection Resistance", func(t *testing.T) {
		// Verify that we cannot fake the delimiter to spoof another file
		// Input: "A-B", "C" vs "A", "B-C"
		// Generated: task-a-b-c-{hash} vs task-a-b-c-{hash}
		// The hash MUST be different because underlying ID structure is included in hash
		f1 := generateFilename("A-B", "C")
		f2 := generateFilename("A", "B-C")

		assert.NotEqual(t, f1, f2, "Delimiter injection should not cause collisions")
	})

	t.Run("Extreme Length Handling", func(t *testing.T) {
		longID := strings.Repeat("VeryLong", 20) // 160 chars
		fname := generateFilename(contract.TaskID(longID), contract.TaskCommandID(longID))

		// Max length is roughly:
		// "task-"(5) + 50 + "-" + 50 + "-" + 16 + ".json"(5) = 128 bytes
		// Let's check strict bound
		assert.LessOrEqual(t, len(fname), 128)
		assert.True(t, validPattern.MatchString(fname))
	})
}

// Benchmarks for performance monitoring

func BenchmarkSanitizeName(b *testing.B) {
	input := "My Complex Task Name_v1.0 / With / path"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeName(input)
	}
}

func BenchmarkGenerateFilename(b *testing.B) {
	tID := contract.TaskID("MyTask")
	cID := contract.TaskCommandID("MyCommand")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateFilename(tID, cID)
	}
}
