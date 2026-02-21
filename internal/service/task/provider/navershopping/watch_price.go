package navershopping

import (
	"context"
	"errors"
	"math"
	"net/url"
	"time"

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

// executeWatchPrice 네이버 쇼핑 API로 상품 가격 정보를 수집하고, 이전 실행 시점의 스냅샷과 비교하여
// 변경 사항이 있을 경우 사용자에게 전달할 알림 메시지를 생성합니다.
//
// 매개변수:
//   - ctx: 요청 취소 등을 위한 컨텍스트
//   - commandSettings: 검색 키워드, 가격 필터 등 이 커맨드의 실행 설정
//   - prevSnapshot: 이전 실행 시 저장된 상품 목록 스냅샷 (nil이면 최초 실행)
//   - supportsHTML: 알림 수신 채널의 HTML 지원 여부
//
// 반환값:
//   - message: 사용자에게 전송할 알림 메시지 (없으면 빈 문자열)
//   - newSnapshot: 변경사항이 감지된 경우 새로 수집된 상품 목록 스냅샷, 없으면 nil
//   - err: 실행 중 발생한 오류
func (t *task) executeWatchPrice(ctx context.Context, commandSettings *watchPriceSettings, prevSnapshot *watchPriceSnapshot, supportsHTML bool) (string, any, error) {
	// 1단계: 네이버 쇼핑 API를 페이지 단위로 호출하여 현재 시점의 상품 목록을 수집합니다.
	// 수집 결과는 직전 스냅샷과의 비교 대상으로 활용됩니다.
	currentProducts, err := t.fetchProducts(ctx, commandSettings)
	if err != nil {
		return "", nil, err
	}

	currentSnapshot := &watchPriceSnapshot{
		Products: currentProducts,
	}

	// 2단계: 수집된 현재 스냅샷을 직전 스냅샷과 비교하여 변경 여부를 판단하고 알림 메시지를 구성합니다.
	//
	// 내부적으로 신규 상품 감지, 상품 이탈 감지, 가격 변동 및 메타 정보 변경 감지를 수행하며,
	// 그 결과를 message(알림 메시지)와 hasChanges(스냅샷 갱신 필요 여부)로 분리하여 반환합니다.
	message, hasChanges := t.analyzeAndReport(commandSettings, currentSnapshot, prevSnapshot, supportsHTML)

	// 3단계: 스냅샷 갱신 여부에 따라 결과를 반환합니다.
	//
	// [변경 사항이 있는 경우 (hasChanges == true)]
	// 신규 상품 등록, 상품 이탈, 가격 변동, 메타 정보 변경 중 하나 이상이 감지된 상태입니다.
	// 다음 실행 시 정확한 비교 기준이 될 수 있도록 현재 스냅샷(currentSnapshot)을 함께 반환합니다.
	// 반환된 currentSnapshot은 호출부에서 데이터베이스에 저장됩니다.
	if hasChanges {
		return message, currentSnapshot, nil
	}

	// [변경 사항이 없는 경우 (hasChanges == false)]
	// 스냅샷 갱신이 불필요하므로 nil을 반환하여 저장을 건너뜁니다.
	// analyzeAndReport 내부에서 수동 실행(TaskRunByUser) 시 현재 상품 목록을 message에 담아두므로,
	// 여기서는 그 값을 그대로 전달합니다. (스케줄러 실행 시에는 message가 빈 문자열입니다.)
	return message, nil, nil
}

// fetchProducts 네이버 쇼핑 검색 API를 페이지 단위로 반복 호출하여 조건에 맞는 전체 상품 목록을 수집합니다.
//
// 매개변수:
//   - ctx: 요청 취소 등을 위한 컨텍스트
//   - commandSettings: 검색 키워드, 가격 필터 등 이 커맨드의 실행 설정
//
// 반환값:
//   - []*product: 필터링 및 중복 제거를 거친 최종 상품 목록
//   - error: 수집 중 발생한 오류 (작업 취소 시 context.Canceled)
func (t *task) fetchProducts(ctx context.Context, commandSettings *watchPriceSettings) ([]*product, error) {
	// =========================================================================
	// [초기화] 페이지 요청 간격 타이머 준비
	// =========================================================================

	// 페이지 수집 간격을 조절하기 위한 타이머를 생성합니다.
	// 반복문 내에서 time.After를 사용하면 매번 객체가 생성되어 GC 부하가 발생하므로,
	// 타이머를 하나만 생성하고 재사용(Reset)하는 최적화 패턴을 사용합니다.
	fetchDelay := time.Duration(commandSettings.PageFetchDelay) * time.Millisecond

	// 초기 생성 시에는 만료되지 않는 시간(time.Hour)으로 설정하고 즉시 정지시킵니다.
	fetchDelayTimer := time.NewTimer(time.Hour)
	fetchDelayTimer.Stop()

	// 함수 종료 시(정상 반환, 에러 반환 등 모든 경우) 타이머를 반드시 정지시킵니다.
	// 이렇게 하지 않으면 Go 런타임이 타이머가 만료될 때까지 타이머 고루틴과 내부 채널(C)을 GC하지 못합니다.
	defer fetchDelayTimer.Stop()

	// =========================================================================
	// [초기화] 루프 상태 변수 및 URL 준비
	// =========================================================================

	// 네이버 쇼핑 검색 API의 'start' 파라미터에 해당하며, 다음 요청에서 조회를 시작할 아이템의 위치입니다.
	// 페이지 번호가 아닌 아이템 절대 위치로 페이지네이션하므로, 매 루프마다 defaultDisplayCount를 더해 전진합니다.
	var startIndex = 1

	// 이번 실행에서 수집할 아이템의 최대 수량입니다.
	// 처음에는 math.MaxInt(무한대)로 초기화하여 루프에 무조건 한 번 진입하게 하고,
	// 첫 번째 API 응답의 Total 값을 수신한 후 policyMaxFetchCount를 초과하지 않는 범위로 갱신됩니다.
	var targetFetchCount = math.MaxInt

	// 여러 페이지에 걸쳐 수집된 상품 데이터를 누적하는 버퍼입니다.
	// 루프를 돌며 각 currentResponse의 Items를 이 구조체에 계속 병합(append)하며,
	// 로깅에 활용하기 위해 첫 번째 응답의 API 메타데이터(Total, Start, Display)도 함께 보존합니다.
	var accumulatedResponse = &productSearchResponse{}

	// URL 파싱을 루프 진입 전에 한 번만 수행하여 오버헤드를 줄입니다.
	// 파싱된 `baseURL`은 `buildProductSearchURL`에서 값 복사하여 재사용됩니다.
	baseURL, err := url.Parse(productSearchEndpoint)
	if err != nil {
		return nil, newErrEndpointParseFailed(err)
	}

	// =========================================================================
	// [메인 루프] 페이지 단위로 상품 데이터를 수집합니다.
	// =========================================================================
	for startIndex <= targetFetchCount {
		// 작업 취소 요청이 들어왔는지 주기적으로 확인하여, 불필요한 리소스 낭비를 방지합니다.
		if t.IsCanceled() {
			t.Log(component, applog.InfoLevel, "수집 중단: 외부 취소 요청", nil, applog.Fields{
				"query":              commandSettings.Query,
				"start_index":        startIndex,
				"target_fetch_count": targetFetchCount,
				"fetched_count":      len(accumulatedResponse.Items),
			})

			return nil, context.Canceled
		}

		t.Log(component, applog.DebugLevel, "페이지 요청: 네이버 쇼핑 상품 검색", nil, applog.Fields{
			"query":              commandSettings.Query,
			"start_index":        startIndex,
			"display_count":      defaultDisplayCount,
			"sort_option":        defaultSortOption,
			"target_fetch_count": targetFetchCount,
			"fetched_count":      len(accumulatedResponse.Items),
		})

		// 이번 루프에서 요청할 검색어·페이지 범위·정렬 조건이 담긴 최종 API 주소를 조립합니다.
		apiURL := buildProductSearchURL(baseURL, commandSettings.Query, startIndex, defaultDisplayCount)

		// 실제 네이버 쇼핑 검색 API에 HTTP 요청을 보내 해당 범위의 상품 목록을 가져옵니다.
		currentResponse, err := t.fetchPageProducts(ctx, apiURL)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				t.Log(component, applog.InfoLevel, "수집 중단: 외부 취소 요청", nil, applog.Fields{
					"query":              commandSettings.Query,
					"start_index":        startIndex,
					"target_fetch_count": targetFetchCount,
					"fetched_count":      len(accumulatedResponse.Items),
					"fetch_error":        err.Error(),
				})

				return nil, context.Canceled
			}

			// [주의] context.Canceled로 교체하지 않고 ctx.Err()를 그대로 반환합니다.
			//
			// 왜 교체하면 안 되는가?
			//   base.go의 finalizeExecution()은 에러가 context.Canceled인 경우에만
			//   "사용자가 직접 취소한 것"으로 간주하여 에러 알림 전송을 의도적으로 생략합니다.
			//
			//   만약 여기서 ctx.Err()(예: context.DeadlineExceeded)를 context.Canceled로
			//   교체해버리면, 타임아웃으로 인한 수집 실패임에도 불구하고 에러 알림이 전송되지 않아
			//   사용자는 서비스가 조용히 실패하고 있다는 사실을 전혀 알 수 없게 됩니다.
			//
			// 올바른 동작:
			//   - context.Canceled  → 사용자가 명시적으로 취소한 경우: 알림 생략 (정상)
			//   - context.DeadlineExceeded → 타임아웃으로 실패한 경우:  알림 전송 (이상 상황)
			if ctxErr := ctx.Err(); ctxErr != nil {
				t.Log(component, applog.WarnLevel, "수집 중단: 컨텍스트 종료", ctxErr, applog.Fields{
					"query":              commandSettings.Query,
					"start_index":        startIndex,
					"target_fetch_count": targetFetchCount,
					"fetched_count":      len(accumulatedResponse.Items),
					"fetch_error":        err.Error(),
				})

				return nil, ctxErr
			}

			return nil, err
		}

		// 첫 번째 API 응답을 받은 직후 단 한 번만 실행되어, 이번 수집 작업의 전체 계획을 확정합니다.
		// targetFetchCount를 math.MaxInt로 초기화해둔 덕분에 첫 루프 진입이 보장되며,
		// 이 블록 이후부터는 실제 수집 목표치(Total과 policyMaxFetchCount 중 작은 값)로 루프가 제어됩니다.
		if targetFetchCount == math.MaxInt {
			// 수집 완료 후 로깅에 활용하기 위해 첫 번째 응답의 API 메타데이터를 보존합니다.
			// (이후 루프에서 Items만 병합되므로, 메타데이터는 반드시 첫 응답 기준으로 고정해야 합니다.)
			accumulatedResponse.Total = currentResponse.Total
			accumulatedResponse.Start = currentResponse.Start
			accumulatedResponse.Display = currentResponse.Display

			// 과도한 API 호출을 방지하기 위해, API가 반환한 Total과 policyMaxFetchCount 중
			// 더 작은 값을 실제 수집 목표치로 확정합니다.
			targetFetchCount = min(currentResponse.Total, policyMaxFetchCount)
		}

		// 이번 페이지에서 가져온 상품 목록을 전체 수집 버퍼에 순서대로 병합합니다.
		accumulatedResponse.Items = append(accumulatedResponse.Items, currentResponse.Items...)

		// 다음 루프에서는 이번 페이지의 마지막 아이템 다음부터 조회를 시작합니다.
		startIndex += defaultDisplayCount

		// 다음 startIndex가 목표 수집량을 초과하면 더 가져올 페이지가 없으므로 루프를 즉시 종료합니다.
		if startIndex > targetFetchCount {
			t.Log(component, applog.DebugLevel, "수집 종료: 데이터 없음", nil, applog.Fields{
				"query":              commandSettings.Query,
				"start_index":        startIndex,
				"target_fetch_count": targetFetchCount,
				"fetched_count":      len(accumulatedResponse.Items),
			})

			break
		}

		// 다음 페이지 요청 전 설정된 시간만큼 대기합니다.
		// (네이버 서버 부하 방지 및 차단 회피 목적)
		//
		// 이미 앞선 루프 끝에서 타이머 만료를 기다렸기 때문에 Stop() 후 Drain이 불필요합니다.
		// 바로 Reset을 호출하여 다음 대기를 준비합니다.
		fetchDelayTimer.Reset(fetchDelay)

		select {
		case <-ctx.Done():
			t.Log(component, applog.InfoLevel, "수집 중단: 외부 취소 요청", nil, applog.Fields{
				"query":              commandSettings.Query,
				"start_index":        startIndex,
				"target_fetch_count": targetFetchCount,
				"fetched_count":      len(accumulatedResponse.Items),
			})

			return nil, ctx.Err()

		case <-fetchDelayTimer.C: // 대기 시간이 만료되면 다음 루프(페이지)로 진행
		}
	}

	// API 수집은 완료되었지만 결과가 하나도 없는 경우, 불필요한 필터링 루프를 생략하고 즉시 종료합니다.
	// 유효한 상태이므로 에러가 아닌 nil을 반환합니다.
	if len(accumulatedResponse.Items) == 0 {
		t.Log(component, applog.InfoLevel, "조기 종료: 검색 결과 없음", nil, applog.Fields{
			"query":              commandSettings.Query,
			"api_total_count":    accumulatedResponse.Total,
			"api_start":          accumulatedResponse.Start,
			"api_display":        accumulatedResponse.Display,
			"target_fetch_count": targetFetchCount,
			"fetched_count":      0,
			"collected_count":    0,
		})

		return nil, nil
	}

	// =========================================================================
	// [필터링 단계] 수집된 상품을 대상으로 키워드 매칭 및 가격 조건을 검사합니다.
	// =========================================================================

	// 루프 진입 전에 포함·제외 키워드를 파싱하여 KeywordFilter를 미리 생성합니다.
	// 루프 내부로 넣으면 키워드 파싱이 상품 항목마다 반복 실행되어 불필요한 연산이 발생합니다.
	keywordFilter := strutil.NewKeywordMatcher(
		strutil.SplitClean(commandSettings.Filters.IncludedKeywords, ","),
		strutil.SplitClean(commandSettings.Filters.ExcludedKeywords, ","),
	)

	// 필터링을 거치면 실제 결과 수는 적어지지만, 사전에 최대 크기로 Capacity를 할당해 둡니다.
	// 이렇게 하면 루프 중 슬라이스가 동적으로 확장될 때마다 발생하는 메모리 재할당과 데이터 복사를 방지합니다.
	collectedProducts := make([]*product, 0, len(accumulatedResponse.Items))

	for _, item := range accumulatedResponse.Items {
		// 매 항목 처리 전에 취소 신호를 확인하여, 대량의 데이터를 처리하는 중에도 빠르게 중단할 수 있도록 합니다.
		if t.IsCanceled() {
			t.Log(component, applog.InfoLevel, "수집 중단: 외부 취소 요청 (필터링 단계)", nil, applog.Fields{
				"query":              commandSettings.Query,
				"start_index":        startIndex,
				"target_fetch_count": targetFetchCount,
				"fetched_count":      len(accumulatedResponse.Items),
				"collected_count":    len(collectedProducts),
			})

			return nil, context.Canceled
		}

		// 네이버 API는 검색어와 일치하는 부분을 <b> 태그로 감싸 반환합니다.
		// 키워드 필터를 적용하기 전에 반드시 HTML 태그를 제거해야, 태그 안에 숨은 제외 키워드도 정확히 감지됩니다.
		plainTitle := strutil.StripHTML(item.Title)

		// [1차 필터] 키워드 조건(포함·제외 키워드)에 부합하지 않으면 건너뜁니다.
		if !keywordFilter.Match(plainTitle) {
			continue
		}

		// [2차 필터] 상품 데이터를 파싱하고, 설정된 가격 상한선 미만인 경우에만 최종 수집 목록에 추가합니다.
		if p := t.parseProduct(item); p != nil {
			if p.isPriceEligible(commandSettings.Filters.PriceLessThan) {
				collectedProducts = append(collectedProducts, p)
			}
		}
	}

	t.Log(component, applog.InfoLevel, "수집 완료: 정상 종료", nil, applog.Fields{
		"query":              commandSettings.Query,
		"api_total_count":    accumulatedResponse.Total,
		"api_start":          accumulatedResponse.Start,
		"api_display":        accumulatedResponse.Display,
		"target_fetch_count": targetFetchCount,
		"fetched_count":      len(accumulatedResponse.Items),
		"collected_count":    len(collectedProducts),
	})

	return collectedProducts, nil
}
