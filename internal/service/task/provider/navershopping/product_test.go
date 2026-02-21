package navershopping

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// product.key() 검증
// =============================================================================

// TestProduct_Key ProductID 값이 key()의 반환값과 정확히 일치하는지 검증합니다.
//
// key()는 스냅샷 비교 시 상품을 식별하는 유일한 기준입니다.
// 빈 문자열을 포함한 모든 값을 그대로 반환해야 합니다.
func TestProduct_Key(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		productID string
		want      string
	}{
		{
			name:      "숫자 ID",
			productID: "1234567890",
			want:      "1234567890",
		},
		{
			name:      "영숫자 혼합 ID",
			productID: "prod-123-abc",
			want:      "prod-123-abc",
		},
		{
			name:      "빈 ID (경계값)",
			productID: "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &product{ProductID: tt.productID}
			assert.Equal(t, tt.want, p.key())
		})
	}
}

// =============================================================================
// product.contentEquals() 검증
// =============================================================================

// TestProduct_ContentEquals 두 상품의 메타 정보(가격·키 제외) 동일 여부를 검증합니다.
//
// contentEquals 는 ProductType, Title, Link, MallName 네 필드를 비교합니다.
// LowPrice 와 ProductID 는 비교 대상이 아님을 함께 검증합니다.
func TestProduct_ContentEquals(t *testing.T) {
	t.Parallel()

	base := &product{
		ProductID:   "1",
		ProductType: "1",
		Title:       "Samsung Galaxy S24",
		Link:        "https://shopping.naver.com/products/1",
		LowPrice:    1000000,
		MallName:    "Samsung",
	}

	tests := []struct {
		name      string
		a         *product
		b         *product
		wantEqual bool
	}{
		{
			name:      "완전히 동일한 두 상품",
			a:         base,
			b:         &product{ProductID: "2", ProductType: "1", Title: "Samsung Galaxy S24", Link: "https://shopping.naver.com/products/1", LowPrice: 999999, MallName: "Samsung"},
			wantEqual: true,
		},
		{
			name:      "LowPrice만 다른 경우 → 동일로 판단",
			a:         base,
			b:         &product{ProductID: "1", ProductType: "1", Title: "Samsung Galaxy S24", Link: "https://shopping.naver.com/products/1", LowPrice: 500000, MallName: "Samsung"},
			wantEqual: true,
		},
		{
			name:      "ProductID만 다른 경우 → 동일로 판단",
			a:         base,
			b:         &product{ProductID: "99", ProductType: "1", Title: "Samsung Galaxy S24", Link: "https://shopping.naver.com/products/1", LowPrice: 1000000, MallName: "Samsung"},
			wantEqual: true,
		},
		{
			name:      "Title이 다른 경우 → 다름",
			a:         base,
			b:         &product{ProductID: "1", ProductType: "1", Title: "Samsung Galaxy S25", Link: "https://shopping.naver.com/products/1", LowPrice: 1000000, MallName: "Samsung"},
			wantEqual: false,
		},
		{
			name:      "Link가 다른 경우 → 다름",
			a:         base,
			b:         &product{ProductID: "1", ProductType: "1", Title: "Samsung Galaxy S24", Link: "https://shopping.naver.com/products/999", LowPrice: 1000000, MallName: "Samsung"},
			wantEqual: false,
		},
		{
			name:      "MallName이 다른 경우 → 다름",
			a:         base,
			b:         &product{ProductID: "1", ProductType: "1", Title: "Samsung Galaxy S24", Link: "https://shopping.naver.com/products/1", LowPrice: 1000000, MallName: "Coupang"},
			wantEqual: false,
		},
		{
			name:      "ProductType이 다른 경우 → 다름",
			a:         base,
			b:         &product{ProductID: "1", ProductType: "2", Title: "Samsung Galaxy S24", Link: "https://shopping.naver.com/products/1", LowPrice: 1000000, MallName: "Samsung"},
			wantEqual: false,
		},
		{
			name:      "receiver가 nil인 경우 → false",
			a:         nil,
			b:         base,
			wantEqual: false,
		},
		{
			name:      "other가 nil인 경우 → false",
			a:         base,
			b:         nil,
			wantEqual: false,
		},
		{
			name:      "둘 다 nil인 경우 → false",
			a:         nil,
			b:         nil,
			wantEqual: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.a.contentEquals(tt.b)
			assert.Equal(t, tt.wantEqual, got)
		})
	}
}

// TestProduct_ContentEquals_Symmetry contentEquals는 교환법칙을 만족해야 합니다.
// a.contentEquals(b) == b.contentEquals(a)
func TestProduct_ContentEquals_Symmetry(t *testing.T) {
	t.Parallel()

	a := &product{ProductType: "1", Title: "iPhone 15", Link: "http://link/a", MallName: "Apple"}
	b := &product{ProductType: "1", Title: "iPhone 15", Link: "http://link/a", MallName: "Apple"}
	c := &product{ProductType: "2", Title: "iPhone 15", Link: "http://link/a", MallName: "Apple"} // ProductType 다름

	assert.Equal(t, a.contentEquals(b), b.contentEquals(a), "교환법칙: 동일한 상품")
	assert.Equal(t, a.contentEquals(c), c.contentEquals(a), "교환법칙: 다른 상품")
}

// =============================================================================
// product.isPriceEligible() 검증
// =============================================================================

// TestProduct_IsPriceEligible 가격 필터링 로직을 경계값 분석(BVA) 기반으로 검증합니다.
//
// 유효 조건: LowPrice > 0 AND LowPrice < priceLessThan
// "미만(strictly less than)" 이므로 priceLessThan과 같은 경우는 false 입니다.
func TestProduct_IsPriceEligible(t *testing.T) {
	t.Parallel()

	const threshold = 100000 // 알림 기준 상한가: 100,000원

	tests := []struct {
		name         string
		lowPrice     int
		wantEligible bool
	}{
		// --- 유효한 케이스 ---
		{name: "상한가 바로 아래 (99,999원) → 대상", lowPrice: 99999, wantEligible: true},
		{name: "일반적인 가격 (50,000원) → 대상", lowPrice: 50000, wantEligible: true},
		{name: "최솟값 (1원) → 대상", lowPrice: 1, wantEligible: true},

		// --- 경계값: 제외 케이스 ---
		{name: "상한가와 같은 경우 (100,000원) → 미포함, 제외", lowPrice: threshold, wantEligible: false},
		{name: "상한가 초과 (100,001원) → 제외", lowPrice: 100001, wantEligible: false},

		// --- 비정상 데이터 케이스 ---
		{name: "0원 (API 오류/무효 데이터) → 제외", lowPrice: 0, wantEligible: false},
		{name: "음수 가격 (비정상 데이터) → 제외", lowPrice: -1, wantEligible: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &product{LowPrice: tt.lowPrice}
			assert.Equal(t, tt.wantEligible, p.isPriceEligible(threshold))
		})
	}
}
