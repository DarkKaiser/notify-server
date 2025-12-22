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

	// newProductMark 신규 상품 알림 메시지에 표시될 강조 마크입니다.
	newProductMark = " 🆕"

	// changeProductPriceMark 가격 변동 알림 메시지에 표시될 강조 마크입니다.
	changeProductPriceMark = " 🔁"

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

	// 수집된 결과가 없는 경우, 불필요한 슬라이스 할당(`make`)과 후속 필터링 로직을 건너뛰고 즉시 종료합니다.
	if len(pageContent.Items) == 0 {
		t.LogWithContext("task.navershopping", logrus.InfoLevel, "상품 정보 수집 및 필터링 프로세스가 완료되었습니다 (검색 결과 없음)", logrus.Fields{
			"collected_count": 0,
			"fetched_count":   0,
			"api_total_count": pageContent.Total,
			"api_start":       pageContent.Start,
			"api_display":     pageContent.Display,
		}, nil)

		return nil, nil
	}

	// 키워드 필터링 조건을 사전 파싱합니다.
	includedKeywords := strutil.SplitAndTrim(commandSettings.Filters.IncludedKeywords, ",")
	excludedKeywords := strutil.SplitAndTrim(commandSettings.Filters.ExcludedKeywords, ",")

	// 결과 슬라이스의 용량(Capacity)을 원본 데이터 크기만큼 미리 확보합니다.
	// 필터링으로 인해 실제 크기는 이보다 작을 수 있지만, Go 슬라이스의 동적 확장(Dynamic Resizing) 및
	// 메모리 재할당/복사(Reallocation & Copy) 비용을 완전히 제거하여 성능을 최적화합니다.
	products := make([]*product, 0, len(pageContent.Items))

	for _, item := range pageContent.Items {
		if !tasksvc.Filter(item.Title, includedKeywords, excludedKeywords) {
			continue
		}

		p := t.mapToProduct(item)
		if p == nil {
			continue
		}

		if t.isPriceEligible(p.LowPrice, commandSettings.Filters.PriceLessThan) {
			products = append(products, p)
		}
	}

	t.LogWithContext("task.navershopping", logrus.InfoLevel, "상품 정보 수집 및 필터링 프로세스가 완료되었습니다", logrus.Fields{
		"collected_count": len(products),
		"fetched_count":   len(pageContent.Items),
		"api_total_count": pageContent.Total,
		"api_start":       pageContent.Start,
		"api_display":     pageContent.Display,
	}, nil)

	return products, nil
}

