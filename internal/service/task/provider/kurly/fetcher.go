package kurly

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	"github.com/tidwall/gjson"
)

// fetchProduct 상품 ID를 받아 마켓컬리 상품 상세 페이지를 수집하고, 파싱하여 최신 상품 정보를 반환합니다.
//
// [이중 추출 전략]
// 이 함수는 상품 정보를 두 가지 방법으로 추출합니다.
//
//  1. JSON 파싱 (판매 상태 판별):
//     Next.js가 HTML에 주입한 <script id="__NEXT_DATA__"> 태그에서 JSON 데이터를 꺼냅니다.
//     gjson을 사용하여 props.pageProps.product 경로를 검사하여, 해당 키가 없거나 null이면
//     '판매 불가(IsUnavailable=true)' 상태로 확정합니다. DOM 렌더링 전에 원본 데이터를
//     직접 보기 때문에, 불필요한 DOM 파싱 없이 빠르고 신뢰성 있게 상태를 판별할 수 있습니다.
//
//  2. DOM 파싱 (이름 및 가격 추출):
//     판매 중인 상품에 한해 CSS 셀렉터로 HTML DOM을 직접 탐색하여 상품명과 가격을 추출합니다.
//
// [매개변수]
//   - ctx: HTTP 요청에 사용할 컨텍스트 (취소 및 타임아웃 전파)
//   - id: 조회할 마켓컬리 상품 고유 코드
//
// [반환값]
//   - *product: 수집된 상품 정보 (판매 불가 상품은 IsUnavailable=true로 반환)
//   - error: HTTP 수집 실패, HTML 파싱 실패, DOM 구조 변경 등의 에러
func (t *task) fetchProduct(ctx context.Context, id int) (*product, error) {
	// =====================================================================
	// [단계 1] 상품 상세 페이지 HTML을 HTTP로 수집합니다.
	// =====================================================================
	targetURL := productPageURL(id)
	doc, err := t.Scraper().FetchHTMLDocument(ctx, targetURL, nil)
	if err != nil {
		return nil, err
	}

	// =====================================================================
	// [단계 2] HTML 전체 소스에서 __NEXT_DATA__ JSON을 추출합니다.
	// =====================================================================

	// goquery를 이용하여 상품 정보가 담긴 <script id="__NEXT_DATA__"> 태그를 빠르고 효율적으로 탐색하여 내용을 추출합니다.
	nextDataSel := doc.Find("script#__NEXT_DATA__")
	if nextDataSel.Length() == 0 {
		return nil, newErrNextDataNotFound(targetURL)
	}

	nextDataJSON := nextDataSel.Text()

	// =====================================================================
	// [단계 3] 수집된 데이터를 담을 반환용 상품 객체를 초기화합니다.
	// =====================================================================

	var product = &product{
		ID:                 id,
		Name:               "",
		Price:              0,
		DiscountedPrice:    0,
		DiscountRate:       0,
		LowestPrice:        0,
		LowestPriceTimeUTC: time.Time{},
		IsUnavailable:      false,
		FetchFailedCount:   0,
	}

	// =====================================================================
	// [단계 4] 판매 상태 및 데이터 구조 유효성을 판별합니다.
	// =====================================================================

	// Next.js의 데이터 주입 구조(__NEXT_DATA__)에서 최상위 필수 노드인 props.pageProps 객체가
	// 존재하는지 우선 검증합니다. 만약 이 노드가 없다면, 이는 상품의 '판매 중지(단종)'가 아니라
	// 마켓컬리 웹 개편 등으로 인한 'JSON 스키마 변경(구조 결함)'을 의미합니다.
	//
	// 이 경우 조용히 IsUnavailable = true 로 넘기면, 모든 상품이 정상적으로 단종된 것으로 오도되어
	// 사용자에게 대량의 잘못된 알림(스팸)이 발송되고 이전 스냅샷 상태가 초기화되는 심각한 장애가 발생합니다.
	// 따라서 명시적인 시스템 에러를 반환하여, Synchronizer의 '연속 실패 횟수(FetchFailedCount)'
	// 보호 로직이 작동하도록 격리해야 합니다.
	if !gjson.Get(nextDataJSON, "props.pageProps").Exists() {
		return nil, newErrNextDataStructureInvalid(targetURL)
	}

	// 최상위 노드가 정상 존재함이 확인된 후, 개별 상품 노드(product)를 검사합니다.
	// 마켓컬리는 판매 중지 또는 존재하지 않는 상품일 경우 props.pageProps.product를
	// null 또는 키 자체를 생략하는 방식으로 표현합니다.
	// Exists()와 Type 검사를 모두 수행하여 두 케이스를 모두 방어합니다.
	if !gjson.Get(nextDataJSON, "props.pageProps.product").Exists() ||
		gjson.Get(nextDataJSON, "props.pageProps.product").Type == gjson.Null {
		product.IsUnavailable = true
	}

	// =====================================================================
	// [단계 5] 판매 중인 상품에 한해 DOM에서 이름과 가격을 추출합니다.
	// =====================================================================
	if !product.IsUnavailable {
		// 상품의 주요 정보(이름, 가격 등)가 포함된 최상위 컨테이너(섹션)를 선택합니다.
		// 동적 해시 클래스명(css-1ua1wyk)을 배제하여 레이아웃 변경 시에도 안전하게 태그 위주로 선택합니다.
		productSection := doc.Find("#product-atf > section")
		if productSection.Length() != 1 {
			// 셀렉터 결과가 정확히 1개가 아니면 페이지 레이아웃이 변경된 것으로 판단합니다.
			return nil, newErrProductSectionExtractionFailed(targetURL)
		}

		// 상품 이름을 추출합니다.
		// 난독화된 div 체인을 모두 걷어내고 section 하위의 첫 번째 h1 또는 h2 타이틀 태그를 찾습니다.
		nameSel := productSection.Find("h1, h2").First()
		if nameSel.Length() == 0 {
			return nil, newErrProductNameExtractionFailed(targetURL)
		}

		product.Name = strutil.NormalizeSpace(nameSel.Text())

		// 상품 가격 정보(정가, 할인가, 할인율)를 추출합니다.
		price, discountedPrice, discountRate, err := extractPriceDetails(productSection, targetURL)
		if err != nil {
			return nil, err
		}

		product.Price = price
		product.DiscountedPrice = discountedPrice
		product.DiscountRate = discountRate
	}

	return product, nil
}

