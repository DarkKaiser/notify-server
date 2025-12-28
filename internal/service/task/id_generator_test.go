package task

import (
	"regexp"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstanceIDGenerator_Base62_Table verifies the encoding logic using a table
func TestInstanceIDGenerator_Base62_Table(t *testing.T) {
	g := &instanceIDGenerator{}

	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "A"},
		{35, "Z"},
		{36, "a"},
		{61, "z"},
		{62, "10"},
	}

	for _, tt := range tests {
		t.Run("Encode "+tt.expected, func(t *testing.T) {
			result := string(g.appendBase62(nil, tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestInstanceIDGenerator_Format_Table verifies format for multiple generated IDs
func TestInstanceIDGenerator_Format_Table(t *testing.T) {
	generator := &instanceIDGenerator{}
	regex := regexp.MustCompile(`^[0-9a-zA-Z]+$`)

	tests := []struct {
		name  string
		check func(*testing.T, InstanceID)
	}{
		{
			name: "Base62 Characters Only",
			check: func(t *testing.T, id InstanceID) {
				assert.True(t, regex.MatchString(string(id)), "ID should contain only Base62 characters")
			},
		},
		{
			name: "Length Constraints",
			check: func(t *testing.T, id InstanceID) {
				l := len(string(id))
				assert.GreaterOrEqual(t, l, 15)
				assert.LessOrEqual(t, l, 25)
			},
		},
	}

	// Generate a few samples to verify
	for i := 0; i < 5; i++ {
		id := generator.New()
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.check(t, id)
			})
		}
	}
}

// Keep concurrent and monotonicity tests as they are best suited for procedural logic
func TestInstanceIDGenerator_New_Uniqueness(t *testing.T) {
	generator := &instanceIDGenerator{}
	const numGoroutines = 100
	const numIDsPerGoroutine = 1000

	ids := make(chan InstanceID, numGoroutines*numIDsPerGoroutine)
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIDsPerGoroutine; j++ {
				ids <- generator.New()
			}
		}()
	}

	wg.Wait()
	close(ids)

	uniqueMap := make(map[InstanceID]bool)
	count := 0
	for id := range ids {
		if uniqueMap[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		uniqueMap[id] = true
		count++
		assert.NotEmpty(t, id)
	}

	assert.Equal(t, numGoroutines*numIDsPerGoroutine, count)
}

func TestInstanceIDGenerator_Monotonicity(t *testing.T) {
	generator := &instanceIDGenerator{}
	count := 1000
	ids := make([]string, count)

	for i := 0; i < count; i++ {
		ids[i] = string(generator.New())
		if i%100 == 0 {
			time.Sleep(time.Nanosecond)
		}
	}

	isSorted := sort.SliceIsSorted(ids, func(i, j int) bool {
		if len(ids[i]) != len(ids[j]) {
			return len(ids[i]) < len(ids[j])
		}
		return ids[i] < ids[j]
	})

	require.True(t, isSorted, "Generated IDs should be monotonic")
}

func BenchmarkInstanceIDGenerator_New(b *testing.B) {
	generator := &instanceIDGenerator{}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = generator.New()
	}
}

func BenchmarkInstanceIDGenerator_New_Parallel(b *testing.B) {
	generator := &instanceIDGenerator{}
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = generator.New()
		}
	})
}
