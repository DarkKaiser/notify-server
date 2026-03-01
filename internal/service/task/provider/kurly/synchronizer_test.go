package kurly

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// mergeWithPreviousState 테스트
//
// 검증 범위:
//  1. [1단계] prevSnapshot=nil 최초 실행 → prevProductsByID 반환 nil 확인
//  2. [2단계] 이전 스냅샷으로부터 LowestPrice / LowestPriceTimeUTC 이월
//  3. [2단계] FetchFailedCount 누적 및 임계값(3) 캡핑
//  4. [2단계] 연속 3회 이상 실패 → IsUnavailable=true 강제 전환
//  5. [2단계] 연속 실패 중 이전 가격 데이터 승계 (오경보 방지)
//  6. [2단계] 단종 상품 이름 유실 방지 (IsUnavailable && Name=="")
//  7. [3단계] tryUpdateLowestPrice 호출 → 최저가 갱신
//  8. [4단계] 감시 목록 포함 누락 상품 → 이월 보존
//  9. [4단계] 감시 목록 제외 누락 상품 → GC(삭제)
// 10. prevProductsByID 반환값 검증 (Diff 계산용 Map)
// =============================================================================

// makeWatchedIDs 슬라이스로 watchedProductIDs map을 간편하게 생성하는 헬퍼입니다.
func makeWatchedIDs(ids ...int) map[int]struct{} {
	m := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

// TestMergeWithPreviousState_FirstRun
// prevSnapshot == nil인 최초 실행 케이스를 검증합니다.
func TestMergeWithPreviousState_FirstRun(t *testing.T) {
	t.Parallel()

	t.Run("최초 실행: prevSnapshot=nil → prevProductsByID도 nil 반환", func(t *testing.T) {
		t.Parallel()
		curr := []*product{
			{ID: 100, Name: "사과", Price: 5000},
		}
		merged, prevMap := mergeWithPreviousState(curr, nil, makeWatchedIDs(100))

		require.Len(t, merged, 1)
		assert.Nil(t, prevMap, "최초 실행 시 prevProductsByID는 nil이어야 합니다")
	})

	t.Run("최초 실행: 수집 성공 상품 → LowestPrice가 이번 가격으로 갱신됨", func(t *testing.T) {
		t.Parallel()
		curr := []*product{
			{ID: 100, Name: "사과", Price: 5000, LowestPrice: 0},
		}
		merged, _ := mergeWithPreviousState(curr, nil, makeWatchedIDs(100))

		require.Len(t, merged, 1)
		assert.Equal(t, 5000, merged[0].LowestPrice, "최초 실행에서 LowestPrice가 현재 가격으로 설정되어야 합니다")
		assert.False(t, merged[0].LowestPriceTimeUTC.IsZero(), "LowestPriceTimeUTC가 설정되어야 합니다")
	})

	t.Run("최초 실행: FetchFailedCount >= 3인 상품 → IsUnavailable 강제 전환", func(t *testing.T) {
		t.Parallel()
		curr := []*product{
			{ID: 100, FetchFailedCount: 3},
		}
		merged, _ := mergeWithPreviousState(curr, nil, makeWatchedIDs(100))

		require.Len(t, merged, 1)
		assert.True(t, merged[0].IsUnavailable, "최초 실행에서도 FetchFailedCount>=3이면 IsUnavailable이어야 합니다")
	})
}

// TestMergeWithPreviousState_StateRestoration
// [2단계] 이전 스냅샷으로부터 누적 이력을 복원하는 로직을 검증합니다.
func TestMergeWithPreviousState_StateRestoration(t *testing.T) {
	t.Parallel()

	lowestTime := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	t.Run("LowestPrice & LowestPriceTimeUTC 이전 스냅샷에서 이월", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{
					ID:                 100,
					Name:               "사과",
					Price:              5000,
					LowestPrice:        4500,
					LowestPriceTimeUTC: lowestTime,
				},
			},
		}
		curr := []*product{
			{ID: 100, Name: "사과", Price: 5500}, // 현재는 더 비쌈 → 최저가 유지
		}

		merged, _ := mergeWithPreviousState(curr, prev, makeWatchedIDs(100))

		require.Len(t, merged, 1)
		assert.Equal(t, 4500, merged[0].LowestPrice, "이전 최저가가 이월되어야 합니다")
		assert.True(t, merged[0].LowestPriceTimeUTC.Equal(lowestTime), "이전 최저가 시각이 이월되어야 합니다")
	})

	t.Run("현재 가격이 더 낮으면 최저가 갱신됨 [3단계]", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 100, Price: 5500, LowestPrice: 5500, LowestPriceTimeUTC: lowestTime},
			},
		}
		curr := []*product{
			{ID: 100, Price: 4000}, // 현재 가격이 더 낮음 → 최저가 갱신
		}

		merged, _ := mergeWithPreviousState(curr, prev, makeWatchedIDs(100))

		require.Len(t, merged, 1)
		assert.Equal(t, 4000, merged[0].LowestPrice, "현재 가격으로 최저가가 갱신되어야 합니다")
		assert.True(t, merged[0].LowestPriceTimeUTC.After(lowestTime), "최저가 갱신 시각이 최신값이어야 합니다")
	})

	t.Run("단종 상품 Name 유실 방지 — IsUnavailable=true && Name=='' → 이전 이름 복원", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 200, Name: "망고", Price: 8000, IsUnavailable: false},
			},
		}
		// 단종 감지 시 Name 없이 반환되는 임시 객체 시뮬레이션
		curr := []*product{
			{ID: 200, Name: "", IsUnavailable: true},
		}

		merged, _ := mergeWithPreviousState(curr, prev, makeWatchedIDs(200))

		require.Len(t, merged, 1)
		assert.Equal(t, "망고", merged[0].Name, "단종 상품의 이름이 이전 스냅샷에서 복원되어야 합니다")
	})

	t.Run("이름이 있는 단종 상품 → 이름 그대로 유지", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 200, Name: "망고", IsUnavailable: false},
			},
		}
		curr := []*product{
			{ID: 200, Name: "망고", IsUnavailable: true}, // Name 있음
		}

		merged, _ := mergeWithPreviousState(curr, prev, makeWatchedIDs(200))

		require.Len(t, merged, 1)
		assert.Equal(t, "망고", merged[0].Name)
	})
}

