package naver

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests: JSON Serialization
// =============================================================================

func TestSnapshot_JSON(t *testing.T) {
	t.Run("Marshaling", func(t *testing.T) {
		snap := &watchNewPerformancesSnapshot{
			Performances: []*performance{
				{Title: "Title1", Place: "Place1", Thumbnail: "Thumb1"},
			},
		}

		data, err := json.Marshal(snap)
		require.NoError(t, err)
		assert.JSONEq(t, `{"performances":[{"title":"Title1","place":"Place1","thumbnail":"Thumb1"}]}`, string(data))
	})

	t.Run("Unmarshaling", func(t *testing.T) {
		jsonStr := `{"performances":[{"title":"Title2","place":"Place2","thumbnail":"Thumb2"}]}`
		var snap watchNewPerformancesSnapshot

		err := json.Unmarshal([]byte(jsonStr), &snap)
		require.NoError(t, err)
		require.Len(t, snap.Performances, 1)
		assert.Equal(t, "Title2", snap.Performances[0].Title)
	})
}

// =============================================================================
// Unit Tests: Compare Logic
// =============================================================================

func TestSnapshot_Compare(t *testing.T) {
	// Helper to create performance objects quickly
	newPerf := func(id string) *performance {
		return &performance{Title: "Title-" + id, Place: "Place-" + id, Thumbnail: "Thumb-" + id}
	}

	tests := []struct {
		name            string
		current         []*performance
		prev            []*performance
		expectedDiffs   int
		expectedChanges bool
		desc            string
	}{
		{
			name:            "First Run (Prev is nil)",
			current:         []*performance{newPerf("A"), newPerf("B")},
			prev:            nil,
			expectedDiffs:   2,
			expectedChanges: true,
			desc:            "이전 스냅샷이 없으면 모든 항목이 신규로 감지되어야 함",
		},
		{
			name:            "No Changes",
			current:         []*performance{newPerf("A")},
			prev:            []*performance{newPerf("A")},
			expectedDiffs:   0,
			expectedChanges: false,
			desc:            "변경 사항이 없으면 diffs는 비어있고 hasChanges는 false여야 함",
		},
		{
			name:            "New Item Added",
			current:         []*performance{newPerf("A"), newPerf("B")},
			prev:            []*performance{newPerf("A")},
			expectedDiffs:   1,
			expectedChanges: true,
			desc:            "새로운 항목이 추가되면 diffs에 포함되고 hasChanges는 true여야 함",
		},
		{
			name:            "Item Removed",
			current:         []*performance{newPerf("A")},
			prev:            []*performance{newPerf("A"), newPerf("B")},
			expectedDiffs:   0,
			expectedChanges: true,
			desc:            "항목이 삭제되면 diffs에는 없지만(알림 X), hasChanges는 true여야 함(스냅샷 갱신)",
		},
		{
			name:            "Content Changed",
			current:         []*performance{{Title: "Title-A", Place: "Place-A", Thumbnail: "Thumb-NEW"}},
			prev:            []*performance{{Title: "Title-A", Place: "Place-A", Thumbnail: "Thumb-OLD"}},
			expectedDiffs:   0,
			expectedChanges: true, // Content changed
			desc:            "내용이 변경되면(썸네일 등) diffs에는 없지만 hasChanges는 true여야 함",
		},
		{
			name:            "Zero Result Protection (Prev items > 0, Curr items == 0)",
			current:         []*performance{},
			prev:            []*performance{newPerf("A")},
			expectedDiffs:   0,
			expectedChanges: false,
			desc:            "이전 데이터가 있는데 현재 0건이면, 일시적 오류로 간주하여 hasChanges는 false여야 함 (데이터 보존)",
		},
		{
			name:            "Both Zero (Initial empty)",
			current:         []*performance{},
			prev:            []*performance{},
			expectedDiffs:   0,
			expectedChanges: false,
			desc:            "둘 다 0건이면 변경 없음",
		},
		{
			name:            "Prev nil, Curr Zero",
			current:         []*performance{},
			prev:            nil,
			expectedDiffs:   0,
			expectedChanges: false,
			desc:            "최초 실행인데 0건이면 변경 없음",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currSnap := &watchNewPerformancesSnapshot{Performances: tt.current}
			var prevSnap *watchNewPerformancesSnapshot
			if tt.prev != nil { // nil slice vs nil pointer distinction
				prevSnap = &watchNewPerformancesSnapshot{Performances: tt.prev}
			}

			diffs, hasChanges := currSnap.Compare(prevSnap)

			assert.Equal(t, tt.expectedChanges, hasChanges, "hasChanges mismatch: "+tt.desc)
			assert.Len(t, diffs, tt.expectedDiffs, "diffs count mismatch: "+tt.desc)

			// If expecting diffs, verify type is New
			for _, d := range diffs {
				assert.Equal(t, performanceEventNew, d.Type)
			}
		})
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkSnapshot_Compare(b *testing.B) {
	// Setup: 1000 prev items, 1005 current (5 new, 5 deleted => total 1000 kept, 5 swapped)
	// Actually let's simulate:
	// Prev: 0~999
	// Curr: 5~1004 (0~4 deleted, 1000~1004 added)
	count := 1000
	prevItems := make([]*performance, count)
	currItems := make([]*performance, count)

	for i := 0; i < count; i++ {
		prevItems[i] = &performance{
			Title: fmt.Sprintf("Title-%d", i),
			Place: "Place",
		}
		currItems[i] = &performance{
			Title: fmt.Sprintf("Title-%d", i+5), // Shifted by 5
			Place: "Place",
		}
	}

	prevSnap := &watchNewPerformancesSnapshot{Performances: prevItems}
	currSnap := &watchNewPerformancesSnapshot{Performances: currItems}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		currSnap.Compare(prevSnap)
	}
}
