package kurly

// mergeWithPreviousState 이번 사이클에 새로 수집된 상품 목록(currentProducts)에
// 이전 스냅샷(prevSnapshot)의 누적 이력(역대 최저가, 연속 실패 횟수 등)을 병합합니다.
//
// 크롤링으로 방금 수집한 상품 객체는 이번 사이클의 데이터만 담긴 'Stateless' 상태입니다.
// 이 함수는 4단계를 순서대로 수행하여 분석(Analyzer) 단계가 항상 '완성된 데이터'를 볼 수 있도록 보장합니다.
//
// 처리 흐름:
//  1. 인덱싱:
//     이전 스냅샷 상품 목록을 ID 기준 Map으로 변환하여 O(1) 조회를 준비합니다.
//  2. 상태 복원:
//     각 상품의 이전 이력(역대 최저가, 연속 실패 횟수 등)을 현재 객체에 이월합니다.
//  3. 최저가 갱신:
//     복원된 역대 최저가와 현재 수집 가격을 비교하여 더 낮은 값으로 갱신합니다.
//  4. 이월/GC:
//     이번 수집에서 누락된 상품 중, watchedProductIDs에 포함된 것(일시적 실패)은 이전 이력 보존을 위해 현재 스냅샷에 포함시키고,
//     제외된 것(사용자가 삭제/비활성화)은 스냅샷에서 제거합니다.
//
// 반환값:
//   - []*product: 상태 복원, 최저가 갱신, 이월/GC가 모두 완료된 최종 상품 목록
//   - map[int]*product: 분석(Analyzer) 단계에서 현재 수집 결과와 과거 상태를 비교할 때 사용하는 상품 Map (Key: 상품 ID)
func mergeWithPreviousState(currentProducts []*product, prevSnapshot *watchProductPriceSnapshot, watchedProductIDs map[int]struct{}) ([]*product, map[int]*product) {
	// -------------------------------------------------------------------------
	// [1단계] 인덱싱: 이전 스냅샷 상품 목록을 ID 기준 Map으로 변환
	// -------------------------------------------------------------------------
	// 이후 단계에서 "이전 상품 ID → 이전 상품 객체"를 O(1)로 조회하기 위해 슬라이스를 Map으로 변환합니다.
	// 최초 실행(prevSnapshot == nil)인 경우, prevProductsByID도 nil로 유지합니다.
	var prevProductsByID map[int]*product
	if prevSnapshot != nil {
		prevProductsByID = make(map[int]*product, len(prevSnapshot.Products))
		for _, p := range prevSnapshot.Products {
			prevProductsByID[p.ID] = p
		}
	}

	// -------------------------------------------------------------------------
	// [2단계 + 3단계] 상태 복원 및 최저가 갱신
	// -------------------------------------------------------------------------

	// 4단계(이월/GC)에서 이전 스냅샷 상품들을 순회할 때,
	// "이번 사이클에 실제로 수집된 상품"인지를 O(1)로 판별하기 위한 ID Set입니다.
	currentProductIDs := make(map[int]struct{}, len(currentProducts))

	// 이번 사이클에 새로 수집된 상품 목록 전체를 순회하면서,
	// 각 상품마다 [2단계] 이전 상태 복원과 [3단계] 이번 사이클 가격 기준 최저가 갱신 작업을 차례로 수행합니다.
	for _, currentProduct := range currentProducts {
		currentProductIDs[currentProduct.ID] = struct{}{}

		// -------------------------------------------------------------------------
		// [2단계] 이전 스냅샷이 존재하는 경우에만 상태 복원을 수행합니다.
		// -------------------------------------------------------------------------
		if prevProductsByID != nil {
			if prevProduct, exists := prevProductsByID[currentProduct.ID]; exists {
				// 크롤러가 방금 수집한 객체에는 이번 사이클의 가격(Price, DiscountedPrice)만 존재하며,
				// 역대 최저가(LowestPrice, LowestPriceTimeUTC)는 이전 스냅샷에만 누적 기록되어 있습니다.
				// [3단계] 최저가 갱신 로직은 "현재 가격 vs 역대 최저가"를 비교하므로,
				// 비교 기준이 되는 역대 최저가를 이전 스냅샷으로부터 먼저 이월(복원)해두어야 합니다.
				// 이 선행 작업이 없으면 LowestPrice가 0인 채로 비교되어 매 사이클마다 최저가가 잘못 갱신됩니다.
				currentProduct.LowestPrice = prevProduct.LowestPrice
				currentProduct.LowestPriceTimeUTC = prevProduct.LowestPriceTimeUTC

				// 크롤링이 실패한 상품(FetchFailedCount > 0)은 실제 가격 데이터 없이 ID만 담긴 임시 객체로 전달됩니다.
				// 이전 스냅샷의 실패 횟수를 누적하고, 임계값(3회) 초과 시 단종·영구 접근 불가로 판정합니다.
				if currentProduct.FetchFailedCount > 0 {
					currentProduct.Name = prevProduct.Name
					currentProduct.FetchFailedCount += prevProduct.FetchFailedCount

					// FetchFailedCount의 상한선을 임계값(3)으로 고정합니다.
					// 이미 IsUnavailable로 전환된 상품이라도 매 수집 사이클마다 실패가 반복되면 FetchFailedCount가 4, 5, 6...으로 계속 증가하게 됩니다.
					// 이 경우 HasChanged()의 비교 구문(p.FetchFailedCount != prevProduct.FetchFailedCount)이 매번 true로 평가되어,
					// 실질적인 상태 변화가 없음에도 스토리지에 불필요한 Save가 무한히 발생하는 리소스 낭비가 발생합니다.
					// 따라서 FetchFailedCount의 상한선을 임계값(3)으로 고정하여 이를 원천 차단합니다.
					if currentProduct.FetchFailedCount > 3 {
						currentProduct.FetchFailedCount = 3
					}

					if currentProduct.FetchFailedCount >= 3 {
						// 연속 3회 이상 실패: 일시적 장애가 아닌 단종 또는 영구적 접근 불가 상태로 판단합니다.
						// IsUnavailable = true로 강제 전환하여 좀비 데이터로 남지 않도록 합니다.
						currentProduct.IsUnavailable = true
					} else {
						// 연속 3회 미만 실패: 일시적 네트워크 장애로 간주합니다.
						// 이번 사이클에는 유효한 데이터가 없으므로, 이전 정상 수집 데이터를 그대로 승계합니다.
						// 이를 통해 일시적 오류가 가격 변동 알림을 잘못 트리거하는 것을 방지합니다.
						currentProduct.Name = prevProduct.Name
						currentProduct.Price = prevProduct.Price
						currentProduct.DiscountedPrice = prevProduct.DiscountedPrice
						currentProduct.DiscountRate = prevProduct.DiscountRate
						currentProduct.IsUnavailable = prevProduct.IsUnavailable
					}
				} else if currentProduct.IsUnavailable && currentProduct.Name == "" {
					// [이름 유실 방지] 단종·판매 중지 상품의 이름을 이전 스냅샷에서 복원합니다.
					// fetchProduct는 상품이 단종임을 감지하면 이름 추출 이전에 조기 반환하므로, 반환된 객체의 Name 필드는 빈 문자열("")인 상태입니다.
					// 이 상태를 그대로 저장하면 이후 사이클에서는 이름 정보를 영영 복구할 수 없습니다.
					// 따라서 이전 스냅샷에 기록된 이름을 현재 객체에 이월하여 영구 유실을 차단합니다.
					currentProduct.Name = prevProduct.Name
				}
			}
		}

		// [예외 처리] 이전 스냅샷이 없는 최초 실행에서 곧바로 실패한 상품은
		// 위 블록(prevProductsByID != nil)을 건너뛰어 FetchFailedCount가 누적되지 않았습니다.
		// 따라서, 이전 스냅샷 유무와 무관하게 임계값(3회) 초과 여부를 여기서 한 번 더 최종 확인합니다.
		if currentProduct.FetchFailedCount >= 3 {
			currentProduct.IsUnavailable = true
		}

		// -------------------------------------------------------------------------
		// [3단계] 최저가 갱신
		// -------------------------------------------------------------------------
		// 2단계에서 이전 스냅샷의 역대 최저가가 복원된 상태에서, 현재 실구매가와 비교하여 최저가를 최종 갱신합니다.
		//   - 현재 가격이 더 낮으면: LowestPrice와 LowestPriceTimeUTC를 즉시 덮어씁니다.
		//   - 현재 가격이 같거나 높으면: 기존 최저가 기록을 그대로 유지합니다.
		//
		// 주의: 반드시 Diff 계산 이전에 완료되어야 합니다.
		// 그래야만 분석 단계에서 "이번 사이클에 역대 최저가가 갱신되었는지"를 정확히 판별할 수 있습니다.
		currentProduct.tryUpdateLowestPrice()
	}

	// -------------------------------------------------------------------------
	// [4단계] 이월 처리: 감시 목록에서 누락된 상품 이월 보존 및 삭제된 상품 GC
	// -------------------------------------------------------------------------

	// 이번 사이클에 수집된 상품 전체를 병합 결과의 베이스로 삼습니다.
	// 이후 루프에서 수집에서 누락된 상품(이전 스냅샷에 존재)을 선별하여 추가 병합합니다.
	mergedProducts := make([]*product, 0, len(currentProducts))
	mergedProducts = append(mergedProducts, currentProducts...)

	// 이전 스냅샷에 존재했지만 이번 사이클에 수집되지 않은 상품들을 순회합니다.
	// 수집 누락의 원인은 크게 두 가지로 나뉩니다:
	//   A. 일시적 수집 실패 (네트워크 오류 등): 여전히 활성 상태이므로 이력을 보존해야 합니다.
	//   B. 사용자의 의도적 삭제/비활성화: 더 이상 감시 대상이 아니므로 GC 처리합니다.
	for prevProductID, prevProduct := range prevProductsByID {
		if _, exists := currentProductIDs[prevProductID]; !exists {
			// watchedProductIDs에 포함된 상품은 사용자가 여전히 감시 중인 상품입니다.
			// 이번 수집 실패는 일시적 장애(원인 A)로 판단하여, 역대 최저가 이력이 유실되지 않도록
			// 이전 상태 그대로 현재 스냅샷에 포함시켜 보존합니다.
			if _, isWatched := watchedProductIDs[prevProductID]; isWatched {
				mergedProducts = append(mergedProducts, prevProduct)
			} else {
				// watchedProductIDs에 포함되지 않은 상품은 사용자가 삭제/비활성화한 것(원인 B)입니다.
				// Ghost 데이터로 남지 않도록 현재 스냅샷에서 조용히 제거합니다. (GC)
			}

		}
	}

	return mergedProducts, prevProductsByID
}