// TestMergeWithPreviousState_FetchFailedCount
// [2단계] FetchFailedCount 누적, 캡핑(3), IsUnavailable 강제 전환 로직을 검증합니다.
func TestMergeWithPreviousState_FetchFailedCount(t *testing.T) {
	t.Parallel()

	t.Run("FetchFailedCount 1차 실패: 이전(0) + 현재(1) = 1, 이전 데이터 승계", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 100, Name: "사과", Price: 5000, FetchFailedCount: 0},
			},
		}
		// 1회 실패 → FetchFailedCount=1, 이전 정상 데이터를 Price 등에 승계
		curr := []*product{
			{ID: 100, FetchFailedCount: 1},
		}

		merged, _ := mergeWithPreviousState(curr, prev, makeWatchedIDs(100))

		require.Len(t, merged, 1)
		p := merged[0]
		assert.Equal(t, 1, p.FetchFailedCount)
		assert.False(t, p.IsUnavailable, "1회 실패는 아직 단종이 아닙니다")
		assert.Equal(t, "사과", p.Name, "일시적 실패 시 이전 이름이 승계되어야 합니다")
		assert.Equal(t, 5000, p.Price, "일시적 실패 시 이전 가격이 승계되어야 합니다")
	})

	t.Run("FetchFailedCount 2차 실패: 이전(1) + 현재(1) = 2, 이전 데이터 승계", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 100, Name: "사과", Price: 5000, FetchFailedCount: 1},
			},
		}
		curr := []*product{
			{ID: 100, FetchFailedCount: 1},
		}

		merged, _ := mergeWithPreviousState(curr, prev, makeWatchedIDs(100))

		require.Len(t, merged, 1)
		p := merged[0]
		assert.Equal(t, 2, p.FetchFailedCount)
		assert.False(t, p.IsUnavailable)
		assert.Equal(t, 5000, p.Price, "2회 실패 시 이전 가격이 승계되어야 합니다")
	})

	t.Run("FetchFailedCount 3차 실패: 이전(2) + 현재(1) = 3 → IsUnavailable=true", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 100, Name: "사과", Price: 5000, FetchFailedCount: 2},
			},
		}
		curr := []*product{
			{ID: 100, FetchFailedCount: 1},
		}

		merged, _ := mergeWithPreviousState(curr, prev, makeWatchedIDs(100))

		require.Len(t, merged, 1)
		p := merged[0]
		assert.Equal(t, 3, p.FetchFailedCount)
		assert.True(t, p.IsUnavailable, "3회 실패 시 IsUnavailable=true 로 강제 전환되어야 합니다")
	})

	t.Run("FetchFailedCount 캡핑: 이전(3) + 현재(1) = 4 → 3으로 상한 고정", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 100, FetchFailedCount: 3, IsUnavailable: true},
			},
		}
		curr := []*product{
			{ID: 100, FetchFailedCount: 1},
		}

		merged, _ := mergeWithPreviousState(curr, prev, makeWatchedIDs(100))

		require.Len(t, merged, 1)
		p := merged[0]
		// 4로 증가하더라도 3으로 캡핑되어야 합니다.
		assert.Equal(t, 3, p.FetchFailedCount, "FetchFailedCount는 임계값(3)으로 상한 고정되어야 합니다")
		assert.True(t, p.IsUnavailable)
	})

	t.Run("현재 사이클 정상 수집 → FetchFailedCount 누적 없이 이전 최저가만 이월", func(t *testing.T) {
		t.Parallel()
		lowestTime := time.Now().Add(-24 * time.Hour).UTC()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 100, Name: "사과", Price: 5000, FetchFailedCount: 2, LowestPrice: 4800, LowestPriceTimeUTC: lowestTime},
			},
		}
		// FetchFailedCount == 0 이면 정상 수집 → 누적 없이 최저가만 이월
		curr := []*product{
			{ID: 100, Name: "사과", Price: 5200, FetchFailedCount: 0},
		}

		merged, _ := mergeWithPreviousState(curr, prev, makeWatchedIDs(100))

		require.Len(t, merged, 1)
		p := merged[0]
		assert.Equal(t, 0, p.FetchFailedCount, "정상 수집 시 FetchFailedCount가 이월되지 않아야 합니다")
		assert.Equal(t, 4800, p.LowestPrice, "이전 최저가는 이월되어야 합니다")
	})
}

