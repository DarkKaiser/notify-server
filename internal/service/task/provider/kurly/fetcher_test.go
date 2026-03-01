package kurly

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 공통 픽스처 / 헬퍼
// =============================================================================

// htmlNormalPage 정상적인 마켓컬리 상품 상세 페이지의 기본 템플릿입니다.
// %d = 상품 ID, %s = 상품명, %s = H2 내부 가격 HTML
const htmlNormalPage = `
<html>
<body>
<script id="__NEXT_DATA__">{"props":{"pageProps":{"product": {"no": %d}}}}</script>
<div id="product-atf">
	<section class="css-1ua1wyk">
		<div class="css-84rb3h"><div class="css-6zfm8o"><div class="css-o3fjh7"><h1>%s</h1></div></div></div>
		<h2 class="css-xrp7wx">%s</h2>
	</section>
</div>
</body>
</html>`

// priceHTML 할인 없는 경우의 가격 HTML 픽스처를 생성합니다.
func priceHTML(price string) string {
	return fmt.Sprintf(`<div class="css-o2nlqt"><span>%s</span><span>원</span></div>`, price)
}

// discountPriceHTML 할인 중인 경우의 가격 HTML 픽스처를 생성합니다.
func discountPriceHTML(rate, salePrice, originalPrice string) string {
	return fmt.Sprintf(
		`<span class="css-8h3us8">%s</span>`+
			`<div class="css-o2nlqt"><span>%s</span><span>원</span></div>`+
			`<span class="css-1s96j0s"><span>%s원</span></span>`,
		rate, salePrice, originalPrice,
	)
}

// newTestTask fetchProduct 테스트를 위한 최소 task를 생성하는 헬퍼입니다.
func newTestTask(fetcher *mocks.MockFetcher) *task {
	return &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID:     "T",
				CommandID:  "C",
				NotifierID: "N",
				RunBy:      contract.TaskRunByScheduler,
			},
			InstanceID: "I",
			Fetcher:    fetcher,
			NewSnapshot: func() interface{} {
				return &watchProductPriceSnapshot{}
			},
		}, true),
	}
}

// =============================================================================
// TestTask_FetchProductInfo
// =============================================================================

