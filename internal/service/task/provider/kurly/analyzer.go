package kurly

import (
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

// @@@@@ 개선 문의
// syncLowestPrices 현재 수집된 상품 정보와 이전 스냅샷을 동기화하여 데이터의 연속성을 보장합니다.
//
// [역할: 상태 동기화]
// 데이터를 변경하고 최신화하는 작업은 오직 여기서만 수행합니다. (Side Effect 전담)
//
// 1. 빠른 조회 준비 (Indexing): 이전 상품 목록을 Map으로 만들어 승계 속도를 높입니다. (O(N))
// 2. 과거 데이터 계승 (Restoration): 지난번 실행 때까지의 '역대 최저가' 기록을 현재 객체로 가져옵니다.
// 3. 최신 상태 반영 (Update): 현재 가격과 비교하여 최저가를 최종 갱신합니다.
// 4. 누락 데이터 이관 (Carry-over): 통신 장애 등 일시적 이유로 현재 사이클에서 누락된 이전 상품을 스냅샷에 포함시켜 과거 데이터를 보존합니다. 단, 감시 대상에서 제외(삭제/비활성화)된 상품은 가비지 컬렉션(GC) 처리하여 스토리지 누수를 방지합니다.
func syncLowestPrices(currentSnapshot, prevSnapshot *watchProductPriceSnapshot, activeRecordIDs map[int]struct{}) map[int]*product {
	// 빠른 조회를 위해 이전 상품 목록을 Map으로 변환한다.
	var prevProductsMap map[int]*product
	if prevSnapshot != nil {
		prevProductsMap = make(map[int]*product, len(prevSnapshot.Products))
		for _, p := range prevSnapshot.Products {
			prevProductsMap[p.ID] = p
		}
	}

	// 누락된 과거 상품을 식별하기 위해 현재 수집된 상품 ID 목록을 기록합니다.
	currentProductIDs := make(map[int]struct{}, len(currentSnapshot.Products))

	// 모든 상품의 최저가 정보를 최신으로 갱신합니다.
	// 이로써 이후의 비교 로직은 순수한 '조회' 작업만 수행하게 됩니다.
	for _, currentProduct := range currentSnapshot.Products {
		currentProductIDs[currentProduct.ID] = struct{}{}

		// 크롤링으로 수집된 '현재 상태(Stateless)'에는 과거의 기록인 '역대 최저가' 정보가 부재합니다.
		// 따라서 이전 실행 결과(Snapshot)로부터 누적된 최저가 데이터를 조회하여
		// 현재 객체로 이월(Carry-over)하는 상태 복원(State Restoration) 과정을 수행합니다.
		if prevProductsMap != nil {
			if prevProduct, exists := prevProductsMap[currentProduct.ID]; exists {
				currentProduct.LowestPrice = prevProduct.LowestPrice
				currentProduct.LowestPriceTimeUTC = prevProduct.LowestPriceTimeUTC

				// 크롤링에 실패하여 임시 객체로 전달된 경우(FetchFailedCount > 0) 실패 횟수를 누적하고 상태를 복원/전이합니다.
				if currentProduct.FetchFailedCount > 0 {
					currentProduct.FetchFailedCount += prevProduct.FetchFailedCount
					if currentProduct.FetchFailedCount >= 3 {
						// 연속 실패 임계값(3회) 도달 시 단종(혹은 접근 불가) 상태로 강제 전환하여 좀비 데이터를 방지합니다.
						currentProduct.IsUnavailable = true
					} else {
						// 임계값 미달 시 일시적 장애로 간주하여 이전 정상 데이터(가격, 상태 등)를 승계합니다.
						currentProduct.Name = prevProduct.Name
						currentProduct.Price = prevProduct.Price
						currentProduct.DiscountedPrice = prevProduct.DiscountedPrice
						currentProduct.DiscountRate = prevProduct.DiscountRate
						currentProduct.IsUnavailable = prevProduct.IsUnavailable
					}
				}
			}
		}

		// 만약 최초 수집부터 계속 실패하여 누적되었거나(기존 기록 없음), 임계값 도달 시 재확인
		if currentProduct.FetchFailedCount >= 3 {
			currentProduct.IsUnavailable = true
		}

		// [최저가 갱신 로직 실행]
		// 현재 시점의 실구매가(Effective Price)와 기존 역대 최저가를 비교하여 상태를 동기화합니다.
		//
		// 이 메서드는 단순 비교를 넘어 다음과 같은 중요한 상태 변경(State Mutation)을 수행합니다:
		// 1. 최저가 갱신 (Atomicity): 현재 가격이 더 낮을 경우 즉시 새로운 최저가로 덮어씁니다.
		// 2. 시계열 기록 (Timestamping): 갱신 시점의 시간(UTC)을 기록하여 데이터의 이력을 보존합니다.
		//
		// 중요: 반드시 Diff 계산(calculateProductDiffs) 이전에 수행되어야 합니다.
		// 이를 통해 '이번 크롤링 사이클에서 최저가가 갱신되었는지'를 정확히 판별할 수 있습니다.
		currentProduct.tryUpdateLowestPrice()
	}

	// [누락 데이터 이관 (Carry-over)]
	// 웹 스크래핑 실패 등 통신 장애로 인해 이번 회차에 수집되지 못한 '과거 상품'들을 찾아냅니다.
	// 발견된 누락 상품 중 사용자가 여전히 감시 중인 상품(activeRecordIDs 포함)은 현재 스냅샷에 추가하여 역대 최저가 이력 유실을 방지합니다.
	// 단, 사용자가 CSV에서 의도적으로 삭제하거나 비활성화한 상품은 저장소 누수(Storage Leak)를 방지하기 위해 가비지 컬렉션(GC) 처리합니다.
	if prevProductsMap != nil {
		for id, prevProduct := range prevProductsMap {
			if _, exists := currentProductIDs[id]; !exists {
				// 현재 수집된 상품 목록(currentProductIDs)에는 없지만, 여전히 감시 대상(activeRecordIDs)으로 설정되어 있다면,
				// 이는 의도적 삭제가 아닌 수집 실패로 간주하여 과거 데이터를 현재 스냅샷으로 이관하여 보존합니다.
				if _, isActive := activeRecordIDs[id]; isActive {
					currentSnapshot.Products = append(currentSnapshot.Products, prevProduct)
				}
			}
		}
	}

	return prevProductsMap
}

// @@@@@ 개선사항 존재유무 확인
// analyzeAndReport 수집된 데이터를 분석하여 사용자에게 보낼 알림 메시지를 생성합니다.
//
// [주요 동작]
// 1. 변화 확인: 이전 데이터와 비교해 새로운 상품이나 가격 변동이 있는지 확인합니다.
// 2. 메시지 작성: 발견된 변화를 보기 좋게 포맷팅합니다.
// 3. 알림 결정:
//   - 스케줄러 실행: 변화가 있을 때만 알림을 보냅니다. (조용히 모니터링)
//   - 사용자 실행: 변화가 없어도 "변경 없음"이라고 알려줍니다. (확실한 피드백)
func analyzeAndReport(runBy contract.TaskRunBy, currentSnapshot *watchProductPriceSnapshot, prevProductsMap map[int]*product, reportedDuplicateIDs []string, records, duplicateRecords [][]string, supportsHTML bool) (message string, shouldSave bool) {
	// 신규 상품 및 가격 변동을 식별합니다.
	diffs := calculateProductDiffs(currentSnapshot, prevProductsMap)

	// 식별된 변동 사항을 사용자가 이해하기 쉬운 알림 메시지로 변환합니다.
	productsDiffMessage := renderProductDiffs(diffs, supportsHTML)

	// 단순한 가격 변동 알림을 넘어, 사용자의 설정 오류(중복 등록)나 외부 요인에 의한 상품 상태 변화(판매 중지)를 식별하여 보고합니다.
	// 이전 상태(reportedDuplicateIDs, prevProductsMap)를 매개변수로 전달하여 이미 이전에 보고가 된 대상은 스킵하는
	// State-machine 무한 알람 스팸 방지 로직이 적용되어 있습니다.
	duplicateRecordsMessage, newDuplicateNotifiedIDs := buildDuplicateRecordsMessage(duplicateRecords, reportedDuplicateIDs, supportsHTML)
	currentSnapshot.DuplicateNotifiedIDs = newDuplicateNotifiedIDs

	unavailableProductsMessage := buildUnavailableProductsMessage(currentSnapshot.Products, prevProductsMap, records, supportsHTML)

	// 최종 알림 메시지 조합
	// 앞서 생성된 핵심 변경 내역과 부가 정보들을 하나의 완결된 사용자 메시지로 통합합니다.
	// 이 단계에서는 각 메시지 조각의 유무에 따라 조건부로 포맷팅을 수행하며, 최종적으로 사용자가 받아볼 깔끔하고 가독성 높은 리포트를 완성합니다.
	message = buildNotificationMessage(runBy, currentSnapshot, productsDiffMessage, duplicateRecordsMessage, unavailableProductsMessage, supportsHTML)

	// 결과 처리 (알림 vs 저장)
	// 알림을 보내는 기준과 데이터를 저장하는 기준을 다르게 적용하여 효율성을 높입니다.
	// - 알림: 사용자가 직접 확인하고 싶어 할 때(RunByUser)는 변경 사항이 없더라도 현재 상태를 리포트하여 안심시켜 줍니다.
	// - 저장: 매번 불필요하게 저장하지 않고, 실제로 가격, 상태, 혹은 내부 추적 데이터(중복 ID 등)가 변했을 때만 저장합니다.
	reportedDuplicateIDsChanged := !isStringSliceEqual(reportedDuplicateIDs, newDuplicateNotifiedIDs)
	hasChanges := len(diffs) > 0 || strutil.AnyContent(duplicateRecordsMessage, unavailableProductsMessage) || reportedDuplicateIDsChanged
	return message, hasChanges
}

// @@@@@
// isStringSliceEqual 두 문자열 슬라이스의 요소가 동일한지 비교하는 헬퍼 함수입니다.
func isStringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// @@@@@
// calculateProductDiffs 현재 상품 정보와 과거 상품 정보를 비교하여 사용자에게 알릴 만한 변화(Diff)를 찾아냅니다.
//
// [동작 흐름]
// 상품의 상태 변화를 세 단계로 나누어 순차적으로 분석합니다.
//
// 1. 신규 여부: "처음 보는 상품인가?" (New Product)
// 2. 판매 상태: "품절되었다가 다시 들어왔는가?" (Restock)
// 3. 가격 변동: "가격이 오르거나 내렸는가? 역대 최저가인가?" (Price Change)
func calculateProductDiffs(currentSnapshot *watchProductPriceSnapshot, prevProductsMap map[int]*product) []productDiff {
	var diffs []productDiff

	for _, currentProduct := range currentSnapshot.Products {
		prevProduct, exists := prevProductsMap[currentProduct.ID]

		// 1. 신규 상품 처리
		// 이전 기록이 없는 경우, 현재 상태가 유효하다면 '신규 상품'으로 처리합니다.
		// 단, 최초 수집부터 실패한 상품(FetchFailedCount > 0)은 정상 수집될 때까지 신규 알림을 보류합니다.
		if !exists {
			if !currentProduct.IsUnavailable && currentProduct.FetchFailedCount == 0 {
				diffs = append(diffs, productDiff{
					Type:    productEventNew,
					Product: currentProduct,
					Prev:    nil,
				})
			}
			continue
		}

		// 2. 상태 전이 처리 (Unavailable <-> Available)
		// 이전 기록이 존재하는 경우, 상품의 판매 가능 여부(IsUnavailable) 변화를 감지합니다.

		// 2-1. 재입고 (Unavailable -> Available)
		// 이전에는 품절/판매중지 상태였으나, 현재 구매 가능해진 경우입니다.
		if prevProduct.IsUnavailable && !currentProduct.IsUnavailable {
			diffs = append(diffs, productDiff{
				Type:    productEventReappeared,
				Product: currentProduct,
				Prev:    nil, // 재입고는 가격 비교보다는 '등장' 자체가 중요하므로 Prev 없이 신규처럼 취급
			})
			continue
		}

		// 2-2. 판매 중지 (Available -> Unavailable)
		// 기존에 판매 중이던 상품이 품절, 판매중지 등의 사유로 정보를 확인할 수 없게 된 경우입니다.
		if !prevProduct.IsUnavailable && currentProduct.IsUnavailable {
			continue
		}

		// 2-3. 계속 판매 불가 (Unavailable -> Unavailable)
		// 이전에도 상품 정보를 확인할 수 없었고(품절/판매중지), 현재도 여전히 확인이 불가능한 상태입니다.
		// 상태의 변화가 없으므로 별도의 알림이나 처리를 수행하지 않고 무시합니다.
		if prevProduct.IsUnavailable && currentProduct.IsUnavailable {
			continue
		}

		// 3. 가격 변동 확인
		//
		// 위 단계에서 상품의 존재 여부와 판매 상태(Availability)에 대한 검증을 모두 마쳤습니다.
		// 즉, 이 시점의 상품은 '과거에도 존재했고 판매 중이었으며', '현재도 여전히 판매 중인' 정상적인 상태임이 보장됩니다.
		//
		// 따라서 이후는 복잡한 상태 판별 로직 없이, 오직 '가격 데이터'의 수치적 변동만을 순수하게 비교합니다.

		// 가격 변동 사항이 없다면 즉시 다음 상품으로 넘어갑니다.
		if !currentProduct.hasPriceChangedFrom(prevProduct) {
			continue
		}

		// 실구매가를 기준으로 최저가 갱신 여부를 최종 판단합니다.
		currentEffectivePrice := currentProduct.effectivePrice()

		if currentEffectivePrice == currentProduct.LowestPrice {
			diffs = append(diffs, productDiff{
				Type:    productEventLowestPriceAchieved,
				Product: currentProduct,
				Prev:    prevProduct,
			})
		} else {
			diffs = append(diffs, productDiff{
				Type:    productEventPriceChanged,
				Product: currentProduct,
				Prev:    prevProduct,
			})
		}
	}

	return diffs
}
