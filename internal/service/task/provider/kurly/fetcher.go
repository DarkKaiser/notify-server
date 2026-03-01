package kurly

import (
	"context"
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
		productSection := doc.Find("#product-atf > section.css-1ua1wyk")
		if productSection.Length() != 1 {
			// 셀렉터 결과가 정확히 1개가 아니면 페이지 레이아웃이 변경된 것으로 판단합니다.
			return nil, newErrProductSectionExtractionFailed(targetURL)
		}

		// 상품 이름을 추출합니다.
		nameSel := productSection.Find("div.css-84rb3h > div.css-6zfm8o > div.css-o3fjh7 > h1")
		if nameSel.Length() != 1 {
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
	// 마켓컬리는 할인 적용 여부에 따라 가격 영역의 DOM 구조가 달라집니다.
	// 할인율 요소(span.css-8h3us8)의 개수를 기준으로 현재 페이지가 어느 구조인지 판별합니다.
	discountRateSel := productSection.Find("h2.css-xrp7wx > span.css-8h3us8")

	if discountRateSel.Length() == 0 {
		// =====================================================================
		// [할인 미적용] 할인이 적용되지 않아 정가만 표시되는 경우입니다.
		// =====================================================================

		// 가격 컨테이너(div.css-o2nlqt) 하위에는 2개의 span 태그가 있어야 정상적인 요소 구조입니다.
		// - Eq(0): 가격 숫자 (예: "10,000")
		// - Eq(1): 가격 단위 (예: "원")
		priceSel := productSection.Find("h2.css-xrp7wx > div.css-o2nlqt > span")
		if priceSel.Length() != 2 {
			return 0, 0, 0, newErrPriceExtractionFailed(targetURL, "h2.css-xrp7wx > div.css-o2nlqt > span")
		}

		// 추출한 가격 숫자 텍스트에서 쉼표(,)를 제거한 후 정수 타입으로 변환합니다.
		text := strings.TrimSpace(priceSel.Eq(0).Text())
		price, err = strconv.Atoi(strings.ReplaceAll(text, ",", ""))
		if err != nil {
			return 0, 0, 0, newErrPriceConversionFailed(err, text)
		}
	} else if discountRateSel.Length() == 1 {
		// =====================================================================
		// [할인 적용 중] 할인율, 할인가(실구매가), 정가(취소선) 세 요소가 모두 존재하는 경우입니다.
		// =====================================================================

		// 1. 할인율을 추출합니다.
		//    span.css-8h3us8의 텍스트(예: "10%")에서 "%" 기호를 제거한 후 정수 타입으로 변환합니다.
		text := strings.TrimSpace(discountRateSel.Eq(0).Text())
		discountRate, err = strconv.Atoi(strings.ReplaceAll(text, "%", ""))
		if err != nil {
			return 0, 0, 0, newErrDiscountRateConversionFailed(err, text)
		}

		// 2. 할인가(실구매가)를 추출합니다.
		//    가격 컨테이너(div.css-o2nlqt) 하위에는 2개의 span 태그가 있어야 정상적인 요소 구조입니다.
		//    - Eq(0): 할인가 숫자 (예: "9,000")
		//    - Eq(1): 가격 단위 (예: "원")
		discountedPriceSel := productSection.Find("h2.css-xrp7wx > div.css-o2nlqt > span")
		if discountedPriceSel.Length() != 2 {
			return 0, 0, 0, newErrPriceExtractionFailed(targetURL, "h2.css-xrp7wx > div.css-o2nlqt > span")
		}

		// 추출한 할인가 숫자 텍스트에서 쉼표(,)를 제거한 후 정수 타입으로 변환합니다.
		text = strings.TrimSpace(discountedPriceSel.Eq(0).Text())
		discountedPrice, err = strconv.Atoi(strings.ReplaceAll(text, ",", ""))
		if err != nil {
			return 0, 0, 0, newErrDiscountedPriceConversionFailed(err, text)
		}

		// 3. 정가(원래 가격)를 취소선 영역에서 추출합니다.
		//    span.css-1s96j0s > span 셀렉터는 정가 숫자만을 담은 span 1개만 반환해야 합니다.
		//    텍스트에 "원" 단위가 포함되어 있으므로 쉼표(",")와 "원"을 모두 제거한 후 정수 타입으로 변환합니다.
		priceSel := productSection.Find("span.css-1s96j0s > span")
		if priceSel.Length() != 1 {
			return 0, 0, 0, newErrPriceExtractionFailed(targetURL, "span.css-1s96j0s > span")
		}

		price, err = strconv.Atoi(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(priceSel.Text()), ",", ""), "원", ""))
		if err != nil {
			// 정가(취소선) 파싱에 실패하더라도 실구매가(할인가) 정보가 정상 수집되었다면,
			// 전체 수집을 실패 처리하지 않고 정가를 할인가와 동일하게 보정합니다.
			price = discountedPrice

			// 정가와 할인가가 같아졌으므로 할인율도 0으로 명시적 초기화합니다.
			discountRate = 0

			// 보정 처리를 완료했으므로 상위로 에러를 전파하지 않습니다.
			err = nil
		}
	} else {
		// =====================================================================
		// [예외 상황] 할인율 요소가 2개 이상 감지된 경우입니다.
		// =====================================================================

		// 정상적인 상품 페이지에서는 할인율 요소(span.css-8h3us8)가 0개(할인 없음) 또는 1개(할인 적용)만 존재해야 합니다.
		// 2개 이상 감지되었다는 것은 마켓컬리의 페이지 레이아웃이 변경되어 전혀 다른 DOM 구조가 나타났음을 의미합니다.
		return 0, 0, 0, newErrPriceStructureInvalid(targetURL)
	}

	return price, discountedPrice, discountRate, nil
}
