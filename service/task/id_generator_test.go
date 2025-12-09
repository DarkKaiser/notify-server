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

func TestInstanceIDGenerator_New_Uniqueness(t *testing.T) {
	generator := &instanceIDGenerator{}

	// 동시성 테스트를 위해 많은 수의 고루틴 실행
	// 100개 고루틴이 각각 1000개 ID 생성 -> 총 10만개
	const numGoroutines = 100
	const numIDsPerGoroutine = 1000

	ids := make(chan TaskInstanceID, numGoroutines*numIDsPerGoroutine)
	var wg sync.WaitGroup

	// 병렬로 ID 대량 생성
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

	// 중복 검사
	uniqueMap := make(map[TaskInstanceID]bool)
	count := 0
	for id := range ids {
		if uniqueMap[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		uniqueMap[id] = true
		count++

		// ID가 비어있지 않은지 확인
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

	// 생성된 ID가 정렬된 상태인지 확인
	isSorted := sort.SliceIsSorted(ids, func(i, j int) bool {
		// 길이 우선 비교 (Base62 특성상 길이가 길면 더 큰 수)
		if len(ids[i]) != len(ids[j]) {
			return len(ids[i]) < len(ids[j])
		}
		return ids[i] < ids[j]
	})

	require.True(t, isSorted, "Generated IDs should be monotonic (sorted by length then value)")
}

func TestInstanceIDGenerator_Format(t *testing.T) {
	generator := &instanceIDGenerator{}
	id := string(generator.New())

	// Base62 문자셋으로만 구성되어야 함
	matched, err := regexp.MatchString(`^[0-9a-zA-Z]+$`, id)
	require.NoError(t, err)
	require.True(t, matched, "ID should contain only Base62 characters")

	// 길이는 어느 정도 일정해야 함
	// 타임스탬프(약 11자) + 시퀀스(고정 6자) = 약 17자 내외
	require.GreaterOrEqual(t, len(id), 15)
	require.LessOrEqual(t, len(id), 25)
}

func TestInstanceIDGenerator_AppendBase62(t *testing.T) {
	g := &instanceIDGenerator{}

	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "A"}, // 이전 'a' (10) -> 이제 'A' (10)
		{35, "Z"}, // 이전 'z' (35) -> 이제 'Z' (35)
		{36, "a"}, // 이전 'A' (36) -> 이제 'a' (36)
		{61, "z"}, // 이전 'Z' (61) -> 이제 'z' (61)
		{62, "10"},
	}

	for _, tt := range tests {
		res := string(g.appendBase62(nil, tt.input))
		assert.Equal(t, tt.expected, res, "Base62 encoding incorrect for %d", tt.input)
	}
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
