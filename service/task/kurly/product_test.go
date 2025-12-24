package kurly

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestFormatProductURL_TableDriven formatProductURL 함수의 다양한 입력 타입 처리를 검증합니다.
// int, string 등 다양한 타입의 ID가 올바른 URL로 변환되는지 테스트합니다.
func TestFormatProductURL_TableDriven(t *testing.T) {
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
			got := formatProductURL(tt.id)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestProduct_URL_Integration product.URL 메서드가 formatProductURL을 올바르게 사용하는지 검증합니다.
func TestProduct_URL_Integration(t *testing.T) {
	t.Parallel()

	p := &product{ID: 99999}
	want := "https://www.kurly.com/goods/99999"

	// product.URL()은 내부적으로 formatProductURL을 호출해야 함
	assert.Equal(t, want, p.URL(), "product.URL() should delegate to formatProductURL correctly")
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
			product:      &product{Name: "Item", Price: 5000, LowestPrice: 4000, LowestPriceTimeUTC: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)},
			supportsHTML: false,
			wants: []string{
				"• 최저 가격 : 4,000원 (2023/01/01 21:00)",
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
		{
			name:         "Text Mode - No Escape Special Chars",
			product:      &product{Name: "특수문자 & 이름 > 테스트"},
			supportsHTML: false,
			wants: []string{
				"☞ 특수문자 & 이름 > 테스트", // Text 모드에서는 이스케이프 없이 그대로 출력, 하지만 KST변환 시간로직등은 영향받으므로 Render 로직 잘타는지 확인
			},
			unwants: []string{"&amp;", "&gt;"},
		},
		{
			name: "UTC to KST Conversion",
			product: &product{
				Name:        "Time Test",
				Price:       10000,
				LowestPrice: 9000,
				// UTC 00:00 -> KST 09:00
				LowestPriceTimeUTC: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			supportsHTML: false,
			wants: []string{
				"(2023/01/01 09:00)", // 00:00 UTC + 9h = 09:00 KST
			},
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
		ID:                 123456,
		Name:               "[브랜드] 아주 긴 상품 이름을 가진 테스트용 상품입니다 (1kg)",
		Price:              125000,
		DiscountedPrice:    110000,
		DiscountRate:       15,
		LowestPrice:        105000,
		LowestPriceTimeUTC: time.Now(),
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