// TestTask_FetchProductInfo fetchProduct 메서드의 전반적인 이중 추출 전략(JSON+DOM)을 검증합니다.
func TestTask_FetchProductInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		productID      int
		mockHTML       string
		mockFetchErr   error
		mockStatusCode int
		wantProduct    *product
		wantErr        bool
		errSubstr      string
	}{
		// ── 정상 수집 ─────────────────────────────────────────────────────────
		{
			name:           "성공: 정상 상품 파싱 (할인 없음)",
			productID:      123,
			mockStatusCode: http.StatusOK,
			mockHTML:       fmt.Sprintf(htmlNormalPage, 123, "맛있는 사과", priceHTML("10,000")),
			wantProduct: &product{
				ID:    123,
				Name:  "맛있는 사과",
				Price: 10000,
			},
		},
		{
			name:           "성공: 정상 상품 파싱 (할인 중)",
			productID:      456,
			mockStatusCode: http.StatusOK,
			mockHTML:       fmt.Sprintf(htmlNormalPage, 456, "할인 바나나", discountPriceHTML("10%", "9,000", "10,000")),
			wantProduct: &product{
				ID:              456,
				Name:            "할인 바나나",
				Price:           10000,
				DiscountedPrice: 9000,
				DiscountRate:    10,
			},
		},
		{
			name:           "성공: 판매 중지 상품 — product 키 null → IsUnavailable=true",
			productID:      101,
			mockStatusCode: http.StatusOK,
			mockHTML:       `<html><body><script id="__NEXT_DATA__">{"props":{"pageProps":{"product":null}}}</script></body></html>`,
			wantProduct:    &product{ID: 101, IsUnavailable: true},
		},
		{
			name:           "성공: 판매 중지 상품 — product 키 자체 없음 → IsUnavailable=true",
			productID:      102,
			mockStatusCode: http.StatusOK,
			// product 키 생략 케이스 (Exists() false)
			mockHTML:    `<html><body><script id="__NEXT_DATA__">{"props":{"pageProps":{}}}</script></body></html>`,
			wantProduct: &product{ID: 102, IsUnavailable: true},
		},
		// ── HTTP / 파싱 레벨 에러 ──────────────────────────────────────────────
		{
			name:         "실패: Fetch 에러 (네트워크 타임아웃)",
			productID:    999,
			mockFetchErr: errors.New("network timeout"),
			wantErr:      true,
			errSubstr:    "network timeout",
		},
		{
			name:           "실패: __NEXT_DATA__ 태그 없음 → ErrNextDataNotFound",
			productID:      100,
			mockStatusCode: http.StatusOK,
			mockHTML:       `<html><body>Nothing here</body></html>`,
			wantErr:        true,
			errSubstr:      "__NEXT_DATA__ JSON 태그를 찾을 수 없습니다",
		},
		// ── JSON 구조 결함 에러 ───────────────────────────────────────────────
		{
			name:           "실패: props.pageProps 노드 없음 → 구조 결함 에러 (스팸 방지 핵심 케이스)",
			productID:      103,
			mockStatusCode: http.StatusOK,
			// pageProps 자체가 없으면 단종이 아니라 페이지 구조 변경으로 판단해야 합니다.
			// 이 경우 조용히 IsUnavailable로 처리하면 대량 잘못된 알림이 발생합니다.
			mockHTML:  `<html><body><script id="__NEXT_DATA__">{"props":{}}</script></body></html>`,
			wantErr:   true,
			errSubstr: "props.pageProps",
		},
		// ── DOM 구조 에러 ─────────────────────────────────────────────────────
		{
			name:           "실패: 상품 정보 섹션 없음 → 레이아웃 변경 감지",
			productID:      104,
			mockStatusCode: http.StatusOK,
			mockHTML:       `<html><body><script id="__NEXT_DATA__">{"props":{"pageProps":{"product":{}}}}</script><div>Changed Layout</div></body></html>`,
			wantErr:        true,
			errSubstr:      "상품 정보 섹션(#product-atf",
		},
		{
			name:           "실패: 상품명 셀렉터 없음 → 이름 추출 실패",
			productID:      105,
			mockStatusCode: http.StatusOK,
			// 섹션은 있지만 h1 없음
			mockHTML: `<html><body>` +
				`<script id="__NEXT_DATA__">{"props":{"pageProps":{"product":{}}}}</script>` +
				`<div id="product-atf"><section class="css-1ua1wyk"><div class="css-84rb3h"><div class="css-6zfm8o"><div class="css-o3fjh7"></div></div></div></section></div>` +
				`</body></html>`,
			wantErr:   true,
			errSubstr: "상품 이름 요소",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFetcher := new(mocks.MockFetcher)
			url := productPageURL(tt.productID)

			if tt.mockFetchErr != nil {
				mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == url
				})).Return(nil, tt.mockFetchErr)
			} else {
				mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == url
				})).Return(mocks.NewMockResponse(tt.mockHTML, tt.mockStatusCode), nil)
			}

			tsk := newTestTask(mockFetcher)
			got, err := tsk.fetchProduct(context.Background(), tt.productID)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tt.wantProduct.ID, got.ID)
				assert.Equal(t, tt.wantProduct.IsUnavailable, got.IsUnavailable)
				if !got.IsUnavailable {
					assert.Equal(t, tt.wantProduct.Name, got.Name)
					assert.Equal(t, tt.wantProduct.Price, got.Price)
					assert.Equal(t, tt.wantProduct.DiscountedPrice, got.DiscountedPrice)
					assert.Equal(t, tt.wantProduct.DiscountRate, got.DiscountRate)
				}
			}
			mockFetcher.AssertExpectations(t)
		})
	}
}

// =============================================================================
// TestExtractPriceDetails
// =============================================================================

// htmlProductSection extractPriceDetails 테스트용 가격 영역 HTML을 생성하는 헬퍼입니다.
func htmlProductSection(priceAreaHTML string) string {
	return fmt.Sprintf(`
<div id="product-atf">
	<section class="css-1ua1wyk">
		<div class="css-84rb3h"><div class="css-6zfm8o"><div class="css-o3fjh7"><h1>상품명</h1></div></div></div>
		<h2 class="css-xrp7wx">%s</h2>
	</section>
</div>`, priceAreaHTML)
}

