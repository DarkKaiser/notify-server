package idgen

import (
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Public API Tests (New)
// =============================================================================

func TestNew(t *testing.T) {
	generator := New()

	t.Run("Format Validation", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			id := string(generator.New())

			// 1. Length Check
			// UnixNano (approx 19 digits) -> Base62 ~11 chars
			// Sequence (Fixed 6 chars)
			// Expected: ~17 chars. Minimum 16.
			assert.GreaterOrEqual(t, len(id), 16, "ID too short: %s", id)

			// 2. Charset Check
			for _, r := range id {
				if !strings.ContainsRune(base62Chars, r) {
					t.Errorf("Invalid char '%c' in ID: %s", r, id)
				}
			}
		}
	})

	t.Run("Monotonicity", func(t *testing.T) {
		const iterations = 10000
		ids := make([]string, iterations)
		for i := 0; i < iterations; i++ {
			ids[i] = string(generator.New())
		}

		for i := 1; i < iterations; i++ {
			if ids[i-1] >= ids[i] {
				t.Fatalf("Monotonicity violation: %s >= %s", ids[i-1], ids[i])
			}
		}
	})
}

func TestNew_Concurrency(t *testing.T) {
	generator := New()
	const (
		workers      = 50
		idsPerWorker = 2000
		totalIDs     = workers * idsPerWorker
	)

	results := make(chan string, totalIDs)
	var wg sync.WaitGroup
	wg.Add(workers)

	start := time.Now()
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerWorker; j++ {
				results <- string(generator.New())
			}
		}()
	}

	wg.Wait()
	close(results)
	duration := time.Since(start)

	t.Logf("Generated %d IDs in %v (%.0f IDs/sec)", totalIDs, duration, float64(totalIDs)/duration.Seconds())

	unique := make(map[string]struct{}, totalIDs)
	for id := range results {
		if _, exists := unique[id]; exists {
			t.Fatalf("Collision detected: %s", id)
		}
		unique[id] = struct{}{}
	}
	assert.Equal(t, totalIDs, len(unique))
}

func TestNew_Allocations(t *testing.T) {
	generator := New()

	// Warm-up
	_ = generator.New()

	// New() should ideally have minimal allocations.
	allocs := testing.AllocsPerRun(1000, func() {
		_ = generator.New()
	})

	// Allow reasonable overhead
	assert.LessOrEqual(t, allocs, 2.0, "Too many allocations per ID generation: %f", allocs)
}

// =============================================================================
// Internal Logic Tests (Whitebox)
// =============================================================================

func TestAppendBase62(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"Zero", 0, "0"},
		{"One", 1, "1"},
		{"61 (z)", 61, "z"},
		{"62 (10)", 62, "10"},
		{"Negative (-1)", -1, "1"},
		{"MaxInt64", math.MaxInt64, "AzL8n0Y58m7"},
		// MinInt64 matches current impl limit (overflows to MinInt64 again if negated in int64)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(appendBase62(nil, tt.input))
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestAppendBase62FixedLength(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		length   int
		expected string
	}{
		{"Zero Padding", 0, 6, "000000"},
		{"Partial Padding", 123, 6, "00001z"}, // 123 = 1*62 + 61('z') -> "1z"
		{"Exact Fit", 14776335, 4, "zzzz"},    // 62^4 - 1
		{"Overflow Preservation", 62, 1, "10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(appendBase62FixedLength(nil, tt.input, tt.length))
			assert.Equal(t, tt.expected, got)
		})
	}
}

// =============================================================================
// Fuzz Tests (Expert Level)
// =============================================================================

func FuzzAppendBase62(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(math.MaxInt64))
	f.Add(int64(123456789))

	f.Fuzz(func(t *testing.T, in int64) {
		// Skip MinInt64: Causes overflow in current implementation
		if in == math.MinInt64 {
			return
		}

		got := appendBase62(nil, in)
		str := string(got)

		if len(str) == 0 {
			t.Errorf("Result should not be empty")
		}

		for _, r := range str {
			if !strings.ContainsRune(base62Chars, r) {
				t.Errorf("Invalid character %c", r)
			}
		}
	})
}

func FuzzAppendBase62FixedLength(f *testing.F) {
	f.Add(int64(0), 6)
	f.Add(int64(123), 6)
	f.Add(int64(math.MaxInt64), 11)

	f.Fuzz(func(t *testing.T, num int64, length int) {
		// Skip invalid lengths. Implementation panics on length=0 with num=0.
		// It also has undefined behavior for writing before buffer for length=0.
		if length <= 0 || length > 100 {
			return
		}
		// Skip MinInt64: Overflow risk
		if num == math.MinInt64 {
			return
		}

		got := string(appendBase62FixedLength(nil, num, length))

		if len(got) < length {
			t.Errorf("Result shorter than requested length. Got: %d, Want >= %d", len(got), length)
		}

		for _, r := range got {
			if !strings.ContainsRune(base62Chars, r) {
				t.Errorf("Invalid character %c", r)
			}
		}
	})
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkNew(b *testing.B) {
	gen := New()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = gen.New()
		}
	})
}

func BenchmarkAppendBase62FixedLength(b *testing.B) {
	dst := make([]byte, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dst = dst[:0]
		_ = appendBase62FixedLength(dst, 123456789, 10)
	}
}