// TestMergeWithPreviousState_CarryOverAndGC
// [4단계] 이전 스냅샷 상품이 이번 사이클에서 누락됐을 때의 이월/GC 로직을 검증합니다.
func TestMergeWithPreviousState_CarryOverAndGC(t *testing.T) {
	t.Parallel()

	t.Run("감시 목록 포함 누락 상품 → 이전 상태 그대로 이월 보존", func(t *testing.T) {
		t.Parallel()
		lowestTime := time.Now().Add(-time.Hour).UTC()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 100, Name: "사과", Price: 5000, LowestPrice: 4800, LowestPriceTimeUTC: lowestTime},
				{ID: 200, Name: "바나나", Price: 3000, LowestPrice: 2800, LowestPriceTimeUTC: lowestTime},
			},
		}
		// 이번 사이클에서 ID=200은 수집 누락
		curr := []*product{
			{ID: 100, Name: "사과", Price: 5000},
		}
		// ID=200도 여전히 감시 중
		watched := makeWatchedIDs(100, 200)

		merged, _ := mergeWithPreviousState(curr, prev, watched)

		// 100, 200 모두 포함되어야 합니다.
		assert.Len(t, merged, 2, "감시 중인 누락 상품은 이전 상태로 보존되어야 합니다")
		ids := make(map[int]bool)
		for _, p := range merged {
			ids[p.ID] = true
		}
		assert.True(t, ids[200], "ID=200이 이월 보존되어야 합니다")
	})

	t.Run("감시 목록 제외 누락 상품 → GC(삭제)", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 100, Name: "사과", Price: 5000},
				{ID: 200, Name: "바나나", Price: 3000},
			},
		}
		// ID=200은 수집 누락 AND 감시 목록에서도 제외됨
		curr := []*product{
			{ID: 100, Name: "사과", Price: 5000},
		}
		// 감시 목록에는 ID=100만 남음
		watched := makeWatchedIDs(100)

		merged, _ := mergeWithPreviousState(curr, prev, watched)

		assert.Len(t, merged, 1, "삭제/비활성화된 상품은 GC되어야 합니다")
		assert.Equal(t, 100, merged[0].ID, "GC 후 ID=100만 남아야 합니다")
	})

	t.Run("이전 스냅샷이 없으면 이월 로직 스킵", func(t *testing.T) {
		t.Parallel()
		curr := []*product{
			{ID: 100, Name: "사과", Price: 5000},
		}
		merged, _ := mergeWithPreviousState(curr, nil, makeWatchedIDs(100))

		// prevSnapshot이 nil이면 4단계 루프가 실행되지 않으므로 현재 수집 목록만 반환
		assert.Len(t, merged, 1)
	})
}

