package navershopping

import (
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
)

// newTestTask RunBy를 지정하여 테스트용 task를 생성하는 헬퍼입니다.
func newTestTask(runBy contract.TaskRunBy) *task {
	return &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID:     "T",
				CommandID:  WatchPriceAnyCommand,
				NotifierID: "N",
				RunBy:      runBy,
			},
			InstanceID:  "I",
			Fetcher:     mocks.NewMockHTTPFetcher(),
			NewSnapshot: func() interface{} { return &watchPriceSnapshot{} },
		}, true),
	}
}

// makePriceSnapshot 상품 목록을 받아 watchPriceSnapshot을 생성하는 헬퍼입니다.
// prevItems가 nil이면 nil을 반환합니다.
func makePriceSnapshot(products []*product) *watchPriceSnapshot {
	if products == nil {
		return nil
	}
	return &watchPriceSnapshot{Products: products}
}

// =============================================================================
// analyzeAndReport — 핵심 분기 검증
// =============================================================================

// TestAnalyzeAndReport_TableDriven analyzeAndReport 메서드의 모든 분기를 검증합니다.
//
// 분기 목록:
//  1. diffs가 있는 경우:            "상품 정보가 변경되었습니다" 메시지 반환
//  2. diffs 없음 + Scheduler:       빈 메시지 반환 (조용히 스냅샷만 갱신)
//  3. diffs 없음 + User + 상품 있음: "변경된 정보가 없습니다 + 현재 목록" 반환
//  4. diffs 없음 + User + 상품 없음: "상품이 존재하지 않습니다" 반환
func TestAnalyzeAndReport_TableDriven(t *testing.T) {
	t.Parallel()

	settings := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(20000).Build()

	p1 := NewProductBuilder().WithID("1").WithPrice(10000).WithTitle("Product A").Build()
	p1Same := NewProductBuilder().WithID("1").WithPrice(10000).WithTitle("Product A").Build()
	p1Cheap := NewProductBuilder().WithID("1").WithPrice(8000).WithTitle("Product A").Build()
	p2 := NewProductBuilder().WithID("2").WithPrice(5000).WithTitle("Product B").Build()

	tests := []struct {
		name         string
		runBy        contract.TaskRunBy
		currentItems []*product
		prevItems    []*product // nil = 최초 실행
		checkMsg     func(*testing.T, string, bool)
	}{
		// ----------------------------------------------------------------
		// 1. diffs 발생 케이스 (신규 등록)
		// ----------------------------------------------------------------
		{
			name:         "신규 상품 발견 (Scheduler) → 변경 메시지 반환",
			runBy:        contract.TaskRunByScheduler,
			currentItems: []*product{p1, p2},
			prevItems:    []*product{p1},
			checkMsg: func(t *testing.T, msg string, hasChanges bool) {
				assert.Contains(t, msg, "상품 정보가 변경되었습니다")
				assert.Contains(t, msg, "Product B")
				assert.Contains(t, msg, mark.New.WithSpace())
				assert.True(t, hasChanges)
			},
		},
		{
			name:         "최초 실행 (prev=nil) → 모든 상품이 신규로 변경 메시지 반환",
			runBy:        contract.TaskRunByScheduler,
			currentItems: []*product{p1},
			prevItems:    nil,
			checkMsg: func(t *testing.T, msg string, hasChanges bool) {
				assert.Contains(t, msg, "변경되었습니다")
				assert.True(t, hasChanges)
			},
		},
		{
			name:         "가격 하락 → 변경 메시지 + 이전 가격 표시",
			runBy:        contract.TaskRunByScheduler,
			currentItems: []*product{p1Cheap},
			prevItems:    []*product{p1},
			checkMsg: func(t *testing.T, msg string, hasChanges bool) {
				assert.Contains(t, msg, "변경되었습니다")
				assert.Contains(t, msg, "8,000원")
				assert.Contains(t, msg, "(이전: 10,000원)")
				assert.True(t, hasChanges)
			},
		},
		{
			name:         "가격 상승 → 변경 메시지 반환",
			runBy:        contract.TaskRunByScheduler,
			currentItems: []*product{NewProductBuilder().WithID("1").WithPrice(12000).WithTitle("Product A").Build()},
			prevItems:    []*product{p1},
			checkMsg: func(t *testing.T, msg string, hasChanges bool) {
				assert.Contains(t, msg, "변경되었습니다")
				assert.Contains(t, msg, "12,000원")
				assert.True(t, hasChanges)
			},
		},
		// ----------------------------------------------------------------
		// 2. diffs 없음 + Scheduler (조용한 갱신)
		// ----------------------------------------------------------------
		{
			name:         "변경 없음 (Scheduler) → 빈 메시지, hasChanges=false",
			runBy:        contract.TaskRunByScheduler,
			currentItems: []*product{p1},
			prevItems:    []*product{p1Same},
			checkMsg: func(t *testing.T, msg string, hasChanges bool) {
				assert.Empty(t, msg)
				assert.False(t, hasChanges)
			},
		},
		{
			name:         "상품 삭제만 발생 (Scheduler) → 빈 메시지, hasChanges=true (조용한 갱신)",
			runBy:        contract.TaskRunByScheduler,
			currentItems: []*product{p1}, // p2가 사라짐
			prevItems:    []*product{p1Same, p2},
			checkMsg: func(t *testing.T, msg string, hasChanges bool) {
				// 삭제는 알림 대상이 아니므로 msg는 비어있음
				assert.Empty(t, msg)
				// 하지만 개수가 바뀌었으므로 스냅샷은 갱신해야 함
				assert.True(t, hasChanges, "상품 삭제 후에도 스냅샷은 갱신되어야 합니다")
			},
		},
		// ----------------------------------------------------------------
		// 3. diffs 없음 + User + 상품 있음
		// ----------------------------------------------------------------
		{
			name:         "변경 없음 (User) → '변경된 정보가 없습니다' + 현재 목록 반환",
			runBy:        contract.TaskRunByUser,
			currentItems: []*product{p1},
			prevItems:    []*product{p1Same},
			checkMsg: func(t *testing.T, msg string, hasChanges bool) {
				assert.Contains(t, msg, "변경된 정보가 없습니다")
				assert.Contains(t, msg, "Product A")
				assert.False(t, hasChanges)
			},
		},
		// ----------------------------------------------------------------
		// 4. diffs 없음 + User + 상품 없음
		// ----------------------------------------------------------------
		{
			name:         "결과 없음 (User) → '상품이 존재하지 않습니다' 반환",
			runBy:        contract.TaskRunByUser,
			currentItems: []*product{},
			prevItems:    []*product{},
			checkMsg: func(t *testing.T, msg string, hasChanges bool) {
				assert.Contains(t, msg, "상품이 존재하지 않습니다")
			},
		},
		// ----------------------------------------------------------------
		// 5. 검색 조건 요약이 메시지에 포함되는지 검증
		// ----------------------------------------------------------------
		{
			name:         "검색 조건 요약 포함 여부 (User + 변경 없음)",
			runBy:        contract.TaskRunByUser,
			currentItems: []*product{p1},
			prevItems:    []*product{p1Same},
			checkMsg: func(t *testing.T, msg string, hasChanges bool) {
				// renderSearchConditionsSummary 결과로 Query가 포함되어야 함
				assert.Contains(t, msg, "test", "검색 조건 요약(Query)이 메시지에 포함되어야 합니다")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tsk := newTestTask(tt.runBy)
			current := makePriceSnapshot(tt.currentItems)
			prev := makePriceSnapshot(tt.prevItems)

			msg, hasChanges := tsk.analyzeAndReport(&settings, current, prev, false)
			tt.checkMsg(t, msg, hasChanges)
		})
	}
}

