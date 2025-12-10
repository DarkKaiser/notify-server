package notification

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotifierID(t *testing.T) {
	t.Run("Type Conversion", func(t *testing.T) {
		str := "test-notifier"
		id := NotifierID(str)
		assert.Equal(t, str, string(id))
	})

	t.Run("Equality", func(t *testing.T) {
		id1 := NotifierID("id1")
		id2 := NotifierID("id1")
		id3 := NotifierID("id2")

		assert.Equal(t, id1, id2)
		assert.NotEqual(t, id1, id3)
	})

	t.Run("Map Key Usage", func(t *testing.T) {
		m := make(map[NotifierID]string)
		id := NotifierID("key")
		m[id] = "value"

		val, exists := m[id]
		assert.True(t, exists)
		assert.Equal(t, "value", val)
	})
}
