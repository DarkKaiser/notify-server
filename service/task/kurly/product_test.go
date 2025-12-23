package kurly

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestProduct_URL_TableDriven Product URL 생성 로직을 검증합니다.
func TestProduct_URL_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   int
		want string
	}{
		{
			name: "Normal ID",
			id:   12345,
			want: "https://www.kurly.com/goods/12345",
		},
		{
			name: "Zero ID",
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
			p := &product{ID: tt.id}
			assert.Equal(t, tt.want, p.URL())
		})
	}
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
// Cold Start 및 Price Drop 시나리오, 시간 갱신 여부를 정밀하게 테스트합니다.
func TestProduct_UpdateLowestPrice_TableDriven(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name              string
		initialProduct    *product
		wantLowestPrice   int
		wantTimeCheck     bool // true: 시간 갱신 확인, false: 시간 유지 확인
		timeShouldBeAfter time.Time
	}{
		{
			name: "Cold Start - Normal Price",
			initialProduct: &product{
				Price: 10000,
			},
			wantLowestPrice: 10000,
			wantTimeCheck:   true,
		},
		{
			name: "Cold Start - Discounted Price (Use Discounted)",
			initialProduct: &product{
				Price:           10000,
				DiscountedPrice: 8000,
			},
			wantLowestPrice: 8000,
			wantTimeCheck:   true,
		},
		{
			name: "Price Drop - New Lowest Found",
			initialProduct: &product{
				Price:           9000,
				LowestPrice:     10000,
				LowestPriceTime: now,
			},
			wantLowestPrice:   9000,
			wantTimeCheck:     true,
			timeShouldBeAfter: now,
		},
		{
			name: "No Change - Higher Price",
			initialProduct: &product{
				Price:           12000,
				LowestPrice:     10000,
				LowestPriceTime: now,
			},
			wantLowestPrice:   10000,
			wantTimeCheck:     false,
			timeShouldBeAfter: now,
		},
		{
			name: "No Change - Same Price",
			initialProduct: &product{
				Price:           10000,
				LowestPrice:     10000,
				LowestPriceTime: now,
			},
			wantLowestPrice:   10000,
			wantTimeCheck:     false,
			timeShouldBeAfter: now,
		},
		{
			name: "Price Drop - Discounted is Lower than Prev Lowest",
			initialProduct: &product{
				Price:           12000,
				DiscountedPrice: 9000,
				LowestPrice:     10000,
			},
			wantLowestPrice: 9000,
			wantTimeCheck:   true,
		},
		{
			name: "Edge Case - Zero Price (Ignored)",
			initialProduct: &product{
				Price:           0,
				DiscountedPrice: 0,
				LowestPrice:     0,
			},
			wantLowestPrice: 0,
			wantTimeCheck:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := tt.initialProduct
			startTime := time.Now()

			// Execute
			p.updateLowestPrice()

			// Verify
			assert.Equal(t, tt.wantLowestPrice, p.LowestPrice, "LowestPrice mismatch")

			if tt.wantTimeCheck {
				// 갱신된 경우: startTime 이후여야 함
				assert.True(t, p.LowestPriceTime.After(startTime.Add(-time.Second)), "LowestPriceTime should be updated")
			} else if !tt.timeShouldBeAfter.IsZero() {
				// 갱신 안 된 경우: 기존 시간 유지 확인
				assert.Equal(t, tt.timeShouldBeAfter, p.LowestPriceTime, "LowestPriceTime should NOT be updated")
			}
		})
	}
}