// TestMergeWithPreviousState_ReturnedPrevMap
// 반환값 두 번째 요소(prevProductsByID)가 Analyzer(Diff 계산)에 올바르게 사용될 수 있도록
// 이전 상품 목록이 정확히 Map으로 인덱싱되어 반환되는지 검증합니다.
func TestMergeWithPreviousState_ReturnedPrevMap(t *testing.T) {
	t.Parallel()

	t.Run("prevProductsByID에 이전 스냅샷 상품이 모두 인덱싱됨", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 100, Name: "사과", Price: 5000},
				{ID: 200, Name: "바나나", Price: 3000},
			},
		}
		curr := []*product{
			{ID: 100, Name: "사과", Price: 4800},
		}

		_, prevMap := mergeWithPreviousState(curr, prev, makeWatchedIDs(100, 200))

		require.NotNil(t, prevMap)
		assert.Len(t, prevMap, 2, "이전 상품 2개가 모두 Map에 인덱싱되어야 합니다")
		assert.Equal(t, 5000, prevMap[100].Price)
		assert.Equal(t, 3000, prevMap[200].Price)
	})

	t.Run("prevSnapshot=nil이면 prevProductsByID도 nil 반환", func(t *testing.T) {
		t.Parallel()
		curr := []*product{{ID: 100}}
		_, prevMap := mergeWithPreviousState(curr, nil, makeWatchedIDs(100))
		assert.Nil(t, prevMap)
	})
}

// TestMergeWithPreviousState_EdgeCases
// 빈 슬라이스, 이전 스냅샷에 없는 신규 상품 등 경계값 케이스를 검증합니다.
func TestMergeWithPreviousState_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("currentProducts가 비어있으면 이월된 상품만 반환", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 100, Name: "사과"},
			},
		}
		// 이번 사이클에 수집된 상품 없음
		curr := []*product{}
		watched := makeWatchedIDs(100)

		merged, _ := mergeWithPreviousState(curr, prev, watched)

		// 감시 목록에 포함된 ID=100이 이월 보존됩니다.
		assert.Len(t, merged, 1)
		assert.Equal(t, 100, merged[0].ID)
	})

	t.Run("이전 스냅샷에 없던 신규 상품 → 상태 복원 없이 최저가만 갱신", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 999, Name: "기존 상품", Price: 1000},
			},
		}
		// ID=100은 이전 스냅샷에 없어 신규 상품
		curr := []*product{
			{ID: 100, Name: "신규 상품", Price: 5000},
		}

		merged, _ := mergeWithPreviousState(curr, prev, makeWatchedIDs(100))

		// ID=100은 이전 스냅샷에 없으므로 상태 복원 없이 최저가만 갱신됩니다.
		found := false
		for _, p := range merged {
			if p.ID == 100 {
				found = true
				assert.Equal(t, 5000, p.LowestPrice, "신규 상품의 최저가가 현재 가격으로 설정되어야 합니다")
			}
		}
		assert.True(t, found, "신규 상품 ID=100이 결과에 포함되어야 합니다")
	})

	t.Run("watchedProductIDs가 비어있으면 이전 상품 전체 GC", func(t *testing.T) {
		t.Parallel()
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 100, Name: "사과"},
				{ID: 200, Name: "바나나"},
			},
		}
		// 이번 사이클에 아무것도 수집 안 됨
		curr := []*product{}
		// 감시 목록도 비어있음 → 모두 GC
		watched := makeWatchedIDs()

		merged, _ := mergeWithPreviousState(curr, prev, watched)

		assert.Len(t, merged, 0, "감시 목록이 비어있으면 이전 상품이 전부 GC되어야 합니다")
	})
}
