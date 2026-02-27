package kurly

// productEventType 상품의 상태 변화 유형을 나타내는 열거형입니다.
type productEventType int

const (
	// productEventNone 상품 정보에 변경 사항이 없음을 나타냅니다.
	productEventNone productEventType = iota

	// productEventNew 이전 스냅샷에 없던 상품이 새롭게 발견된 경우입니다.
	// 단, 최초 수집부터 실패한 상품(FetchFailedCount > 0)은 정상 수집될 때까지 이 이벤트 발생이 보류됩니다.
	productEventNew

	// productEventReappeared 이전에는 품절·판매중지 상태였던 상품이 다시 구매 가능해진 경우입니다.
	// '재등장' 자체가 핵심이므로, productDiff.Prev는 nil로 설정되어 가격 비교 없이 신규 상품처럼 알림을 생성합니다.
	productEventReappeared

	// productEventPriceChanged 판매 중인 상품의 실구매가가 이전 대비 변동되었으나, 역대 최저가 갱신에는 해당하지 않는 경우입니다.
	// 가격 상승과 하락 모두 이 이벤트로 처리됩니다.
	productEventPriceChanged

	// productEventLowestPriceAchieved 현재 실구매가가 이전 수집 결과들 중 가장 낮은 가격(역대 최저가)과 일치하는 경우입니다.
	// productEventPriceChanged와 상호 배타적이며, 최저가 갱신 여부를 우선 판단하여 이 이벤트로 분기됩니다.
	productEventLowestPriceAchieved

	// productEventDisappeared 상품이 판매 중지된 경우입니다.
	// 현재 로직에서는 Diff 목록에 포함되지 않아 알림이 발생하지 않습니다.
	// 향후 "판매 중지 알림" 기능 추가 시 활용하기 위해 확장성 차원에서 미리 정의해 둡니다.
	productEventDisappeared
)

// productDiff 이전 스냅샷과 현재 수집된 상품 정보를 비교하여 발견된 개별 상품의 변동 사항입니다.
type productDiff struct {
	// Type 발견된 변동 사항의 종류를 식별합니다. (예: 신규 등록, 재입고, 가격 변동)
	Type productEventType

	// Product 현재 수집된 최신 상태의 상품 정보입니다.
	Product *product

	// Prev 이전 스냅샷의 상품 정보입니다.
	// 가격 변동 시에만 값이 존재하며, 신규 등록이나 재입고의 경우에는 nil입니다.
	Prev *product
}

// watchProductPriceSnapshot 특정 수집 작업의 전체 상품 상태를 기록하는 스냅샷 구조체입니다.
//
// 스토리지에 저장되어 다음 수집 작업과의 비교 기준 데이터로 활용됩니다.
// 단순한 상품 목록 외에도 중복 알림 방지를 위한 State-Machine 상태 정보를 함께 영속화합니다.
type watchProductPriceSnapshot struct {
	// Products 이번 수집 작업에서 감시 대상으로 처리된 전체 상품 목록입니다.
	Products []*product `json:"products"`

	// DuplicateNotifiedIDs 이미 중복 등록 알림이 발송된 상품 ID 목록입니다.
	// 동일한 중복 상품에 대해 매 수집 작업마다 알림이 반복 발송되는 스팸을 방지하기 위한 State-Machine의 상태값입니다.
	DuplicateNotifiedIDs []string `json:"duplicate_notified_ids,omitempty"`
}