// TestProduct_Render_Comprehensive Render 메서드의 모든 포맷팅 로직을 검증하는 통합 테스트입니다.
// HTML/Text 모드, 할인/비할인, 가격 변동, 특수문자 등 다양한 조합을 커버합니다.
func TestProduct_Render_Comprehensive(t *testing.T) {
	t.Parallel()

	// 공통 테스트 데이터
	baseProduct := &product{
		ID:    12345,
		Name:  "Fresh Apple",
		Price: 10000,
	}
	discountProduct := &product{
		ID:              12345,
		Name:            "Sale Apple",
		Price:           10000,
		DiscountedPrice: 8000,
		DiscountRate:    20,
	}

	tests := []struct {
		name         string
		product      *product
		supportsHTML bool
		mark         string
		prev         *product
		wants        []string // Expected substrings
		unwants      []string // Unexpected substrings
	}{
		// [Text Mode Tests]
		{
			name:         "Text Mode - Basic",
			product:      baseProduct,
			supportsHTML: false,
			wants: []string{
				"☞ Fresh Apple",
				"• 현재 가격 : 10,000원",
				// Text 모드에서는 Link가 자동으로 추가되지 않음 (기존 동작 유지)
			},
			unwants: []string{"<b>", "</a>", "<s>"},
		},
		{
			name:         "Text Mode - Discounted",
			product:      discountProduct,
			supportsHTML: false,
			wants: []string{
				"10,000원 ⇒ 8,000원 (20%)",
			},
			unwants: []string{"<s>", "</s>"},
		},
		{
			name:         "Text Mode - With Mark",
			product:      baseProduct,
			supportsHTML: false,
			mark:         " 🆕",
			wants: []string{
				"Fresh Apple 🆕",
			},
		},
		{
			name:         "Text Mode - With Previous Price",
			product:      baseProduct,
			supportsHTML: false,
			prev: &product{
				Price: 12000,
			},
			wants: []string{
				"• 이전 가격 : 12,000원",
			},
		},
		{
			name:         "Text Mode - With Lowest Price",
			product:      &product{Name: "Item", Price: 5000, LowestPrice: 4000, LowestPriceTime: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)},
			supportsHTML: false,
			wants: []string{
				"• 최저 가격 : 4,000원 (2023/01/01 12:00)",
			},
		},

		// [HTML Mode Tests]
		{
			name:         "HTML Mode - Basic",
			product:      baseProduct,
			supportsHTML: true,
			wants: []string{
				`<a href="https://www.kurly.com/goods/12345"><b>Fresh Apple</b></a>`,
				"10,000원",
			},
			unwants: []string{"https://www.kurly.com/goods/12345\n"}, // Link should be inside <a> tag, not standalone line
		},
		{
			name:         "HTML Mode - Discounted",
			product:      discountProduct,
			supportsHTML: true,
			wants: []string{
				"<s>10,000원</s> 8,000원 (20%)",
			},
			unwants: []string{"⇒"},
		},
		{
			name:         "HTML Mode - XSS Protection",
			product:      &product{ID: 1, Name: "<script>alert('XSS')</script>", Price: 100},
			supportsHTML: true,
			wants: []string{
				"&lt;script&gt;alert(&#39;XSS&#39;)&lt;/script&gt;",
			},
			unwants: []string{"<script>"},
		},

		// [Detailed Loop Logic Tests - writeFormattedPrice Coverage]
		{
			name: "Discount Rate 0% Handling",
			product: &product{
				ID:              999, // ID 추가 (URL 확인용)
				Name:            "No Rate Item",
				Price:           10000,
				DiscountedPrice: 9900,
				DiscountRate:    0,
			},
			supportsHTML: false,
			wants: []string{
				"10,000원 ⇒ 9,900원", // Rate literal "(%)" should be absent
			},
			unwants: []string{"(0%)", "(%)"},
		},
		{
			name: "Invalid Discount Price Handling (Higher than Price)",
			product: &product{
				ID:              99999, // ID 추가
				Name:            "Error Item",
				Price:           10000,
				DiscountedPrice: 11000, // Invalid
			},
			supportsHTML: false,
			wants: []string{
				"10,000원", // Should show original price only
			},
			unwants: []string{"11,000원"},
		},
		{
			name: "Zero Discount Price Handling",
			product: &product{
				ID:              88888, // ID 추가
				Name:            "Zero Discount Item",
				Price:           10000,
				DiscountedPrice: 0,
			},
			supportsHTML: false,
			wants: []string{
				"10,000원", // Should show original price only
			},
			unwants: []string{"⇒ 0원", "⇒"}, // "0원"은 "10,000원"에 포함되므로 오탐지 발생 가능. 구체화함.
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.product.Render(tt.supportsHTML, tt.mark, tt.prev)

			for _, want := range tt.wants {
				assert.Contains(t, got, want, "Result missing expected substring: %s", want)
			}
			for _, unwant := range tt.unwants {
				assert.NotContains(t, got, unwant, "Result contains unexpected substring: %s", unwant)
			}
		})
	}
}

// BenchmarkProduct_Render_Memory Render 함수의 메모리 할당 효율성을 검증합니다.
// Grow(512) 적용 후 할당 수(Allocs/op)가 최소화되었는지 확인합니다.
func BenchmarkProduct_Render_Memory(b *testing.B) {
	p := &product{
		ID:              123456,
		Name:            "[브랜드] 아주 긴 상품 이름을 가진 테스트용 상품입니다 (1kg)",
		Price:           125000,
		DiscountedPrice: 110000,
		DiscountRate:    15,
		LowestPrice:     105000,
		LowestPriceTime: time.Now(),
	}
	prev := &product{
		Price: 130000, // 이전 가격
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// strings.Builder.Grow 적용으로 인해 내부 재할당이 발생하지 않아야 함
		// 결과 문자열 생성 시의 1회 할당(String()) 외에 추가 할당이 없어야 이상적
		_ = p.Render(true, " 🔻", prev)
	}
}

// Example_render demonstrates usage of Render method.
func Example_render() {
	p := &product{
		ID:              12345,
		Name:            "Example Item",
		Price:           10000,
		DiscountedPrice: 9000,
		DiscountRate:    10,
	}

	// Render without previous price info (nil)
	fmt.Println(p.Render(false, "", nil))
	// Output:
	// ☞ Example Item
	//       • 현재 가격 : 10,000원 ⇒ 9,000원 (10%)
}
