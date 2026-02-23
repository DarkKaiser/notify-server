package navershopping

import (
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// formatProductItem / renderProduct 검증
// =============================================================================

// TestFormatProductItem_HTML HTML 모드의 출력 포맷을 검증합니다.
//
// HTML 모드에서는:
//   - 상품명이 <a href>...</a><b>...</b>로 하이퍼링크와 볼드 처리되어야 합니다.
//   - 링크가 별도 줄로 출력되지 않아야 합니다.
func TestFormatProductItem_HTML(t *testing.T) {
	t.Parallel()

	p := &product{
		Title:    "Apple iPad <Air>",
		Link:     "https://shopping.naver.com/products/1234",
		LowPrice: 850000,
		MallName: "A & B Shop",
	}

	got := formatProductItem(p, true, "", nil)

	// HTML 포맷 구조 검증
	assert.Contains(t, got, `<a href="https://shopping.naver.com/products/1234">`)
	assert.Contains(t, got, `<b>Apple iPad &lt;Air&gt;</b>`)
	assert.Contains(t, got, "(A &amp; B Shop)")
	assert.Contains(t, got, "850,000원")

	// 텍스트 모드 전용 요소는 없어야 함
	assert.NotContains(t, got, "\nhttps://", "HTML 모드에서는 링크를 별도 줄에 표시하지 않습니다")
}

// TestFormatProductItem_Text 텍스트 모드의 출력 포맷을 검증합니다.
//
// 텍스트 모드에서는:
//   - HTML 태그가 없어야 합니다.
//   - 상품명, 판매처, 가격이 한 줄로 출력되어야 합니다.
//   - 링크가 다음 줄에 별도로 표시되어야 합니다.
func TestFormatProductItem_Text(t *testing.T) {
	t.Parallel()

	p := &product{
		Title:    "Samsung Galaxy S24",
		Link:     "https://product.link/galaxy",
		LowPrice: 1200000,
		MallName: "Samsung Store",
	}

	got := formatProductItem(p, false, "", nil)

	assert.Contains(t, got, "Samsung Galaxy S24 (Samsung Store) 1,200,000원")
	// 텍스트 모드에서는 링크를 다음 줄에 별도로 표시
	assert.Contains(t, got, "\nhttps://product.link/galaxy")
	// HTML 태그는 없어야 함
	assert.NotContains(t, got, "<a href")
	assert.NotContains(t, got, "<b>")
}

// TestFormatProductItem_WithMark 마크(🆕, 🔄 등)가 올바르게 포함되는지 검증합니다.
func TestFormatProductItem_WithMark(t *testing.T) {
	t.Parallel()

	p := &product{Title: "Product", Link: "http://link", LowPrice: 10000, MallName: "Shop"}

	tests := []struct {
		name string
		m    mark.Mark
	}{
		{name: "New 마크", m: mark.New},
		{name: "Modified 마크", m: mark.Modified},
		{name: "마크 없음", m: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatProductItem(p, false, tt.m, nil)
			if tt.m != "" {
				assert.Contains(t, got, tt.m.WithSpace())
			}
		})
	}
}

// TestFormatProductItem_WithPrev 이전 가격이 제공된 경우 가격 변동 표시가 포함되는지 검증합니다.
func TestFormatProductItem_WithPrev(t *testing.T) {
	t.Parallel()

	curr := &product{Title: "Product", Link: "http://link", LowPrice: 8000, MallName: "Shop"}
	prevHigher := &product{LowPrice: 10000}
	prevLower := &product{LowPrice: 5000}
	prevSame := &product{LowPrice: 8000}

	t.Run("가격 하락 → 이전 가격 표시", func(t *testing.T) {
		t.Parallel()
		got := formatProductItem(curr, false, mark.Modified, prevHigher)
		assert.Contains(t, got, "(이전: 10,000원)")
		assert.Contains(t, got, "8,000원")
	})

	t.Run("가격 상승 → 이전 가격 표시", func(t *testing.T) {
		t.Parallel()
		got := formatProductItem(curr, false, mark.Modified, prevLower)
		assert.Contains(t, got, "(이전: 5,000원)")
	})

	t.Run("가격 동일 → 이전 가격 미표시", func(t *testing.T) {
		t.Parallel()
		got := formatProductItem(curr, false, "", prevSame)
		assert.NotContains(t, got, "(이전:", "동일한 가격은 이전 가격 표시를 하지 않습니다")
	})

	t.Run("prev=nil → 이전 가격 미표시", func(t *testing.T) {
		t.Parallel()
		got := formatProductItem(curr, false, "", nil)
		assert.NotContains(t, got, "(이전:")
	})
}

