package navershopping

import (
	"context"
	"math"
	"net/url"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

const (
	// watchPriceCommandPrefix 상품 가격 감시(WatchPrice) 계열 커맨드를 식별하기 위한
	// 와일드카드 접두어입니다.
	//
	// 동작 방식:
	//   커맨드 등록 시 이 접두어가 핸들러 매핑 키로 사용되며, 런타임에 CommandID가
	//   이 접두어로 시작하는지를 검사하여 모두 동일한 executeWatchPrice 핸들러로 라우팅합니다.
	//   덕분에 설정 파일에서 접두어 뒤에 자유로운 식별자를 붙여 감시 대상별로
	//   독립적인 커맨드를 손쉽게 정의할 수 있습니다.
	//
	// 설정 파일 사용 예시:
	//   - "WatchPrice_Apple"   → 애플 관련 제품 가격 감시
	//   - "WatchPrice_Samsung" → 삼성 관련 제품 가격 감시
	watchPriceCommandPrefix = "WatchPrice_"

	// policyMaxFetchCount 커맨드 1회 실행 시 네이버 쇼핑 API에서 수집할 수 있는 최대 상품 수입니다.
	//
	// 검색 결과가 이 값을 초과하더라도 수집은 이 시점에서 중단됩니다.
	// 단일 실행에서 과도한 API 호출이 발생하는 것을 방지하기 위한 안전 장치입니다.
	policyMaxFetchCount = 1000
)

// @@@@@
// executeWatchPrice 네이버 쇼핑 API로 상품 가격 정보를 수집하고, 이전 실행 시점의 스냅샷과 비교하여
// 변경 사항이 있을 경우 사용자에게 전달할 알림 메시지를 생성합니다.
//
// 매개변수:
//   - commandSettings: 검색 키워드, 가격 필터 등 이 커맨드의 실행 설정
//   - prevSnapshot: 직전 실행 결과 (최초 실행 시 nil이며, 이 경우 모든 수집 결과가 '신규'로 처리됨)
//   - supportsHTML: 알림 메시지를 HTML 형식으로 렌더링할지 여부
//
// 반환값:
//   - message: 사용자에게 보낼 알림 메시지 (변경 없음 또는 작업 취소 시 빈 문자열)
//   - newSnapshot: 저장할 스냅샷 (변경 감지 시 현재 스냅샷, 변경 없으면 nil)
//   - error: 실행 중 발생한 오류
func (t *task) executeWatchPrice(ctx context.Context, commandSettings *watchPriceSettings, prevSnapshot *watchPriceSnapshot, supportsHTML bool) (string, interface{}, error) {
	// 1단계: 네이버 쇼핑 API를 페이지 단위로 호출하여 현재 시점의 상품 목록을 수집합니다.
	// 결과는 이후 단계에서 이전 스냅샷과의 비교 대상(currentSnapshot)이 됩니다.
	currentProducts, err := t.fetchProducts(ctx, commandSettings)
	if err != nil {
		return "", nil, err
	}

	currentSnapshot := &watchPriceSnapshot{
		Products: currentProducts,
	}

	// 2단계: 현재 스냅샷을 직전 스냅샷과 비교하여 신규 상품 또는 가격 변동을 감지합니다.
	// 변경 사항이 있으면 알림 메시지(message)를 생성하고, 스냅샷 저장 여부(shouldSave)를 결정합니다.
	message, shouldSave := t.analyzeAndReport(commandSettings, currentSnapshot, prevSnapshot, supportsHTML)

	if shouldSave {
		// 방어 코드: shouldSave=true이면서 message가 비어있으면 구현 버그가 존재하는 것입니다.
		// 이 상태에서 스냅샷을 덮어쓰면 변경이 발생했음에도 사용자가 알림을 전혀 받지 못하게 됩니다.
		// 데이터의 무결성을 지키기 위해 저장을 차단하고, 즉시 인지할 수 있도록 경고 로그를 남깁니다.
		if message == "" {
			t.Log("task.navershopping", applog.WarnLevel, "변경 사항 감지 후 저장 프로세스를 시도했으나, 알림 메시지가 비어있습니다 (저장 건너뜀)", nil, nil)
			return "", nil, nil
		}

		return message, currentSnapshot, nil
	}

	return message, nil, nil
}

// @@@@@
// fetchProducts 네이버 쇼핑 검색 API를 페이지 단위로 반복 호출하여 조건에 맞는 전체 상품 목록을 수집합니다.
//
// 이 함수는 두 단계로 나뉘어 동작합니다.
//  1. [수집 단계] API를 `defaultDisplayCount`씩 페이지네이션하여 원시 상품 데이터를 모두 수집합니다.
//     수집할 총 개수는 첫 번째 API 응답의 Total 값으로 결정하며,
//     `policyMaxFetchCount`을 초과하지 않도록 상한선을 적용합니다.
//  2. [필터링 단계] 수집된 원시 데이터를 대상으로 키워드 매칭·가격 조건 등을 적용하여
//     최종적으로 조건에 부합하는 상품만 추려 반환합니다.
//
// 작업이 취소되거나 검색 결과가 없는 경우에는 nil을 반환합니다.
func (t *task) fetchProducts(ctx context.Context, commandSettings *watchPriceSettings) ([]*product, error) {
	var (
		startIndex       = 1
		targetFetchCount = math.MaxInt // 첫 페이지 응답 수신 후 실제 값으로 갱신됩니다.

		pageContent = &productSearchResponse{}
	)

	// URL 파싱을 루프 진입 전에 한 번만 수행하여 오버헤드를 줄입니다.
	// 파싱된 `baseURL`은 `buildProductSearchURL`에서 값 복사(Value Copy)하여 재사용됩니다.
	baseURL, err := url.Parse(productSearchEndpoint)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.Internal, "네이버 쇼핑 검색 API 엔드포인트 URL 파싱에 실패하였습니다")
	}

	// [수집 단계] 첫 응답의 Total을 받은 이후부터는 targetFetchCount가 실제 값으로 설정되어
	// 루프가 자연스럽게 종료됩니다. 모든 데이터를 수집하거나 policyMaxFetchCount에 도달하면 중단됩니다.
	for startIndex <= targetFetchCount {
		// 매 페이지 요청 전에 취소 여부를 확인하여 불필요한 API 호출을 방지합니다.
		if t.IsCanceled() {
			t.Log("task.navershopping", applog.WarnLevel, "작업 취소 요청이 감지되어 상품 정보 수집 프로세스를 중단합니다", nil, applog.Fields{
				"start_index":          startIndex,
				"total_fetched_so_far": len(pageContent.Items),
			})

			return nil, nil
		}

		t.Log("task.navershopping", applog.DebugLevel, "네이버 쇼핑 검색 API 페이지를 요청합니다", nil, applog.Fields{
			"query":         commandSettings.Query,
			"start_index":   startIndex,
			"display_count": defaultDisplayCount,
			"sort_option":   defaultSortOption,
		})

		reqURL := buildProductSearchURL(baseURL, commandSettings.Query, startIndex, defaultDisplayCount)

		currentPage, err := t.fetchPageProducts(ctx, reqURL)
		if err != nil {
			return nil, err
		}

		// 첫 번째 응답을 받은 직후에 한 번만 실행되어 전체 수집 계획을 확정합니다.
		if targetFetchCount == math.MaxInt {
			// 로깅에 활용하기 위해 원본 API 메타데이터(Total, Start, Display)를 보존합니다.
			pageContent.Total = currentPage.Total
			pageContent.Start = currentPage.Start
			pageContent.Display = currentPage.Display

			targetFetchCount = currentPage.Total

			// 단일 실행에서 과도한 API 호출을 방지하기 위해 상한선을 적용합니다.
			if targetFetchCount > policyMaxFetchCount {
				targetFetchCount = policyMaxFetchCount
			}
		}

		// 이번 페이지의 결과를 전체 수집 버퍼에 병합합니다.
		pageContent.Items = append(pageContent.Items, currentPage.Items...)

		startIndex += defaultDisplayCount
	}

	// 검색 결과가 없을 경우 후속 필터링 단계를 건너뛰고 즉시 반환합니다.
	if len(pageContent.Items) == 0 {
		t.Log("task.navershopping", applog.InfoLevel, "상품 정보 수집 및 키워드 매칭 프로세스가 완료되었습니다 (검색 결과 없음)", nil, applog.Fields{
			"collected_count": 0,
			"fetched_count":   0,
			"api_total_count": pageContent.Total,
			"api_start":       pageContent.Start,
			"api_display":     pageContent.Display,
		})

		return nil, nil
	}

	// [필터링 단계] 수집된 전체 항목을 대상으로 키워드 매칭 및 가격 조건을 검사합니다.
	//
	// Matcher를 루프 진입 전에 생성하여, 키워드 문자열 파싱이 항목마다 반복되지 않도록 합니다.
	includedKeywords := strutil.SplitClean(commandSettings.Filters.IncludedKeywords, ",")
	excludedKeywords := strutil.SplitClean(commandSettings.Filters.ExcludedKeywords, ",")
	matcher := strutil.NewKeywordMatcher(includedKeywords, excludedKeywords)

	// 실제 결과는 필터링으로 인해 더 적을 수 있지만, 최대 크기로 Capacity를 미리 확보하여
	// 동적 확장(Dynamic Resizing)에 따른 메모리 재할당·복사 비용을 제거합니다.
	products := make([]*product, 0, len(pageContent.Items))

	for _, item := range pageContent.Items {
		// 항목 처리 중에도 취소 여부를 확인하여 빠르게 중단할 수 있도록 합니다.
		if t.IsCanceled() {
			t.Log("task.navershopping", applog.WarnLevel, "작업 취소 요청이 감지되어 키워드 매칭 프로세스를 중단합니다", nil, applog.Fields{
				"total_items":     len(pageContent.Items),
				"processed_items": len(products),
			})

			return nil, nil
		}

		// 네이버 API는 검색어 매칭 키워드를 <b> 태그로 감싸 반환합니다.
		// HTML 태그를 먼저 제거하지 않으면, 제외 키워드가 태그 안에 숨어 매칭에 실패할 수 있습니다.
		plainTitle := strutil.StripHTML(item.Title)

		if !matcher.Match(plainTitle) {
			continue
		}

		if p := t.parseProduct(item); p != nil {
			if p.isPriceEligible(commandSettings.Filters.PriceLessThan) {
				products = append(products, p)
			}
		}
	}

	t.Log("task.navershopping", applog.InfoLevel, "상품 정보 수집 및 키워드 매칭 프로세스가 완료되었습니다", nil, applog.Fields{
		"collected_count": len(products),
		"fetched_count":   len(pageContent.Items),
		"api_total_count": pageContent.Total,
		"api_start":       pageContent.Start,
		"api_display":     pageContent.Display,
	})

	return products, nil
}
