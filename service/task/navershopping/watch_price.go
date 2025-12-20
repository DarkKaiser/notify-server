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
	// watchPriceAnyCommandPrefix는 동적 커맨드 라우팅을 위한 식별자 접두어입니다.
	//
	// 이 접두어로 시작하는 모든 CommandID는 `executeWatchPrice` 핸들러로 라우팅되어 처리됩니다.
	// 이를 통해 사용자는 "WatchPrice_Apple", "WatchPrice_Samsung" 등과 같이
	// 하나의 로직으로 처리되는 다수의 커맨드를 유연하게 생성할 수 있습니다.
	watchPriceAnyCommandPrefix = "WatchPrice_"

	// searchAPIURL은 네이버 쇼핑 상품 검색을 위한 OpenAPI 엔드포인트입니다.
	// 공식 문서: https://developers.naver.com/docs/serviceapi/search/shopping/shopping.md
	searchAPIURL = "https://openapi.naver.com/v1/search/shop.json"

	// API 매개변수 상수
	//
	// paramSortOrder: 검색 결과 정렬 기준 (sim: 유사도순, date: 날짜순, asc: 가격오름차순, dsc: 가격내림차순)
	paramSortOrder = "sim"
	// paramMaxSearchItemCount: 1회 요청 시 반환받을 검색 결과의 최대 개수 (API 제한: 10~100)
	paramMaxSearchItemCount = 100
	// paramMaxTotalSearchLimit: 수집할 최대 상품 개수 제한 (과도한 요청 방지)
	paramMaxTotalSearchLimit = 1000
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
	if strings.TrimSpace(s.Query) == "" {
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
	Title       string `json:"title"`
	Link        string `json:"link"`
	LowPrice    int    `json:"lprice"`
	MallName    string `json:"mallName"`
	ProductID   string `json:"productId"`
	ProductType string `json:"productType"`
}

// Key 상품을 고유하게 식별하기 위한 키를 반환합니다.
// Link는 추적 파라미터 등으로 인해 변할 수 있으므로, 불변 값인 ProductID를 사용합니다.
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

type searchResponseItem struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	LowPrice    string `json:"lprice"`
	MallName    string `json:"mallName"`
	ProductID   string `json:"productId"`
	ProductType string `json:"productType"`
}

type searchResponse struct {
	Total   int                   `json:"total"`
	Start   int                   `json:"start"`
	Display int                   `json:"display"`
	Items   []*searchResponseItem `json:"items"`
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

	// 2. 변경 내역 비교 및 알림 생성
	return t.diffAndNotify(commandSettings, currentSnapshot, prevSnapshot, supportsHTML)
}

func (t *task) fetchProducts(commandSettings *watchPriceSettings) ([]*product, error) {
	var (
		header = map[string]string{
			"X-Naver-Client-Id":     t.clientID,
			"X-Naver-Client-Secret": t.clientSecret,
		}
		searchResultItemStartNo    = 1
		searchResultItemTotalCount = math.MaxInt

		searchResultData = &searchResponse{}
	)

	// API 호출 및 데이터 수집
	// Loop Invariant: URL 파싱은 루프 밖에서 한 번만 수행합니다.
	parsedURL, err := url.Parse(searchAPIURL)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.Internal, "검색 URL 파싱 실패")
	}

	for searchResultItemStartNo < searchResultItemTotalCount {
		var _searchResultData_ = &searchResponse{}

		// 매 호출마다 쿼리 파라미터를 새로 설정하기 위해 복사본을 사용하거나,
		// Query() 메서드는 매번 새로운 Values 맵을 반환하므로 안전하게 제어합니다.
		u := *parsedURL // 구조체 복사 (URL은 포인터 필드가 없으므로 값 복사 안전)
		q := u.Query()
		q.Set("query", commandSettings.Query)
		q.Set("display", strconv.Itoa(paramMaxSearchItemCount))
		q.Set("start", strconv.Itoa(searchResultItemStartNo))
		q.Set("sort", paramSortOrder)
		u.RawQuery = q.Encode()

		err = tasksvc.FetchJSON(t.GetFetcher(), "GET", u.String(), header, nil, _searchResultData_)
		if err != nil {
			return nil, err
		}

		if searchResultItemTotalCount == math.MaxInt {
			searchResultData.Total = _searchResultData_.Total
			searchResultData.Start = _searchResultData_.Start
			searchResultData.Display = _searchResultData_.Display

			searchResultItemTotalCount = _searchResultData_.Total

			// 최대 1000건의 데이터를 읽어들이도록 한다.
			if searchResultData.Total > paramMaxTotalSearchLimit {
				searchResultData.Total = paramMaxTotalSearchLimit
				searchResultItemTotalCount = paramMaxTotalSearchLimit
			}
		}
		searchResultData.Items = append(searchResultData.Items, _searchResultData_.Items...)

		searchResultItemStartNo += paramMaxSearchItemCount
	}

	// 데이터 필터링
	// Slice Pre-allocation: 결과 슬라이스의 용량을 미리 할당하여 재할당 오버헤드를 방지합니다.
	// 정확한 개수는 알 수 없으므로 최대 크기(검색 결과 수)만큼 할당하거나, 0부터 시작하되 capacity만 확보합니다.
	products := make([]*product, 0, len(searchResultData.Items))
	includedKeywords := strutil.SplitAndTrim(commandSettings.Filters.IncludedKeywords, ",")
	excludedKeywords := strutil.SplitAndTrim(commandSettings.Filters.ExcludedKeywords, ",")

	for _, item := range searchResultData.Items {
		if !tasksvc.Filter(item.Title, includedKeywords, excludedKeywords) {
			continue
		}

		// 가격 정보 파싱 (쉼표 제거 및 에러 처리)
		cleanPrice := strings.ReplaceAll(item.LowPrice, ",", "")
		lowPrice, err := strconv.Atoi(cleanPrice)
		if err != nil {
			t.LogWithContext("task.navershopping", logrus.WarnLevel, "상품 가격 파싱 실패", logrus.Fields{
				"title": item.Title,
				"price": item.LowPrice,
				"error": err,
			}, nil)
			continue
		}

		if lowPrice > 0 && lowPrice < commandSettings.Filters.PriceLessThan {
			products = append(products, &product{
				Title:       item.Title,
				Link:        item.Link,
				LowPrice:    lowPrice,
				MallName:    item.MallName,
				ProductID:   item.ProductID,
				ProductType: item.ProductType,
			})
		}
	}

	return products, nil
}

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
				sb.WriteString(prevProduct.String(supportsHTML, fmt.Sprintf(" ⇒ %s원 🔁", strutil.FormatCommas(currentProduct.LowPrice))))
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
