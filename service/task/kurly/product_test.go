package kurly

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestFormatProductPageURL_TableDriven formatProductPageURL 함수의 다양한 입력 타입 처리를 검증합니다.
// int, string 등 다양한 타입의 ID가 올바른 URL로 변환되는지 테스트합니다.
func TestFormatProductPageURL_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   any
		want string
	}{
		{
			name: "Integer ID",
			id:   12345,
			want: "https://www.kurly.com/goods/12345",
		},
		{
			name: "String ID",
			id:   "67890",
			want: "https://www.kurly.com/goods/67890",
		},
		{
			name: "String ID with surrounding spaces (Function does NOT trim)",
			id:   "  11111  ",
			want: "https://www.kurly.com/goods/  11111  ", // fmt.Sprintf assumes caller handles trimming
		},
		{
			name: "Zero ID (Integer)",
			id:   0,
			want: "https://www.kurly.com/goods/0",
		},
		{
			name: "Negative ID (Edge Case)",
			id:   -1,
			want: "https://www.kurly.com/goods/-1",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatProductPageURL(tt.id)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestProduct_URL_Integration product.URL 메서드가 formatProductPageURL을 올바르게 사용하는지 검증합니다.
func TestProduct_URL_Integration(t *testing.T) {
	t.Parallel()

	p := &product{ID: 99999}
	want := "https://www.kurly.com/goods/99999"

	// product.URL()은 내부적으로 formatProductPageURL을 호출해야 함
	assert.Equal(t, want, p.URL(), "product.URL() should delegate to formatProductPageURL correctly")
}

// TestProduct_IsOnSale_TableDriven 할인 여부 판단 로직을 검증합니다.
// 경계값 테스트(Boundary Testing)를 포함하여 다양한 가격 시나리오를 커버합니다.
func TestProduct_IsOnSale_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		price           int
		discountedPrice int
		want            bool
	}{
		{
			name:            "Not on sale: No discount price (0)",
			price:           10000,
			discountedPrice: 0,
			want:            false,
		},
		{
			name:            "On sale: Normal discount case",
			price:           10000,
			discountedPrice: 9000,
			want:            true,
		},
		{
			name:            "Not on sale: Discounted price equals original price",
			price:           10000,
			discountedPrice: 10000,
			want:            false,
		},
		{
			name:            "Not on sale: Discounted price higher than original (Data Error)",
			price:           10000,
			discountedPrice: 11000,
			want:            false,
		},
		{
			name:            "Not on sale: Negative discounted price (Data Error)",
			price:           10000,
			discountedPrice: -100,
			want:            false,
		},
		{
			name:            "Not on sale: Zero original price",
			price:           0,
			discountedPrice: 0,
			want:            false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &product{
				Price:           tt.price,
				DiscountedPrice: tt.discountedPrice,
			}
			assert.Equal(t, tt.want, p.IsOnSale())
		})
	}
}