// =============================================================================
// analyzeAndReport — 정렬 검증
// =============================================================================

// TestAnalyzeAndReport_SortOrder 메시지의 상품 목록이 가격 오름차순,
// 동가격 시 이름 오름차순으로 정렬되는지 검증합니다.
func TestAnalyzeAndReport_SortOrder(t *testing.T) {
	t.Parallel()

	settings := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(99999).Build()

	current := &watchPriceSnapshot{
		Products: []*product{
			NewProductBuilder().WithID("3").WithTitle("Charlie").WithPrice(20000).Build(),
			NewProductBuilder().WithID("1").WithTitle("Alpha").WithPrice(10000).Build(),
			NewProductBuilder().WithID("2").WithTitle("Bravo").WithPrice(10000).Build(),
		},
	}

	tsk := newTestTask(contract.TaskRunByUser)
	msg, _ := tsk.analyzeAndReport(&settings, current, nil, false)

	idxAlpha := strings.Index(msg, "Alpha")
	idxBravo := strings.Index(msg, "Bravo")
	idxCharlie := strings.Index(msg, "Charlie")

	assert.Greater(t, idxAlpha, -1)
	assert.Greater(t, idxBravo, -1)
	assert.Greater(t, idxCharlie, -1)

	assert.Less(t, idxAlpha, idxBravo, "동가격 시 이름 오름차순: Alpha → Bravo")
	assert.Less(t, idxBravo, idxCharlie, "가격 오름차순: 10000원(Bravo) → 20000원(Charlie)")
}

