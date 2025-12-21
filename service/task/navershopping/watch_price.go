package navershopping

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/sirupsen/logrus"
)

const (
	// watchPriceAnyCommandPrefix 동적 커맨드 라우팅을 위한 식별자 접두어입니다.
	//
	// 이 접두어로 시작하는 모든 CommandID는 `executeWatchPrice` 핸들러로 라우팅되어 처리됩니다.
	// 이를 통해 사용자는 "WatchPrice_Apple", "WatchPrice_Samsung" 등과 같이
	// 하나의 로직으로 처리되는 다수의 커맨드를 유연하게 생성할 수 있습니다.
	watchPriceAnyCommandPrefix = "WatchPrice_"

	// searchAPIURL 네이버 쇼핑 상품 검색을 위한 OpenAPI 엔드포인트입니다.
	// 공식 문서: https://developers.naver.com/docs/serviceapi/search/shopping/shopping.md
	searchAPIURL = "https://openapi.naver.com/v1/search/shop.json"

	// ------------------------------------------------------------------------------------------------
	// API 매개변수 설정
	// ------------------------------------------------------------------------------------------------

	// apiSortOption 검색 결과 정렬 기준 (sim: 유사도순, date: 날짜순, asc: 가격오름차순, dsc: 가격내림차순)
	apiSortOption = "sim"

	// apiDisplayCount 1회 요청 시 반환받을 검색 결과의 최대 개수 (API 제한: 10~100)
	apiDisplayCount = 100

	// ------------------------------------------------------------------------------------------------
	// 정책 설정
	// ------------------------------------------------------------------------------------------------

	// policyFetchLimit 단일 커맨드당 최대 수집 제한 (과도한 요청 방지)
	policyFetchLimit = 1000
)

type watchPriceSettings struct {
	Query   string `json:"query"`
	Filters struct {
		IncludedKeywords string `json:"included_keywords"`
		ExcludedKeywords string `json:"excluded_keywords"`
		PriceLessThan    int    `json:"price_less_than"`
	} `json:"filters"`
}

func (s *watchPriceSettings) validate() error {
	s.Query = strings.TrimSpace(s.Query)
	if s.Query == "" {
		return apperrors.New(apperrors.InvalidInput, "query가 입력되지 않았거나 공백입니다")
	}
	if s.Filters.PriceLessThan <= 0 {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("price_less_than은 0보다 커야 합니다 (입력값: %d)", s.Filters.PriceLessThan))
	}
	return nil
}

// watchPriceSnapshot 가격 변동을 감지하기 위한 상품 데이터의 스냅샷입니다.
type watchPriceSnapshot struct {
	Products []*product `json:"products"`
}

// product 검색 API를 통해 조회된 개별 상품 정보를 담는 도메인 모델입니다.
type product struct {
	ProductID   string `json:"productId"`   // 네이버 쇼핑 상품 ID (상품 고유 식별자)
	ProductType string `json:"productType"` // 상품 유형 (1: 일반, 2: 중고, 3: 단종, 4: 판매예정 등)

	Title    string `json:"title"`    // 상품명 (HTML 태그가 포함될 수 있음)
	Link     string `json:"link"`     // 상품 상세 정보 페이지 URL
	LowPrice int    `json:"lprice"`   // 판매 최저가 (단위: 원)
	MallName string `json:"mallName"` // 판매 쇼핑몰 상호 (예: "네이버", "쿠팡" 등)
}

// Key 상품을 고유하게 식별하기 위한 키를 반환합니다.
func (p *product) Key() string {
	return p.ProductID
}

// String 상품 정보를 사용자에게 발송하기 위한 알림 메시지 포맷으로 변환합니다.
func (p *product) String(supportsHTML bool, mark string) string {
	if supportsHTML {
		const htmlFormat = `☞ <a href="%s"><b>%s</b></a> (%s) %s원%s`

		return fmt.Sprintf(
			htmlFormat,
			p.Link,
			p.Title,
			p.MallName,
			strutil.FormatCommas(p.LowPrice),
			mark,
		)
	}

	const textFormat = `☞ %s (%s) %s원%s
%s`

	return strings.TrimSpace(fmt.Sprintf(textFormat, p.Title, p.MallName, strutil.FormatCommas(p.LowPrice), mark, p.Link))
}

