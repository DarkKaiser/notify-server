package navershopping

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

const (
	// productSearchEndpoint 네이버 쇼핑 상품 검색을 위한 OpenAPI 엔드포인트입니다.
	//
	// 공식 문서: https://developers.naver.com/docs/serviceapi/search/shopping/shopping.md
	// 인증 방식: 요청 헤더에 X-Naver-Client-Id, X-Naver-Client-Secret 값을 포함해야 합니다.
	productSearchEndpoint = "https://openapi.naver.com/v1/search/shop.json"

	// ------------------------------------------------------------------------------------------------
	// API 매개변수 기본값
	// ------------------------------------------------------------------------------------------------

	// defaultSortOption 검색 결과 정렬 기준의 기본값입니다.
	//
	// 네이버 쇼핑 API가 지원하는 정렬 옵션:
	//   - sim:  유사도순 (기본값, 검색어와의 연관도가 높은 순서)
	//   - date: 날짜순 (최신 등록 상품 우선)
	//   - asc:  가격 오름차순 (저가 → 고가)
	//   - dsc:  가격 내림차순 (고가 → 저가)
	defaultSortOption = "sim"

	// defaultDisplayCount 1회 API 요청 시 반환받을 검색 결과의 최대 개수입니다.
	//
	// 네이버 쇼핑 API는 요청당 최소 10개, 최대 100개까지 허용합니다.
	// 전체 수집 시간을 줄이기 위해 허용 최대치인 100으로 설정합니다.
	defaultDisplayCount = 100
)

// productSearchResponse 네이버 쇼핑 상품 검색 API의 최상위 응답 구조체입니다.
//
// Total을 기준으로 전체 결과의 규모를 파악하고, Items를 반복(Pagination)하여 수집합니다.
// 페이지 경계를 벗어난 경우 API는 Items를 빈 슬라이스로 반환하므로, len(Items)==0을 종료 조건으로 사용합니다.
type productSearchResponse struct {
	Total   int                          `json:"total"`   // 해당 검색어에 대한 전체 상품 수
	Start   int                          `json:"start"`   // 이번 요청의 시작 인덱스 (1부터 시작)
	Display int                          `json:"display"` // 이번 응답에 실제로 포함된 상품 수 (마지막 페이지에서는 요청값보다 작을 수 있음)
	Items   []*productSearchResponseItem `json:"items"`   // 이번 페이지의 상품 목록
}

// productSearchResponseItem 네이버 쇼핑 API 응답에서 상품 1건의 원시(Raw) 데이터를 담는 구조체입니다.
//
// 이 구조체는 API의 JSON 응답을 그대로 매핑하는 목적으로만 사용됩니다.
// 이후 `parseProduct` 함수를 통해 도메인 모델(`*product`)로 변환될 때
// HTML 태그 제거, 가격 문자열 파싱 등 정제 작업이 함께 수행됩니다.
type productSearchResponseItem struct {
	ProductID   string `json:"productId"`   // 네이버 쇼핑 상품 고유 ID
	ProductType string `json:"productType"` // 상품 유형 코드 (1: 일반, 2: 중고, 3: 단종, 4: 판매예정)
	Title       string `json:"title"`       // 상품명
	Link        string `json:"link"`        // 네이버 쇼핑 상품 상세 페이지 URL
	LowPrice    string `json:"lprice"`      // 최저가 (쉼표가 포함된 문자열, 예: "1,234,000")
	MallName    string `json:"mallName"`    // 최저가를 제공하는 쇼핑몰 이름 (예: "쿠팡", "네이버")
}

