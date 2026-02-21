package navershopping

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 테스트 헬퍼
// =============================================================================

// makeProduct 간결한 테스트용 상품 생성 헬퍼입니다.
func makeProduct(id, title, link, mallName, productType string, price int) *product {
	return &product{
		ProductID:   id,
		Title:       title,
		Link:        link,
		MallName:    mallName,
		ProductType: productType,
		LowPrice:    price,
	}
}

func makeSimpleProduct(id string, price int) *product {
	return makeProduct(id, "상품 "+id, "http://link/"+id, "mall", "1", price)
}

// =============================================================================
// Compare: 최초 실행 (prev == nil)
// =============================================================================

// TestCompare_FirstRun_NoPrev 최초 실행 시 (prev=nil) 모든 상품이 신규로 감지되는지 검증합니다.
func TestCompare_FirstRun_NoPrev(t *testing.T) {
	t.Parallel()

	curr := &watchPriceSnapshot{
		Products: []*product{
			makeSimpleProduct("A", 30000),
			makeSimpleProduct("B", 10000),
			makeSimpleProduct("C", 20000),
		},
	}

	diffs, hasChanges := curr.Compare(nil)

	// 모두 신규 이벤트
	require.Len(t, diffs, 3)
	assert.True(t, hasChanges)

	for _, d := range diffs {
		assert.Equal(t, productEventNew, d.Type)
		assert.NotNil(t, d.Product)
		assert.Nil(t, d.Prev, "신규 상품의 Prev는 nil이어야 합니다")
	}
}

// TestCompare_FirstRun_EmptyProducts 최초 실행인데 수집 결과도 0건이면 hasChanges=false.
func TestCompare_FirstRun_EmptyProducts(t *testing.T) {
	t.Parallel()

	curr := &watchPriceSnapshot{Products: []*product{}}
	diffs, hasChanges := curr.Compare(nil)

	assert.Empty(t, diffs)
	assert.False(t, hasChanges)
}

// =============================================================================
// Compare: 변경 없음
// =============================================================================

// TestCompare_NoChanges 이전과 동일한 상품 목록이라면 diffs는 비어있고 hasChanges=false.
func TestCompare_NoChanges(t *testing.T) {
	t.Parallel()

	products := []*product{
		makeSimpleProduct("A", 10000),
		makeSimpleProduct("B", 20000),
	}
	prev := &watchPriceSnapshot{Products: products}
	curr := &watchPriceSnapshot{
		Products: []*product{
			makeSimpleProduct("A", 10000),
			makeSimpleProduct("B", 20000),
		},
	}

	diffs, hasChanges := curr.Compare(prev)

	assert.Empty(t, diffs)
	assert.False(t, hasChanges)
}

// =============================================================================
// Compare: 신규 상품 감지
// =============================================================================

// TestCompare_NewProduct 이전에 없던 상품이 현재 결과에 등장하면 productEventNew 이벤트를 생성합니다.
func TestCompare_NewProduct(t *testing.T) {
	t.Parallel()

	prev := &watchPriceSnapshot{
		Products: []*product{makeSimpleProduct("A", 10000)},
	}
	curr := &watchPriceSnapshot{
		Products: []*product{
			makeSimpleProduct("A", 10000),
			makeSimpleProduct("B", 20000), // 신규
		},
	}

	diffs, hasChanges := curr.Compare(prev)

	require.Len(t, diffs, 1)
	assert.True(t, hasChanges)
	assert.Equal(t, productEventNew, diffs[0].Type)
	assert.Equal(t, "B", diffs[0].Product.ProductID)
	assert.Nil(t, diffs[0].Prev)
}

// =============================================================================
// Compare: 가격 변동 감지
// =============================================================================