// TestProduct_UpdateLowestPrice_TableDriven 최저가 갱신 로직을 검증합니다.
// Cold Start 및 Price Drop 시나리오, 시간 갱신 여부, 반환값, UTC 저장 여부를 정밀하게 테스트합니다.
func TestProduct_UpdateLowestPrice_TableDriven(t *testing.T) {
	t.Parallel()

	// 테스트 실행 시점 (UTC)
	// 주의: 실제 코드 실행 시점과 미세한 차이가 있을 수 있으므로 WithinDuration으로 검증합니다.
	now := time.Now().UTC()

	tests := []struct {
		name            string
		initialProduct  *product
		wantLowestPrice int
		wantUpdated     bool // 갱신 발생 여부 (반환값)
	}{
		{
			name: "Cold Start - Normal Price",
			initialProduct: &product{
				Price: 10000,
			},
			wantLowestPrice: 10000,
			wantUpdated:     true,
		},
		{
			name: "Cold Start - Discounted Price (Use Discounted)",
			initialProduct: &product{
				Price:           10000,
				DiscountedPrice: 8000,
			},
			wantLowestPrice: 8000,
			wantUpdated:     true,
		},
		{
			name: "Price Drop - New Lowest Found",
			initialProduct: &product{
				Price:              9000,
				LowestPrice:        10000,
				LowestPriceTimeUTC: now.Add(-1 * time.Hour), // 1시간 전
			},
			wantLowestPrice: 9000,
			wantUpdated:     true,
		},
		{
			name: "No Change - Higher Price",
			initialProduct: &product{
				Price:              12000,
				LowestPrice:        10000,
				LowestPriceTimeUTC: now.Add(-1 * time.Hour),
			},
			wantLowestPrice: 10000,
			wantUpdated:     false,
		},
		{
			name: "No Change - Same Price",
			initialProduct: &product{
				Price:              10000,
				LowestPrice:        10000,
				LowestPriceTimeUTC: now.Add(-1 * time.Hour),
			},
			wantLowestPrice: 10000,
			wantUpdated:     false,
		},
		{
			name: "Price Drop - Discounted is Lower than Prev Lowest",
			initialProduct: &product{
				Price:              12000,
				DiscountedPrice:    9000,
				LowestPrice:        10000,
				LowestPriceTimeUTC: now.Add(-1 * time.Hour),
			},
			wantLowestPrice: 9000,
			wantUpdated:     true,
		},
		{
			name: "Edge Case - Zero Price (Ignored)",
			initialProduct: &product{
				Price:           0,
				DiscountedPrice: 0,
				LowestPrice:     0,
			},
			wantLowestPrice: 0,
			wantUpdated:     false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := tt.initialProduct
			// 기존 시간 백업 (갱신 안 된 경우 비교용)
			originalTime := p.LowestPriceTimeUTC

			// Execute
			gotUpdated := p.updateLowestPrice()

			// Verify
			assert.Equal(t, tt.wantLowestPrice, p.LowestPrice, "LowestPrice mismatch")
			assert.Equal(t, tt.wantUpdated, gotUpdated, "Updated return value mismatch")

			if tt.wantUpdated {
				// 갱신된 경우:
				// 1. 시간은 "현재" 시점으로 갱신되어야 함 (1초 오차 허용)
				//    (단순히 After 비교보다 WithinDuration이 훨씬 정밀하고 안전합니다)
				assert.WithinDuration(t, time.Now().UTC(), p.LowestPriceTimeUTC, 1*time.Second, "LowestPriceTimeUTC should be updated to now")

				// 2. 시간은 반드시 UTC여야 함
				assert.Equal(t, time.UTC, p.LowestPriceTimeUTC.Location(), "LowestPriceTimeUTC should be in UTC")
			} else {
				// 갱신 안 된 경우: 기존 시간 유지 확인
				assert.Equal(t, originalTime, p.LowestPriceTimeUTC, "LowestPriceTimeUTC should NOT be updated")
			}
		})
	}
}

// TestProduct_Render_TableDriven Render 메서드의 단일 상품 렌더링 로직을 정밀 검증합니다.
// HTML/Text 모드, 할인, 최저가 표시 등 다양한 시나리오를 커버합니다.
func TestProduct_Render_TableDriven(t *testing.T) {
	t.Parallel()

	baseProduct := &product{
		ID:    12345,
		Name:  "Base Product",
		Price: 10000,
	}

	tests := []struct {
		name         string
		product      *product
		supportsHTML bool
		mark         string
		wants        []string
		unwants      []string
	}{
		{
			name:         "Text Mode - Basic",
			product:      baseProduct,
			supportsHTML: false,
			wants: []string{
				"☞ Base Product",
				"• 현재 가격 : 10,000원",
			},
			unwants: []string{"<b>", "</a>", "<s>"},
		},
		{
			name:         "HTML Mode - Basic with Link",
			product:      baseProduct,
			supportsHTML: true,
			wants: []string{
				`<a href="https://www.kurly.com/goods/12345"><b>Base Product</b></a>`,
				"10,000원",
			},
		},
		{
			name: "HTML Mode - XSS Protection",
			product: &product{
				ID:    123,
				Name:  "<script>alert(1)</script>",
				Price: 1000,
			},
			supportsHTML: true,
			wants: []string{
				"&lt;script&gt;alert(1)&lt;/script&gt;",
			},
			unwants: []string{"<script>"},
		},
		{
			name: "Text Mode - Discounted Price",
			product: &product{
				Name:            "Sale Item",
				Price:           20000,
				DiscountedPrice: 15000,
				DiscountRate:    25,
			},
			supportsHTML: false,
			wants: []string{
				"20,000원 ⇒ 15,000원 (25%)",
			},
		},
		{
			name: "HTML Mode - Discounted Price",
			product: &product{
				Name:            "Sale Item HTML",
				Price:           20000,
				DiscountedPrice: 15000,
				DiscountRate:    25,
			},
			supportsHTML: true,
			wants: []string{
				"<s>20,000원</s> 15,000원 (25%)",
			},
		},
		{
			name: "With Lowest Price History",
			product: &product{
				Name:               "History Item",
				Price:              10000,
				LowestPrice:        9000,
				LowestPriceTimeUTC: time.Date(2023, 5, 5, 0, 0, 0, 0, time.UTC), // KST 09:00
			},
			supportsHTML: false,
			wants: []string{
				"• 최저 가격 : 9,000원",
				"(2023/05/05 09:00)",
			},
		},
		{
			name: "Edge Case - Discount Rate 0%",
			product: &product{
				Name:            "Zero Rate",
				Price:           10000,
				DiscountedPrice: 9900,
				DiscountRate:    0,
			},
			supportsHTML: false,
			wants: []string{
				"10,000원 ⇒ 9,900원",
			},
			// 0%는 표시하지 않음
			unwants: []string{"(0%)", "(%)"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.product.Render(tt.supportsHTML, tt.mark)
			for _, want := range tt.wants {
				assert.Contains(t, got, want, "Missing expected substring: %s", want)
			}
			for _, unwant := range tt.unwants {
				assert.NotContains(t, got, unwant, "Contains unexpected substring: %s", unwant)
			}
		})
	}
}

