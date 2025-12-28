package navershopping

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	tasksvc "github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/darkkaiser/notify-server/pkg/strutil"
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

	// allocSizePerProduct 알림 메시지 생성 시, 단일 상품 정보를 렌더링하는 데 필요한 예상 버퍼 크기(Byte)입니다.
	//
	// 이 상수는 `strings.Builder.Grow()`를 통해 내부 버퍼를 선제적으로 확보(Pre-allocation)하는 데 사용됩니다.
	// 적절한 초기 용량을 설정함으로써, 메시지 조합 과정에서 발생하는 불필요한 슬라이스 재할당(Reallocation)과
	// 데이터 복사(Memory Copy) 비용을 최소화하여 렌더링 성능을 최적화합니다.
	allocSizePerProduct = 300

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

// productEventType 상품 데이터의 상태 변화(변경 유형)를 식별하기 위한 열거형입니다.
type productEventType int

const (
	eventNone         productEventType = iota
	eventNewProduct                    // 신규 상품 (이전 검색 결과에 없던 상품)
	eventPriceChanged                  // 가격 변동 (이전과 동일 상품이나 최저가 변동)
)

// productDiff 상품 데이터의 변동 사항(신규, 가격 변화 등)을 캡슐화한 중간 객체입니다.
type productDiff struct {
	Type    productEventType
	Product *product
	Prev    *product
}

// searchResponse 네이버 쇼핑 검색 API의 응답 데이터를 담는 구조체입니다.
type searchResponse struct {
	Total   int                   `json:"total"`   // 검색된 전체 상품의 총 개수 (페이징 처리에 사용)
	Start   int                   `json:"start"`   // 검색 시작 위치 (1부터 시작하는 인덱스)
	Display int                   `json:"display"` // 현재 응답에 포함된 상품 개수 (요청한 display 값과 같거나 작음)
	Items   []*searchResponseItem `json:"items"`   // 검색된 개별 상품 리스트
}

// searchResponseItem 네이버 쇼핑 검색 API 응답에서 개별 상품 정보를 담는 로우(Raw) 데이터 구조체입니다.
type searchResponseItem struct {
	ProductID   string `json:"productId"`   // 네이버 쇼핑 상품 ID (상품 고유 식별자)
	ProductType string `json:"productType"` // 상품 유형 (1: 일반, 2: 중고, 3: 단종, 4: 판매예정 등)
	Title       string `json:"title"`       // 상품명 (HTML 태그 <b>가 포함된 원본 문자열)
	Link        string `json:"link"`        // 상품 상세 정보 페이지 URL
	LowPrice    string `json:"lprice"`      // 판매 최저가 (단위: 원)
	MallName    string `json:"mallName"`    // 판매 쇼핑몰 상호 (예: "네이버", "쿠팡" 등)
}

// executeWatchPrice 작업을 실행하여 상품 가격 정보를 확인합니다.
func (t *task) executeWatchPrice(commandSettings *watchPriceSettings, prevSnapshot *watchPriceSnapshot, supportsHTML bool) (string, interface{}, error) {
	// 1. 상품 정보를 수집한다.
	currentProducts, err := t.fetchProducts(commandSettings)
	if err != nil {
		return "", nil, err
	}

	currentSnapshot := &watchPriceSnapshot{
		Products: currentProducts,
	}

	// 2. 빠른 조회를 위해 이전 상품 목록을 Map으로 변환한다.
	prevProductsMap := make(map[string]*product)
	if prevSnapshot != nil {
		for _, p := range prevSnapshot.Products {
			prevProductsMap[p.Key()] = p
		}
	}

	// 3. 신규 상품 확인 및 알림 메시지 생성
	message, shouldSave := t.analyzeAndReport(commandSettings, currentSnapshot, prevProductsMap, supportsHTML)

	if shouldSave {
		// "변경 사항이 있다면(shouldSave=true), 반드시 알림 메시지도 존재해야 한다"는 규칙을 확인합니다.
		// 만약 메시지 없이 데이터만 갱신되면, 사용자는 변경 사실을 영영 모르게 될 수 있습니다.
		// 이를 방지하기 위해, 이런 비정상적인 상황에서는 저장을 차단하고 즉시 로그를 남깁니다.
		if message == "" {
			t.LogWithContext("task.navershopping", logrus.WarnLevel, "변경 사항 감지 후 저장 프로세스를 시도했으나, 알림 메시지가 비어있습니다 (저장 건너뜀)", nil, nil)
			return "", nil, nil
		}

		return message, currentSnapshot, nil
	}

	return message, nil, nil
}