// TestCompare_PriceChanged 동일 상품의 가격이 변동되면 productEventPriceChanged가 생성됩니다.
func TestCompare_PriceChanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		prevPrice int
		currPrice int
	}{
		{name: "가격 하락", prevPrice: 20000, currPrice: 15000},
		{name: "가격 상승", prevPrice: 10000, currPrice: 12000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prev := &watchPriceSnapshot{Products: []*product{makeSimpleProduct("A", tt.prevPrice)}}
			curr := &watchPriceSnapshot{Products: []*product{makeSimpleProduct("A", tt.currPrice)}}

			diffs, hasChanges := curr.Compare(prev)

			require.Len(t, diffs, 1)
			assert.True(t, hasChanges)
			assert.Equal(t, productEventPriceChanged, diffs[0].Type)
			assert.Equal(t, tt.currPrice, diffs[0].Product.LowPrice)
			assert.Equal(t, tt.prevPrice, diffs[0].Prev.LowPrice, "Prev에 이전 가격 정보가 있어야 합니다")
		})
	}
}

// =============================================================================
// Compare: 메타 정보 변경 (알림 없음, 스냅샷 갱신만)
// =============================================================================

// TestCompare_MetaChanged 가격은 동일하지만 메타 정보(제목, 링크, 판매처, 유형)가 바뀐 경우
// diffs에는 추가되지 않지만 hasChanges=true로 스냅샷은 갱신됩니다.
func TestCompare_MetaChanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		prev *product
		curr *product
	}{
		{
			name: "상품명 변경",
			prev: makeProduct("A", "구형 상품명", "http://link/A", "mall", "1", 10000),
			curr: makeProduct("A", "신형 상품명", "http://link/A", "mall", "1", 10000),
		},
		{
			name: "링크 변경",
			prev: makeProduct("A", "상품 A", "http://link/old", "mall", "1", 10000),
			curr: makeProduct("A", "상품 A", "http://link/new", "mall", "1", 10000),
		},
		{
			name: "판매처 변경",
			prev: makeProduct("A", "상품 A", "http://link/A", "old-mall", "1", 10000),
			curr: makeProduct("A", "상품 A", "http://link/A", "new-mall", "1", 10000),
		},
		{
			name: "상품 유형 변경",
			prev: makeProduct("A", "상품 A", "http://link/A", "mall", "1", 10000),
			curr: makeProduct("A", "상품 A", "http://link/A", "mall", "2", 10000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prev := &watchPriceSnapshot{Products: []*product{tt.prev}}
			curr := &watchPriceSnapshot{Products: []*product{tt.curr}}

			diffs, hasChanges := curr.Compare(prev)

			assert.Empty(t, diffs, "메타 변경은 알림 대상이 아닙니다")
			assert.True(t, hasChanges, "메타 변경도 스냅샷 갱신 대상이어야 합니다")
		})
	}
}

// =============================================================================
// Compare: 상품 이탈 감지
// =============================================================================

// TestCompare_ProductRemoved 이전에 있던 상품이 현재 결과에서 사라지면
// diffs는 비어있지만, 개수 변동으로 hasChanges=true가 됩니다.
func TestCompare_ProductRemoved(t *testing.T) {
	t.Parallel()

	prev := &watchPriceSnapshot{
		Products: []*product{
			makeSimpleProduct("A", 10000),
			makeSimpleProduct("B", 20000),
		},
	}
	// B 이탈
	curr := &watchPriceSnapshot{
		Products: []*product{makeSimpleProduct("A", 10000)},
	}

	diffs, hasChanges := curr.Compare(prev)

	assert.Empty(t, diffs, "삭제된 상품은 알림 대상이 아닙니다")
	assert.True(t, hasChanges, "상품 수 감소는 스냅샷 갱신 대상이어야 합니다")
}

// =============================================================================
// Compare: 0건 방어 로직 (스팸 방지)
// =============================================================================

// TestCompare_EmptyResult_Protection 이전 데이터가 있는데 현재 수집 결과가 0건이면
// 일시적 오류로 간주하여 hasChanges=false를 반환합니다.
func TestCompare_EmptyResult_Protection(t *testing.T) {
	t.Parallel()

	prev := &watchPriceSnapshot{
		Products: []*product{
			makeSimpleProduct("A", 10000),
			makeSimpleProduct("B", 20000),
		},
	}
	curr := &watchPriceSnapshot{Products: []*product{}} // 0건 수집

	diffs, hasChanges := curr.Compare(prev)

	assert.Nil(t, diffs, "0건 방어: diffs는 nil이어야 합니다")
	assert.False(t, hasChanges, "0건 방어: 스냅샷을 갱신하지 않습니다")
}

