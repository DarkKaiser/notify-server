package kurly

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// watchProductPriceSnapshot.HasChanged 테스트
// =============================================================================

// TestWatchProductPriceSnapshot_HasChanged
// 이전 스냅샷 대비 변경 여부를 판단하는 HasChanged 메서드를 전방위적으로 검증합니다.
// 각 필드 단위의 변경이 올바르게 감지되는지 개별 검사합니다.
func TestWatchProductPriceSnapshot_HasChanged(t *testing.T) {
	t.Parallel()

	productNow := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// 기준이 되는 "변경 없는" 상태의 스냅샷 팩토리
	makeSnapshot := func() *watchProductPriceSnapshot {
		return &watchProductPriceSnapshot{
			Products: []*product{
				{
					ID:                 100,
					Name:               "사과",
					Price:              5000,
					DiscountedPrice:    0,
					DiscountRate:       0,
					LowestPrice:        5000,
					LowestPriceTimeUTC: productNow,
					IsUnavailable:      false,
					FetchFailedCount:   0,
				},
			},
			DuplicateNotifiedIDs: []string{"200"},
		}
	}

	t.Run("이전 스냅샷 nil → 항상 true (최초 실행)", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		assert.True(t, curr.HasChanged(nil))
	})

	t.Run("완전히 동일한 스냅샷 → false", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		prev := makeSnapshot()
		assert.False(t, curr.HasChanged(prev))
	})

	// ── 상품 목록 변경 감지 ────────────────────────────────────────────────

	t.Run("상품 개수 증가 → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.Products = append(curr.Products, &product{ID: 999})
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("상품 개수 감소 → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.Products = nil
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("이전에 없던 ID의 상품 → true (순서 독립적 Map 비교)", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.Products[0].ID = 999 // ID 변경 → prev에 ID=999가 없으므로 변경 감지
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	// ── 개별 상품 필드 단위 변경 감지 ─────────────────────────────────────

	t.Run("Price 변경 → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.Products[0].Price = 6000
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("DiscountedPrice 변경 → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.Products[0].DiscountedPrice = 4500
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("DiscountRate 변경 → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.Products[0].DiscountRate = 10
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("LowestPrice 변경 → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.Products[0].LowestPrice = 4000
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("LowestPriceTimeUTC 변경 → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.Products[0].LowestPriceTimeUTC = productNow.Add(time.Second)
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("IsUnavailable 변경 → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.Products[0].IsUnavailable = true
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("FetchFailedCount 변경 → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.Products[0].FetchFailedCount = 1
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("Name만 변경 → false (Name은 비교 대상 제외)", func(t *testing.T) {
		t.Parallel()
		// 설계상 상품명(Name)은 크롤링 신뢰도가 낮아 변경 판단 기준에서 명시적으로 제외됩니다.
		curr := makeSnapshot()
		curr.Products[0].Name = "변경된 사과 이름"
		prev := makeSnapshot()
		assert.False(t, curr.HasChanged(prev))
	})

	// ── DuplicateNotifiedIDs 변경 감지 ────────────────────────────────────

	t.Run("DuplicateNotifiedIDs 개수 증가 → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.DuplicateNotifiedIDs = append(curr.DuplicateNotifiedIDs, "300")
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("DuplicateNotifiedIDs 개수 감소 → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.DuplicateNotifiedIDs = nil
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("DuplicateNotifiedIDs 내용 변경 (같은 개수, 다른 값) → true", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.DuplicateNotifiedIDs = []string{"999"} // prev는 "200"
		prev := makeSnapshot()
		assert.True(t, curr.HasChanged(prev))
	})

	t.Run("DuplicateNotifiedIDs 둘 다 비어있음 → false", func(t *testing.T) {
		t.Parallel()
		curr := makeSnapshot()
		curr.DuplicateNotifiedIDs = nil
		prev := makeSnapshot()
		prev.DuplicateNotifiedIDs = nil
		assert.False(t, curr.HasChanged(prev))
	})

	// ── 다중 상품 및 순서 독립성 ────────────────────────────────────────────

	t.Run("다중 상품 — 순서가 달라도 내용이 같으면 false", func(t *testing.T) {
		t.Parallel()
		p1 := &product{ID: 1, Price: 1000, LowestPriceTimeUTC: productNow}
		p2 := &product{ID: 2, Price: 2000, LowestPriceTimeUTC: productNow}

		curr := &watchProductPriceSnapshot{Products: []*product{p2, p1}}
		prev := &watchProductPriceSnapshot{Products: []*product{p1, p2}}

		assert.False(t, curr.HasChanged(prev))
	})

	t.Run("다중 상품 — 특정 상품 하나의 가격만 변경 → true", func(t *testing.T) {
		t.Parallel()
		curr := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 1, Price: 1000, LowestPriceTimeUTC: productNow},
				{ID: 2, Price: 9999, LowestPriceTimeUTC: productNow}, // 변경됨
			},
		}
		prev := &watchProductPriceSnapshot{
			Products: []*product{
				{ID: 1, Price: 1000, LowestPriceTimeUTC: productNow},
				{ID: 2, Price: 2000, LowestPriceTimeUTC: productNow},
			},
		}
		assert.True(t, curr.HasChanged(prev))
	})
}

// =============================================================================
// product.tryUpdateLowestPrice 시간 검증 전용 테스트
// =============================================================================

// TestProduct_TryUpdateLowestPrice_TimestampUTC
// product_test.go의 tryUpdateLowestPrice 테스트를 보완하여 UTC 타임스탬프 정확성을 검증합니다.
func TestProduct_TryUpdateLowestPrice_TimestampUTC(t *testing.T) {
	t.Parallel()

	t.Run("갱신된 LowestPriceTimeUTC는 반드시 UTC여야 함", func(t *testing.T) {
		t.Parallel()
		p := &product{Price: 3000, LowestPrice: 0}

		before := time.Now().UTC()
		updated := p.tryUpdateLowestPrice()
		after := time.Now().UTC()

		require.True(t, updated)
		assert.Equal(t, time.UTC, p.LowestPriceTimeUTC.Location(), "UTC 타임존이어야 합니다")
		assert.True(t, !p.LowestPriceTimeUTC.Before(before), "갱신 시각은 호출 이전보다 이르면 안 됩니다")
		assert.True(t, !p.LowestPriceTimeUTC.After(after), "갱신 시각은 호출 이후보다 늦으면 안 됩니다")
	})

	t.Run("미갱신 시 기존 LowestPriceTimeUTC 보존", func(t *testing.T) {
		t.Parallel()
		fixedTime := time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC)
		p := &product{Price: 8000, LowestPrice: 7000, LowestPriceTimeUTC: fixedTime}

		updated := p.tryUpdateLowestPrice()

		assert.False(t, updated)
		assert.True(t, p.LowestPriceTimeUTC.Equal(fixedTime), "기존 최저가 시각이 변경되어서는 안 됩니다")
	})
}