// fetchProducts 네이버 쇼핑 검색 API를 호출하여 조건에 맞는 상품 목록을 수집합니다.
func (t *task) fetchProducts(commandSettings *watchPriceSettings) ([]*product, error) {
	var (
		header = map[string]string{
			"X-Naver-Client-Id":     t.clientID,
			"X-Naver-Client-Secret": t.clientSecret,
		}

		startIndex       = 1
		targetFetchCount = math.MaxInt

		pageContent = &searchResponse{}
	)

	// API 호출을 위한 기본 URL을 파싱합니다.
	// 반복문 내에서 불필요한 URL 파싱(`url.Parse`) 오버헤드를 방지하기 위해 루프 진입 전에 수행합니다.
	// 파싱된 `baseURL` 객체는 루프 내에서 값 복사되어 안전하게 쿼리 파라미터를 조작하는 데 사용됩니다.
	baseURL, err := url.Parse(searchAPIURL)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.Internal, "네이버 쇼핑 검색 API 엔드포인트 URL 파싱에 실패하였습니다")
	}

	for startIndex <= targetFetchCount {
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

		// `baseURL`은 루프 불변 템플릿으로, 파싱 비용을 절감하는 동시에 상태 격리를 보장합니다.
		// 구조체 역참조(*baseURL)를 통한 값 복사(Value Copy)는 매 반복마다 깨끗한(Clean) 상태를 보장하며,
		// 이는 이전 루프의 쿼리 파라미터 잔여물(Residue)이 현재 요청에 영향을 주는 Side-Effect를 완벽하게 차단합니다.
		u := *baseURL // 구조체 복사 (URL은 포인터 필드가 없으므로 값 복사 안전)
		q := u.Query()
		q.Set("query", commandSettings.Query)
		q.Set("display", strconv.Itoa(apiDisplayCount))
		q.Set("start", strconv.Itoa(startIndex))
		q.Set("sort", apiSortOption)
		u.RawQuery = q.Encode()

		var currentPage = &searchResponse{}
		err = tasksvc.FetchJSON(t.GetFetcher(), "GET", u.String(), header, nil, currentPage)
		if err != nil {
			return nil, err
		}

		// 첫 번째 페이지 응답을 수신한 시점에 전체 수집 계획을 확정합니다.
		if targetFetchCount == math.MaxInt {
			// API가 반환한 원본 메타데이터(Total, Start, Display)를 결과 객체에 보존합니다.
			// 이는 로직 처리와 무관하게 "실제 검색 결과 현황"을 정확히 기록하기 위함입니다.
			pageContent.Total = currentPage.Total
			pageContent.Start = currentPage.Start
			pageContent.Display = currentPage.Display

			// 기본적으로 검색된 모든 상품을 수집 대상으로 설정합니다.
			targetFetchCount = currentPage.Total

			// 과도한 API 요청을 방지하기 위해 내부 정책(`policyFetchLimit`)에 따라 수집 상한선을 적용합니다.
			if targetFetchCount > policyFetchLimit {
				targetFetchCount = policyFetchLimit
			}
		}

		// 현재 페이지의 상품 목록을 전체 결과 슬라이스에 병합합니다.
		pageContent.Items = append(pageContent.Items, currentPage.Items...)

		startIndex += apiDisplayCount
	}

	// 수집된 결과가 없는 경우, 불필요한 슬라이스 할당(`make`)과 후속 키워드 매칭 로직을 건너뛰고 즉시 종료합니다.
	if len(pageContent.Items) == 0 {
		t.LogWithContext("task.navershopping", logrus.InfoLevel, "상품 정보 수집 및 키워드 매칭 프로세스가 완료되었습니다 (검색 결과 없음)", logrus.Fields{
			"collected_count": 0,
			"fetched_count":   0,
			"api_total_count": pageContent.Total,
			"api_start":       pageContent.Start,
			"api_display":     pageContent.Display,
		}, nil)

		return nil, nil
	}

	// 키워드 매칭을 위한 Matcher를 생성합니다.
	// 반복문 내부에서 파싱 비용을 절감하기 위해 루프 진입 전에 미리 생성합니다.
	includedKeywords := strutil.SplitAndTrim(commandSettings.Filters.IncludedKeywords, ",")
	excludedKeywords := strutil.SplitAndTrim(commandSettings.Filters.ExcludedKeywords, ",")
	matcher := strutil.NewKeywordMatcher(includedKeywords, excludedKeywords)

	// 결과 슬라이스의 용량(Capacity)을 원본 데이터 크기만큼 미리 확보합니다.
	// 키워드 매칭으로 인해 실제 크기는 이보다 작을 수 있지만, Go 슬라이스의 동적 확장(Dynamic Resizing) 및
	// 메모리 재할당/복사(Reallocation & Copy) 비용을 완전히 제거하여 성능을 최적화합니다.
	products := make([]*product, 0, len(pageContent.Items))

	for _, item := range pageContent.Items {
		// 키워드 매칭 검사 전에 HTML 태그를 제거합니다.
		// 네이버 검색 API는 매칭된 키워드를 <b> 태그로 감싸서 반환하므로,
		// 이를 제거해야 정확한 키워드 매칭(특히 제외 키워드)이 가능합니다.
		plainTitle := strutil.StripHTMLTags(item.Title)

		if !matcher.Match(plainTitle) {
			continue
		}

		if p := t.mapToProduct(item); p != nil {
			if t.isPriceEligible(p.LowPrice, commandSettings.Filters.PriceLessThan) {
				products = append(products, p)
			}
		}
	}

	t.LogWithContext("task.navershopping", logrus.InfoLevel, "상품 정보 수집 및 키워드 매칭 프로세스가 완료되었습니다", logrus.Fields{
		"collected_count": len(products),
		"fetched_count":   len(pageContent.Items),
		"api_total_count": pageContent.Total,
		"api_start":       pageContent.Start,
		"api_display":     pageContent.Display,
	}, nil)

	return products, nil
}

