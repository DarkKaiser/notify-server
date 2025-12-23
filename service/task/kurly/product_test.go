package kurly

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProduct_URL(t *testing.T) {
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

func TestProduct_IsOnSale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		price           int
		discountedPrice int
		want            bool
	}{
		{
			name:            "Not on sale: No discount price",
			price:           10000,
			discountedPrice: 0,
			want:            false,
		},
		{
			name:            "On sale: Discounted price lower than price",
			price:           10000,
			discountedPrice: 9000,
			want:            true,
		},
		{
			name:            "Not on sale: Discounted price equals price",
			price:           10000,
			discountedPrice: 10000,
			want:            false,
		},
		{
			name:            "Not on sale: Discounted price higher than price (Error case)",
			price:           10000,
			discountedPrice: 11000,
			want:            false,
		},
		{
			name:            "Not on sale: Negative discounted price (Error case)",
			price:           10000,
			discountedPrice: -100,
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

func TestProduct_UpdateLowestPrice(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name              string
		initialProduct    *product
		wantLowestPrice   int
		wantTimeCheck     bool // 최저가 갱신 시간 업데이트 여부 확인
		timeShouldBeAfter time.Time
	}{
		{
			name: "초기 상태: 최저가가 0일 때 현재 가격으로 설정",
			initialProduct: &product{
				Price: 10000,
			},
			wantLowestPrice: 10000,
			wantTimeCheck:   true,
		},
		{
			name: "초기 상태: 최저가가 0일 때 할인 가격 우선 설정",
			initialProduct: &product{
				Price:           10000,
				DiscountedPrice: 8000,
			},
			wantLowestPrice: 8000,
			wantTimeCheck:   true,
		},
		{
			name: "갱신: 기존 최저가보다 낮은 가격 발생",
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
			name: "유지: 기존 최저가보다 높은 가격",
			initialProduct: &product{
				Price:           12000,
				LowestPrice:     10000,
				LowestPriceTime: now,
			},
			wantLowestPrice:   10000,
			wantTimeCheck:     false, // 시간 업데이트 안 됨
			timeShouldBeAfter: now,   // 시간은 그대로 now여야 함
		},
		{
			name: "갱신: 할인 가격이 최저가보다 낮음",
			initialProduct: &product{
				Price:           12000,
				DiscountedPrice: 9000,
				LowestPrice:     10000,
			},
			wantLowestPrice: 9000,
			wantTimeCheck:   true,
		},
		{
			name: "엣지 케이스: 가격이 0원인 경우 (오류 상황)",
			initialProduct: &product{
				Price:           0,
				DiscountedPrice: 0,
				LowestPrice:     0,
			},
			wantLowestPrice: 0, // 0원은 무시 (로직상 0 < 0 은 false, 0 == 0 일때도 무시)
			wantTimeCheck:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			p := tt.initialProduct
			startTime := time.Now()

			// Execute
			p.updateLowestPrice()

			// Verify
			assert.Equal(t, tt.wantLowestPrice, p.LowestPrice)

			if tt.wantTimeCheck {
				// 시간이 갱신되었어야 함 (startTime 이후)
				assert.True(t, p.LowestPriceTime.After(startTime.Add(-time.Second)), "최저가 갱신 시간이 업데이트 되어야 합니다")
			} else if !tt.timeShouldBeAfter.IsZero() {
				// 시간이 변경되지 않았어야 함
				assert.Equal(t, tt.timeShouldBeAfter, p.LowestPriceTime, "최저가 갱신 시간이 변경되지 않아야 합니다")
			}
		})
	}
}

