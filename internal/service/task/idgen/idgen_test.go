package idgen

import (
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGenerator_New_Structure 생성된 ID가 Base62 문자셋과 길이 제약 조건을 충족하는지 검증합니다.
func TestGenerator_New_Structure(t *testing.T) {
	generator := &Generator{}

	for i := 0; i < 100; i++ {
		id := string(generator.New())

		// 1. 길이 검증
		// 타임스탬프(약 10~11자) + 시퀀스(6자) = 16 ~ 17자
		// 현재 UnixNano는 19자리 숫자로, Base62 변환 시 약 11자리입니다.
		assert.True(t, len(id) >= 16 && len(id) <= 18, "ID length should be reasonable (16-18 chars), got: %d (%s)", len(id), id)

		// 2. 문자셋 검증 (Base62)
		for _, r := range id {
			assert.True(t, strings.ContainsRune(base62Chars, r), "ID should only contain Base62 characters. Invalid char: %c in %s", r, id)
		}
	}
}

// TestGenerator_New_Uniqueness 단일 스레드 및 멀티 스레드 환경에서 ID 유일성을 검증합니다.
func TestGenerator_New_Uniqueness(t *testing.T) {
	generator := &Generator{}

	t.Run("Single Thread Monotonicity", func(t *testing.T) {
		// 단일 스레드에서는 시간 순서대로 생성되므로, 문자열 정렬 순서도 유지되어야 합니다.
		// (동일 나노초 내에서도 시퀀스가 증가하므로)
		prevID := string(generator.New())
		for i := 0; i < 1000; i++ {
			currID := string(generator.New())
			assert.Less(t, prevID, currID, "IDs should be strictly increasing in lexicographical order")
			prevID = currID
		}
	})

	t.Run("Concurrent Uniqueness", func(t *testing.T) {
		const goroutines = 50
		const iterations = 1000
		totalIDs := goroutines * iterations

		ids := make(chan string, totalIDs)
		var wg sync.WaitGroup

		wg.Add(goroutines)
		start := time.Now()
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
		duration := time.Since(start)

		// 맵을 사용하여 중복 검사
		uniqueIDs := make(map[string]struct{}, totalIDs)
		for id := range ids {
			uniqueIDs[id] = struct{}{}
		}

		assert.Equal(t, totalIDs, len(uniqueIDs), "All generated IDs must be unique")
		t.Logf("Generated %d IDs in %v (%.0f IDs/sec)", totalIDs, duration, float64(totalIDs)/duration.Seconds())
	})
}

// TestGenerator_AppendBase62 Base62 인코딩 로직의 정확성을 다양한 입력값으로 검증합니다.
func TestGenerator_AppendBase62(t *testing.T) {
	g := &Generator{}

	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"Zero", 0, "0"},
		{"One", 1, "1"},
		{"Base62-1", 61, "z"},
		{"Base62", 62, "10"},
		{"Base62+1", 63, "11"},
		{"Large Number", 123456789, "8M0kX"},
		{"Max Int64", math.MaxInt64, "AzL8n0Y58m7"}, // 2^63 - 1
		{"Negative (Abs)", -62, "10"},               // 음수는 절대값으로 처리됨
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(g.appendBase62(nil, tt.input))
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestGenerator_AppendBase62FixedLength 고정 길이 패딩 로직을 검증합니다.
func TestGenerator_AppendBase62FixedLength(t *testing.T) {
	g := &Generator{}

	tests := []struct {
		name     string
		input    int64
		length   int
		expected string
	}{
		{"Zero Padding", 0, 6, "000000"},
		{"Partial Padding", 123, 6, "00001z"},
		{"Exact Length", 12345, 4, "03D7"}, // 12345 -> 03D7 (4 chars)
		{"No Padding Needed", 4294967295, 6, "4gfFC3"},
		{"Overflow Length", 4294967295, 4, "4gfFC3"}, // 이미 목표 길이보다 길면 그대로 반환
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(g.appendBase62FixedLength(nil, tt.input, tt.length))
			assert.Equal(t, tt.expected, got)
		})
	}

	t.Run("Padding Preserves Order", func(t *testing.T) {
		// 패딩이 적용된 문자열은 원래 숫자의 크기 순서와 사전순 정렬이 일치해야 합니다.
		// "1" (000001) < "2" (000002)
		val1 := string(g.appendBase62FixedLength(nil, 1, 6))
		val2 := string(g.appendBase62FixedLength(nil, 2, 6))
		assert.Less(t, val1, val2)

		// "61" (00000z) < "62" (000010)
		valMaxDigit := string(g.appendBase62FixedLength(nil, 61, 6))
		valNextBase := string(g.appendBase62FixedLength(nil, 62, 6))
		assert.Less(t, valMaxDigit, valNextBase)
	})
}

// BenchmarkGenerator_New ID 생성 성능을 측정합니다.
func BenchmarkGenerator_New(b *testing.B) {
	g := &Generator{}
	b.ResetTimer()

	// 병렬 실행 성능 측정 (실제 서비스 환경과 유사)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = g.New()
		}
	})
}

// BenchmarkAppendBase62 내부 인코딩 함수의 성능을 측정합니다 (메모리 할당 확인).
func BenchmarkAppendBase62(b *testing.B) {
	g := &Generator{}
	dst := make([]byte, 0, 20)
	input := int64(time.Now().UnixNano())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dst = dst[:0] // Reset usage, keep capacity
		_ = g.appendBase62(dst, input)
	}
}