// TestRenderProduct renderProduct가 formatProductItem의 wrapper로 동일하게 동작하는지 검증합니다.
func TestRenderProduct(t *testing.T) {
	t.Parallel()

	p := &product{Title: "Test Product", Link: "http://link", LowPrice: 50000, MallName: "Mall"}

	// renderProduct는 formatProductItem(p, supportsHTML, m, nil)의 alias이므로 동일해야 함
	got := renderProduct(p, false, mark.New)
	want := formatProductItem(p, false, mark.New, nil)

	assert.Equal(t, want, got)
}

// TestFormatProductItem_PriceFormatComma 천 단위 콤마 포맷이 올바르게 적용되는지 검증합니다.
func TestFormatProductItem_PriceFormatComma(t *testing.T) {
	t.Parallel()

	tests := []struct {
		price     int
		wantPrice string
	}{
		{price: 1000, wantPrice: "1,000원"},
		{price: 1000000, wantPrice: "1,000,000원"},
		{price: 0, wantPrice: "0원"},
		{price: 999, wantPrice: "999원"},
	}

	for _, tt := range tests {
		t.Run(tt.wantPrice, func(t *testing.T) {
			t.Parallel()
			p := &product{Title: "P", Link: "http://l", LowPrice: tt.price, MallName: "M"}
			got := formatProductItem(p, false, "", nil)
			assert.Contains(t, got, tt.wantPrice)
		})
	}
}

// =============================================================================
// renderProductDiffs 검증
// =============================================================================

// TestRenderProductDiffs_Empty diffs가 비어있으면 빈 문자열을 반환합니다.
func TestRenderProductDiffs_Empty(t *testing.T) {
	t.Parallel()

	got := renderProductDiffs(nil, false)
	assert.Empty(t, got)

	got = renderProductDiffs([]productDiff{}, false)
	assert.Empty(t, got)
}

// TestRenderProductDiffs_NewProduct 신규 상품 diff가 올바르게 렌더링되는지 검증합니다.
func TestRenderProductDiffs_NewProduct(t *testing.T) {
	t.Parallel()

	p := &product{Title: "New Product", Link: "http://link/new", LowPrice: 20000, MallName: "Shop"}
	diffs := []productDiff{
		{Type: productEventNew, Product: p, Prev: nil},
	}

	got := renderProductDiffs(diffs, false)

	assert.Contains(t, got, "New Product")
	assert.Contains(t, got, mark.New.WithSpace())
	assert.NotContains(t, got, "(이전:")
}

// TestRenderProductDiffs_PriceChanged 가격 변동 diff가 이전 가격을 포함하여 렌더링되는지 검증합니다.
func TestRenderProductDiffs_PriceChanged(t *testing.T) {
	t.Parallel()

	curr := &product{Title: "Changed Product", Link: "http://link", LowPrice: 15000, MallName: "Shop"}
	prev := &product{LowPrice: 20000}
	diffs := []productDiff{
		{Type: productEventPriceChanged, Product: curr, Prev: prev},
	}

	got := renderProductDiffs(diffs, false)

	assert.Contains(t, got, "15,000원")
	assert.Contains(t, got, "(이전: 20,000원)")
	assert.Contains(t, got, mark.Modified.WithSpace())
}

// TestRenderProductDiffs_MultipleDiffs 여러 상품이 있을 때 구분자(빈 줄)로 분리되는지 검증합니다.
func TestRenderProductDiffs_MultipleDiffs(t *testing.T) {
	t.Parallel()

	p1 := &product{Title: "Alpha", Link: "http://link/1", LowPrice: 10000, MallName: "Shop"}
	p2 := &product{Title: "Bravo", Link: "http://link/2", LowPrice: 20000, MallName: "Shop"}
	diffs := []productDiff{
		{Type: productEventNew, Product: p1},
		{Type: productEventNew, Product: p2},
	}

	got := renderProductDiffs(diffs, false)

	// 두 상품 모두 포함되어야 함
	assert.Contains(t, got, "Alpha")
	assert.Contains(t, got, "Bravo")

	// 구분자(빈 줄 \n\n)가 삽입되어야 함
	assert.Contains(t, got, "\n\n", "여러 상품 간에는 빈 줄 구분자가 있어야 합니다")

	// Alpha가 Bravo보다 먼저 출력되어야 함
	assert.Less(t, strings.Index(got, "Alpha"), strings.Index(got, "Bravo"))
}

