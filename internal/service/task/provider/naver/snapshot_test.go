package naver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnapshot_Compare(t *testing.T) {
	// 헬퍼: 공연 객체 생성
	newPerf := func(title, place, thumbnail string) *performance {
		return &performance{Title: title, Place: place, Thumbnail: thumbnail}
	}

	tests := []struct {
		name              string
		currentSnapshot   *watchNewPerformancesSnapshot
		prevSnapshot      *watchNewPerformancesSnapshot
		expectedDiffCount int
		expectedChanges   bool
		validator         func(t *testing.T, diffs []performanceDiff)
	}{
		{
			name: "최초 실행 (prev is nil): 모든 공연이 신규로 감지됨",
			currentSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					newPerf("Musical A", "Seoul", "img1"),
					newPerf("Musical B", "Busan", "img2"),
				},
			},
			prevSnapshot:      nil,
			expectedDiffCount: 2,
			expectedChanges:   true,
			validator: func(t *testing.T, diffs []performanceDiff) {
				assert.Equal(t, performanceEventNew, diffs[0].Type)
				assert.Equal(t, "Musical A", diffs[0].Performance.Title)
				assert.Equal(t, performanceEventNew, diffs[1].Type)
				assert.Equal(t, "Musical B", diffs[1].Performance.Title)
			},
		},
		{
			name: "변경 없음: 동일한 스냅샷",
			currentSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					newPerf("Musical A", "Seoul", "img1"),
				},
			},
			prevSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					newPerf("Musical A", "Seoul", "img1"),
				},
			},
			expectedDiffCount: 0,
			expectedChanges:   false,
		},
		{
			name: "신규 공연 추가됨",
			currentSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					newPerf("Musical A", "Seoul", "img1"),
					newPerf("Musical B", "Busan", "img2"), // New
				},
			},
			prevSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					newPerf("Musical A", "Seoul", "img1"),
				},
			},
			expectedDiffCount: 1,
			expectedChanges:   true,
			validator: func(t *testing.T, diffs []performanceDiff) {
				assert.Equal(t, performanceEventNew, diffs[0].Type)
				assert.Equal(t, "Musical B", diffs[0].Performance.Title)
			},
		},
		{
			name: "공연 삭제됨",
			currentSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					// Musical A removed
					newPerf("Musical B", "Busan", "img2"),
				},
			},
			prevSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					newPerf("Musical A", "Seoul", "img1"),
					newPerf("Musical B", "Busan", "img2"),
				},
			},
			expectedDiffCount: 0,    // 삭제는 diffs에 포함되지 않음 (알림 대상 아님)
			expectedChanges:   true, // 개수 변경으로 인한 스냅샷 갱신 필요
		},
		{
			name: "공연 정보 변경됨 (썸네일 변경)",
			currentSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					newPerf("Musical A", "Seoul", "img_changed"), // Thumbnail Changed
				},
			},
			prevSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					newPerf("Musical A", "Seoul", "img1"),
				},
			},
			expectedDiffCount: 0,    // 단순 정보 변경은 diffs에 포함되지 않음 (알림 대상 아님, 정책에 따름)
			expectedChanges:   true, // 내용 변경으로 인한 스냅샷 갱신 필요
		},
		{
			name: "복합 케이스: 추가 + 삭제",
			currentSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					newPerf("Musical C", "Daegu", "img3"), // New
					// Musical A removed
				},
			},
			prevSnapshot: &watchNewPerformancesSnapshot{
				Performances: []*performance{
					newPerf("Musical A", "Seoul", "img1"),
				},
			},
			// Musical C 추가됨
			expectedDiffCount: 1,
			expectedChanges:   true,
			validator: func(t *testing.T, diffs []performanceDiff) {
				assert.Equal(t, "Musical C", diffs[0].Performance.Title)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			diffs, hasChanges := tt.currentSnapshot.Compare(tt.prevSnapshot)

			// Then
			assert.Equal(t, tt.expectedChanges, hasChanges, "hasChanges 값 불일치")
			assert.Len(t, diffs, tt.expectedDiffCount, "diffs 개수 불일치")

			if tt.validator != nil {
				tt.validator(t, diffs)
			}
		})
	}
}
