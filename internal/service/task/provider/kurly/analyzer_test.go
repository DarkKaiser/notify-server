package kurly

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 공통 헬퍼
// =============================================================================

// newSaleProduct 정상 판매 중인 상품 객체를 생성하는 헬퍼 (최저가 자동 설정).
func newSaleProduct(id, price int) *product {
	p := &product{ID: id, Name: "상품", Price: price}
	p.tryUpdateLowestPrice()
	return p
}

// prevMapFrom []*product 슬라이스에서 ID 기반 Map을 생성하는 헬퍼.
func prevMapFrom(products ...*product) map[int]*product {
	m := make(map[int]*product, len(products))
	for _, p := range products {
		m[p.ID] = p
	}
	return m
}

// =============================================================================
// extractProductDiffs 테스트
// =============================================================================

// TestExtractProductDiffs_NewProduct [1단계] 신규 상품 감지 로직을 검증합니다.
func TestExtractProductDiffs_NewProduct(t *testing.T) {
	t.Parallel()

	t.Run("1단계: 이전에 없던 ID → productEventNew", func(t *testing.T) {
		t.Parallel()
		curr := &watchProductPriceSnapshot{Products: []*product{newSaleProduct(1, 5000)}}
		prev := prevMapFrom() // 비어있음

		diffs := extractProductDiffs(curr, prev)

		require.Len(t, diffs, 1)
		assert.Equal(t, productEventNew, diffs[0].Type)
		assert.Nil(t, diffs[0].Prev, "신규 상품은 Prev가 nil이어야 합니다")
	})

	t.Run("1단계: prevProductsByID=nil → 모든 상품이 신규로 처리됨", func(t *testing.T) {
		t.Parallel()
		curr := &watchProductPriceSnapshot{Products: []*product{
			newSaleProduct(1, 5000),
			newSaleProduct(2, 3000),
		}}

		diffs := extractProductDiffs(curr, nil)

		require.Len(t, diffs, 2)
		for _, d := range diffs {
			assert.Equal(t, productEventNew, d.Type)
		}
	})

	t.Run("1단계: 임시 실패 객체(Price=0, FetchFailedCount>0, LowestPrice=0) → 신규 처리", func(t *testing.T) {
		t.Parallel()
		// 이전에 단 한 번도 정상 수집되지 않은 상태 (실패만 누적)
		failedPrev := &product{ID: 1, Price: 0, FetchFailedCount: 1, LowestPrice: 0}
		curr := &watchProductPriceSnapshot{Products: []*product{newSaleProduct(1, 5000)}}
		prev := prevMapFrom(failedPrev)

		diffs := extractProductDiffs(curr, prev)

		require.Len(t, diffs, 1)
		assert.Equal(t, productEventNew, diffs[0].Type, "임시 실패 객체는 신규로 처리되어야 합니다")
	})

	t.Run("1단계: 신규 상품이지만 FetchFailedCount>0 → 알림 보류(diffs에 추가 안 됨)", func(t *testing.T) {
		t.Parallel()
		// 아직 수집 실패 중인 신규 상품
		failProd := &product{ID: 1, FetchFailedCount: 1}
		curr := &watchProductPriceSnapshot{Products: []*product{failProd}}

		diffs := extractProductDiffs(curr, nil) // nil = 신규

		assert.Len(t, diffs, 0, "FetchFailedCount>0이면 신규 상품 알림을 보류해야 합니다")
	})

	t.Run("1단계: 신규 상품이지만 IsUnavailable=true → 알림 보류", func(t *testing.T) {
		t.Parallel()
		unavailProd := &product{ID: 1, IsUnavailable: true}
		curr := &watchProductPriceSnapshot{Products: []*product{unavailProd}}

		diffs := extractProductDiffs(curr, nil)

		assert.Len(t, diffs, 0, "IsUnavailable=true인 신규 상품은 알림을 보류해야 합니다")
	})

	t.Run("1단계: LowestPrice>0인 이전 실패 상품 — 신규 아님, 3단계(가격 변동)로 이동", func(t *testing.T) {
		t.Parallel()
		// LowestPrice > 0이면 이전에 한 번은 정상 수집된 적 있으므로 신규가 아닙니다.
		// prevProduct.Price=0 이지만 currentProduct.Price=5000 이므로 hasPriceChangedFrom이 true가 되어
		// 3단계에서 가격 변동 이벤트가 발생합니다.
		prevWithHistory := &product{ID: 1, Price: 0, FetchFailedCount: 2, LowestPrice: 4500}
		currProduct := newSaleProduct(1, 5000)
		currProduct.LowestPrice = 4500 // mergeWithPreviousState가 이월했다고 가정

		curr := &watchProductPriceSnapshot{Products: []*product{currProduct}}
		prev := prevMapFrom(prevWithHistory)

		diffs := extractProductDiffs(curr, prev)

		// prevPrice(0) != currEffectivePrice(5000) → 가격 변동 감지됨
		require.Len(t, diffs, 1, "LowestPrice>0이면 신규 아님, hasPriceChangedFrom에 의해 가격 변동 이벤트 발생")
		assert.Equal(t, productEventPriceChanged, diffs[0].Type)
	})
}

