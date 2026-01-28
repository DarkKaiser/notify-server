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
			name:     "Normal String",
			input:    "MyTaskName",
			expected: "my-task-name",
		},
		{
			name:     "With Spaces",
			input:    "Task Name With Spaces",
			expected: "task-name-with-spaces",
		},
		{
			name:     "With Underscores",
			input:    "task_name_example",
			expected: "task-name-example",
		},
		{
			name:     "Path Traversal (DotDot)",
			input:    "../Secret",
			expected: "---secret", // ../ -> --/ -> --- + secret
		},
		{
			name:     "Path Separators",
			input:    "dir/file\\name",
			expected: "dir-file-name",
		},
		{
			name:     "Windows Reserved Chars",
			input:    `Key<|>"?*`,
			expected: "key------",
		},
		{
			name:     "Mixed Complex Case",
			input:    `My..Cool/Task\Name <V2>`,
			expected: "my--cool-task-name--v-2-",
		},
		{
			name:     "Already Kebab",
			input:    "already-kebab-case",
			expected: "already-kebab-case",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeName(tt.input)
			assert.Equal(t, tt.expected, result)
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
		{
			name:     "Under Limit",
			input:    "short",
			limit:    10,
			expected: "short",
		},
		{
			name:     "Exact Limit",
			input:    "exact",
			limit:    5,
			expected: "exact",
		},
		{
			name:     "Over Limit (ASCII)",
			input:    "loooooooooong",
			limit:    5,
			expected: "loooo",
		},
		{
			name:     "Multi-byte (Korean)",
			input:    "안녕하세요", // 3 bytes per char
			limit:    6,       // 2 chars = 6 bytes
			expected: "안녕",
		},
		{
			name:     "Multi-byte Cut in Middle",
			input:    "안녕하세요", // 3 * 5 = 15 bytes
			limit:    7,       // Try to cut at 7 (2 chars + 1 byte)
			expected: "안녕",    // Should drop the partial char
		},
		{
			name:     "Empty String",
			input:    "",
			limit:    10,
			expected: "",
		},
		{
			name:     "Emoji",
			input:    "🚀Rock", // 🚀 is 4 bytes
			limit:    5,
			expected: "🚀R",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateByBytes(tt.input, tt.limit)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), tt.limit)
			assert.True(t, utf8.ValidString(result), "Result must be valid UTF-8")
		})
	}
}

func TestGenerateFilename(t *testing.T) {
	t.Run("Format Check", func(t *testing.T) {
		taskID := contract.TaskID("MyTask")
		commandID := contract.TaskCommandID("MyCommand")

		filename := generateFilename(taskID, commandID)

		// Expected format: task-{sanitized-task}-{sanitized-cmd}-{16hex}.json
		matched, err := regexp.MatchString(`^task-my-task-my-command-[0-9a-f]{16}\.json$`, filename)
		assert.NoError(t, err)
		assert.True(t, matched, "Filename %s does not match expected format", filename)
	})

	t.Run("Collision Avoidance (Same Content Different Structure)", func(t *testing.T) {
		// Case that would collide with naïve concatenation "A|B" vs "A|B"
		// implementation uses length prefix so they should differ.
		f1 := generateFilename("A|", "B")
		f2 := generateFilename("A", "|B")

		assert.NotEqual(t, f1, f2, "Filenames must be different for different inputs even if concat is same")
	})

	t.Run("Collision Avoidance (Kebab Case Collision)", func(t *testing.T) {
		// "Task_A" -> "task-a"
		// "Task-A" -> "task-a"
		// These sanitize to same string, but hash source is original ID.
		f1 := generateFilename("Task_A", "cmd")
		f2 := generateFilename("Task-A", "cmd")

		assert.NotEqual(t, f1, f2, "Filenames must be different due to hash of original IDs")
		// Yet prefix part should be same
		parts1 := strings.Split(f1, "-")
		parts2 := strings.Split(f2, "-")
		// task-task-a-cmd-{hash}.json
		// parts: [task, task, a, cmd, {hash}.json]
		// Verify the readable parts are identical
		assert.Equal(t, parts1[:4], parts2[:4], "Readable parts should be identical")
	})

	t.Run("Length Limit Enforcement", func(t *testing.T) {
		longStr := strings.Repeat("A", 100)
		taskID := contract.TaskID(longStr)
		commandID := contract.TaskCommandID(longStr)

		filename := generateFilename(taskID, commandID)

		t.Logf("Generated Filename Length: %d", len(filename))

		// Max length calculation:
		// "task-" (5) + Task(50) + "-" (1) + Command(50) + "-" (1) + Hash(16) + ".json" (5)
		// Total max = 128
		assert.LessOrEqual(t, len(filename), 128)
	})
}
