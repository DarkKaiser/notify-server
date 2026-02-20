package navershopping

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/stretchr/testify/assert"
)

// TestProduct_key는 ProductID가 Key로 올바르게 사용되는지 검증합니다.
func TestProduct_key(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		productID string
		want      string
	}{
		{"Normal ID", "1234567890", "1234567890"},
		{"Empty ID", "", ""},
		{"Alphanumeric ID", "prod-123-abc", "prod-123-abc"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &product{ProductID: tt.productID}
			assert.Equal(t, tt.want, p.key())
		})
	}
}

// TestRenderProduct 단일 상품 표시 렌더링 동작을 다양한 시나리오에서 검증합니다.
func TestRenderProduct(t *testing.T) {
	t.Parallel()

	baseProduct := &product{
		Title:     "Apple iPad Air 5th Gen",
		Link:      "https://shopping.naver.com/products/1234567890",
		LowPrice:  850000,
		MallName:  "Apple Official",
		ProductID: "1234567890",
	}

	tests := []struct {
		name         string
		product      *product
		supportsHTML bool
		mark         mark.Mark
		wants        []string // 결과 문자열에 반드시 포함되어야 할 부분 문자열
		unwants      []string // 결과 문자열에 포함되지 말아야 할 부분 문자열
	}{
		{
			name:         "HTML Format - Basic",
			product:      baseProduct,
			supportsHTML: true,
			mark:         "",
			wants: []string{
				`<a href="https://shopping.naver.com/products/1234567890"><b>Apple iPad Air 5th Gen</b></a>`,
				"(Apple Official)",
				"850,000원",
			},
			unwants: []string{"🆕", "☞ Apple iPad Air"}, // Text format elements
		},
		{
			name:         "HTML Format - With New Mark",
			product:      baseProduct,
			supportsHTML: true,
			mark:         mark.New,
			wants:        []string{"850,000원 🆕"},
		},
		{
			name:         "Text Format - Basic",
			product:      baseProduct,
			supportsHTML: false,
			mark:         "",
			wants: []string{
				"☞ Apple iPad Air 5th Gen (Apple Official) 850,000원",
				"https://shopping.naver.com/products/1234567890",
			},
			unwants: []string{"<a href", "<b>", "</b>"},
		},
		{
			name:         "Text Format - With New Mark",
			product:      baseProduct,
			supportsHTML: false,
			mark:         mark.New,
			wants:        []string{"850,000원 🆕"},
		},
		{
			name: "Zero Price Handling",
			product: &product{
				Title:    "Free Sample",
				LowPrice: 0,
				MallName: "Promo",
				Link:     "http://example.com",
			},
			supportsHTML: false,
			mark:         "",
			wants:        []string{"0원"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := renderProduct(tt.product, tt.supportsHTML, tt.mark)
			for _, want := range tt.wants {
				assert.Contains(t, got, want, "Expected substring missing")
			}
			for _, unwant := range tt.unwants {
				assert.NotContains(t, got, unwant, "Unexpected substring found")
			}
		})
	}
}

// TestRenderProductDiff 변경 사항 비교 표시 렌더링 동작을 검증합니다.
func TestRenderProductDiff(t *testing.T) {
	t.Parallel()

	current := &product{
		Title:     "Galaxy S24",
		Link:      "http://link",
		LowPrice:  1000000,
		MallName:  "Samsung",
		ProductID: "1",
	}

	tests := []struct {
		name         string
		product      *product
		prev         *product
		supportsHTML bool
		mark         mark.Mark
		wants        []string
	}{
		{
			name:         "Price Drop (Text)",
			product:      current,
			prev:         &product{LowPrice: 1100000}, // 110만원 -> 100만원
			supportsHTML: false,
			mark:         mark.Mark("🔻"),
			wants: []string{
				"1,000,000원",
				"(이전: 1,100,000원)",
				"🔻",
			},
		},
		{
			name:         "Price Increase (HTML)",
			product:      current,
			prev:         &product{LowPrice: 900000}, // 90만원 -> 100만원
			supportsHTML: true,
			mark:         mark.Mark("🔺"),
			wants: []string{
				"1,000,000원",
				"(이전: 900,000원)",
				"🔺",
				"<b>Galaxy S24</b>",
			},
		},
		{
			name:         "Same Price (No diff text shown)",
			product:      current,
			prev:         &product{LowPrice: 1000000},
			supportsHTML: false,
			mark:         "",
			wants: []string{
				"1,000,000원",
			},
			// Same price should NOT show "(이전: ...)"
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := renderProductDiff(tt.product, tt.supportsHTML, tt.mark, tt.prev)
			for _, want := range tt.wants {
				assert.Contains(t, got, want)
			}
			// 동일 가격일 경우 "이전:" 텍스트가 없어야 함을 검증
			if tt.prev.LowPrice == tt.product.LowPrice {
				assert.NotContains(t, got, "(이전:")
			}
		})
	}
}

// BenchmarkRenderProduct 단일 상품 렌더링 성능 측정
func BenchmarkRenderProduct(b *testing.B) {
	p := &product{
		Title:    "Benchmark Product",
		LowPrice: 1234567,
		MallName: "Benchmark Mall",
		Link:     "http://example.com",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderProduct(p, false, "")
	}
}

// BenchmarkRenderProductDiff 변경 사항 렌더링 성능 측정
func BenchmarkRenderProductDiff(b *testing.B) {
	p := &product{
		Title:    "Benchmark Product",
		LowPrice: 1000000,
		MallName: "Benchmark Mall",
		Link:     "http://example.com",
	}
	prev := &product{LowPrice: 1100000}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderProductDiff(p, false, mark.Modified, prev)
	}
}