// TestExtractProductDiffs_UnavailableTransition [2단계] 판매 가능 여부 전이 감지를 검증합니다.
func TestExtractProductDiffs_UnavailableTransition(t *testing.T) {
	t.Parallel()

	t.Run("2-1: Unavailable→Available (재입고) → productEventReappeared", func(t *testing.T) {
		t.Parallel()
		prevUnavail := &product{ID: 1, Name: "사과", Price: 5000, IsUnavailable: true}
		currAvail := newSaleProduct(1, 5000)

		curr := &watchProductPriceSnapshot{Products: []*product{currAvail}}
		prev := prevMapFrom(prevUnavail)

		diffs := extractProductDiffs(curr, prev)

		require.Len(t, diffs, 1)
		assert.Equal(t, productEventReappeared, diffs[0].Type)
		assert.Nil(t, diffs[0].Prev, "재입고 알림에는 Prev가 nil이어야 합니다")
	})

	t.Run("2-2: Available→Unavailable (판매 중지) → 알림 없음, diffs 0개", func(t *testing.T) {
		t.Parallel()
		prevAvail := newSaleProduct(1, 5000)
		currUnavail := &product{ID: 1, Name: "사과", IsUnavailable: true}

		curr := &watchProductPriceSnapshot{Products: []*product{currUnavail}}
		prev := prevMapFrom(prevAvail)

		diffs := extractProductDiffs(curr, prev)

		assert.Len(t, diffs, 0, "판매 중지는 조용히 처리되어 알림이 없어야 합니다")
	})

	t.Run("2-3: Unavailable→Unavailable (유지) → 무시, diffs 0개", func(t *testing.T) {
		t.Parallel()
		prevUnavail := &product{ID: 1, IsUnavailable: true}
		currUnavail := &product{ID: 1, IsUnavailable: true}

		curr := &watchProductPriceSnapshot{Products: []*product{currUnavail}}
		prev := prevMapFrom(prevUnavail)

		diffs := extractProductDiffs(curr, prev)

		assert.Len(t, diffs, 0, "Unavailable 유지 상태는 무시되어야 합니다")
	})
}

