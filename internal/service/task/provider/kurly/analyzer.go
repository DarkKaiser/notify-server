package kurly

import (
	"strconv"
	"strings"
)

// extractProductDiffs 현재 스냅샷의 상품 목록과 이전 스냅샷의 상품 맵을 비교하여,
// 사용자에게 알림을 전송해야 하는 변화(Diff) 목록을 추출합니다.
//
// [동작 흐름]
// 각 상품을 대상으로 아래 세 단계를 순서대로 수행하며, 각 단계에서 조건이 충족되면
// 해당 상품에 대한 처리를 완료하고 다음 상품으로 넘어갑니다.
//
//  1. 신규 상품 감지 ("이전 스냅샷에 없던 상품인가?")
//     이전 기록이 없거나, 최초 수집 시 통신 장애로 임시 실패 객체(Price=0)로만 기록된 경우입니다.
//     현재 상태가 정상 수집된 경우에만 productEventNew 이벤트로 기록합니다.
//     단, 아직 수집 실패 중인 상품(FetchFailedCount > 0)은 알림을 보류합니다.
//
//  2. 판매 가능 여부 전이 감지 ("품절 또는 재입고가 발생했는가?")
//     - Available → Unavailable(판매 중지): 내부 상태만 갱신하고 별도 알림 없이 조용히 넘어갑니다.
//     - Unavailable → Available(재입고): productEventReappeared 이벤트로 기록합니다.
//     - Unavailable → Unavailable(유지): 상태 변화가 없으므로 무시합니다.
//
//  3. 가격 변동 감지 ("현재 판매 중인 상품의 가격이 달라졌는가?")
//     위 두 단계를 통과한 상품은 '이전에도, 현재도 정상 판매 중'임이 보장된 상태입니다.
//     실구매가 기준으로 변동이 없으면 무시하고, 변동이 있으면 역대 최저가 갱신 여부에 따라
//     productEventLowestPriceAchieved 또는 productEventPriceChanged 이벤트로 분기합니다.
//
// 매개변수:
//   - currentSnapshot: 이번 수집 결과로 구성된 현재 상태의 스냅샷
//   - prevProductsByID: 이전 스냅샷의 상품 목록을 ID 기준으로 색인한 Map (빠른 조회용)
//
// 반환값:
//   - []productDiff: 이번 사이클에 감지된 모든 변화 목록. 알림 전송 대상이 없으면 nil.
func extractProductDiffs(currentSnapshot *watchProductPriceSnapshot, prevProductsByID map[int]*product) []productDiff {
	var diffs []productDiff

	for _, currentProduct := range currentSnapshot.Products {
		prevProduct, exists := prevProductsByID[currentProduct.ID]

		// -------------------------------------------------------------------------
		// 1단계: 신규 상품 감지 — "이전 스냅샷에 없던 상품인가?"
		// -------------------------------------------------------------------------
		// 아래 두 가지 경우를 모두 '논리적 신규 등장'으로 동일하게 취급합니다.
		//   (A) !exists: 이전 스냅샷 자체에 해당 ID가 없는 경우. 진짜 새로운 상품입니다.
		//   (B) Price==0 && FetchFailedCount>0 && LowestPrice==0: 감시 목록에 추가된 이후
		//       단 한 번도 정상 수집에 성공하지 못해, 스냅샷에 임시 실패 객체(가격 0원)만 남아있는 경우입니다.
		//       기존에 한 번이라도 정상 수집 이력이 있었다면 LowestPrice가 0보다 큽니다.
		//       실제 정보를 단 한 번도 정상 수집한 적이 없으므로, 이번에 처음 정상 수집된 것과 사실상 동일합니다.
		if !exists || (prevProduct.Price == 0 && prevProduct.FetchFailedCount > 0 && prevProduct.LowestPrice == 0) {
			// 현재 사이클에서도 여전히 수집에 실패 중이거나(FetchFailedCount > 0),
			// 정상 수집됐지만 품절·판매중지(IsUnavailable=true) 상태라면 아직 알림을 보낼 수 없습니다.
			// 조건이 갖춰질 때까지 productEventNew 이벤트 발행을 조용히 보류합니다.
			if currentProduct.FetchFailedCount == 0 && !currentProduct.IsUnavailable {
				diffs = append(diffs, productDiff{
					Type:    productEventNew,
					Product: currentProduct,

					// 이전 유효 가격 데이터가 없는 신규 상품이므로 Prev를 nil로 설정합니다.
					// 0원짜리 임시 실패 객체와 현재 가격을 비교하여 렌더링하면 잘못된 인상을 줄 수 있습니다.
					Prev: nil,
				})
			}

			continue
		}

		// -------------------------------------------------------------------------
		// 2단계: 판매 가능 여부 전이 감지 — "품절이 됐거나, 다시 판매가 시작됐는가?"
		// -------------------------------------------------------------------------
		// 이전 기록이 존재하는 상품에 대해 IsUnavailable 필드의 변화를 감지합니다.
		// 총 3가지 경우의 수(재입고/판매중지/유지)를 순서대로 처리합니다.

		// [2-1] 재입고: Unavailable → Available
		// 이전에는 품절·판매중지였던 상품이 현재 다시 구매 가능해진 경우입니다.
		// 사용자에게 '다시 살 수 있다'는 사실 자체가 핵심이므로, 가격 비교 없이 신규처럼 알림을 보냅니다.
		if prevProduct.IsUnavailable && !currentProduct.IsUnavailable {
			diffs = append(diffs, productDiff{
				Type:    productEventReappeared,
				Product: currentProduct,

				// 재입고 알림은 가격 변동 정보보다 '재등장' 사실이 핵심입니다.
				// 이전 품절 시점의 가격과 비교하는 것은 의미가 없으므로 Prev를 nil로 설정합니다.
				Prev: nil,
			})

			continue
		}

		// [2-2] 판매 중지: Available → Unavailable
		// 이전까지 정상 판매 중이던 상품이 품절·판매중지 상태로 전환된 경우입니다.
		// 사용자에게 알림은 보내지 않지만, 이 상태 변화는 스냅샷에 자동으로 반영됩니다.
		if !prevProduct.IsUnavailable && currentProduct.IsUnavailable {
			continue
		}

		// [2-3] 판매 불가 유지: Unavailable → Unavailable
		// 이전에도, 현재도 판매 불가 상태입니다. 아무런 상태 변화가 없으므로 무시합니다.
		if prevProduct.IsUnavailable && currentProduct.IsUnavailable {
			continue
		}

		// -------------------------------------------------------------------------
		// 3단계: 가격 변동 감지 — "실구매가가 달라졌는가? 역대 최저가를 경신했는가?"
		// -------------------------------------------------------------------------
		// 여기까지 도달한 상품은 '이전에도 정상 판매 중이었고, 현재도 정상 판매 중'임이 보장됩니다.
		// 복잡한 상태 판별은 이미 1·2단계에서 모두 처리했으므로, 이후는 순수하게 가격 숫자만 비교합니다.

		// 실구매가(할인가 또는 정가) 기준으로 변동이 없다면 알림 없이 다음 상품으로 넘어갑니다.
		if !currentProduct.hasPriceChangedFrom(prevProduct) {
			continue
		}

		// 가격이 변동된 상품에 대해 역대 최저가 갱신 여부를 최종 판단합니다.
		// 이전 스냅샷의 역대 최저가(prevProduct.LowestPrice)와 비교하여,
		// 이번에 새롭게 최저가를 갱신했거나(이전보다 낮아짐), 이전 기록이 없는(0) 경우에만 최저가 달성으로 분기합니다.
		if prevProduct.LowestPrice == 0 || currentProduct.LowestPrice < prevProduct.LowestPrice {
			diffs = append(diffs, productDiff{
				Type:    productEventLowestPriceAchieved,
				Product: currentProduct,

				// 이전 가격과 현재 최저가를 비교 렌더링하기 위해 Prev를 전달합니다.
				Prev: prevProduct,
			})
		} else {
			diffs = append(diffs, productDiff{
				Type:    productEventPriceChanged,
				Product: currentProduct,

				// 가격이 얼마나 오르거나 내렸는지 비교 렌더링하기 위해 Prev를 전달합니다.
				Prev: prevProduct,
			})
		}
	}

	return diffs
}