// searchResponse 네이버 쇼핑 검색 API의 응답 데이터를 담는 구조체입니다.
type searchResponse struct {
	Total   int                   `json:"total"`   // 검색된 전체 상품의 총 개수 (페이징 처리에 사용)
	Start   int                   `json:"start"`   // 검색 시작 위치 (1부터 시작하는 인덱스)
	Display int                   `json:"display"` // 현재 응답에 포함된 상품 개수 (요청한 display 값과 같거나 작음)
	Items   []*searchResponseItem `json:"items"`   // 검색된 개별 상품 리스트
}

// searchResponseItem 검색 API 응답에서 개별 상품 정보를 담는 로우(Raw) 데이터 구조체입니다.
type searchResponseItem struct {
	ProductID   string `json:"productId"`   // 네이버 쇼핑 상품 ID (상품 고유 식별자)
	ProductType string `json:"productType"` // 상품 유형 (1: 일반, 2: 중고, 3: 단종, 4: 판매예정 등)

	Title    string `json:"title"`    // 상품명 (HTML 태그 <b>가 포함된 원본 문자열)
	Link     string `json:"link"`     // 상품 상세 정보 페이지 URL
	LowPrice string `json:"lprice"`   // 판매 최저가 (단위: 원)
	MallName string `json:"mallName"` // 판매 쇼핑몰 상호 (예: "네이버", "쿠팡" 등)
}

// executeWatchPrice 작업을 실행하여 상품 가격 정보를 확인합니다.
func (t *task) executeWatchPrice(commandSettings *watchPriceSettings, prevSnapshot *watchPriceSnapshot, supportsHTML bool) (string, interface{}, error) {
	// 1. 상품 정보 수집 및 필터링
	currentProducts, err := t.fetchProducts(commandSettings)
	if err != nil {
		return "", nil, err
	}

	currentSnapshot := &watchPriceSnapshot{
		Products: currentProducts,
	}

	// 2. 신규 상품 확인 및 알림 메시지 생성
	return t.diffAndNotify(commandSettings, currentSnapshot, prevSnapshot, supportsHTML)
}