// TestRenderProductDiffs_SingleItem_NoSeparator 상품이 1개이면 구분자가 없어야 합니다.
func TestRenderProductDiffs_SingleItem_NoSeparator(t *testing.T) {
	t.Parallel()

	p := &product{Title: "Single", Link: "http://link", LowPrice: 10000, MallName: "Shop"}
	diffs := []productDiff{{Type: productEventNew, Product: p}}

	got := renderProductDiffs(diffs, false)

	// 항목이 1개이므로 앞에 \n\n이 있어서는 안 됨
	assert.False(t, strings.HasPrefix(got, "\n\n"), "단일 항목 앞에 구분자가 있어서는 안 됩니다")
}

// =============================================================================
// renderCurrentStatus 검증
// =============================================================================

// TestRenderCurrentStatus_NilSnapshot snapshot이 nil이면 빈 문자열을 반환합니다.
func TestRenderCurrentStatus_NilSnapshot(t *testing.T) {
	t.Parallel()

	got := renderCurrentStatus(nil, false)
	assert.Empty(t, got)
}

// TestRenderCurrentStatus_EmptyProducts 상품이 0건이면 빈 문자열을 반환합니다.
func TestRenderCurrentStatus_EmptyProducts(t *testing.T) {
	t.Parallel()

	snap := &watchPriceSnapshot{Products: []*product{}}
	got := renderCurrentStatus(snap, false)
	assert.Empty(t, got)
}

// TestRenderCurrentStatus_SingleProduct 상품 1개는 구분자 없이 렌더링됩니다.
func TestRenderCurrentStatus_SingleProduct(t *testing.T) {
	t.Parallel()

	p := &product{Title: "Lone Product", Link: "http://link", LowPrice: 5000, MallName: "Shop"}
	snap := &watchPriceSnapshot{Products: []*product{p}}

	got := renderCurrentStatus(snap, false)

	require.NotEmpty(t, got)
	assert.Contains(t, got, "Lone Product")
	assert.False(t, strings.HasPrefix(got, "\n\n"), "첫 줄 앞에 구분자가 있어서는 안 됩니다")
}

// TestRenderCurrentStatus_MultipleProducts 여러 상품이 구분자로 분리되는지 검증합니다.
func TestRenderCurrentStatus_MultipleProducts(t *testing.T) {
	t.Parallel()

	snap := &watchPriceSnapshot{
		Products: []*product{
			{Title: "First", Link: "http://link/1", LowPrice: 10000, MallName: "Shop"},
			{Title: "Second", Link: "http://link/2", LowPrice: 20000, MallName: "Shop"},
			{Title: "Third", Link: "http://link/3", LowPrice: 30000, MallName: "Shop"},
		},
	}

	got := renderCurrentStatus(snap, false)

	assert.Contains(t, got, "First")
	assert.Contains(t, got, "Second")
	assert.Contains(t, got, "Third")
	// 구분자(빈 줄)가 존재해야 함
	assert.Contains(t, got, "\n\n")
}

// TestRenderCurrentStatus_NoMarkDisplayed renderCurrentStatus는 마크 없이 렌더링됩니다.
func TestRenderCurrentStatus_NoMarkDisplayed(t *testing.T) {
	t.Parallel()

	p := &product{Title: "Product", Link: "http://link", LowPrice: 5000, MallName: "Shop"}
	snap := &watchPriceSnapshot{Products: []*product{p}}

	got := renderCurrentStatus(snap, false)

	// 현재 상태를 단순 나열하는 것이므로 🆕, 🔄 등의 마크가 없어야 함
	assert.NotContains(t, got, mark.New.String())
	assert.NotContains(t, got, mark.Modified.String())
}

// =============================================================================
// renderSearchConditionsSummary 검증
// =============================================================================

// TestRenderSearchConditionsSummary 조회 조건 요약 문자열이 올바르게 생성되는지 검증합니다.
func TestRenderSearchConditionsSummary(t *testing.T) {
	t.Parallel()

	t.Run("모든 조건이 설정된 경우", func(t *testing.T) {
		t.Parallel()

		s := NewSettingsBuilder().
			WithQuery("갤럭시 S24").
			WithIncludedKeywords("공식,정품").
			WithExcludedKeywords("중고,리퍼").
			WithPriceLessThan(1500000).
			Build()

		got := renderSearchConditionsSummary(&s, false)

		assert.Contains(t, got, "갤럭시 S24")
		assert.Contains(t, got, "공식,정품")
		assert.Contains(t, got, "중고,리퍼")
		assert.Contains(t, got, "1,500,000원 미만")
	})

	t.Run("선택 키워드가 없는 경우 (빈 문자열로 표시)", func(t *testing.T) {
		t.Parallel()

		s := NewSettingsBuilder().WithQuery("MacBook Air").WithPriceLessThan(2000000).Build()
		got := renderSearchConditionsSummary(&s, false)

		assert.Contains(t, got, "MacBook Air")
		assert.Contains(t, got, "2,000,000원 미만")
		// 키워드가 없으면 빈 칸으로 표시
		assert.Contains(t, got, "상품명 포함 키워드 : \n")
		assert.Contains(t, got, "상품명 제외 키워드 : \n")
	})

	t.Run("가격 천 단위 콤마 포맷 검증", func(t *testing.T) {
		t.Parallel()

		s := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(1000000).Build()
		got := renderSearchConditionsSummary(&s, false)

		assert.Contains(t, got, "1,000,000원 미만")
	})

	t.Run("HTML 이스케이프 검증", func(t *testing.T) {
		t.Parallel()

		s := NewSettingsBuilder().
			WithQuery("MacBook < 15\"").
			WithIncludedKeywords("M1 & M2").
			WithExcludedKeywords("Refurbished > 1Year").
			WithPriceLessThan(1500000).
			Build()

		got := renderSearchConditionsSummary(&s, true)

		assert.Contains(t, got, "MacBook &lt; 15&#34;")
		assert.Contains(t, got, "M1 &amp; M2")
		assert.Contains(t, got, "Refurbished &gt; 1Year")
	})
}