// TestProduct_RenderDiff_TableDriven RenderDiff 메서드의 비교 렌더링 로직을 검증합니다.
// 이전 가격(prev) 유무에 따른 동작 차이를 중점적으로 테스트합니다.
func TestProduct_RenderDiff_TableDriven(t *testing.T) {
	t.Parallel()

	curr := &product{
		Name:  "Diff Item",
		Price: 10000,
	}

	tests := []struct {
		name         string
		product      *product
		prev         *product
		supportsHTML bool
		wants        []string
		unwants      []string
	}{
		{
			name:         "With Previous Price",
			product:      curr,
			prev:         &product{Price: 12000},
			supportsHTML: false,
			wants: []string{
				"• 현재 가격 : 10,000원",
				"• 이전 가격 : 12,000원",
			},
		},
		{
			name:         "Previous Price is Nil (Should behave like Render)",
			product:      curr,
			prev:         nil,
			supportsHTML: false,
			wants: []string{
				"• 현재 가격 : 10,000원",
			},
			unwants: []string{
				"• 이전 가격",
			},
		},
		{
			name:         "Previous Price with Discount",
			product:      curr,
			prev:         &product{Price: 12000, DiscountedPrice: 11000, DiscountRate: 8},
			supportsHTML: false,
			wants: []string{
				"12,000원 ⇒ 11,000원 (8%)",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.product.RenderDiff(tt.supportsHTML, "", tt.prev)
			for _, want := range tt.wants {
				assert.Contains(t, got, want, "Missing expected substring: %s", want)
			}
			for _, unwant := range tt.unwants {
				assert.NotContains(t, got, unwant, "Contains unexpected substring: %s", unwant)
			}
		})
	}
}

// BenchmarkProduct_Render Render 메서드의 성능을 측정합니다.
// 메모리 할당(Allocs) 최적화 상태를 점검합니다.
func BenchmarkProduct_Render(b *testing.B) {
	p := &product{
		ID:                 123456,
		Name:               "[브랜드] 벤치마크용 아주 긴 이름을 가진 상품입니다 (1kg)",
		Price:              50000,
		DiscountedPrice:    45000,
		DiscountRate:       10,
		LowestPrice:        40000,
		LowestPriceTimeUTC: time.Now(),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = p.Render(true, " 🔥")
	}
}

// BenchmarkProduct_RenderDiff RenderDiff 메서드의 성능을 측정합니다.
// 이전 가격 포맷팅이 추가됨에 따른 오버헤드를 확인합니다.
func BenchmarkProduct_RenderDiff(b *testing.B) {
	p := &product{
		ID:                 123456,
		Name:               "[브랜드] 벤치마크용 아주 긴 이름을 가진 상품입니다 (1kg)",
		Price:              50000,
		DiscountedPrice:    45000,
		DiscountRate:       10,
		LowestPrice:        40000,
		LowestPriceTimeUTC: time.Now(),
	}
	prev := &product{
		Price:           55000,
		DiscountedPrice: 50000,
		DiscountRate:    9,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = p.RenderDiff(true, " 🔥", prev)
	}
}

// Example_render demonstrates usage of Render and RenderDiff methods.
func Example_render() {
	p := &product{
		ID:              12345,
		Name:            "Example Item",
		Price:           10000,
		DiscountedPrice: 9000,
		DiscountRate:    10,
	}
	prev := &product{
		Price: 11000, // Previous was more expensive
	}

	// 1. Basic Render
	fmt.Println("--- Render ---")
	fmt.Println(p.Render(false, ""))

	// 2. Diff Render
	fmt.Println("\n--- RenderDiff ---")
	fmt.Println(p.RenderDiff(false, " 📉", prev))

	// Output:
	// --- Render ---
	// ☞ Example Item
	//       • 현재 가격 : 10,000원 ⇒ 9,000원 (10%)
	//
	// --- RenderDiff ---
	// ☞ Example Item 📉
	//       • 현재 가격 : 10,000원 ⇒ 9,000원 (10%)
	//       • 이전 가격 : 11,000원
}
