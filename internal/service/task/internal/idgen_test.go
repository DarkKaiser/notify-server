package internal

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstanceIDGenerator_New(t *testing.T) {
	generator := &InstanceIDGenerator{}

	t.Run("Generate ID", func(t *testing.T) {
		id := generator.New()
		assert.NotEmpty(t, id)
		assert.Len(t, id, 17) // length depends on base62 encoding of timestamp (~11 chars) + padding (6 chars) = approx 17-18 chars
		// Note: The comment above says approx 17-18, the implementation seems to do:
		// timestamp base62 (variable len) + sequence fixed 6 chars.
		// Current time Nano fits in int64.
		// Let's just check not empty and reasonable length for now to avoid flaky test on timestamp length change
		assert.True(t, len(id) >= 10)
	})

	t.Run("Uniqueness in single thread", func(t *testing.T) {
		id1 := generator.New()
		id2 := generator.New()
		assert.NotEqual(t, id1, id2)
	})

	t.Run("Uniqueness in concurrent execution", func(t *testing.T) {
		const goroutines = 100
		const iterations = 1000

		ids := make(chan string, goroutines*iterations)
		var wg sync.WaitGroup

		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					ids <- string(generator.New())
				}
			}()
		}

		wg.Wait()
		close(ids)

		uniqueIDs := make(map[string]bool)
		for id := range ids {
			uniqueIDs[id] = true
		}

		assert.Equal(t, goroutines*iterations, len(uniqueIDs))
	})

	t.Run("Lexicographical Sort Order", func(t *testing.T) {
		// ID가 시간 순서대로 정렬되는지 확인
		// 주의: New() 호출 간격이 매우 짧으면 타임스탬프가 동일할 수 있음.
		// 이 경우 시퀀스 번호로 인해 정렬이 보장되어야 함.

		id1 := generator.New()
		// Ensure time passes if necessary, although atomic counter implementation should handle same-nanosecond order
		// However, different nanoseconds are definitely sortable.
		// For robustness, we rely on the generator's design.

		id2 := generator.New()

		// String comparison
		assert.Less(t, string(id1), string(id2))
	})
}

func TestAppendBase62(t *testing.T) {
	g := &InstanceIDGenerator{}

	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "A"},
		{61, "z"},
		{62, "10"},
		{3843, "zz"},
	}

	for _, tt := range tests {
		var dst []byte
		result := string(g.appendBase62(dst, tt.input))
		assert.Equal(t, tt.expected, result)
	}
}

func TestAppendBase62FixedLength(t *testing.T) {
	g := &InstanceIDGenerator{}

	tests := []struct {
		input    int64
		length   int
		expected string
	}{
		{0, 6, "000000"},
		{1, 6, "000001"},
		{61, 6, "00000z"},
		{62, 6, "000010"},
		{12345, 4, "03D7"},
	}

	for _, tt := range tests {
		var dst []byte
		result := string(g.appendBase62FixedLength(dst, tt.input, tt.length))
		assert.Equal(t, tt.expected, result)

		// Sorting check
		// 패딩된 문자열은 사전순 정렬이 숫자 크기 순서와 일치해야 함
		// (단, Base62 문자셋 순서가 '0'-'9', 'A'-'Z', 'a'-'z' 일 때)
	}

	// 정렬 테스트
	// 1 ("000001") < 62 ("000010")
	res1 := string(g.appendBase62FixedLength(nil, 1, 6))
	res2 := string(g.appendBase62FixedLength(nil, 62, 6))
	assert.Less(t, res1, res2)

	// ASCII 순서 확인: '0' <> 'A' <> 'a'
	// 0-9 (48-57) < A-Z (65-90) < a-z (97-122)
	// Base62Chars 순서와 ASCII 순서가 일치하므로 문자열 정렬 OK
	assert.True(t, '9' < 'A')
	assert.True(t, 'Z' < 'a')
	assert.True(t, strings.Index(base62Chars, "9") < strings.Index(base62Chars, "A"))
}
