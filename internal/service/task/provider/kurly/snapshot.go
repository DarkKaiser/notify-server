package kurly

// productEventType 상품 데이터의 상태 변화(변경 유형)를 식별하기 위한 열거형입니다.
type productEventType int

const (
	eventNone               productEventType = iota
	eventNewProduct                          // 신규 상품 등록
	eventRestocked                           // 재입고
	eventPriceChanged                        // 가격 변동
	eventLowestPriceRenewed                  // 역대 최저가 갱신
	eventDiscontinued                        // 판매 중지 (현재 로직에서는 Diff에 포함되지 않으나 확장성을 위해 정의)
)

// productDiff 상품 데이터의 변동 사항(신규, 가격 변화 등)을 캡슐화한 중간 객체입니다.
type productDiff struct {
	Type    productEventType
	Product *product
	Prev    *product
}

// watchProductPriceSnapshot 가격 변동을 감지하기 위한 상품 데이터의 스냅샷입니다.
type watchProductPriceSnapshot struct {
	Products []*product `json:"products"`
}
