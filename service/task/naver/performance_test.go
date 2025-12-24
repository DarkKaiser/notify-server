package naver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPerformance_Render Table-Driven 방식을 사용하여 Render 메서드의 HTML 및 Text 포맷팅을 검증합니다.
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
		mark         string
		wants        []string // 결과에 포함되어야 하는 문자열들
		unwants      []string // 결과에 포함되지 않아야 하는 문자열들
	}{
		{
			name:         "HTML 포맷 - 기본",
			perf:         defaultPerf,
			supportsHTML: true,
			mark:         " 🆕",
			wants: []string{
				fmt.Sprintf("<a href=\"%s?query=%%ED%%85%%8C%%EC%%8A%%A4%%ED%%8A%%B8+%%EA%%B3%%B5%%EC%%97%%B0\"><b>테스트 공연</b></a>", searchResultPageURL),
				"테스트 극장",
				" 🆕",
				"• 장소 :",
			},
			unwants: []string{"☞ 테스트 공연 🆕"}, // Text 포맷 확인용
		},
		{
			name:         "Text 포맷 - 기본",
			perf:         defaultPerf,
			supportsHTML: false,
			mark:         "",
			wants: []string{
				"☞ 테스트 공연",
				"• 장소 : 테스트 극장",
			},
			unwants: []string{"<b>", "</a>", "<a href"},
		},
		{
			name: "Text 포맷 - 특수문자 처리 (HTML Escape 방지)",
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
			unwants: []string{"Tom &amp; Jerry", "Cinema &amp; Theater"},
		},
		{
			name: "HTML 포맷 - 특수문자 이스케이프 (XSS 방지)",
			perf: &performance{
				Title: "<script>alert(1)</script>",
				Place: "Hack Place",
			},
			supportsHTML: true,
			mark:         "",
			wants: []string{
				"&lt;script&gt;alert(1)&lt;/script&gt;", // 이스케이프 확인
			},
			unwants: []string{"<script>"},
		},
		{
			name: "빈 필드 처리",
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
				assert.Contains(t, result, want, "Result should contain expected string")
			}
			for _, unwant := range tt.unwants {
				assert.NotContains(t, result, unwant, "Result should NOT contain unexpected string")
			}
		})
	}
}

// TestPerformance_Key Key 메서드가 고유 식별자를 올바르게 생성하는지 검증합니다.
func TestPerformance_Key(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		perf     *performance
		expected string
	}{
		{
			name: "정상적인 키 생성",
			perf: &performance{
				Title: "뮤지컬 캣츠",
				Place: "브로드웨이극장",
			},
			expected: "뮤지컬 캣츠|브로드웨이극장",
		},
		{
			name: "특수문자 포함",
			perf: &performance{
				Title: "공연|제목",
				Place: "장소|이름",
			},
			expected: "공연|제목|장소|이름",
		},
		{
			name: "빈 문자열",
			perf: &performance{
				Title: "",
				Place: "",
			},
			expected: "|",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.perf.Key()
			assert.Equal(t, tt.expected, result, "Key() 결과가 예상과 일치해야 합니다")
		})
	}
}

// TestPerformance_Equals Equals 메서드가 객체 동등성을 올바르게 판단하는지 검증합니다.
func TestPerformance_Equals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		perf1    *performance
		perf2    *performance
		expected bool
	}{
		{
			name: "동일한 공연 (Title, Place 일치)",
			perf1: &performance{
				Title:     "뮤지컬 캣츠",
				Place:     "브로드웨이극장",
				Thumbnail: "thumb1.jpg",
			},
			perf2: &performance{
				Title:     "뮤지컬 캣츠",
				Place:     "브로드웨이극장",
				Thumbnail: "thumb2.jpg",
			},
			expected: true,
		},
		{
			name: "다른 공연 (Title 불일치)",
			perf1: &performance{
				Title: "뮤지컬 캣츠",
				Place: "브로드웨이극장",
			},
			perf2: &performance{
				Title: "뮤지컬 레미제라블",
				Place: "브로드웨이극장",
			},
			expected: false,
		},
		{
			name: "다른 공연 (Place 불일치)",
			perf1: &performance{
				Title: "뮤지컬 캣츠",
				Place: "브로드웨이극장",
			},
			perf2: &performance{
				Title: "뮤지컬 캣츠",
				Place: "샤롯데씨어터",
			},
			expected: false,
		},
		{
			name:     "첫 번째가 nil",
			perf1:    nil,
			perf2:    &performance{Title: "T", Place: "P"},
			expected: false,
		},
		{
			name:     "두 번째가 nil",
			perf1:    &performance{Title: "T", Place: "P"},
			perf2:    nil,
			expected: false,
		},
		{
			name:     "둘 다 nil",
			perf1:    nil,
			perf2:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.perf1.Equals(tt.perf2)
			assert.Equal(t, tt.expected, result, "Equals() 결과가 예상과 일치해야 합니다")
		})
	}
}

// TestPerformance_Consistency Key()와 Equals()의 논리적 일관성을 검증합니다.
func TestPerformance_Consistency(t *testing.T) {
	t.Parallel()

	perf1 := &performance{Title: "A", Place: "B"}
	perf2 := &performance{Title: "A", Place: "B"}
	perf3 := &performance{Title: "A", Place: "C"}

	t.Run("Reflexivity (반사성)", func(t *testing.T) {
		assert.True(t, perf1.Equals(perf1))
	})

	t.Run("Symmetry (대칭성)", func(t *testing.T) {
		assert.Equal(t, perf1.Equals(perf2), perf2.Equals(perf1))
	})

	t.Run("Key Consistency (Key 일관성)", func(t *testing.T) {
		if perf1.Equals(perf2) {
			assert.Equal(t, perf1.Key(), perf2.Key(), "Equals가 true이면 Key도 동일해야 함")
		}
		if !perf1.Equals(perf3) {
			assert.NotEqual(t, perf1.Key(), perf3.Key(), "Equals가 false이면 Key도 달라야 함")
		}
	})
}

// TestPerformance_Scenario_Example Render 및 Key 메서드의 실제 사용 시나리오를 보여주는 테스트입니다.
func TestPerformance_Scenario_Example(t *testing.T) {
	t.Parallel()

	p := &performance{
		Title: "Test Concert",
		Place: "Seoul Arts Center",
	}

	t.Run("Rendering Workflow", func(t *testing.T) {
		// 1. Text 알림 생성
		text := p.Render(false, "")
		assert.Contains(t, text, "Test Concert")
		assert.Contains(t, text, "Seoul Arts Center")

		// 2. HTML 알림 생성 (Web/Telegram 등)
		html := p.Render(true, " NEW")
		assert.Contains(t, html, "<b>Test Concert</b>")
		assert.Contains(t, html, " NEW")
	})

	t.Run("Identifier Workflow", func(t *testing.T) {
		key := p.Key()
		assert.Equal(t, "Test Concert|Seoul Arts Center", key)
	})
}

// -----------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkPerformance_Render_Text(b *testing.B) {
	p := &performance{
		Title: "Very Long Performance Title To Simulate Real World Scenario",
		Place: "Very Long Place Name To Simulate Real World Scenario",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Render(false, " MARK")
	}
}

func BenchmarkPerformance_Render_HTML(b *testing.B) {
	p := &performance{
		Title: "Very Long Performance Title To Simulate Real World Scenario",
		Place: "Very Long Place Name To Simulate Real World Scenario",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Render(true, " MARK")
	}
}

func BenchmarkPerformance_Key(b *testing.B) {
	p := &performance{
		Title: "Title",
		Place: "Place",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Key()
	}
}