// extractNewDuplicateRecords 중복 상품 레코드 목록을 훑어, 이번 수집 사이클에서 처음으로 감지된 중복 상품 레코드만 골라내고,
// 다음 사이클을 위해 갱신된 '발송 완료 상품 ID 목록'도 함께 반환합니다.
//
// [역할: 중복 상품 알림 스팸 방지 State-Machine]
// 감시 목록에 같은 상품이 실수로 두 번 이상 등록된 경우, 수집 사이클마다 동일한 "중복 등록" 알림이 반복 발송되는 스팸이 발생할 수 있습니다.
// 이를 방지하기 위해 이전 스냅샷에 저장된 발송 이력(prevDuplicateNotifiedIDs)을 참조하여, 이미 알림을 보낸 상품은 조용히 건너뛰고
// 이번에 처음 발견된 중복 상품에만 알림을 발행합니다.
//
// 매개변수:
//   - duplicateRecords: 이번 수집 사이클에서 감지된 중복 상품 레코드 목록 (CSV 원시 행 배열)
//   - prevDuplicateNotifiedIDs: 이전 스냅샷에 기록된 '이미 중복 알림이 발송된 상품 ID' 목록
//
// 반환값:
//   - newDuplicateRecords: 이번 사이클에 처음으로 중복이 감지되어 알림을 보내야 하는 레코드 목록
//   - updatedDuplicateNotifiedIDs: 이번 사이클 기준으로 갱신된 '발송 완료 상품 ID 전체 목록'
//     중복 목록에서 상품이 사라지면 자동으로 제외됩니다. 다음 스냅샷에 저장하여 State-Machine 기억으로 활용합니다.
func extractNewDuplicateRecords(duplicateRecords [][]string, prevDuplicateNotifiedIDs []string) (newDuplicateRecords [][]string, updatedDuplicateNotifiedIDs []string) {
	if len(duplicateRecords) == 0 {
		return nil, nil
	}

	// O(1) 조회를 위해 이전 발송 이력 슬라이스를 Set(map)으로 변환합니다.
	notifiedSet := make(map[string]struct{}, len(prevDuplicateNotifiedIDs))
	for _, id := range prevDuplicateNotifiedIDs {
		notifiedSet[id] = struct{}{}
	}

	// 이번 사이클의 갱신된 발송 이력을 담을 슬라이스를 미리 확보합니다.
	// 중복 레코드 수만큼 용량을 사전에 배정하여, append 과정에서의 불필요한 재할당을 방지합니다.
	updatedDuplicateNotifiedIDs = make([]string, 0, len(duplicateRecords))

	for _, record := range duplicateRecords {
		if len(record) <= int(columnID) {
			continue
		}

		productID := strings.TrimSpace(record[columnID])

		// 이번 사이클의 갱신된 발송 이력에 현재 상품 ID를 추가합니다.
		// 중복 목록에서 사라진 상품은 자동으로 제외되므로, 이 슬라이스가 유효한 최신 상태가 됩니다.
		updatedDuplicateNotifiedIDs = append(updatedDuplicateNotifiedIDs, productID)

		if _, alreadyNotified := notifiedSet[productID]; !alreadyNotified {
			newDuplicateRecords = append(newDuplicateRecords, record)

			// [중요] 알림 대상으로 추가한 즉시 notifiedSet에도 등록하여 상태를 동기화합니다.
			// 만약 이 갱신을 누락하면, 동일 루프 내에서 같은 상품이 연달아 등장했을 때 후속 레코드가 또다시 '새로운 중복'으로 오인되어
			// 결국 단일 사이클 안에서 스팸 알림이 중복 발송되는 심각한 버그가 발생합니다.
			notifiedSet[productID] = struct{}{}
		}
	}

	return newDuplicateRecords, updatedDuplicateNotifiedIDs
}

