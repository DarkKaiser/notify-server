package idgen

import (
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGenerator_New_Format은 생성된 ID가 Base62 형식과 길이 제약 조건을 충족하는지 검증합니다.
func TestGenerator_New_Format(t *testing.T) {
	generator := &Generator{}

	t.Run("ID Format Validation", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			id := string(generator.New())

			// 1. 길이 검증
			// UnixNano(약 19자리) -> Base62 변환 시 약 11자리
			// Sequence(6자리) -> Fixed 6자리
			// 예상 길이: 11 + 6 = 17 ~ 18 (시간 흐름에 따라 증가 가능)
			// 최소 16자 이상이어야 안전함.
			assert.GreaterOrEqual(t, len(id), 16, "ID length check failed: %s", id)

			// 2. 문자셋 검증 (Base62)
			for _, r := range id {
				if !strings.ContainsRune(base62Chars, r) {
					t.Errorf("Invalid character '%c' found in ID: %s", r, id)
				}
			}
		}
	})
}

// TestGenerator_New_Monotonicity는 단일 스레드 환경에서 ID의 단조 증가(시간순 정렬)를 검증합니다.
func TestGenerator_New_Monotonicity(t *testing.T) {
	generator := &Generator{}

	// 매우 짧은 시간 내에 여러 번 호출하여 시퀀스 증가 확인
	const iterations = 10000

	ids := make([]string, iterations)
	for i := 0; i < iterations; i++ {
		ids[i] = string(generator.New())
	}

	for i := 1; i < iterations; i++ {
		prev := ids[i-1]
		curr := ids[i]

		// Lexicographical comparison (String Sort) must strictly increase
		if prev >= curr {
			t.Fatalf("Monotonicity violation at index %d: prev(%s) >= curr(%s)", i, prev, curr)
		}
	}
}

// TestGenerator_New_Concurrency는 멀티 고루틴 환경에서의 충돌 없는 고유성을 검증합니다.
func TestGenerator_New_Concurrency(t *testing.T) {
	generator := &Generator{}

	// Go Race Detector(-race)를 위해 적절한 부하 설정
	const (
		goroutines    = 50
		idsPerRoutine = 2000
	)

	totalIDs := goroutines * idsPerRoutine
	results := make(chan string, totalIDs)
	var wg sync.WaitGroup

	wg.Add(goroutines)

	start := time.Now()
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerRoutine; j++ {
				results <- string(generator.New())
			}
		}()
	}

	wg.Wait()
	close(results)

	duration := time.Since(start)
	t.Logf("Generated %d IDs in %v (%.0f IDs/sec)", totalIDs, duration, float64(totalIDs)/duration.Seconds())

	// 중복 검사
	uniqueMap := make(map[string]struct{}, totalIDs)
	for id := range results {
		if _, exists := uniqueMap[id]; exists {
			t.Fatalf("Collision detected: ID %s generated twice", id)
		}
		uniqueMap[id] = struct{}{}
	}

	assert.Equal(t, totalIDs, len(uniqueMap), "Total unique IDs count mismatch")
}

// TestAppendBase62는 Base62 인코딩 로직을 정밀 검증합니다.
func TestAppendBase62(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"Zero", 0, "0"},
		{"One", 1, "1"},
		{"Base62-1 (z)", 61, "z"},
		{"Base62 (10)", 62, "10"},
		{"Base62+1 (11)", 63, "11"},
		{"Negative (-1 -> 1)", -1, "1"}, // Implementation treats negative as abs
		{"MaxInt64", math.MaxInt64, "AzL8n0Y58m7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 빈 슬라이스에 append
			got := string(appendBase62(nil, tt.input))
			assert.Equal(t, tt.expected, got)

			// 기존 데이터가 있는 슬라이스에 append
			prefix := []byte("PREFIX_")
			gotWithPrefix := string(appendBase62(prefix, tt.input))
			assert.Equal(t, "PREFIX_"+tt.expected, gotWithPrefix)
		})
	}
}

// TestAppendBase62FixedLength는 고정 길이 패딩 로직을 검증합니다.
func TestAppendBase62FixedLength(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		length   int
		expected string // Expected output (padded)
	}{
		{"Zero Padding", 0, 6, "000000"},
		{"Small Number", 1, 6, "000001"},
		{"Exact Fit", 12345, 4, "03D7"}, // Assuming 12345 base62 is "3D7", padded to "03D7"
		{"Negative Number", -1, 6, "000001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(appendBase62FixedLength(nil, tt.input, tt.length))
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestAppendBase62FixedLength_OverflowSafety는 고정 길이를 초과하는 입력에 대한 동작을 검증합니다.
// 현재 구현은 버퍼를 늘려 끼워넣기(Prepend-like)를 수행하여 데이터를 보존합니다.
func TestAppendBase62FixedLength_OverflowSafety(t *testing.T) {
	// Base62(62) = "10" (2글자), Length 1 요청 -> Overflow
	input := int64(62)
	length := 1

	// 기대 결과: "10" (비록 길이는 1을 요청했으나 데이터 손실 없이 "10"이 나와야 함)
	// 경고: 이 경우 정렬 순서는 깨질 수 있음. (보고서 참조)
	got := string(appendBase62FixedLength(nil, input, length))

	assert.Equal(t, "10", got, "Overflow input should be preserved completely")
	assert.Greater(t, len(got), length, "Result length should exceed requested length on overflow")
}

// BenchmarkGenerator_New는 ID 생성기의 성능과 메모리 할당을 측정합니다.
func BenchmarkGenerator_New(b *testing.B) {
	g := &Generator{}

	b.ReportAllocs() // 메모리 할당 횟수 보고
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = g.New()
		}
	})
}

// BenchmarkAppendBase62FixedLength는 내부 함수의 할당 효율성을 측정합니다.
func BenchmarkAppendBase62FixedLength(b *testing.B) {
	dst := make([]byte, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dst = dst[:0]
		_ = appendBase62FixedLength(dst, 123456789, 10)
	}
}