// mapToProduct 검색 결과 항목을 도메인 모델로 변환합니다.
func (t *task) mapToProduct(item *searchResponseItem) *product {
	// 가격 정보 파싱 (쉼표 제거)
	cleanPrice := strings.ReplaceAll(item.LowPrice, ",", "")
	lowPrice, err := strconv.Atoi(cleanPrice)
	if err != nil {
		t.LogWithContext("task.navershopping", logrus.WarnLevel, "상품 가격 데이터의 형식이 유효하지 않아 파싱할 수 없습니다 (해당 상품 건너뜀)", logrus.Fields{
			"product_id":      item.ProductID,
			"product_type":    item.ProductType,
			"title":           item.Title,
			"raw_price_value": item.LowPrice,
			"clean_price":     cleanPrice,
			"parse_error":     err.Error(),
		}, nil)

		return nil
	}

	return &product{
		ProductID:   item.ProductID,
		ProductType: item.ProductType,
		Title:       strutil.StripHTMLTags(item.Title), // HTML 태그 제거
		Link:        item.Link,
		LowPrice:    lowPrice,
		MallName:    item.MallName,
	}
}

// isPriceEligible 상품의 가격이 설정된 조건(상한가)에 부합하는지 검사합니다.
func (t *task) isPriceEligible(price, priceLessThan int) bool {
	// 0원 이하(유효하지 않은 가격) 또는 상한가 이상인 경우 제외
	return price > 0 && price < priceLessThan
}

// analyzeAndReport 수집된 데이터를 분석하여 사용자에게 보낼 알림 메시지를 생성합니다.
//
// [주요 동작]
// 1. 변화 확인: 이전 데이터와 비교해 새로운 상품이나 가격 변동이 있는지 확인합니다.
// 2. 메시지 작성: 발견된 변화를 보기 좋게 포맷팅합니다.
// 3. 알림 결정:
//   - 스케줄러 실행: 변화가 있을 때만 알림을 보냅니다. (조용히 모니터링)
//   - 사용자 실행: 변화가 없어도 "변경 없음"이라고 알려줍니다. (확실한 피드백)
func (t *task) analyzeAndReport(commandSettings *watchPriceSettings, currentSnapshot *watchPriceSnapshot, prevProductsMap map[string]*product, supportsHTML bool) (message string, shouldSave bool) {
	// 신규 상품 및 가격 변동을 식별합니다.
	// (단순 비교뿐만 아니라, 사용자 편의를 위한 정렬 로직이 포함됩니다)
	diffs := t.calculateProductDiffs(currentSnapshot, prevProductsMap)

	// 식별된 변동 사항을 사용자가 이해하기 쉬운 알림 메시지로 변환합니다.
	diffMessage := t.renderProductDiffs(diffs, supportsHTML)

	// 변경 내역(New/Price Change)이 집계된 경우, 즉시 알림 메시지를 구성하여 반환합니다.
	if len(diffs) > 0 {
		searchConditionsSummary := t.buildSearchConditionsSummary(commandSettings)

		return fmt.Sprintf("조회 조건에 해당되는 상품 정보가 변경되었습니다.\n\n%s\n\n%s", searchConditionsSummary, diffMessage), true
	}

	// 스케줄러(Scheduler)에 의한 자동 실행이 아닌, 사용자 요청에 의한 수동 실행인 경우입니다.
	//
	// 자동 실행 시에는 변경 사항이 없으면 불필요한 알림(Noise)을 방지하기 위해 침묵하지만,
	// 수동 실행 시에는 "변경 없음"이라는 명시적인 피드백을 제공하여 시스템이 정상 동작 중임을 사용자가 인지할 수 있도록 합니다.
	if t.GetRunBy() == tasksvc.RunByUser {
		searchConditionsSummary := t.buildSearchConditionsSummary(commandSettings)

		if len(currentSnapshot.Products) == 0 {
			return fmt.Sprintf("조회 조건에 해당되는 상품이 존재하지 않습니다.\n\n%s", searchConditionsSummary), false
		}

		var sb strings.Builder

		// 예상 메시지 크기로 초기 용량 할당 (상수 기반 최적화)
		sb.Grow(len(currentSnapshot.Products) * allocSizePerProduct)

		for i, p := range currentSnapshot.Products {
			if i > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(p.Render(supportsHTML, ""))
		}

		return fmt.Sprintf("조회 조건에 해당되는 상품의 변경된 정보가 없습니다.\n\n%s\n\n조회 조건에 해당되는 상품은 아래와 같습니다:\n\n%s", searchConditionsSummary, sb.String()), false
	}

	return "", false
}

