package naver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------
// Unit Tests: Core Logic (Identity & Equality)
// -----------------------------------------------------------------------------

// TestPerformance_Key Key 메서드가 고유 식별자를 유니크하고 일관성 있게 생성하는지 검증합니다.
func TestPerformance_Key(t *testing.T) {
	tests := []struct {
		name string
		p    *performance
		want string
	}{
		{
			name: "Normal Case",
			p:    &performance{Title: "Title", Place: "Place"},
			want: "5:Title|5:Place",
		},
		{
			name: "Empty Fields",
			p:    &performance{Title: "", Place: ""},
			want: "0:|0:",
		},
		{
			name: "Contains Delimiter",
			p:    &performance{Title: "A|B", Place: "C:D"},
			want: "3:A|B|3:C:D",
		},
		{
			name: "Emoji & Unicode",
			p:    &performance{Title: "공연🎭", Place: "장소🏰"},
			// len("공연🎭") -> 3(공) + 3(연) + 4(🎭) = 10 bytes
			// len("장소🏰") -> 3(장) + 3(소) + 4(🏰) = 10 bytes
			want: "10:공연🎭|10:장소🏰",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.key(); got != tt.want {
				t.Errorf("performance.key() = %v, want %v", got, tt.want)
			}
			// Idempotency Check (반복 호출 시에도 동일한 값 반환)
			if got := tt.p.key(); got != tt.want {
				t.Errorf("performance.key() second call = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPerformance_Key_Collision 이전에 발생 가능했던 Key 충돌 시나리오가 해결되었는지 검증합니다.
func TestPerformance_Key_Collision(t *testing.T) {
	t.Parallel()

	// (A|B, C) 와 (A, B|C) 는 단순 결합 시 "A|B|C"로 동일했으나,
	// 이스케이프 로직 적용 후에는 "A||B|C"와 "A|B||C"로 명확히 구분되어야 합니다.
	p1 := &performance{Title: "A|B", Place: "C"}
	p2 := &performance{Title: "A", Place: "B|C"}

	assert.NotEqual(t, p1.key(), p2.key(), "서로 다른 데이터 구성에 대해 고유한 Key가 생성되어야 합니다")
}

func TestPerformance_Key_Concurrency(t *testing.T) {
	p := &performance{Title: "Title", Place: "Place"}
	want := "5:Title|5:Place"

	// 100개의 고루틴에서 동시에 Key() 호출
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			if got := p.key(); got != want {
				t.Errorf("Concurrent key() = %v, want %v", got, want)
			}
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

// TestPerformance_Equals Equals 메서드의 동등성 판단 로직을 검증합니다.
func TestPerformance_equals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		perf1    *performance
		perf2    *performance
		expected bool
	}{
		{
			name:     "Equal: 모든 필드(Title, Place) 일치",
			perf1:    &performance{Title: "A", Place: "B", Thumbnail: "img1"},
			perf2:    &performance{Title: "A", Place: "B", Thumbnail: "img2"}, // 썸네일 달라도 키가 같으면 동등
			expected: true,
		},
		{
			name:     "Not Equal: Title 불일치",
			perf1:    &performance{Title: "A", Place: "B"},
			perf2:    &performance{Title: "X", Place: "B"},
			expected: false,
		},
		{
			name:     "Not Equal: Place 불일치",
			perf1:    &performance{Title: "A", Place: "B"},
			perf2:    &performance{Title: "A", Place: "Y"},
			expected: false,
		},
		{
			name:     "Edge: Comparison with Nil",
			perf1:    &performance{Title: "A", Place: "B"},
			perf2:    nil,
			expected: false,
		},
		{
			name:     "Edge: Receiver is Nil (Safe Check via Helper? No, method call on nil panics unless handled)",
			perf1:    nil,
			perf2:    &performance{Title: "A", Place: "B"},
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.perf1.equals(tt.perf2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPerformance_contentEquals(t *testing.T) {
	base := &performance{Title: "T", Place: "P", Thumbnail: "URL1"}

	tests := []struct {
		name  string
		other *performance
		want  bool
	}{
		{
			name:  "Identical",
			other: &performance{Title: "T", Place: "P", Thumbnail: "URL1"},
			want:  true,
		},
		{
			name:  "Different Title",
			other: &performance{Title: "T2", Place: "P", Thumbnail: "URL1"},
			want:  false,
		},
		{
			name:  "Different Place",
			other: &performance{Title: "T", Place: "P2", Thumbnail: "URL1"},
			want:  false,
		},
		{
			name:  "Different Thumbnail",
			other: &performance{Title: "T", Place: "P", Thumbnail: "URL2"},
			want:  false,
		},
		{
			name:  "Nil Comparison",
			other: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := base.contentEquals(tt.other); got != tt.want {
				t.Errorf("performance.contentEquals() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPerformance_Consistency key(), equals(), 그리고 Render() 간의 논리적 일관성을 검증합니다.
// 이는 불변식(Invariant)을 테스트하여 코드의 신뢰성을 높입니다.
func TestPerformance_Consistency(t *testing.T) {
	t.Parallel()

	p1 := &performance{Title: "A", Place: "B"}
	p2 := &performance{Title: "A", Place: "B"}
	p3 := &performance{Title: "A", Place: "C"}

	// 1. Equals의 반사성 (Reflexivity)
	assert.True(t, p1.equals(p1), "객체는 자기 자신과 같아야 합니다")

	// 2. Equals의 대칭성 (Symmetry)
	assert.Equal(t, p1.equals(p2), p2.equals(p1), "동등성 비교는 대칭적이어야 합니다")

	// 3. Key와 Equals의 일관성
	if p1.equals(p2) {
		assert.Equal(t, p1.key(), p2.key(), "동등한 객체는 동일한 Key를 가져야 합니다")
	}
	if !p1.equals(p3) {
		assert.NotEqual(t, p1.key(), p3.key(), "다른 객체는 다른 Key를 가져야 합니다 (해시 충돌 제외)")
	}
}

// -----------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------