// mapToProduct 검색 API의 원본 결과를 비즈니스 도메인 모델로 변환합니다.
func (t *task) mapToProduct(item *searchResponseItem) *product {
	// 가격 정보 파싱 (쉼표 제거)
	cleanPrice := strings.ReplaceAll(item.LowPrice, ",", "")
	lowPrice, err := strconv.Atoi(cleanPrice)
	if err != nil {
		t.LogWithContext("task.navershopping", logrus.DebugLevel, "상품 가격 데이터의 형식이 유효하지 않아 파싱할 수 없습니다 (해당 상품 건너뜀)", logrus.Fields{
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

		Title:    item.Title,
		Link:     item.Link,
		LowPrice: lowPrice,
		MallName: item.MallName,
	}
}

// isPriceEligible 상품의 가격이 설정된 조건(상한가)에 부합하는지 검사합니다.
func (t *task) isPriceEligible(price, priceLessThan int) bool {
	// 0원 이하(유효하지 않은 가격) 또는 상한가 이상인 경우 제외
	return price > 0 && price < priceLessThan
}

// diffAndNotify 현재 스냅샷과 이전 스냅샷을 비교하여 변경된 상품을 확인하고 알림 메시지를 생성합니다.
func (t *task) diffAndNotify(commandSettings *watchPriceSettings, currentSnapshot, prevSnapshot *watchPriceSnapshot, supportsHTML bool) (string, interface{}, error) {
	// 예상 메시지 크기로 초기 용량 할당 (상품당 약 400바이트 추정)
	var sb strings.Builder
	if len(currentSnapshot.Products) > 0 {
		sb.Grow(len(currentSnapshot.Products) * 400)
	}

	// 최초 실행 시에는 이전 스냅샷이 존재하지 않아 nil 상태일 수 있습니다.
	// 따라서 비교 대상을 명시적으로 nil(또는 빈 슬라이스)로 처리하여,
	// 1. nil 포인터 역참조(Nil Pointer Dereference)로 인한 런타임 패닉을 방지하고 (Safety)
	// 2. 현재 수집된 모든 상품 정보를 '신규'로 식별되도록 유도합니다. (Logic)
	var prevProducts []*product
	if prevSnapshot != nil {
		prevProducts = prevSnapshot.Products
	}

	// 빠른 조회를 위해 이전 상품 목록을 Map으로 변환한다.
	prevMap := make(map[string]*product, len(prevProducts))
	for _, p := range prevProducts {
		prevMap[p.Key()] = p
	}

	// 현재 상품 목록을 순회하며 신규 상품을 식별한다.
	lineSpacing := "\n\n"
	for _, p := range currentSnapshot.Products {
		prevProduct, exists := prevMap[p.Key()]

		if !exists {
			// 이전 스냅샷에 존재하지 않는 상품 키(ProductID)가 감지되었습니다.
			// 이는 새로운 상품이 등록되었거나, 검색 순위 진입 등으로 수집 범위에 새롭게 포함된 경우입니다.
			if sb.Len() > 0 {
				sb.WriteString(lineSpacing)
			}
			sb.WriteString(p.String(supportsHTML, newProductMark))
		} else {
			// 동일한 상품(Key 일치)이 이전에도 존재했으나, 최저가(LowPrice)가 변경되었습니다.
			// 단순 재수집된 경우는 무시하고, 실제 가격 변화가 발생한 경우에만 알림을 생성합니다.
			if p.LowPrice != prevProduct.LowPrice {
				if sb.Len() > 0 {
					sb.WriteString(lineSpacing)
				}

				sb.WriteString(p.String(supportsHTML, fmt.Sprintf(" (이전: %s원)%s", strutil.FormatCommas(prevProduct.LowPrice), changeProductPriceMark)))
			}
		}
	}

	// [알림 메시지 상단 요약 메시지]
	// 사용자가 알림을 받았을 때, 이 결과가 '어떤 조건'에 의해 필터링된 것인지 즉시 파악할 수 있도록 돕습니다.
	searchConditionsSummary := fmt.Sprintf(`조회 조건은 아래와 같습니다:
• 검색 키워드 : %s
• 상품명 포함 키워드 : %s
• 상품명 제외 키워드 : %s
• %s원 미만의 상품`,
		commandSettings.Query,
		commandSettings.Filters.IncludedKeywords,
		commandSettings.Filters.ExcludedKeywords,
		strutil.FormatCommas(commandSettings.Filters.PriceLessThan),
	)

	// [알림 메시지 생성 및 반환]
	// 변경 내역(New/Price Change)이 집계된 경우(sb.Len() > 0), 즉시 알림 메시지를 구성하여 반환합니다.
	if sb.Len() > 0 {
		return fmt.Sprintf("조회 조건에 해당되는 상품의 정보가 변경되었습니다.\n\n%s\n\n%s",
				searchConditionsSummary,
				sb.String()),
			currentSnapshot,
			nil
	}

	// 스케줄러(Scheduler)에 의한 자동 실행이 아닌, 사용자 요청에 의한 수동 실행인 경우입니다.
	//
	// 자동 실행 시에는 변경 사항이 없으면 불필요한 알림(Noise)을 방지하기 위해 침묵하지만,
	// 수동 실행 시에는 "변경 없음"이라는 명시적인 피드백을 제공하여 시스템이 정상 동작 중임을 사용자가 인지할 수 있도록 합니다.
	if t.GetRunBy() == tasksvc.RunByUser {
		if len(currentSnapshot.Products) == 0 {
			return fmt.Sprintf("조회 조건에 해당되는 상품이 존재하지 않습니다.\n\n%s",
					searchConditionsSummary),
				nil,
				nil
		}

		for _, p := range currentSnapshot.Products {
			if sb.Len() > 0 {
				sb.WriteString(lineSpacing)
			}
			sb.WriteString(p.String(supportsHTML, ""))
		}

		return fmt.Sprintf("조회 조건에 해당되는 상품의 변경된 정보가 없습니다.\n\n%s\n\n조회 조건에 해당되는 상품은 아래와 같습니다:\n\n%s",
				searchConditionsSummary,
				sb.String()),
			nil,
			nil
	}

	return "", nil, nil
}