// TestExtractProductDiffs_PriceChange [3단계] 가격 변동 및 최저가 경신 분기를 검증합니다.
func TestExtractProductDiffs_PriceChange(t *testing.T) {
	t.Parallel()

	t.Run("3단계: 가격 변동 없음 → diffs 0개", func(t *testing.T) {
		t.Parallel()
		prevProd := newSaleProduct(1, 5000)
		currProd := newSaleProduct(1, 5000)
		currProd.LowestPrice = prevProd.LowestPrice

		curr := &watchProductPriceSnapshot{Products: []*product{currProd}}
		prev := prevMapFrom(prevProd)

		diffs := extractProductDiffs(curr, prev)

		assert.Len(t, diffs, 0)
	})

	t.Run("3단계: 역대 최저가 첫 기록(prevLowestPrice=0) → productEventLowestPriceAchieved", func(t *testing.T) {
		t.Parallel()
		// prevProduct.LowestPrice = 0 이면 '이전 기록 없음' → 최저가 달성으로 분기
		prevProd := &product{ID: 1, Name: "사과", Price: 5000, LowestPrice: 0}
		currProd := newSaleProduct(1, 4000)

		curr := &watchProductPriceSnapshot{Products: []*product{currProd}}
		prev := prevMapFrom(prevProd)

		diffs := extractProductDiffs(curr, prev)

		require.Len(t, diffs, 1)
		assert.Equal(t, productEventLowestPriceAchieved, diffs[0].Type)
		assert.Equal(t, prevProd, diffs[0].Prev)
	})

	t.Run("3단계: 이전 최저가보다 낮음 → productEventLowestPriceAchieved", func(t *testing.T) {
		t.Parallel()
		prevProd := newSaleProduct(1, 5000) // LowestPrice = 5000
		currProd := newSaleProduct(1, 3000) // LowestPrice = 3000 < 5000
		currProd.LowestPrice = 3000

		curr := &watchProductPriceSnapshot{Products: []*product{currProd}}
		prev := prevMapFrom(prevProd)

		diffs := extractProductDiffs(curr, prev)

		require.Len(t, diffs, 1)
		assert.Equal(t, productEventLowestPriceAchieved, diffs[0].Type)
		assert.Equal(t, prevProd, diffs[0].Prev)
	})

	t.Run("3단계: 가격 상승(최저가 갱신 아님) → productEventPriceChanged", func(t *testing.T) {
		t.Parallel()
		prevProd := newSaleProduct(1, 5000) // LowestPrice = 5000
		currProd := newSaleProduct(1, 6000) // 가격 상승
		currProd.LowestPrice = 5000         // 이전 최저가 이월

		curr := &watchProductPriceSnapshot{Products: []*product{currProd}}
		prev := prevMapFrom(prevProd)

		diffs := extractProductDiffs(curr, prev)

		require.Len(t, diffs, 1)
		assert.Equal(t, productEventPriceChanged, diffs[0].Type)
		assert.Equal(t, prevProd, diffs[0].Prev)
	})

	t.Run("3단계: 가격 하락했지만 역대 최저가보다 높음 → productEventPriceChanged", func(t *testing.T) {
		t.Parallel()
		prevProd := newSaleProduct(1, 8000)
		prevProd.LowestPrice = 3000         // 역대 최저가는 이미 3000원
		currProd := newSaleProduct(1, 6000) // 하락했지만 3000원보다는 높음
		currProd.LowestPrice = 3000         // 이전 최저가 이월

		curr := &watchProductPriceSnapshot{Products: []*product{currProd}}
		prev := prevMapFrom(prevProd)

		diffs := extractProductDiffs(curr, prev)

		require.Len(t, diffs, 1)
		assert.Equal(t, productEventPriceChanged, diffs[0].Type)
	})
}

