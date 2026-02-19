package naver

import (
	"fmt"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/stretchr/testify/assert"
)

func TestRenderPerformance(t *testing.T) {
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
				fmt.Sprintf("<a href=\"%s?query=%%ED%%85%%8C%%EC%%8A%%A4%%ED%%8A%%B8+%%EA%%B3%%B5%%EC%%97%%B0\"><b>테스트 공연</b></a>", naverSearchURL),
				"테스트 극장",
				" 🆕",
				"• 장소 :",
			},
			unwants: []string{"☞ 테스트 공연 🆕"}, // Plain Text 포맷이 섞이지 않아야 함
		},
		{
			name:         "Text 포맷 - 표준 케이스",
			perf:         defaultPerf,
			supportsHTML: false,
			mark:         "",
			wants: []string{
				"☞ 테스트 공연 (" + naverSearchURL + "?query=%ED%85%8C%EC%8A%A4%ED%8A%B8+%EA%B3%B5%EC%97%B0)",
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
				"☞ Tom & Jerry (" + naverSearchURL + "?query=Tom+%26+Jerry)",
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
				"Hack &lt;Place&gt;",                    // 장소 이스케이프
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
			result := renderPerformance(tt.perf, tt.supportsHTML, tt.mark)
			for _, want := range tt.wants {
				assert.Contains(t, result, want, "결과 메시지에 예상된 문자열이 포함되어야 합니다")
			}
			for _, unwant := range tt.unwants {
				assert.NotContains(t, result, unwant, "결과 메시지에 예상치 못한 문자열이 포함되면 안 됩니다")
			}
		})
	}
}

func TestRenderPerformanceDiffs(t *testing.T) {
	// 헬퍼
	newDiff := func(title string, eventType performanceEventType) performanceDiff {
		return performanceDiff{
			Type:        eventType,
			Performance: &performance{Title: title, Place: "Place"},
		}
	}

	t.Run("빈 리스트: 빈 문자열 반환", func(t *testing.T) {
		assert.Empty(t, renderPerformanceDiffs([]performanceDiff{}, true))
	})

	t.Run("신규 공연만 렌더링", func(t *testing.T) {
		diffs := []performanceDiff{
			newDiff("New Musical 1", performanceEventNew),
			newDiff("Deleted Musical", performanceEventNone), // Should be ignored (Type None or specific Delete type if exists, but snapshot uses EventNew/None mainly)
			// snapshot.go 정의상 Delete 이벤트는 diffs에 포함되지 않으므로,
			// 여기서는 performanceEventNew가 아닌 다른 타입이 왔을 때 무시되는지 확인 (코드상 if diff.Type == performanceEventNew)
			{Type: performanceEventType(99), Performance: &performance{Title: "Unknown Event"}},
			newDiff("New Musical 2", performanceEventNew),
		}

		result := renderPerformanceDiffs(diffs, false)

		assert.Contains(t, result, "New Musical 1")
		assert.Contains(t, result, "New Musical 2")
		assert.NotContains(t, result, "Deleted Musical")
		assert.NotContains(t, result, "Unknown Event")

		// 줄바꿈으로 구분되는지 확인
		assert.Contains(t, result, "\n\n")
		// 항목이 2개이므로 구분자는 1개여야 함 (New Musical 1 ... \n\n ... New Musical 2)
		assert.Equal(t, 1, strings.Count(result, "\n\n"))
	})
}

func TestRenderCurrentStatus(t *testing.T) {
	t.Run("데이터 없음: 안내 메시지 반환", func(t *testing.T) {
		assert.Contains(t, renderCurrentStatus(nil, false), "등록된 공연정보가 존재하지 않습니다.")
		assert.Contains(t, renderCurrentStatus(&watchNewPerformancesSnapshot{}, false), "등록된 공연정보가 존재하지 않습니다.")
	})

	t.Run("데이터 있음: 헤더와 목록 반환", func(t *testing.T) {
		snapshot := &watchNewPerformancesSnapshot{
			Performances: []*performance{
				{Title: "Musical A", Place: "Place A"},
				{Title: "Musical B", Place: "Place B"},
			},
		}

		result := renderCurrentStatus(snapshot, false)

		// 헤더 확인
		assert.Contains(t, result, "신규로 등록된 공연정보가 없습니다.")
		assert.Contains(t, result, "현재 등록된 공연정보는 아래와 같습니다:")

		// 목록 확인
		assert.Contains(t, result, "Musical A")
		assert.Contains(t, result, "Place A")
		assert.Contains(t, result, "Musical B")
		assert.Contains(t, result, "Place B")

		// 구분선 확인
		assert.Contains(t, result, "\n\n")
	})
}
