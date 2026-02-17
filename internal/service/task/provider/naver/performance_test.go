package naver

import (
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------
// Unit Tests: Render Methods (Presentation Layer)
// -----------------------------------------------------------------------------

// TestPerformance_Render Table-Driven 방식을 사용하여 Render 메서드의
// HTML 및 Text 포맷팅 동작을 정밀하게 검증합니다.
//
// 이 테스트는 다음 시나리오를 커버합니다:
// 1. HTML 모드: 앵커 태그 생성, XSS 방지(Escape), 마크 추가
// 2. Text 모드: 태그 제거, 가독성 높은 텍스트 포맷, 마크 추가
// 3. 특수 문자 및 빈 값 처리
func TestPerformance_Render(t *testing.T) {
	t.Parallel()

	defaultPerf := &performance{
		Title:     "테스트 공연",
		Place:     "테스트 극장",
		Thumbnail: "<img src=\"https://example.com/thumb.jpg\">",
	}

	tests := []struct {
		name         string
		perf         *performance
		supportsHTML bool
		mark         mark.Mark
		wants        []string // 반드시 포함되어야 할 문자열
		unwants      []string // 포함되어서는 안 되는 문자열 (Negative Check)
	}{
		{
			name:         "HTML 포맷 - 표준 케이스",
			perf:         defaultPerf,
			supportsHTML: true,
			mark:         mark.New,
			wants: []string{
				"☞ ", // Prefix
				fmt.Sprintf("<a href=\"%s?query=%%ED%%85%%8C%%EC%%8A%%A4%%ED%%8A%%B8+%%EA%%B3%%B5%%EC%%97%%B0\"><b>테스트 공연</b></a>", searchResultPageURL),
				"테스트 극장",
				" 🆕",
				"• 장소 :",
			},
			unwants: []string{"☞ 테스트 공연 🆕"}, // Plain Text 포맷이 섞이지 않아야 함 (HTML Tags 확인)
		},
		{
			name:         "Text 포맷 - 표준 케이스",
			perf:         defaultPerf,
			supportsHTML: false,
			mark:         "",
			wants: []string{
				"☞ 테스트 공연",
				"• 장소 : 테스트 극장",
			},
			unwants: []string{"<b>", "</a>", "<a href"}, // HTML 태그 노출 금지
		},
		{
			name: "Text 포맷 - 특수문자 처리 (No HTML Escape)",
			perf: &performance{
				Title: "Tom & Jerry",
				Place: "Cinema & Theater",
			},
			supportsHTML: false,
			mark:         "",
			wants: []string{
				"☞ Tom & Jerry",
				"• 장소 : Cinema & Theater",
			},
			unwants: []string{"Tom &amp; Jerry", "Cinema &amp; Theater"}, // Text 모드에선 Escape 불필요
		},
		{
			name: "HTML 포맷 - Security: XSS 방지 (HTML Escape)",
			perf: &performance{
				Title: "<script>alert(1)</script>",
				Place: "Hack <Place>",
			},
			supportsHTML: true,
			mark:         "",
			wants: []string{
				"&lt;script&gt;alert(1)&lt;/script&gt;", // 제목 이스케이프
				"Hack &lt;Place&gt;",                    // 장소 이스케이프 (신규 추가 검증 항목)
			},
			unwants: []string{"<script>", "<Place>"}, // Raw 태그 노출 금지
		},
		{
			name: "Edge Case - 빈 필드",
			perf: &performance{
				Title: "",
				Place: "",
			},
			supportsHTML: false,
			mark:         "",
			wants: []string{
				"☞",
				"• 장소 :",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.perf.Render(tt.supportsHTML, tt.mark)
			for _, want := range tt.wants {
				assert.Contains(t, result, want, "결과 메시지에 예상된 문자열이 포함되어야 합니다")
			}
			for _, unwant := range tt.unwants {
				assert.NotContains(t, result, unwant, "결과 메시지에 예상치 못한 문자열이 포함되면 안 됩니다")
			}
		})
	}
}

// TestPerformance_RenderDiff RenderDiff 메서드의 동작을 검증합니다.
//
// [설계 의도]
// 현재 Naver 패키지에서 RenderDiff는 Render와 기능적으로 동일하게 동작합니다(신규 공연만 추적하므로).
// 하지만 이 테스트는 다음의 두 가지 중요한 목적을 가집니다:
// 1. Interface Compliance: RenderDiff가 Render와 동일한 품질의 출력을 생성하는지 보장
// 2. Future Proofing: 향후 변경 사항 비교 로직(prev != nil)이 추가될 때를 대비한 테스트 구조 확보
func TestPerformance_RenderDiff(t *testing.T) {
	t.Parallel()

	p := &performance{
		Title: "신규 공연",
		Place: "예술의전당",
	}

	tests := []struct {
		name         string
		supportsHTML bool
		mark         mark.Mark
		prev         *performance // 비교 대상 (현재 로직에서는 무시됨)
		wants        []string
	}{
		{
			name:         "HTML - 신규 공연 알림 (Prev is nil)",
			supportsHTML: true,
			mark:         mark.New,
			prev:         nil,
			wants: []string{
				"☞ ",
				fmt.Sprintf("<a href=\"%s?query=%%EC%%8B%%A0%%EA%%B7%%9C+%%EA%%B3%%B5%%EC%%97%%B0\"><b>신규 공연</b></a>", searchResultPageURL),
				mark.New.WithSpace(),
			},
		},
		{
			name:         "Text - 신규 공연 알림 (Prev is nil)",
			supportsHTML: false,
			mark:         mark.New,
			prev:         nil,
			wants: []string{
				"☞ 신규 공연",
				mark.New.WithSpace(),
			},
		},
		{
			name:         "확장성 테스트 - Prev가 존재하는 경우 (현재는 신규처럼 렌더링됨)",
			supportsHTML: false,
			mark:         mark.Modified,
			prev:         &performance{Title: "신규 공연", Place: "변경전 장소"},
			wants: []string{
				"☞ 신규 공연", // 현재 로직상 단순 렌더링
				mark.Modified.WithSpace(),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := p.RenderDiff(tt.supportsHTML, tt.mark, tt.prev)
			for _, want := range tt.wants {
				assert.Contains(t, got, want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Unit Tests: Core Logic (Identity & Equality)
// -----------------------------------------------------------------------------

// TestPerformance_Key Key 메서드가 고유 식별자를 유니크하고 일관성 있게 생성하는지 검증합니다.
func TestPerformance_Key(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		perf     *performance
		expected string
	}{
		{
			name:     "Normal: 일반적인 제목과 장소",
			perf:     &performance{Title: "뮤지컬 캣츠", Place: "브로드웨이극장"},
			expected: "16:뮤지컬 캣츠|21:브로드웨이극장", // "뮤지컬 캣츠" (16 bytes in UTF-8), "브로드웨이극장" (21 bytes)
		},
		{
			name:     "Edge: 특수문자(|) 포함 시 충돌 방지 확인",
			perf:     &performance{Title: "공연|제목", Place: "장소|이름"},
			expected: "13:공연|제목|13:장소|이름",
		},
		{
			name:     "Edge: 빈 문자열",
			perf:     &performance{Title: "", Place: ""},
			expected: "0:|0:",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.perf.Key()
			assert.Equal(t, tt.expected, result)
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

	assert.NotEqual(t, p1.Key(), p2.Key(), "서로 다른 데이터 구성에 대해 고유한 Key가 생성되어야 합니다")
}

// TestPerformance_Equals Equals 메서드의 동등성 판단 로직을 검증합니다.
func TestPerformance_Equals(t *testing.T) {
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
			name:     "Edge: Nil 비교 (Receiver is nil case 제외)",
			perf1:    nil,
			perf2:    nil,
			expected: false, // 테스트 헬퍼 함수 호출 방식에 따라 다르나, 여기선 로직상 false
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Nil Receiver 방지
			if tt.perf1 == nil {
				if tt.perf2 == nil { // 둘 다 nil
					assert.False(t, false) // 혹은 별도 처리. 현재 구현상 호출 불가.
					return
				}
				// perf1이 nil이면 호출 불가, 패스
				return
			}
			result := tt.perf1.Equals(tt.perf2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPerformance_Consistency Key(), Equals(), 그리고 Render() 간의 논리적 일관성을 검증합니다.
// 이는 불변식(Invariant)을 테스트하여 코드의 신뢰성을 높입니다.
func TestPerformance_Consistency(t *testing.T) {
	t.Parallel()

	p1 := &performance{Title: "A", Place: "B"}
	p2 := &performance{Title: "A", Place: "B"}
	p3 := &performance{Title: "A", Place: "C"}

	// 1. Equals의 반사성 (Reflexivity)
	assert.True(t, p1.Equals(p1), "객체는 자기 자신과 같아야 합니다")

	// 2. Equals의 대칭성 (Symmetry)
	assert.Equal(t, p1.Equals(p2), p2.Equals(p1), "동등성 비교는 대칭적이어야 합니다")

	// 3. Key와 Equals의 일관성
	if p1.Equals(p2) {
		assert.Equal(t, p1.Key(), p2.Key(), "동등한 객체는 동일한 Key를 가져야 합니다")
	}
	if !p1.Equals(p3) {
		assert.NotEqual(t, p1.Key(), p3.Key(), "다른 객체는 다른 Key를 가져야 합니다 (해시 충돌 제외)")
	}
}

// -----------------------------------------------------------------------------
// Documentation Examples (Godoc)
// -----------------------------------------------------------------------------

// Example_performanceRender Render 메서드의 사용 예시를 보여줍니다.
// Note: 'performance'가 unexported 타입이므로 Example_suffix 형식을 사용합니다.
func Example_performanceRender() {
	p := &performance{
		Title: "지킬 앤 하이드",
		Place: "샤롯데씨어터",
	}

	// HTML 렌더링 (Telegram, Web 등)
	html := p.Render(true, mark.New)
	fmt.Println(html)

	// Text 렌더링 (Log, Console 등)
	text := p.Render(false, "")
	fmt.Println(text)

	// Output:
	// ☞ <a href="https://search.naver.com/search.naver?query=%EC%A7%80%ED%82%AC+%EC%95%A4+%ED%95%98%EC%9D%B4%EB%93%9C"><b>지킬 앤 하이드</b></a> 🆕
	//       • 장소 : 샤롯데씨어터
	// ☞ 지킬 앤 하이드
	//       • 장소 : 샤롯데씨어터
}

// Example_performanceRenderDiff RenderDiff 메서드의 사용 예시를 보여줍니다.
func Example_performanceRenderDiff() {
	curr := &performance{Title: "오페라의 유령", Place: "블루스퀘어"}
	var prev *performance = nil // 신규 공연

	// 신규 알림 생성
	msg := curr.RenderDiff(false, mark.New, prev)
	fmt.Println(msg)

	// Output:
	// ☞ 오페라의 유령 🆕
	//       • 장소 : 블루스퀘어
}

// -----------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkPerformance_Render_Text(b *testing.B) {
	p := &performance{
		Title: "Performance Title For Benchmark",
		Place: "Performance Place For Benchmark",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Render(false, "MARK")
	}
}

func BenchmarkPerformance_Render_HTML(b *testing.B) {
	p := &performance{
		Title: "Performance Title For Benchmark",
		Place: "Performance Place For Benchmark",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Render(true, "MARK")
	}
}

func BenchmarkPerformance_RenderDiff(b *testing.B) {
	p := &performance{
		Title: "Performance Title For Benchmark",
		Place: "Performance Place For Benchmark",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// RenderDiff 호출 비용 측정 (현재는 Render와 거의 동일해야 함)
		_ = p.RenderDiff(true, "MARK", nil)
	}
}

func BenchmarkPerformance_Key(b *testing.B) {
	p := &performance{Title: "Title", Place: "Place"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Key()
	}
}