// TestCompare_EmptyResult_FirstRun 최초 실행 (prev=nil)에서 0건이면 방어 로직이 작동하지 않습니다.
func TestCompare_EmptyResult_FirstRun(t *testing.T) {
	t.Parallel()

	curr := &watchPriceSnapshot{Products: []*product{}}
	diffs, hasChanges := curr.Compare(nil)

	// prev=nil 이면 prevLen=0 이므로 0건 방어 조건 불충족 → 정상 처리
	assert.Empty(t, diffs)
	assert.False(t, hasChanges)
}

// =============================================================================
// Compare: 정렬 검증
// =============================================================================

// TestCompare_SortByPrice 반환된 diffs가 LowPrice 오름차순으로 정렬되는지 검증합니다.
func TestCompare_SortByPrice(t *testing.T) {
	t.Parallel()

	curr := &watchPriceSnapshot{
		Products: []*product{
			makeSimpleProduct("C", 30000),
			makeSimpleProduct("A", 10000),
			makeSimpleProduct("B", 20000),
		},
	}

	diffs, _ := curr.Compare(nil)

	require.Len(t, diffs, 3)
	assert.Equal(t, 10000, diffs[0].Product.LowPrice)
	assert.Equal(t, 20000, diffs[1].Product.LowPrice)
	assert.Equal(t, 30000, diffs[2].Product.LowPrice)
}

// TestCompare_SortByTitleOnSamePrice 가격이 동일하면 상품명 오름차순으로 2차 정렬됩니다.
func TestCompare_SortByTitleOnSamePrice(t *testing.T) {
	t.Parallel()

	curr := &watchPriceSnapshot{
		Products: []*product{
			makeProduct("3", "Cherry", "http://link/3", "mall", "1", 10000),
			makeProduct("1", "Apple", "http://link/1", "mall", "1", 10000),
			makeProduct("2", "Banana", "http://link/2", "mall", "1", 10000),
		},
	}

	diffs, _ := curr.Compare(nil)

	require.Len(t, diffs, 3)
	assert.Equal(t, "Apple", diffs[0].Product.Title)
	assert.Equal(t, "Banana", diffs[1].Product.Title)
	assert.Equal(t, "Cherry", diffs[2].Product.Title)
}

// =============================================================================
// Compare: 복합 시나리오
// =============================================================================

// TestCompare_Mixed 신규·가격변동·유지·삭제가 동시에 발생하는 실전 시나리오를 검증합니다.
func TestCompare_Mixed(t *testing.T) {
	t.Parallel()

	prev := &watchPriceSnapshot{
		Products: []*product{
			makeSimpleProduct("keep", 10000),       // 변경 없음
			makeSimpleProduct("price_down", 20000), // 가격 하락 예정
			makeSimpleProduct("price_up", 5000),    // 가격 상승 예정
			makeSimpleProduct("removed", 30000),    // 이탈 예정
		},
	}

	curr := &watchPriceSnapshot{
		Products: []*product{
			makeSimpleProduct("keep", 10000),       // 변경 없음
			makeSimpleProduct("price_down", 15000), // 가격 하락
			makeSimpleProduct("price_up", 7000),    // 가격 상승
			makeSimpleProduct("new_product", 8000), // 신규
			// "removed"는 없음
		},
	}

	diffs, hasChanges := curr.Compare(prev)

	assert.True(t, hasChanges)

	// diffs에는 신규 1개 + 가격변동 2개 = 3개
	require.Len(t, diffs, 3)

	// 이벤트 타입별 분류
	var newEvents, priceEvents []productDiff
	for _, d := range diffs {
		switch d.Type {
		case productEventNew:
			newEvents = append(newEvents, d)
		case productEventPriceChanged:
			priceEvents = append(priceEvents, d)
		}
	}

	require.Len(t, newEvents, 1)
	assert.Equal(t, "new_product", newEvents[0].Product.ProductID)
	assert.Nil(t, newEvents[0].Prev)

	require.Len(t, priceEvents, 2)
	for _, d := range priceEvents {
		assert.NotNil(t, d.Prev, "가격 변동 이벤트에는 이전 가격 정보가 있어야 합니다")
		assert.NotEqual(t, d.Product.LowPrice, d.Prev.LowPrice)
	}
}