// =============================================================================
// analyzeAndReport — Invariant 검증
// =============================================================================

// TestAnalyzeAndReport_Invariants 시스템 불변식을 검증합니다.
//
// 불변식:
//   - (A) diffs가 있으면 msg는 항상 비어있지 않습니다.
//   - (B) Scheduler + diffs 없음 + 상품 삭제만 있는 경우: msg=""이지만 hasChanges=true 가능.
//     이는 정상이며 Invariant 위반이 아닙니다.
func TestAnalyzeAndReport_Invariants(t *testing.T) {
	t.Parallel()

	settings := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(99999).Build()

	t.Run("Invariant(A): diffs 존재 시 msg는 반드시 비어있지 않음", func(t *testing.T) {
		t.Parallel()

		// 신규 상품 → diff 발생
		current := &watchPriceSnapshot{
			Products: []*product{NewProductBuilder().WithID("1").WithPrice(1000).Build()},
		}

		for _, runBy := range []contract.TaskRunBy{contract.TaskRunByScheduler, contract.TaskRunByUser} {
			tsk := newTestTask(runBy)
			msg, _ := tsk.analyzeAndReport(&settings, current, nil, false)
			assert.NotEmpty(t, msg, "diffs가 있으면 msg는 빈 문자열이 될 수 없습니다 (runBy=%v)", runBy)
		}
	})

	t.Run("Invariant(B): 삭제만 발생 + Scheduler → msg=\"\" && hasChanges=true는 정상", func(t *testing.T) {
		t.Parallel()

		prev := &watchPriceSnapshot{Products: []*product{
			NewProductBuilder().WithID("1").WithPrice(1000).Build(),
			NewProductBuilder().WithID("2").WithPrice(2000).Build(),
		}}
		// 상품 1개 삭제 (ID "2" 이탈)
		current := &watchPriceSnapshot{Products: []*product{
			NewProductBuilder().WithID("1").WithPrice(1000).Build(),
		}}

		tsk := newTestTask(contract.TaskRunByScheduler)
		msg, hasChanges := tsk.analyzeAndReport(&settings, current, prev, false)

		assert.Empty(t, msg, "삭제만 발생한 경우 Scheduler는 조용히 처리합니다")
		assert.True(t, hasChanges, "삭제 발생 시에는 스냅샷 갱신이 필요합니다")
	})
}