// fetchProducts 네이버 쇼핑 검색 API를 호출하여 조건에 맞는 상품 목록을 수집합니다.
func (t *task) fetchProducts(commandSettings *watchPriceSettings) ([]*product, error) {
	var (
		header = map[string]string{
			"X-Naver-Client-Id":     t.clientID,
			"X-Naver-Client-Secret": t.clientSecret,
		}

		startIndex        = 1
		fetchedTotalCount = math.MaxInt

		pageContent = &searchResponse{}
	)

	// API 호출을 위한 기본 URL을 파싱합니다.
	// 반복문 내에서 불필요한 URL 파싱(`url.Parse`) 오버헤드를 방지하기 위해 루프 진입 전에 수행합니다.
	// 파싱된 `baseURL` 객체는 루프 내에서 값 복사되어 안전하게 쿼리 파라미터를 조작하는 데 사용됩니다.
	baseURL, err := url.Parse(searchAPIURL)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.Internal, "네이버 쇼핑 검색 API 엔드포인트 URL 파싱에 실패하였습니다")
	}

	for startIndex < fetchedTotalCount {
		// 작업 취소 여부 확인
		if t.IsCanceled() {
			t.LogWithContext("task.navershopping", logrus.WarnLevel, "작업 취소 요청이 감지되어 상품 정보 수집 프로세스를 중단합니다", logrus.Fields{
				"start_index":          startIndex,
				"total_fetched_so_far": len(pageContent.Items),
			}, nil)

			return nil, nil
		}

		t.LogWithContext("task.navershopping", logrus.DebugLevel, "네이버 쇼핑 검색 API 페이지를 요청합니다", logrus.Fields{
			"query":         commandSettings.Query,
			"start_index":   startIndex,
			"display_count": apiDisplayCount,
			"sort_option":   apiSortOption,
		}, nil)

		// @@@@@
		var currentPage = &searchResponse{}

		// 매 호출마다 쿼리 파라미터를 새로 설정하기 위해 복사본을 사용하거나,
		// Query() 메서드는 매번 새로운 Values 맵을 반환하므로 안전하게 제어합니다.
		u := *baseURL // 구조체 복사 (URL은 포인터 필드가 없으므로 값 복사 안전)
		q := u.Query()
		q.Set("query", commandSettings.Query)
		q.Set("display", strconv.Itoa(apiDisplayCount))
		q.Set("start", strconv.Itoa(startIndex))
		q.Set("sort", apiSortOption)
		u.RawQuery = q.Encode()

		err = tasksvc.FetchJSON(t.GetFetcher(), "GET", u.String(), header, nil, currentPage)
		if err != nil {
			return nil, err
		}

		if fetchedTotalCount == math.MaxInt {
			pageContent.Total = currentPage.Total
			pageContent.Start = currentPage.Start
			pageContent.Display = currentPage.Display

			fetchedTotalCount = currentPage.Total

			// 최대 1000건의 데이터를 읽어들이도록 한다.
			if pageContent.Total > policyFetchLimit {
				pageContent.Total = policyFetchLimit
				fetchedTotalCount = policyFetchLimit
			}
		}
		pageContent.Items = append(pageContent.Items, currentPage.Items...)

		startIndex += apiDisplayCount
	}

	// @@@@@
	// 데이터 필터링
	// Slice Pre-allocation: 결과 슬라이스의 용량을 미리 할당하여 재할당 오버헤드를 방지합니다.
	// 정확한 개수는 알 수 없으므로 최대 크기(검색 결과 수)만큼 할당하거나, 0부터 시작하되 capacity만 확보합니다.
	products := make([]*product, 0, len(pageContent.Items))
	includedKeywords := strutil.SplitAndTrim(commandSettings.Filters.IncludedKeywords, ",")
	excludedKeywords := strutil.SplitAndTrim(commandSettings.Filters.ExcludedKeywords, ",")

	for _, item := range pageContent.Items {
		if p := t.filterAndMapProduct(item, includedKeywords, excludedKeywords, commandSettings.Filters.PriceLessThan); p != nil {
			products = append(products, p)
		}
	}

	t.LogWithContext("task.navershopping", logrus.InfoLevel, "상품 정보 수집 및 필터링 프로세스가 완료되었습니다", logrus.Fields{
		"collected_count": len(products),
		"fetched_count":   len(pageContent.Items),
		"api_total_count": pageContent.Total,
	}, nil)

	return products, nil
}

// @@@@@
// filterAndMapProduct 검색 API의 원본 결과를 비즈니스 도메인 모델로 변환하고 필터링을 수행합니다.
func (t *task) filterAndMapProduct(item *searchResponseItem, includedKeywords, excludedKeywords []string, priceLessThan int) *product {
	// 1. 키워드 필터링
	if !tasksvc.Filter(item.Title, includedKeywords, excludedKeywords) {
		return nil
	}

	// 2. 가격 정보 파싱 (쉼표 제거 및 에러 처리)
	cleanPrice := strings.ReplaceAll(item.LowPrice, ",", "")
	lowPrice, err := strconv.Atoi(cleanPrice)
	if err != nil {
		t.LogWithContext("task.navershopping", logrus.WarnLevel, "상품 가격 파싱 실패", logrus.Fields{
			"title": item.Title,
			"price": item.LowPrice,
			"error": err,
		}, nil)
		return nil
	}

	// 3. 가격 조건 필터링 및 변환
	if lowPrice > 0 && lowPrice < priceLessThan {
		return &product{
			Title:       item.Title,
			Link:        item.Link,
			LowPrice:    lowPrice,
			MallName:    item.MallName,
			ProductID:   item.ProductID,
			ProductType: item.ProductType,
		}
	}

	return nil
}