// HasChanged 현재 스냅샷과 이전 스냅샷을 비교하여 변경 여부를 반환합니다.
//
// 알림 메시지 생성 여부와 무관하게, 내부 상태(상품 목록, 중복 발송 이력 등)가 단 하나라도
// 달라졌다면 true를 반환하여 호출자가 스냅샷을 Storage에 저장하도록 유도합니다.
//
// [이 함수가 필요한 이유]
// 알림 메시지가 없다고 해서 저장을 건너뛰면, 다음과 같은 State-Machine 동기화 오류가 발생합니다.
//   - 좀비 데이터 방치: FetchFailedCount 누적이나 IsUnavailable 전이 같이 내부적으로 상태가 바뀌었음에도 저장이 누락되면,
//     다음 사이클에서 "이전과 동일한 에러 상태"를 또다시 감지하여 알림을 중복 발송합니다.
//   - 스토리지 누수: 사용자가 감시 대상 상품을 삭제했을 때 스냅샷이 갱신되지 않으면,
//     삭제된 상품의 데이터가 스토리지에 영구히 잔류하게 됩니다.
func (s *watchProductPriceSnapshot) HasChanged(prev *watchProductPriceSnapshot) bool {
	// 이전 스냅샷이 아예 없는 경우, 이것이 최초 실행임을 의미합니다.
	// 저장할 기준 데이터 자체가 없으므로 무조건 변경된 것으로 간주합니다.
	if prev == nil {
		return true
	}

	// [비교 1] 중복 알림 발송 이력 변경 여부
	// 중복 등록된 상품에 대해 이미 알림을 발송했는지 추적하는 State-Machine의 상태값입니다.
	// 발송 이력 목록이 조금이라도 달라졌다면 스팸 방지 상태가 변경된 것이므로 반드시 저장해야 합니다.
	if len(s.DuplicateNotifiedIDs) != len(prev.DuplicateNotifiedIDs) {
		return true
	}
	for i, id := range s.DuplicateNotifiedIDs {
		if id != prev.DuplicateNotifiedIDs[i] {
			return true
		}
	}

	// [비교 2] 상품 개수 변경 여부
	// 감시 대상 상품이 추가·삭제되거나, 수집 과정에서 일부 상품이 누락된 경우 개수가 달라집니다.
	// 개수만 달라도 이미 변경이 확실하므로, 불필요한 세부 비교 없이 즉시 반환합니다.
	if len(s.Products) != len(prev.Products) {
		return true
	}

	// [비교 3] 개별 상품 상태 필드 깊은 비교 (Deep Compare)
	// 상품 수가 같더라도 가격·품절 상태 등 내부 필드가 달라진 상품이 있을 수 있습니다.
	// 이전·현재 스냅샷 간에 상품 순서가 달라질 수 있으므로, 슬라이스 인덱스 순서에 의존하지 않고
	// ID를 키로 하는 Map을 구성하여 대조합니다.
	prevProductsByID := make(map[int]*product, len(prev.Products))
	for _, prevProduct := range prev.Products {
		prevProductsByID[prevProduct.ID] = prevProduct
	}

	for _, p := range s.Products {
		prevProduct, exists := prevProductsByID[p.ID]
		if !exists {
			// 이전 스냅샷에 같은 ID의 상품이 존재하지 않는다면, 이번 수집 사이클에서 새로 감지된 상품입니다.
			// 감시 대상 상품 목록에 신규 추가되었거나 상품 ID가 교체된 경우에 해당하므로, 변경된 것으로 판단합니다.
			return true
		}

		// 가격·품절 상태 등 알림 발송 여부에 직결되는 핵심 필드들을 이전 상품과 하나씩 비교합니다.
		//
		// 상품명(Name)은 비교 대상에서 제외합니다.
		//   - 크롤링 실패 시 수집되지 않아 신뢰도가 낮습니다.
		//   - 알림 메시지에서는 스냅샷의 상품명 대신 CSV 파일의 이름을 우선 사용하므로, 변경 판단 기준으로 삼지 않습니다.
		if p.Price != prevProduct.Price ||
			p.DiscountedPrice != prevProduct.DiscountedPrice ||
			p.DiscountRate != prevProduct.DiscountRate ||
			p.LowestPrice != prevProduct.LowestPrice ||
			!p.LowestPriceTimeUTC.Equal(prevProduct.LowestPriceTimeUTC) ||
			p.IsUnavailable != prevProduct.IsUnavailable ||
			p.FetchFailedCount != prevProduct.FetchFailedCount {
			return true
		}
	}

	return false
}
