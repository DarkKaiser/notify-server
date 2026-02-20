package navershopping

// @@@@@
// productEventType 상품 데이터의 상태 변화(변경 유형)를 식별하기 위한 열거형입니다.
type productEventType int

// @@@@@
const (
	eventNone         productEventType = iota
	eventNewProduct                    // 신규 상품 (이전 검색 결과에 없던 상품)
	eventPriceChanged                  // 가격 변동 (이전과 동일 상품이나 최저가 변동)
)

// @@@@@
// productDiff 상품 데이터의 변동 사항(신규, 가격 변화 등)을 캡슐화한 중간 객체입니다.
type productDiff struct {
	Type    productEventType
	Product *product
	Prev    *product
}

// @@@@@
// watchPriceSnapshot executeWatchPrice 실행 시 수집한 상품 목록을 저장하는 스냅샷 구조체입니다.
// 이전 실행의 스냅샷과 현재 스냅샷을 비교하여 가격 변동·신규 등록·삭제된 상품을 감지하는 데 사용됩니다.
// JSON으로 직렬화되어 Storage에 영속적으로 저장되며, 다음 실행 시 이전 스냅샷으로 복원됩니다.
type watchPriceSnapshot struct {
	Products []*product `json:"products"`
}
