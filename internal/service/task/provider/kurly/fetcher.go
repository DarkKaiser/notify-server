package kurly

import (
	"context"
	"fmt"
	"regexp"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

var (
	// reExtractNextData 마켓컬리 상품 페이지의 핵심 데이터가 담긴 <script> 태그 내용을 추출합니다.
	// (페이지 소스에 포함된 초기 데이터를 직접 긁어와서, 별도의 API 호출 없이도 상품 정보를 얻을 수 있게 해줍니다)
	reExtractNextData = regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*>([\s\S]*?)</script>`)

	// reDetectUnavailable 추출한 데이터에서 "상품 정보 없음(null)" 패턴이 있는지 검사합니다.
	// 이 패턴이 발견되면 '판매 중지'되거나 '삭제된 상품'으로 판단하여 불필요한 알림을 보내지 않도록 합니다.
	reDetectUnavailable = regexp.MustCompile(`"product":\s*null`)
)

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
	productPageURL := formatProductPageURL(id)
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
	if reDetectUnavailable.MatchString(jsonProductData) {
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
		if err := product.extractPriceDetails(sel, productPageURL); err != nil {
			return nil, err
		}
	}

	return product, nil
}
