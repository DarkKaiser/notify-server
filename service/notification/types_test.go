package notification

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Constants
// =============================================================================

const (
	testID1 = "test-notifier-1"
	testID2 = "test-notifier-2"
)

// =============================================================================
// NotifierID Type Tests
// =============================================================================

// TestNotifierID는 NotifierID 타입의 기본 동작을 검증합니다.
//
// 검증 항목:
//   - 타입 변환 (string ↔ NotifierID)
//   - 동등성 비교
//   - Map 키로 사용 가능 여부
//   - 엣지 케이스 (빈 문자열, 특수 문자, 긴 문자열)
func TestNotifierID(t *testing.T) {
	t.Run("Type Conversion", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
		}{
			{"Normal ID", testID1},
			{"Empty string", ""},
			{"With special chars", "notifier-id_123"},
			{"Long ID", "very-long-notifier-id-with-many-characters-0123456789"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				id := NotifierID(tt.input)
				assert.Equal(t, tt.input, string(id))
			})
		}
	})

	t.Run("Equality", func(t *testing.T) {
		tests := []struct {
			name     string
			id1      NotifierID
			id2      NotifierID
			expected bool
		}{
			{"Same IDs", NotifierID(testID1), NotifierID(testID1), true},
			{"Different IDs", NotifierID(testID1), NotifierID(testID2), false},
			{"Empty IDs", NotifierID(""), NotifierID(""), true},
			{"Case sensitive", NotifierID("ID"), NotifierID("id"), false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.expected {
					assert.Equal(t, tt.id1, tt.id2)
				} else {
					assert.NotEqual(t, tt.id1, tt.id2)
				}
			})
		}
	})

	t.Run("Map Key Usage", func(t *testing.T) {
		m := make(map[NotifierID]string)
		id1 := NotifierID(testID1)
		id2 := NotifierID(testID2)

		// Set values
		m[id1] = "value1"
		m[id2] = "value2"

		// Verify id1
		val1, exists1 := m[id1]
		require.True(t, exists1, "ID1 should exist in map")
		assert.Equal(t, "value1", val1)

		// Verify id2
		val2, exists2 := m[id2]
		require.True(t, exists2, "ID2 should exist in map")
		assert.Equal(t, "value2", val2)

		// Verify non-existent key
		_, exists3 := m[NotifierID("non-existent")]
		assert.False(t, exists3, "Non-existent ID should not be in map")
	})

	t.Run("Map Key Overwrite", func(t *testing.T) {
		m := make(map[NotifierID]string)
		id := NotifierID(testID1)

		m[id] = "original"
		m[id] = "updated"

		val, exists := m[id]
		require.True(t, exists)
		assert.Equal(t, "updated", val, "Value should be overwritten")
	})
}