// @@@@@
// diffAndNotify 현재 스냅샷과 이전 스냅샷을 비교하여 변경된 상품을 확인하고 알림 메시지를 생성합니다.
func (t *task) diffAndNotify(commandSettings *watchPriceSettings, currentSnapshot, prevSnapshot *watchPriceSnapshot, supportsHTML bool) (string, interface{}, error) {
	var sb strings.Builder
	lineSpacing := "\n\n"
	if supportsHTML {
		lineSpacing = "\n"
	}

	// 1. 이전 스냅샷이 있다면 Map으로 변환하여 조회 성능 최적화 (O(N))
	// Pre-allocation: 맵의 크기를 미리 할당하여 재할당 오버헤드를 방지합니다.
	var prevMap map[string]*product
	if prevSnapshot != nil {
		prevMap = make(map[string]*product, len(prevSnapshot.Products))
		for _, p := range prevSnapshot.Products {
			prevMap[p.Key()] = p
		}
	}

	// 2. 현재 상품 목록을 순회하며 변경 내역 확인
	for _, currentProduct := range currentSnapshot.Products {
		key := currentProduct.Key()
		prevProduct, exists := prevMap[key]

		if !exists {
			// 신규 상품 (New)
			if sb.Len() > 0 {
				sb.WriteString(lineSpacing)
			}
			sb.WriteString(currentProduct.String(supportsHTML, " 🆕"))
		} else {
			// 기존 상품: 가격 변동 확인
			if currentProduct.LowPrice != prevProduct.LowPrice {
				if sb.Len() > 0 {
					sb.WriteString(lineSpacing)
				}
				// Stale Link Protection: 링크나 상품명이 변경되었을 수 있으므로,
				// 알림 메시지는 최신 정보(currentProduct)를 기준으로 생성하고,
				// 가격 변동 내역만 과거 가격(prevProduct.LowPrice)을 참조하여 표시합니다.
				sb.WriteString(currentProduct.String(supportsHTML, fmt.Sprintf(" (전: %s원) 🔁", strutil.FormatCommas(prevProduct.LowPrice))))
			}
		}
	}

	filtersDescription := fmt.Sprintf("조회 조건은 아래와 같습니다:\n• 검색 키워드 : %s\n• 상풍명 포함 키워드 : %s\n• 상품명 제외 키워드 : %s\n• %s원 미만의 상품", commandSettings.Query, commandSettings.Filters.IncludedKeywords, commandSettings.Filters.ExcludedKeywords, strutil.FormatCommas(commandSettings.Filters.PriceLessThan))

	var message string
	var changedTaskResultData interface{}

	if sb.Len() > 0 {
		message = fmt.Sprintf("조회 조건에 해당되는 상품의 정보가 변경되었습니다.\n\n%s\n\n%s", filtersDescription, sb.String())
		changedTaskResultData = currentSnapshot
	} else {
		// 사용자가 수동으로 실행한 경우, 변경 사항이 없더라도 현재 상태를 알려줌
		if t.GetRunBy() == tasksvc.RunByUser {
			if len(currentSnapshot.Products) == 0 {
				message = fmt.Sprintf("조회 조건에 해당되는 상품이 존재하지 않습니다.\n\n%s", filtersDescription)
			} else {
				for _, p := range currentSnapshot.Products {
					if sb.Len() > 0 {
						sb.WriteString(lineSpacing)
					}
					sb.WriteString(p.String(supportsHTML, ""))
				}

				message = fmt.Sprintf("조회 조건에 해당되는 상품의 변경된 정보가 없습니다.\n\n%s\n\n조회 조건에 해당되는 상품은 아래와 같습니다:\n\n%s", filtersDescription, sb.String())
			}
		}
	}

	return message, changedTaskResultData, nil
}