// extractNewlyUnavailableProducts 현재 상품 목록과 이전 스냅샷을 비교하여,
// 이번 수집 사이클에서 새롭게 판매 불가(IsUnavailable=true) 상태로 전이된 상품들만 추려냅니다.
//
// [역할: 단종 알림 스팸 방지]
// '처음부터 판매 불가였던 상품'에 대해 수집 사이클마다 알림이 반복 발송되는 스팸을 방지합니다.
// 이전 스냅샷과 대조하여, 오직 이번 수집 사이클에 '판매 중 → 판매 불가'로 전이된 상품만 추출합니다.
//
// 매개변수:
//   - currentProducts: 이번 수집 사이클에서 상태 병합까지 완료된 최종 상품 목록
//   - prevProductsByID: 이전 스냅샷의 상품 목록을 ID 기준으로 색인한 Map (상태 전이 판별용)
//     최초 실행 시(이전 스냅샷 없음)에는 nil을 전달하며, 이 경우 상태 전이 비교를 건너뜁니다.
//   - records: 감시 대상 CSV 레코드 목록. 아래 두 가지 목적으로 활용됩니다.
//     1. 알림에 표시할 상품명 조회 (columnName 칼럼). 판매 불가 상태에서는 크롤링이 실패하여
//     product.Name이 비어 있거나 오래된 값일 수 있으므로, 사용자가 직접 입력한 CSV 값을 우선합니다.
//     2. 감시 대상 여부 필터링 — 목록에 없는 상품(삭제되거나 비활성화된 상품)은 알림 대상에서 제외
//
// 반환값:
//   - newlyUnavailableProducts: 새롭게 판매 불가로 전이된 상품들의 ID·Name 슬라이스
//     상품명이 CSV에 없으면 fallbackProductName으로 대체됩니다.
func extractNewlyUnavailableProducts(currentProducts []*product, prevProductsByID map[int]*product, records [][]string) []struct{ ID, Name string } {
	if len(currentProducts) == 0 {
		return nil
	}

	// 루프 안에서 상품 ID(문자열)로 상품명을 O(1)에 조회하기 위해 CSV 레코드를 Map으로 변환합니다.
	// key: 상품 ID 문자열, value: 상품명
	//
	// 이 Map은 아래 루프에서 두 가지 목적으로 활용됩니다.
	//   1. 알림 메시지에 표시할 상품명 조회
	//   2. 감시 대상 여부 확인 — Map에 키가 없는 상품은 CSV에서 삭제됐거나 비활성화된 상품이므로 알림 대상에서 제외
	productNamesByID := make(map[string]string, len(records))
	for _, record := range records {
		if len(record) > int(columnName) {
			id := strings.TrimSpace(record[columnID])
			name := strings.TrimSpace(record[columnName])
			productNamesByID[id] = name
		}
	}

	// 이번 사이클에 새롭게 판매 불가로 전이된 상품들을 담을 결과 슬라이스입니다.
	var newlyUnavailableProducts []struct{ ID, Name string }

	for _, currentProduct := range currentProducts {
		// 현재 사이클에서 정상 판매 중인 상품은 이 함수의 관심 대상(판매 불가 전이)이 아니므로 건너뜁니다.
		if !currentProduct.IsUnavailable {
			continue
		}

		// [스팸 방지] 이전 스냅샷에서도 이미 판매 불가(Unavailable) 상태였던 상품은 건너뜁니다.
		// 이 함수는 '이번 사이클에 처음으로 판매 불가로 전이된 상품'만 추출하는 것이 목표입니다.
		// 직전에도 이미 단종 상태였다면 상태 변화가 없으므로, 알림을 다시 보낼 이유가 없습니다.
		//
		// prevProductsByID가 nil인 경우는 서버 최초 실행 등으로 비교할 이전 스냅샷이 없는 상황을 뜻합니다.
		// 이 경우에는 전이 여부를 판별할 수 없으므로 비교를 건너뛰고, 아래의 감시 대상 여부 확인으로 진행합니다.
		if prevProductsByID != nil {
			if prevProduct, exists := prevProductsByID[currentProduct.ID]; exists && prevProduct.IsUnavailable {
				continue
			}
		}

		productID := strconv.Itoa(currentProduct.ID)
		productName, exists := productNamesByID[productID]
		if !exists {
			// productNamesByID에 해당 ID가 없다는 것은, 이 상품이 현재 감시 대상 CSV에 없다는 의미입니다.
			// CSV에서 행이 삭제됐거나 사용자가 감시를 비활성화한 상품일 가능성이 높으므로, 알림 대상에서 제외합니다.
			continue
		}

		// CSV의 상품명 칼럼이 비어있는 경우, 알림 메시지에 공백이 그대로 노출되지 않도록 대체 텍스트를 사용합니다.
		if productName == "" {
			productName = fallbackProductName
		}

		newlyUnavailableProducts = append(newlyUnavailableProducts, struct{ ID, Name string }{
			ID:   productID,
			Name: productName,
		})
	}

	return newlyUnavailableProducts
}