// =============================================================================
// 통합 검증: HTML vs Text 모드 비교
// =============================================================================

// TestHTMLvsText_FormatDiff HTML 모드와 Text 모드의 출력이 다른지 검증합니다.
func TestHTMLvsText_FormatDiff(t *testing.T) {
	t.Parallel()

	p := &product{Title: "Product", Link: "http://link", LowPrice: 10000, MallName: "Shop"}

	htmlOutput := formatProductItem(p, true, "", nil)
	textOutput := formatProductItem(p, false, "", nil)

	assert.NotEqual(t, htmlOutput, textOutput)
	assert.Contains(t, htmlOutput, "<a href")
	assert.NotContains(t, textOutput, "<a href")
}

// =============================================================================
// 보안 및 예외(Edge Case) 방어 검증
// =============================================================================

// TestFormatProductItem_XSS_Escape 악의적인 HTML 태그(XSS)가 입력으로 들어왔을 때
// 안전하게 Escape 처리되어 렌더러가 무너지지 않는지(Security) 검증합니다.
func TestFormatProductItem_XSS_Escape(t *testing.T) {
	t.Parallel()

	// 악의적인 스크립트 및 태그가 포함된 상품 정보 시뮬레이션
	maliciousProduct := &product{
		Title:    `<script>alert("XSS")</script> Bad Product`,
		Link:     `https://example.com/item?q=<iframe src="javascript:alert(1)">`,
		LowPrice: 1000,
		MallName: `Store <img src=x onerror=alert("XSS")>`,
	}

	// HTML 렌더링 모드일 때만 Escape가 발생하므로 true로 테스트
	got := formatProductItem(maliciousProduct, true, "", nil)

	// 공격용 태그 문자(<, >)가 안전한 HTML Entity(&lt;, &gt;)로 변환되었는지 단언(Assert)
	assert.Contains(t, got, "&lt;script&gt;alert(&#34;XSS&#34;)&lt;/script&gt;")
	assert.Contains(t, got, "&lt;iframe src=&#34;javascript:alert(1)&#34;&gt;")
	assert.Contains(t, got, "&lt;img src=x onerror=alert(&#34;XSS&#34;)&gt;")

	// 원본 악성 태그가 그대로 노출되면 안 됨
	assert.NotContains(t, got, "<script>")
	assert.NotContains(t, got, "<iframe>")
	assert.NotContains(t, got, "<img")
}

// TestRenderSearchConditionsSummary_NilPointer 예외적인 빈 값(nil 또는 초기화 안 됨)이
// 전달되었을 때 렌더러가 패닉(Panic) 없이 안전하게 동작하는지(Robustness) 검증합니다.
func TestRenderSearchConditionsSummary_NilPointer(t *testing.T) {
	t.Parallel()

	// 1. Settings 값의 필터 필드가 초기화되지 않은 극한 상황 시뮬레이션
	// (기본적으로 NewSettingsBuilder를 쓰면 안전하지만 외부 요인 배제 불가)
	emptySettings := &watchPriceSettings{
		Query: "Empty Query",
		// Filters: 익명 구조체이므로 명시적으로 초기화하지 않으면 0값(zero-value) 할당됨
	}

	gotText := renderSearchConditionsSummary(emptySettings, false)
	assert.Contains(t, gotText, "Empty Query")
	assert.Contains(t, gotText, "0원 미만") // 0값은 0원으로 정상 렌더링

	gotHTML := renderSearchConditionsSummary(emptySettings, true)
	assert.Contains(t, gotHTML, "Empty Query")
	assert.Contains(t, gotHTML, "0원 미만")
}
