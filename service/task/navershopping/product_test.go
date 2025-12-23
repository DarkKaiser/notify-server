package navershopping

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProduct_Key는 ProductID가 Key로 올바르게 사용되는지 검증합니다.
func TestProduct_Key(t *testing.T) {
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
			assert.Equal(t, tt.want, p.Key())
		})
	}
}

// TestProduct_Render_TableDriven 다양한 시나리오에 대한 Render 메서드의 동작을 검증합니다.
func TestProduct_Render_TableDriven(t *testing.T) {
	t.Parallel()

	// 테스트 데이터 셋업
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
		mark         string
		wants        []string // 결과 문자열에 반드시 포함되어야 할 부분 문자열들
		unwants      []string // 결과 문자열에 포함되지 말아야 할 부분 문자열들
	}{
		{
			name:         "HTML Fomat - Basic",
			product:      baseProduct,
			supportsHTML: true,
			mark:         "",
			wants: []string{
				`<a href="https://shopping.naver.com/products/1234567890"><b>Apple iPad Air 5th Gen</b></a>`,
				"(Apple Official)",
				"850,000원",
			},
			unwants: []string{"🆕", "☞ Apple iPad Air"}, // Text format check
		},
		{
			name:         "HTML Format - With New Mark",
			product:      baseProduct,
			supportsHTML: true,
			mark:         " 🆕",
			wants: []string{
				"850,000원 🆕",
			},
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
			mark:         " 🔻",
			wants: []string{
				"850,000원 🔻",
			},
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
		{
			// 아주 큰 가격에 대해서도 쉼표 포맷팅이 잘 되는지 확인 (Test logic for strutil via product)
			name: "High Price Formatting",
			product: &product{
				Title:    "Luxury Car",
				LowPrice: 150000000, // 1.5억
				MallName: "Auto",
				Link:     "http://example.com/car",
			},
			supportsHTML: false,
			mark:         "",
			wants:        []string{"150,000,000원"},
		},
		{
			// MallName이 비어있는 경우 (Edge case)
			name: "Empty Mall Name",
			product: &product{
				Title:    "Item",
				LowPrice: 1000,
				MallName: "", // Empty
				Link:     "http://link",
			},
			supportsHTML: false,
			mark:         "",
			wants:        []string{"Item () 1,000원"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.product.Render(tt.supportsHTML, tt.mark)
			for _, want := range tt.wants {
				assert.Contains(t, got, want, "Result should contain expected substring")
			}
			for _, unwant := range tt.unwants {
				assert.NotContains(t, got, unwant, "Result should NOT contain unexpected substring")
			}
		})
	}
}

// TestProduct_Scenario_Example Render 메서드의 전체적인 사용 시나리오를 검증합니다.
func TestProduct_Scenario_Example(t *testing.T) {
	t.Parallel()

	p := &product{
		Title:    "Example Product",
		LowPrice: 50000,
		MallName: "MyStore",
		Link:     "http://example.com/prod/1",
	}

	t.Run("Text Mode", func(t *testing.T) {
		got := p.Render(false, "")
		want := `☞ Example Product (MyStore) 50,000원
http://example.com/prod/1`
		assert.Equal(t, want, got)
	})

	t.Run("Text Mode With Mark", func(t *testing.T) {
		got := p.Render(false, " NEW")
		want := `☞ Example Product (MyStore) 50,000원 NEW
http://example.com/prod/1`
		assert.Equal(t, want, got)
	})

	t.Run("HTML Mode", func(t *testing.T) {
		got := p.Render(true, "")
		want := `☞ <a href="http://example.com/prod/1"><b>Example Product</b></a> (MyStore) 50,000원`
		assert.Equal(t, want, got)
	})
}

// BenchmarkProduct_Render_Text Text 모드에서의 Render 성능을 측정합니다.
func BenchmarkProduct_Render_Text(b *testing.B) {
	p := &product{
		Title:    "Benchmark Product Name is Quite Long To Simulate Real World Scenario",
		LowPrice: 1234567,
		MallName: "Benchmarks R Us",
		Link:     "https://shopping.naver.com/products/1234567890/very/long/url/path/to/simulate/reality",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Render(false, " MARK")
	}
}

// BenchmarkProduct_Render_HTML HTML 모드에서의 Render 성능을 측정합니다.
func BenchmarkProduct_Render_HTML(b *testing.B) {
	p := &product{
		Title:    "Benchmark Product Name is Quite Long To Simulate Real World Scenario",
		LowPrice: 1234567,
		MallName: "Benchmarks R Us",
		Link:     "https://shopping.naver.com/products/1234567890/very/long/url/path/to/simulate/reality",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Render(true, " MARK")
	}
}