func TestProduct_Render(t *testing.T) {
	t.Parallel()

	baseProduct := &product{
		ID:    12345,
		Name:  "맛있는 사과",
		Price: 10000,
	}
	discountProduct := &product{
		ID:              12345,
		Name:            "할인 사과",
		Price:           10000,
		DiscountedPrice: 8000,
		DiscountRate:    20,
	}

	expectedIDString := "12345" // For URL check

	tests := []struct {
		name         string
		product      *product
		supportsHTML bool
		mark         string
		old          *product // Renamed from prevProduct to match Render signature
		wantContains []string
		wantNot      []string
	}{
		{
			name:         "HTML: 일반 상품",
			product:      baseProduct,
			supportsHTML: true,
			wantContains: []string{
				"맛있는 사과",
				"10,000원",
				fmt.Sprintf("https://www.kurly.com/goods/%v", expectedIDString), // URL 포맷 사용 검증
				"<b>", "</b>", "<a href=", // HTML 태그 확인
			},
		},
		{
			name:         "Text: 일반 상품",
			product:      baseProduct,
			supportsHTML: false,
			wantContains: []string{
				"맛있는 사과",
				"10,000원",
				"☞", // Prefix 확인
			},
			wantNot: []string{"<a href=", "<b>", "</b>"},
		},
		{
			name:         "HTML: 할인 상품 (with Old Price comparison)",
			product:      discountProduct,
			supportsHTML: true,
			old: &product{
				Price: 10000, // 이전 가격은 정가 동일
			},
			wantContains: []string{
				"<s>10,000원</s>", // 취소선
				"8,000원",         // 할인가
				"(20%)",          // 할인율
				"이전 가격 : 10,000원",
			},
		},
		{
			name:         "Text: 할인 상품",
			product:      discountProduct,
			supportsHTML: false,
			wantContains: []string{
				"10,000원 ⇒ 8,000원 (20%)", // 텍스트 포맷
			},
			wantNot: []string{"<s>", "</s>"},
		},
		{
			name: "Text: 할인율 0% (숨김 처리 확인)",
			product: &product{
				ID:              12345,
				Name:            "0퍼 할인 사과",
				Price:           10000,
				DiscountedPrice: 9900, // 100원 할인되었으나
				DiscountRate:    0,    // 비율이 0인 경우
			},
			supportsHTML: false,
			wantContains: []string{
				"10,000원 ⇒ 9,900원", // 비율 표기 없음
			},
			wantNot: []string{"(0%)", "(%)"},
		},
		{
			name: "HTML: 할인율 0% (숨김 처리 확인)",
			product: &product{
				ID:              12345,
				Name:            "0퍼 할인 사과",
				Price:           10000,
				DiscountedPrice: 9900,
				DiscountRate:    0,
			},
			supportsHTML: true,
			wantContains: []string{
				"<s>10,000원</s> 9,900원", // 비율 표기 없음
			},
			wantNot: []string{"(0%)", "(%)"},
		},
		{
			name: "방어적 로직: 할인가가 정가보다 비쌈 (할인 무시)",
			product: &product{
				ID:              111,
				Name:            "이상한 사과",
				Price:           10000,
				DiscountedPrice: 20000, // Error Data
				DiscountRate:    50,
			},
			supportsHTML: false,
			wantContains: []string{
				"10,000원", // 정가만 표시
			},
			wantNot: []string{"20,000원", "50%", "=>", "⇒"},
		},
		{
			name:         "Text: 마크(Mark) 포함",
			product:      baseProduct,
			supportsHTML: false,
			mark:         " 🆕",
			wantContains: []string{"맛있는 사과 🆕"},
		},
		{
			name:         "Text: 이전 가격 비교 (old product exists)",
			product:      baseProduct,
			supportsHTML: false,
			old: &product{
				Price: 12000,
			},
			wantContains: []string{
				"이전 가격 : 12,000원",
			},
		},
		{
			name:         "XSS 방지: 특수문자 이스케이프 확인",
			product:      &product{ID: 1, Name: "<script>alert(1)</script>", Price: 1000},
			supportsHTML: true,
			wantContains: []string{"&lt;script&gt;alert(1)&lt;/script&gt;"},
			wantNot:      []string{"<script>"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Updated to use 'old' field name from struct
			got := tt.product.Render(tt.supportsHTML, tt.mark, tt.old)

			for _, s := range tt.wantContains {
				assert.Contains(t, got, s)
			}
			for _, s := range tt.wantNot {
				assert.NotContains(t, got, s)
			}
		})
	}
}

// Example_render renders the product status message.
// This example demonstrates how to generate a notification message for a product.
func Example_render() {
	p := &product{
		ID:              12345,
		Name:            "Fresh Apple",
		Price:           10000,
		DiscountedPrice: 9000,
		DiscountRate:    10,
	}

	// Render for Text-based clients (e.g., Log, Simple Terminal)
	// Using 'old' as nil implies no previous price comparison.
	msg := p.Render(false, " [Sale]", nil)
	fmt.Println(msg)

	// Output:
	// ☞ Fresh Apple [Sale]
	//       • 현재 가격 : 10,000원 ⇒ 9,000원 (10%)
}