// TestExtractProductDiffs_MultiProduct 다중 상품이 혼재할 때의 복합 시나리오를 검증합니다.
func TestExtractProductDiffs_MultiProduct(t *testing.T) {
	t.Parallel()

	t.Run("복합: 신규·가격변동·변화없음 혼재 → 각각 올바른 이벤트", func(t *testing.T) {
		t.Parallel()
		p1Prev := newSaleProduct(1, 5000)
		p2Prev := newSaleProduct(2, 3000)

		p1Curr := newSaleProduct(1, 4000) // 가격 하락 → LowestPriceAchieved
		p1Curr.LowestPrice = 4000
		p2Curr := newSaleProduct(2, 3000) // 변화 없음
		p2Curr.LowestPrice = p2Prev.LowestPrice
		p3Curr := newSaleProduct(3, 7000) // 신규

		curr := &watchProductPriceSnapshot{Products: []*product{p1Curr, p2Curr, p3Curr}}
		prev := prevMapFrom(p1Prev, p2Prev)

		diffs := extractProductDiffs(curr, prev)

		require.Len(t, diffs, 2)
		eventTypes := map[productEventType]bool{}
		for _, d := range diffs {
			eventTypes[d.Type] = true
		}
		assert.True(t, eventTypes[productEventNew])
		assert.True(t, eventTypes[productEventLowestPriceAchieved])
	})
}

// =============================================================================
// extractNewDuplicateRecords 테스트
// =============================================================================

// TestExtractNewDuplicateRecords 중복 상품 레코드의 알림 스팸 방지 State-Machine을 검증합니다.
func TestExtractNewDuplicateRecords(t *testing.T) {
	t.Parallel()

	makeRecord := func(id, name string) []string {
		return []string{id, name, "1"}
	}

	t.Run("빈 duplicateRecords → nil, nil 반환", func(t *testing.T) {
		t.Parallel()
		newRec, updatedIDs := extractNewDuplicateRecords(nil, nil)
		assert.Nil(t, newRec)
		assert.Nil(t, updatedIDs)
	})

	t.Run("빈 슬라이스 → nil, nil 반환", func(t *testing.T) {
		t.Parallel()
		newRec, updatedIDs := extractNewDuplicateRecords([][]string{}, nil)
		assert.Nil(t, newRec)
		assert.Nil(t, updatedIDs)
	})

	t.Run("이전 발송 이력 없음 → 전량 신규 알림 대상", func(t *testing.T) {
		t.Parallel()
		records := [][]string{
			makeRecord("100", "사과"),
			makeRecord("200", "바나나"),
		}
		newRec, updatedIDs := extractNewDuplicateRecords(records, nil)

		assert.Len(t, newRec, 2)
		assert.Len(t, updatedIDs, 2)
		assert.Contains(t, updatedIDs, "100")
		assert.Contains(t, updatedIDs, "200")
	})

	t.Run("이미 알림을 보낸 상품 → 스킵, 새로운 상품만 알림 대상", func(t *testing.T) {
		t.Parallel()
		records := [][]string{
			makeRecord("100", "사과"),  // 이미 발송됨
			makeRecord("200", "바나나"), // 신규
		}
		prevNotified := []string{"100"}

		newRec, updatedIDs := extractNewDuplicateRecords(records, prevNotified)

		require.Len(t, newRec, 1)
		assert.Equal(t, "200", newRec[0][columnID])
		assert.Len(t, updatedIDs, 2, "updatedIDs에는 100, 200 모두 포함되어야 합니다")
	})

	t.Run("전체 이미 발송됨 → newDuplicateRecords=nil, updatedIDs만 갱신", func(t *testing.T) {
		t.Parallel()
		records := [][]string{
			makeRecord("100", "사과"),
			makeRecord("200", "바나나"),
		}
		prevNotified := []string{"100", "200"}

		newRec, updatedIDs := extractNewDuplicateRecords(records, prevNotified)

		assert.Nil(t, newRec)
		assert.Len(t, updatedIDs, 2)
	})

	t.Run("중복 목록에서 상품 사라지면 updatedIDs에서도 자동 제외", func(t *testing.T) {
		t.Parallel()
		// 이전에는 100, 200, 300이 중복. 이번 사이클에서 200이 해소됨
		records := [][]string{
			makeRecord("100", "사과"),
			makeRecord("300", "포도"),
		}
		prevNotified := []string{"100", "200", "300"}

		newRec, updatedIDs := extractNewDuplicateRecords(records, prevNotified)

		assert.Nil(t, newRec, "기존 발송 상품은 신규 알림 없어야 합니다")
		assert.Len(t, updatedIDs, 2)
		assert.Contains(t, updatedIDs, "100")
		assert.Contains(t, updatedIDs, "300")
		assert.NotContains(t, updatedIDs, "200", "중복 해소된 상품은 updatedIDs에서 제거되어야 합니다")
	})

	t.Run("같은 ID가 루프 내에서 연속 등장 — notifiedSet 즉시 동기화로 스팸 방지", func(t *testing.T) {
		t.Parallel()
		// 동일 ID가 두 번 등장하는 경우 (CSV에 실수로 중복)
		records := [][]string{
			makeRecord("100", "사과"),
			makeRecord("100", "사과(중복)"),
		}
		newRec, updatedIDs := extractNewDuplicateRecords(records, nil)

		// 첫 번째 등장에만 알림, 두 번째는 이미 notifiedSet에 등록되어 스킵
		require.Len(t, newRec, 1, "동일 ID 두 번째 등장은 스팸 방지로 스킵되어야 합니다")
		assert.Len(t, updatedIDs, 2, "updatedIDs에는 중복 포함 모두 기록됩니다")
	})

	t.Run("columnID보다 짧은 레코드 — len 체크로 건너뜀", func(t *testing.T) {
		t.Parallel()
		records := [][]string{
			{}, // 빈 레코드 → columnID(0) 접근 불가 → 스킵
			makeRecord("200", "바나나"),
		}
		newRec, updatedIDs := extractNewDuplicateRecords(records, nil)

		require.Len(t, newRec, 1)
		assert.Equal(t, "200", newRec[0][columnID])
		assert.Len(t, updatedIDs, 1)
	})

	t.Run("상품 ID 앞뒤 공백 → TrimSpace 처리 후 비교", func(t *testing.T) {
		t.Parallel()
		records := [][]string{
			{"  100  ", "사과", "1"}, // 공백 포함 ID
		}
		newRec, updatedIDs := extractNewDuplicateRecords(records, []string{"100"}) // trim 없이 "100"과 같아야 match

		assert.Nil(t, newRec, "TrimSpace 후 이미 발송된 ID와 일치해야 합니다")
		assert.Len(t, updatedIDs, 1)
		assert.Equal(t, "100", updatedIDs[0], "TrimSpace된 값이 저장되어야 합니다")
	})
}

