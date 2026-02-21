package naver

import (
	"fmt"
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
	// Helper function for creating performance diffs
	newDiff := func(title string, eventType performanceEventType) performanceDiff {
		return performanceDiff{
			Type:        eventType,
			Performance: &performance{Title: title, Place: "Test Place"},
		}
	}

	tests := []struct {
		name         string
		diffs        []performanceDiff
		supportsHTML bool
		wants        []string
		unwants      []string
	}{
		{
			name:         "빈 리스트: 빈 문자열 반환",
			diffs:        []performanceDiff{},
			supportsHTML: false,
			wants:        []string{},
			unwants:      []string{"☞", "Test Place"},
		},
		{
			name: "신규 공연만 렌더링 (Text 모드)",
			diffs: []performanceDiff{
				newDiff("New Musical 1", performanceEventNew),
				newDiff("Old Musical", performanceEventNone),                                        // 무시되어야 함
				{Type: performanceEventType(99), Performance: &performance{Title: "Unknown Event"}}, // 무시되어야 함
				newDiff("New Musical 2", performanceEventNew),
			},
			supportsHTML: false,
			wants: []string{
				"New Musical 1",
				"New Musical 2",
				"🆕",
				"\n\n", // 항목 간 구분자
			},
			unwants: []string{
				"Old Musical",
				"Unknown Event",
				"<a href=", // 일반 텍스트이므로 HTML 태그 없어야 함
			},
		},
		{
			name: "신규 공연 렌더링 (HTML 모드)",
			diffs: []performanceDiff{
				newDiff("HTML Musical 1", performanceEventNew),
			},
			supportsHTML: true,
			wants: []string{
				"HTML Musical 1",
				"<a href=",
				"<b>",
				"🆕",
			},
			unwants: []string{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := renderPerformanceDiffs(tt.diffs, tt.supportsHTML)

			if len(tt.diffs) == 0 {
				assert.Empty(t, result, "빈 리스트인 경우 빈 문자열을 반환해야 합니다.")
				return
			}

			for _, want := range tt.wants {
				assert.Contains(t, result, want, "결과 메시지에 예상된 문자열이 포함되어야 합니다.")
			}
			for _, unwant := range tt.unwants {
				assert.NotContains(t, result, unwant, "결과 메시지에 예상치 못한 문자열이 포함되면 안 됩니다.")
			}
		})
	}
}

func TestRenderCurrentStatus(t *testing.T) {
	tests := []struct {
		name         string
		snapshot     *watchNewPerformancesSnapshot
		supportsHTML bool
		wants        []string
		unwants      []string
	}{
		{
			name:         "Snapshot이 nil인 경우: 빈 문자열",
			snapshot:     nil,
			supportsHTML: false,
			wants:        []string{},
			unwants:      []string{"☞"},
		},
		{
			name:         "Performances가 비어있는 경우: 빈 문자열",
			snapshot:     &watchNewPerformancesSnapshot{Performances: []*performance{}},
			supportsHTML: false,
			wants:        []string{},
			unwants:      []string{"☞"},
		},
		{
			name: "데이터가 여러 개 있는 경우 - Text 모드",
			snapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					{Title: "Current Musical A", Place: "Seoul"},
					{Title: "Current Musical B", Place: "Busan"},
				},
			},
			supportsHTML: false,
			wants: []string{
				"Current Musical A",
				"Seoul",
				"Current Musical B",
				"Busan",
				"\n\n", // 아이템 간 구분
			},
			unwants: []string{
				"🆕", // 현재 상태 목록에는 New 마크가 뜨면 안 됨
				"<a href=",
				"<b>",
			},
		},
		{
			name: "데이터가 여러 개 있는 경우 - HTML 모드",
			snapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					{Title: "HTML Musical A", Place: "Seoul"},
					{Title: "HTML Musical B", Place: "Busan"},
				},
			},
			supportsHTML: true,
			wants: []string{
				"HTML Musical A",
				"Seoul",
				"HTML Musical B",
				"Busan",
				"<a href=",
				"<b>",
				"\n\n", // 아이템 간 구분
			},
			unwants: []string{
				"🆕", // 현재 상태 목록에는 New 마크가 뜨면 안 됨
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := renderCurrentStatus(tt.snapshot, tt.supportsHTML)

			if tt.snapshot == nil || len(tt.snapshot.Performances) == 0 {
				assert.Empty(t, result, "데이터가 없는 경우 빈 문자열을 반환해야 합니다.")
				return
			}

			for _, want := range tt.wants {
				assert.Contains(t, result, want, "결과 메시지에 예상된 문자열이 포함되어야 합니다.")
			}
			for _, unwant := range tt.unwants {
				assert.NotContains(t, result, unwant, "결과 메시지에 예상치 못한 문자열이 포함되면 안 됩니다.")
			}
		})
	}
}