// fetchPageProducts 네이버 쇼핑 검색 API에 1회 HTTP 요청을 보내어
// 특정 페이지(구간)에 해당하는 상품 데이터를 가져옵니다.
//
// 인증 헤더(Client ID/Secret)를 자동으로 설정한 뒤 FetchJSON을 통해
// 요청-응답-디코딩을 일괄 처리하므로, 호출부에서는 URL만 전달하면 됩니다.
//
// 매개변수:
//   - ctx: 타임아웃 및 외부 취소 신호를 전파하기 위한 컨텍스트
//   - apiURL: 검색 키워드, 페이지 번호 등 쿼리 파라미터가 결합된 최종 요청 URL
//
// 반환값:
//   - *productSearchResponse: JSON 디코딩이 완료된 API 응답 객체
//   - error: 네트워크 오류, API 서버 오류, JSON 파싱 실패 시 에러 반환
func (t *task) fetchPageProducts(ctx context.Context, apiURL string) (*productSearchResponse, error) {
	header := http.Header{
		"X-Naver-Client-Id":     []string{t.clientID},
		"X-Naver-Client-Secret": []string{t.clientSecret},
	}

	var resp = &productSearchResponse{}
	if err := t.Scraper().FetchJSON(ctx, "GET", apiURL, nil, header, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// buildProductSearchURL 검색 키워드와 페이지네이션 매개변수를 조합하여
// 네이버 쇼핑 API 요청용 최종 URL 문자열을 생성합니다.
//
// 매개변수:
//   - baseURL: 엔드포인트 원본 URL (이 함수 호출 후에도 변경되지 않음)
//   - query: 검색할 상품 키워드
//   - start: 결과 조회 시작 인덱스 (1부터 시작)
//   - display: 한 페이지에 포함할 상품 수 (최대 100)
func buildProductSearchURL(baseURL *url.URL, query string, start, display int) string {
	// baseURL의 얕은 복사(Shallow Copy)를 수행합니다.
	// 필드 중 User는 포인터이나, 여기서는 값 타입인 RawQuery 필드만 새로운 값으로 교체하여
	// URL 문자열을 생성하므로 원본 baseURL 객체를 오염시키지 않고 안전하게 재사용할 수 있습니다.
	u := *baseURL

	q := u.Query()
	q.Set("query", query)
	q.Set("display", strconv.Itoa(display))
	q.Set("start", strconv.Itoa(start))
	q.Set("sort", defaultSortOption)
	u.RawQuery = q.Encode()

	return u.String()
}

// parseProduct API 응답의 원시 상품 데이터(productSearchResponseItem)를 도메인 모델(*product)로 변환합니다.
//
// 변환 과정에서 다음과 같은 정제 작업이 수행됩니다:
//   - Title: API가 검색어 매칭 부분을 <b> 태그로 감싸서 반환하므로, HTML 태그를 모두 제거합니다.
//   - LowPrice: 쉼표가 포함된 문자열("1,234,000")을 정수형으로 변환합니다.
//
// 가격 파싱에 실패하면 경고 로그를 남기고 nil을 반환합니다.
// 호출부에서는 nil 여부를 확인하여 해당 상품을 건너뛰어야 합니다.
func (t *task) parseProduct(item *productSearchResponseItem) *product {
	// 가격 정보 파싱: API가 제공하는 가격 데이터(LowPrice)는 "1,234,000"과 같이 쉼표가 구분자로
	// 포함된 문자열 형태입니다. 이를 시스템에서 사용하기 위해 순수 숫자만 남긴 후 정수형(int)으로 변환합니다.
	cleanedPriceStr := strings.ReplaceAll(item.LowPrice, ",", "")
	lowPrice, err := strconv.Atoi(cleanedPriceStr)
	if err != nil {
		t.Log(component, applog.WarnLevel, "상품 정보 무시됨: 유효하지 않은 가격 데이터 형식", nil, applog.Fields{
			"product_id":        item.ProductID,
			"product_type":      item.ProductType,
			"title":             item.Title,
			"link":              item.Link,
			"low_price":         item.LowPrice,
			"cleaned_low_price": cleanedPriceStr,
			"mall_name":         item.MallName,
			"error":             err.Error(),
		})

		return nil
	}

	return &product{
		ProductID:   item.ProductID,
		ProductType: item.ProductType,
		Title:       strutil.StripHTML(item.Title),
		Link:        item.Link,
		LowPrice:    lowPrice,
		MallName:    item.MallName,
	}
}
