package kurly

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	"github.com/tidwall/gjson"
)

const (
	// productPageURLFormat 마켓컬리 상품 상세 페이지 URL을 생성하기 위한 fmt.Sprintf 포맷 문자열입니다.
	//
	// %v 자리에 상품 코드(int)를 대입하면 해당 상품의 상세 페이지 URL이 완성됩니다.
	//
	// 사용 예시:
	//
	//  url := fmt.Sprintf(productPageURLFormat, 12345) // → "https://www.kurly.com/goods/12345"
	productPageURLFormat = "https://www.kurly.com/goods/%v"
)

var (
	// @@@@@
	// reExtractNextData 마켓컬리 상품 페이지의 핵심 데이터가 담긴 <script> 태그 내용을 추출합니다.
	// (페이지 소스에 포함된 초기 데이터를 직접 긁어와서, 별도의 API 호출 없이도 상품 정보를 얻을 수 있게 해줍니다)
	reExtractNextData = regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*>([\s\S]*?)</script>`)
)

// @@@@@
// fetchProductInfo 상품 상세 페이지(HTML)를 Fetch하여 상품의 최신 상태 및 가격 정보를 조회합니다.
//
// [구현 상세]
// 본 함수는 데이터의 정확성을 위해 '이중 추출(Dual Extraction)' 기법을 사용합니다.
//
//  1. Metadata Parsing (JSON):
//     Next.js가 주입한 `<script id="__NEXT_DATA__">`에서 상품의 판매 상태(Unavailable 여부)를 1차적으로 검증합니다.
//     이는 DOM 렌더링 이전에 원본 데이터의 상태를 확인하여 불필요한 파싱을 방지합니다.
//
//  2. Price Parsing (DOM):
//     HTML DOM 구조를 직접 탐색하여 실제 사용자에게 노출되는 '최종 가격(Pricing)'을 추출합니다.
//     할인 정책(Rate, Discounted)에 따른 동적 구조 변화를 처리합니다.
func (t *task) fetchProductInfo(ctx context.Context, id int) (*product, error) {
	// 상품 페이지를 읽어들인다.
	productPageURL := buildProductPageURL(id)
	doc, err := t.Scraper().FetchHTMLDocument(ctx, productPageURL, nil)
	if err != nil {
		return nil, err
	}

	// 읽어들인 페이지에서 상품 데이터가 JSON 포맷으로 저장된 자바스크립트 구문을 추출한다.
	html, err := doc.Html()
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("불러온 페이지(%s)에서 HTML 추출이 실패하였습니다", productPageURL))
	}
	match := reExtractNextData.FindStringSubmatch(html)
	if len(match) < 2 {
		return nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("불러온 페이지(%s)에서 상품에 대한 JSON 데이터 추출이 실패하였습니다.(error:%s)", productPageURL, err))
	}
	jsonProductData := match[1]

	var product = &product{
		ID:                 id,
		Name:               "",
		Price:              0,
		DiscountedPrice:    0,
		DiscountRate:       0,
		LowestPrice:        0,
		LowestPriceTimeUTC: time.Time{},
		IsUnavailable:      false,
	}

	// 알 수 없는 상품(현재 판매중이지 않은 상품)인지 확인한다.
	// gjson을 사용하여 빠르고 안전하게 지정된 경로의 실제 값이 Null 타입인지 검사합니다.
	if !gjson.Get(jsonProductData, "props.pageProps.product").Exists() ||
		gjson.Get(jsonProductData, "props.pageProps.product").Type == gjson.Null {
		product.IsUnavailable = true
	}

	if !product.IsUnavailable {
		sel := doc.Find("#product-atf > section.css-1ua1wyk")
		if sel.Length() != 1 {
			return nil, scraper.NewErrHTMLStructureChanged(productPageURL, "상품정보 섹션 추출 실패")
		}

		// 상품 이름을 확인한다.
		ps := sel.Find("div.css-84rb3h > div.css-6zfm8o > div.css-o3fjh7 > h1")
		if ps.Length() != 1 {
			return nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 이름 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productPageURL))
		}
		product.Name = strutil.NormalizeSpace(ps.Text())

		// 상품 가격 정보를 추출한다.
		price, discountedPrice, discountRate, err := extractPriceDetails(sel, productPageURL)
		if err != nil {
			return nil, err
		}

		product.Price = price
		product.DiscountedPrice = discountedPrice
		product.DiscountRate = discountRate
	}

	return product, nil
}

// @@@@@
// extractPriceDetails HTML DOM에서 가격 상세 정보(정상가, 할인가, 할인율)를 추출합니다.
//
// [동작 방식]
// 마켓컬리 상세 페이지의 가격 표시 DOM 구조는 할인 적용 여부에 따라 상이합니다.
// 본 함수는 이 구조적 차이를 식별하여 적절한 값을 추출하여 반환합니다.
//
//  1. 할인 미적용: 단일 가격 요소(Price)만 존재
//  2. 할인 적용중: 할인율(Rate) + 할인가(Discounted) + 정상가(Price, 취소선) 모두 존재
//
// [매개변수]
//   - sel: 가격 정보가 포함된 DOM Selection
//   - productPageURL: 에러 발생 시 디버깅을 돕기 위해 로그에 포함할 상품 페이지 URL
//
// [반환값]
//   - price: 할인 전 정상 가격
//   - discountedPrice: 할인이 적용된 가격
//   - discountRate: 할인율
//   - error: 추출 실패 시의 에러
func extractPriceDetails(sel *goquery.Selection, productPageURL string) (price, discountedPrice, discountRate int, err error) {
	ps := sel.Find("h2.css-xrp7wx > span.css-8h3us8")
	if ps.Length() == 0 /* 가격, 단위(원) */ {
		ps = sel.Find("h2.css-xrp7wx > div.css-o2nlqt > span")
		if ps.Length() != 2 /* 가격 + 단위(원) */ {
			return 0, 0, 0, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productPageURL))
		}

		// 가격
		price, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), ",", ""))
		if err != nil {
			return 0, 0, 0, apperrors.Wrap(err, apperrors.ExecutionFailed, "상품 가격의 숫자 변환이 실패하였습니다")
		}
	} else if ps.Length() == 1 /* 할인율, 할인 가격, 단위(원) */ {
		// 할인율
		discountRate, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), "%", ""))
		if err != nil {
			return 0, 0, 0, apperrors.Wrap(err, apperrors.ExecutionFailed, "상품 할인율의 숫자 변환이 실패하였습니다")
		}

		// 할인 가격
		ps = sel.Find("h2.css-xrp7wx > div.css-o2nlqt > span")
		if ps.Length() != 2 /* 가격 + 단위(원) */ {
			return 0, 0, 0, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productPageURL))
		}

		discountedPrice, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), ",", ""))
		if err != nil {
			return 0, 0, 0, apperrors.Wrap(err, apperrors.ExecutionFailed, "상품 할인 가격의 숫자 변환이 실패하였습니다")
		}

		// 가격
		ps = sel.Find("span.css-1s96j0s > span")
		if ps.Length() != 1 /* 가격 + 단위(원) */ {
			return 0, 0, 0, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productPageURL))
		}
		price, _ = strconv.Atoi(strings.ReplaceAll(strings.ReplaceAll(ps.Text(), ",", ""), "원", ""))
	} else {
		return 0, 0, 0, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(1) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productPageURL))
	}
	return price, discountedPrice, discountRate, nil
}

// @@@@@
// buildProductPageURL 상품 ID를 받아 상품 상세 페이지의 전체 URL을 반환합니다.
func buildProductPageURL(id any) string {
	return fmt.Sprintf(productPageURLFormat, id)
}