// calculateProductDiffs 현재 스냅샷과 이전 스냅샷을 비교하여 신규 상품이나 가격 변동을 찾아냅니다.
// 즉, 이전에 없던 새로운 상품이 발견되거나 가격이 바뀐 경우 이를 결과 목록에 담아 반환합니다.
func (t *task) calculateProductDiffs(currentSnapshot *watchPriceSnapshot, prevProductsMap map[string]*product) []productDiff {
	// 상품 목록을 가격 오름차순으로 정렬하여 사용자가 가장 저렴한 상품을 먼저 확인할 수 있도록 합니다.
	// 가격이 동일한 경우, 일관된 순서를 보장하기 위해 상품명으로 2차 정렬을 수행합니다.
	sort.Slice(currentSnapshot.Products, func(i, j int) bool {
		p1 := currentSnapshot.Products[i]
		p2 := currentSnapshot.Products[j]

		if p1.LowPrice != p2.LowPrice {
			return p1.LowPrice < p2.LowPrice
		}

		// 가격이 같으면 이름순으로 정렬 (안정성 확보)
		return p1.Title < p2.Title
	})

	var diffs []productDiff

	for _, currrentProduct := range currentSnapshot.Products {
		prevProduct, exists := prevProductsMap[currrentProduct.Key()]

		if !exists {
			// 이전 스냅샷에 존재하지 않는 상품 키(ProductID)가 감지되었습니다.
			// 이는 새로운 상품이 등록되었거나, 검색 순위 진입 등으로 수집 범위에 새롭게 포함된 경우입니다.
			diffs = append(diffs, productDiff{
				Type:    eventNewProduct,
				Product: currrentProduct,
				Prev:    nil,
			})
		} else {
			// 동일한 상품이 이전에도 존재했으나, 최저가가 변경되었습니다.
			// 단순 재수집된 경우는 무시하고, 실제 가격 변화가 발생한 경우에만 알림을 생성합니다.
			if currrentProduct.LowPrice != prevProduct.LowPrice {
				diffs = append(diffs, productDiff{
					Type:    eventPriceChanged,
					Product: currrentProduct,
					Prev:    prevProduct,
				})
			}
		}
	}

	return diffs
}

// renderProductDiffs 찾아낸 변동 사항(신규, 가격대)을 사용자가 보기 편한 알림 메시지로 변환합니다.
func (t *task) renderProductDiffs(diffs []productDiff, supportsHTML bool) string {
	if len(diffs) == 0 {
		return ""
	}

	var sb strings.Builder

	// 예상 메시지 크기로 초기 용량 할당 (상수 기반 최적화)
	sb.Grow(len(diffs) * allocSizePerProduct)

	for i, diff := range diffs {
		if i > 0 {
			sb.WriteString("\n\n")
		}

		switch diff.Type {
		case eventNewProduct:
			sb.WriteString(diff.Product.Render(supportsHTML, mark.New))
		case eventPriceChanged:
			sb.WriteString(diff.Product.RenderDiff(supportsHTML, mark.Change, diff.Prev))
		}
	}

	return sb.String()
}

// buildSearchConditionsSummary 사용자가 설정한 조회 조건(검색어, 필터 등)을 요약하여 문자열로 반환합니다.
func (t *task) buildSearchConditionsSummary(commandSettings *watchPriceSettings) string {
	return fmt.Sprintf(`조회 조건은 아래와 같습니다:

  • 검색 키워드 : %s
  • 상품명 포함 키워드 : %s
  • 상품명 제외 키워드 : %s
  • %s원 미만의 상품`,
		commandSettings.Query,
		commandSettings.Filters.IncludedKeywords,
		commandSettings.Filters.ExcludedKeywords,
		strutil.FormatCommas(commandSettings.Filters.PriceLessThan),
	)
}