// extractPriceDetails 상품의 가격 상세 정보(정가, 할인가, 할인율)를 DOM에서 추출합니다.
//
// [추출 전략]
// 마켓컬리 상품 페이지는 '할인 적용 여부'에 따라 가격을 표시하는 DOM 구조가 다릅니다.
// 이 함수는 할인율을 나타내는 요소의 존재 여부를 기준으로 분기하여,
// 각 상황에 맞는 최적의 CSS 셀렉터를 통해 다양한 포맷의 가격 수치를 정확히 파싱합니다.
//
// [매개변수]
//   - productSection: 상품 정보가 담긴 영역의 파싱된 HTML 노드 (*goquery.Selection)
//   - targetURL: 에러 발생 시 로그에 출처를 남기기 위한 원본 페이지 URL
//
// [반환값]
//   - price: 정가
//   - discountedPrice: 할인가 (할인이 없는 경우 0)
//   - discountRate: 할인율 (예: 10% -> 10. 할인이 없는 경우 0)
//   - err: DOM 구조를 찾을 수 없거나 데이터 변환에 실패한 경우의 에러
func extractPriceDetails(productSection *goquery.Selection, targetURL string) (price, discountedPrice, discountRate int, err error) {
	var rates []int
	var prices []int
	priceRegex := regexp.MustCompile(`^([0-9,]+)원?$`)

	productSection.Find("span").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		
		if strings.HasSuffix(text, "%") && len(text) <= 4 {
			if rate, err := strconv.Atoi(strings.TrimSuffix(text, "%")); err == nil {
				rates = append(rates, rate)
			}
		}

		if matches := priceRegex.FindStringSubmatch(text); len(matches) == 2 {
			priceStr := strings.ReplaceAll(matches[1], ",", "")
			if p, err := strconv.Atoi(priceStr); err == nil && p > 0 { // 0원(총 상품금액 등) 제외
				prices = append(prices, p)
			}
		}
	})

	if len(rates) > 1 {
		return 0, 0, 0, newErrPriceStructureInvalid(targetURL)
	}

	if len(prices) == 0 {
		return 0, 0, 0, newErrPriceExtractionFailed(targetURL, "price elements not found")
	}

	if len(rates) == 1 {
		discountRate = rates[0]
	}

	if len(prices) == 1 {
		// 할인 미적용 또는 가격 요소가 1개만 감지된 경우 (예: 정가 파싱 실패 시 자동 보정)
		price = prices[0]
		discountedPrice = prices[0]
		discountRate = 0 // 보정: 할인가만 있으면 할인율을 무시함
	} else {
		// 가격 요소가 2개 이상이면 첫번째가 정가, 두번째가 할인가 (DOM 상 정가가 먼저 나타남)
		price = prices[0]
		discountedPrice = prices[1]
	}

	return price, discountedPrice, discountRate, nil
}