// TestExtractPriceDetails HTML DOM에서 상품 가격 정보를 올바르게 추출하는지 검증합니다.
// 할인 미적용/적용/예외 세 가지 분기를 모두 커버합니다.
func TestExtractPriceDetails(t *testing.T) {
	t.Parallel()

	const targetURL = "https://www.kurly.com/goods/test"

	tests := []struct {
		name                string
		html                string
		wantPrice           int
		wantDiscountedPrice int
		wantDiscountRate    int
		wantErr             bool
		errSubstr           string
	}{
		// ── 할인 미적용 ───────────────────────────────────────────────────────
		{
			name:                "성공: 할인 없음 — 정가만 출력",
			html:                htmlProductSection(priceHTML("10,000")),
			wantPrice:           10000,
			wantDiscountedPrice: 0,
			wantDiscountRate:    0,
		},
		{
			name:                "성공: 할인 없음 — 천단위 쉼표 포함 큰 금액",
			html:                htmlProductSection(priceHTML("1,234,500")),
			wantPrice:           1234500,
			wantDiscountedPrice: 0,
			wantDiscountRate:    0,
		},
		{
			name:      "실패: 할인 없음 — price span 개수 부족",
			html:      htmlProductSection(`<div class="css-o2nlqt"><span>10,000</span></div>`),
			wantErr:   true,
			errSubstr: "상품 가격 요소",
		},
		{
			name:      "실패: 할인 없음 — price span 가격이 숫자가 아님",
			html:      htmlProductSection(`<div class="css-o2nlqt"><span>N/A</span><span>원</span></div>`),
			wantErr:   true,
			errSubstr: "정가 텍스트",
		},
		// ── 할인 적용 중 ──────────────────────────────────────────────────────
		{
			name:                "성공: 할인 중 — 정가·할인가·할인율 모두 추출",
			html:                htmlProductSection(discountPriceHTML("10%", "9,000", "10,000")),
			wantPrice:           10000,
			wantDiscountedPrice: 9000,
			wantDiscountRate:    10,
		},
		{
			name:                "성공: 할인 중 — 높은 할인율",
			html:                htmlProductSection(discountPriceHTML("50%", "5,000", "10,000")),
			wantPrice:           10000,
			wantDiscountedPrice: 5000,
			wantDiscountRate:    50,
		},
		{
			name:      "실패: 할인 중 — 할인율이 숫자가 아님",
			html:      htmlProductSection(`<span class="css-8h3us8">N/A%</span><div class="css-o2nlqt"><span>9,000</span><span>원</span></div><span class="css-1s96j0s"><span>10,000원</span></span>`),
			wantErr:   true,
			errSubstr: "할인율 텍스트",
		},
		{
			name:      "실패: 할인 중 — discountedPrice span 개수 부족",
			html:      htmlProductSection(`<span class="css-8h3us8">10%</span><div class="css-o2nlqt"><span>9,000</span></div><span class="css-1s96j0s"><span>10,000원</span></span>`),
			wantErr:   true,
			errSubstr: "상품 가격 요소",
		},
		{
			// 정가(취소선) 파싱 실패 시 → discountedPrice 동일하게 보정되고 에러 없이 반환
			name:                "성공: 할인 중 — 정가(취소선) 파싱 실패 → 자동 보정 (price=discountedPrice, rate=0)",
			html:                htmlProductSection(`<span class="css-8h3us8">10%</span><div class="css-o2nlqt"><span>9,000</span><span>원</span></div><span class="css-1s96j0s"><span>숫자아님</span></span>`),
			wantPrice:           9000, // discountedPrice로 보정
			wantDiscountedPrice: 9000,
			wantDiscountRate:    0, // 보정 후 0으로 초기화
		},
		{
			name:      "실패: 할인 중 — 정가(취소선) 셀렉터 없음",
			html:      htmlProductSection(`<span class="css-8h3us8">10%</span><div class="css-o2nlqt"><span>9,000</span><span>원</span></div>`),
			wantErr:   true,
			errSubstr: "상품 가격 요소",
		},
		// ── 예외 상황 (DOM 구조 이상) ─────────────────────────────────────────
		{
			// 할인율 span이 2개 이상 → 레이아웃 변경으로 간주
			name: "실패: 할인율 span 2개 이상 → 가격 구조 이상 에러",
			html: htmlProductSection(
				`<span class="css-8h3us8">10%</span>` +
					`<span class="css-8h3us8">20%</span>` +
					`<div class="css-o2nlqt"><span>9,000</span><span>원</span></div>`,
			),
			wantErr:   true,
			errSubstr: "할인율 요소(span.css-8h3us8)가 2개 이상",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			doc, err := goquery.NewDocumentFromReader(strings.NewReader(tt.html))
			require.NoError(t, err)
			sel := doc.Find("#product-atf > section.css-1ua1wyk")

			price, discountedPrice, discountRate, err := extractPriceDetails(sel, targetURL)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPrice, price)
				assert.Equal(t, tt.wantDiscountedPrice, discountedPrice)
				assert.Equal(t, tt.wantDiscountRate, discountRate)
			}
		})
	}
}