// =============================================================================
// extractNewlyUnavailableProducts 테스트
// =============================================================================

// TestExtractNewlyUnavailableProducts 새롭게 판매 불가로 전이된 상품 감지를 검증합니다.
func TestExtractNewlyUnavailableProducts(t *testing.T) {
	t.Parallel()

	makeCSVRecord := func(id, name string) []string {
		return []string{id, name, "1"}
	}

	t.Run("currentProducts가 비어있으면 nil 반환", func(t *testing.T) {
		t.Parallel()
		result := extractNewlyUnavailableProducts(nil, nil, nil)
		assert.Nil(t, result)
	})

	t.Run("모든 상품이 판매 중 → nil 반환", func(t *testing.T) {
		t.Parallel()
		products := []*product{
			{ID: 1, IsUnavailable: false},
			{ID: 2, IsUnavailable: false},
		}
		result := extractNewlyUnavailableProducts(products, nil, nil)
		assert.Nil(t, result)
	})

	t.Run("이전에도 Unavailable이었던 상품 → 스팸 방지로 제외", func(t *testing.T) {
		t.Parallel()
		curr := []*product{{ID: 1, IsUnavailable: true}}
		prev := prevMapFrom(&product{ID: 1, IsUnavailable: true}) // 이전에도 단종
		records := [][]string{makeCSVRecord("1", "사과")}

		result := extractNewlyUnavailableProducts(curr, prev, records)

		assert.Nil(t, result, "이미 Unavailable이었던 상품은 제외되어야 합니다")
	})

	t.Run("이번에 처음 Unavailable 전이된 상품 → 알림 대상", func(t *testing.T) {
		t.Parallel()
		curr := []*product{{ID: 1, IsUnavailable: true}}
		prev := prevMapFrom(&product{ID: 1, IsUnavailable: false}) // 이전에는 정상
		records := [][]string{makeCSVRecord("1", "사과")}

		result := extractNewlyUnavailableProducts(curr, prev, records)

		require.Len(t, result, 1)
		assert.Equal(t, "1", result[0].ID)
		assert.Equal(t, "사과", result[0].Name)
	})

	t.Run("prevProductsByID=nil (최초 실행) → Unavailable 상품 알림 대상에 포함", func(t *testing.T) {
		t.Parallel()
		// 최초 실행이라 이전 스냅샷 없음 → 전이 여부 판별 불가 → 포함
		curr := []*product{{ID: 1, IsUnavailable: true}}
		records := [][]string{makeCSVRecord("1", "사과")}

		result := extractNewlyUnavailableProducts(curr, nil, records)

		require.Len(t, result, 1)
		assert.Equal(t, "1", result[0].ID)
	})

	t.Run("이전에 없던 ID (신규면서 바로 Unavailable) → 알림 대상", func(t *testing.T) {
		t.Parallel()
		// prevProductsByID에 ID 없음 → 신규 상품이지만 바로 단종
		curr := []*product{{ID: 99, IsUnavailable: true}}
		prev := prevMapFrom(&product{ID: 1}) // 다른 ID만 있음
		records := [][]string{makeCSVRecord("99", "신규단종상품")}

		result := extractNewlyUnavailableProducts(curr, prev, records)

		require.Len(t, result, 1)
		assert.Equal(t, "99", result[0].ID)
	})

	t.Run("CSV에 없는 상품 (삭제/비활성화) → 알림 제외", func(t *testing.T) {
		t.Parallel()
		curr := []*product{{ID: 1, IsUnavailable: true}}
		prev := prevMapFrom(&product{ID: 1, IsUnavailable: false})
		records := [][]string{} // CSV에 해당 상품 없음

		result := extractNewlyUnavailableProducts(curr, prev, records)

		assert.Nil(t, result, "CSV에 없는 상품은 알림 대상에서 제외되어야 합니다")
	})

	t.Run("CSV 상품명 비어있음 → fallbackProductName으로 대체", func(t *testing.T) {
		t.Parallel()
		curr := []*product{{ID: 1, IsUnavailable: true}}
		prev := prevMapFrom(&product{ID: 1, IsUnavailable: false})
		records := [][]string{makeCSVRecord("1", "")} // 이름 없음

		result := extractNewlyUnavailableProducts(curr, prev, records)

		require.Len(t, result, 1)
		assert.Equal(t, fallbackProductName, result[0].Name)
	})

	t.Run("다중 상품 복합 — 일부만 전이됨", func(t *testing.T) {
		t.Parallel()
		curr := []*product{
			{ID: 1, IsUnavailable: true},  // 신규 전이 → 알림
			{ID: 2, IsUnavailable: true},  // 이전에도 단종 → 스킵
			{ID: 3, IsUnavailable: false}, // 정상 → 스킵
		}
		prev := prevMapFrom(
			&product{ID: 1, IsUnavailable: false}, // 이전 정상
			&product{ID: 2, IsUnavailable: true},  // 이전 단종
			&product{ID: 3, IsUnavailable: false},
		)
		records := [][]string{
			makeCSVRecord("1", "상품A"),
			makeCSVRecord("2", "상품B"),
			makeCSVRecord("3", "상품C"),
		}

		result := extractNewlyUnavailableProducts(curr, prev, records)

		require.Len(t, result, 1)
		assert.Equal(t, "1", result[0].ID)
		assert.Equal(t, "상품A", result[0].Name)
	})
}
