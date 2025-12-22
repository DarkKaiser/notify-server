package navershopping

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiffAndNotify_PriceSorting 신규 상품과 가격 변경 상품이 함께 가격 오름차순으로 정렬되는지 검증합니다.
func TestDiffAndNotify_PriceSorting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		currentProducts []*product
		prevProducts    []*product
		wantOrder       []string // 기대되는 ProductID 순서 (가격 오름차순)
	}{
		{
			name: "신규 상품만 - 가격 오름차순 정렬",
			currentProducts: []*product{
				{ProductID: "A", ProductType: "1", Title: "Product A", LowPrice: 30000},
				{ProductID: "B", ProductType: "1", Title: "Product B", LowPrice: 10000},
				{ProductID: "C", ProductType: "1", Title: "Product C", LowPrice: 20000},
			},
			prevProducts: []*product{},
			wantOrder:    []string{"B", "C", "A"}, // 10000 → 20000 → 30000
		},
		{
			name: "가격 변경 상품만 - 가격 오름차순 정렬",
			currentProducts: []*product{
				{ProductID: "A", ProductType: "1", Title: "Product A", LowPrice: 25000},
				{ProductID: "B", ProductType: "1", Title: "Product B", LowPrice: 15000},
				{ProductID: "C", ProductType: "1", Title: "Product C", LowPrice: 35000},
			},
			prevProducts: []*product{
				{ProductID: "A", ProductType: "1", Title: "Product A", LowPrice: 30000},
				{ProductID: "B", ProductType: "1", Title: "Product B", LowPrice: 20000},
				{ProductID: "C", ProductType: "1", Title: "Product C", LowPrice: 40000},
			},
			wantOrder: []string{"B", "A", "C"}, // 15000 → 25000 → 35000
		},
		{
			name: "신규 + 가격 변경 혼합 - 전체 가격 오름차순 정렬",
			currentProducts: []*product{
				{ProductID: "NEW1", ProductType: "1", Title: "New 1", LowPrice: 50000},
				{ProductID: "CHANGED1", ProductType: "1", Title: "Changed 1", LowPrice: 12000},
				{ProductID: "NEW2", ProductType: "1", Title: "New 2", LowPrice: 8000},
				{ProductID: "CHANGED2", ProductType: "1", Title: "Changed 2", LowPrice: 18000},
				{ProductID: "UNCHANGED", ProductType: "1", Title: "Unchanged", LowPrice: 5000}, // 가격 변경 없음 (메시지 미생성)
			},
			prevProducts: []*product{
				{ProductID: "CHANGED1", ProductType: "1", Title: "Changed 1", LowPrice: 15000},
				{ProductID: "CHANGED2", ProductType: "1", Title: "Changed 2", LowPrice: 20000},
				{ProductID: "UNCHANGED", ProductType: "1", Title: "Unchanged", LowPrice: 5000},
			},
			wantOrder: []string{"NEW2", "CHANGED1", "CHANGED2", "NEW1"}, // 8000 → 12000 → 18000 → 50000 (UNCHANGED 제외)
		},
		{
			name: "동일 가격 상품 - 원본 순서 유지",
			currentProducts: []*product{
				{ProductID: "A", ProductType: "1", Title: "Product A", LowPrice: 10000},
				{ProductID: "B", ProductType: "1", Title: "Product B", LowPrice: 10000},
				{ProductID: "C", ProductType: "1", Title: "Product C", LowPrice: 10000},
			},
			prevProducts: []*product{},
			wantOrder:    []string{"A", "B", "C"}, // 동일 가격이므로 원본 순서 유지
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			tsk := &task{}
			currentSnapshot := &watchPriceSnapshot{Products: tt.currentProducts}
			prevSnapshot := &watchPriceSnapshot{Products: tt.prevProducts}
			settings := &watchPriceSettings{
				Query: "test",
			}
			settings.Filters.PriceLessThan = 100000

			// Execute
			message, _, err := tsk.diffAndNotify(settings, currentSnapshot, prevSnapshot, false)

			// Verify
			require.NoError(t, err)

			if len(tt.wantOrder) == 0 {
				assert.Empty(t, message, "변경 사항이 없으면 메시지가 비어야 합니다")
				return
			}

			require.NotEmpty(t, message, "변경 사항이 있으면 메시지가 생성되어야 합니다")

			// 메시지에서 ProductID 출현 순서 추출
			lines := strings.Split(message, "\n")
			var actualOrder []string
			for _, line := range lines {
				for _, p := range tt.currentProducts {
					if strings.Contains(line, p.Title) {
						// 중복 방지: 이미 추가된 ProductID는 건너뜀
						alreadyAdded := false
						for _, id := range actualOrder {
							if id == p.ProductID {
								alreadyAdded = true
								break
							}
						}
						if !alreadyAdded {
							actualOrder = append(actualOrder, p.ProductID)
						}
						break
					}
				}
			}

			// 기대 순서와 실제 순서 비교
			assert.Equal(t, tt.wantOrder, actualOrder, "상품이 가격 오름차순으로 정렬되어야 합니다")
		})
	}
}
