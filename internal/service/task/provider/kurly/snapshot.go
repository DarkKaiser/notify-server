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
