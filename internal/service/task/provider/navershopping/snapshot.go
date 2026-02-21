package navershopping

import "sort"

// productEventType 상품 데이터의 상태 변화(변경 유형)를 식별하기 위한 열거형입니다.
type productEventType int

const (
	// productEventNone 상품 정보에 변경 사항이 없음을 나타냅니다. (기본값)
	productEventNone productEventType = iota

	// productEventNew 이전 수집 결과에는 없었으나, 이번 검색에서 새롭게 발견된 신규 상품임을 나타냅니다.
	productEventNew

	// productEventPriceChanged 이전과 동일한 상품이지만, 판매 가격(최저가)이 변동된 상품임을 나타냅니다.
	productEventPriceChanged
)

// productDiff 이전 스냅샷과 현재 수집된 상품 정보를 비교하여 발견된 개별 상품의 변경 사항을 정의합니다.
//
// watchPriceSnapshot.Compare()의 결과물로 생성되며, 신규 상품 등록이나 가격 변동과 같은 이벤트를 발생시킵니다.
// 수집된 원본 데이터와 이전 상태를 동시에 보유하여, 나중에 알림 메시지를 구성할 때 구체적인 차이점을 보여줄 수 있도록 돕습니다.
type productDiff struct {
	// Type 발견된 변경 사항의 종류를 식별합니다. (예: 신규 상품 등록, 최저가 변동)
	Type productEventType

	// Product 최신 상태의 상품 정보입니다.
	Product *product

	// Prev 이전 상태의 상품 정보입니다.
	// 최저가가 변동된 경우에만 값이 존재하며, 신규 상품 등록 시에는 nil입니다.
	Prev *product
}

// watchPriceSnapshot 네이버 쇼핑 수집 작업의 특정 시점 상태를 기록하는 스냅샷 구조체입니다.
//
// API를 통해 조회된 상품 정보들을 보관하며, 데이터베이스나 캐시에 저장되어
// 다음번 수집 결과와 비교(Compare)하기 위한 기준 데이터로 활용됩니다.
type watchPriceSnapshot struct {
	// Products 수집된 시점에 발견된 상품 정보들의 전체 목록입니다.
	Products []*product `json:"products"`
}

// Compare 현재 수집된 상품 스냅샷을 이전 상태와 대조하여 유의미한 변화를 감지합니다.
//
// 다음 시나리오에 대한 변화를 감지합니다:
//  1. 신규 상품 등록: 이전 스냅샷에 없던 상품이 현재 검색 결과에 새롭게 등장한 경우
//  2. 가격 변동: 이전과 동일한 상품이지만 최저가(lprice)가 변동된 경우 (상승/하락 모두 포함)
//  3. 메타 정보 변경: 상품 가격과 무관하게 상품명·링크·판매처·유형 등이 수정된 경우
//  4. 상품 이탈: 이전 스냅샷에 있던 상품이 현재 검색 결과에서 더 이상 조회되지 않는 경우
//
// 반환되는 diffs는 가격 오름차순으로 정렬되어 있어, 호출부에서는 가장 유리한 조건의 상품부터
// 즉시 알림으로 구성하여 보낼 수 있습니다.
//
// 매개변수:
//   - prev: 비교의 기준이 되는 이전 실행 시점의 스냅샷 데이터 (최초 실행 시 nil)
//
// 반환값:
//   - diffs: 알림 대상인 변동 상품 목록 (신규 등록, 가격 변동)
//   - hasChanges: 스냅샷 갱신이 필요한지 여부 (신규/삭제/가격변동/메타변경 모두 포함)
func (s *watchPriceSnapshot) Compare(prev *watchPriceSnapshot) (diffs []productDiff, hasChanges bool) {
	// 1. 빠른 조회를 위해 이전 상품 목록을 Map으로 변환한다.
	prevMap := make(map[string]*product)
	if prev != nil {
		for _, p := range prev.Products {
			prevMap[p.key()] = p
		}
	}

	// 2. 상품 목록을 가격 오름차순으로 정렬하여 사용자가 가장 저렴한 상품을 먼저 확인할 수 있도록 합니다.
	// 가격이 동일한 경우, 일관된 순서를 보장하기 위해 상품명으로 2차 정렬을 수행합니다.
	sort.Slice(s.Products, func(i, j int) bool {
		p1 := s.Products[i]
		p2 := s.Products[j]

		if p1.LowPrice != p2.LowPrice {
			return p1.LowPrice < p2.LowPrice
		}

		// 가격이 같으면 이름순으로 정렬 (안정성 확보)
		return p1.Title < p2.Title
	})

	// 3. 현재 스냅샷의 상품들을 순회하며 신규 등록 및 가격 변동 감지
	for _, p := range s.Products {
		prevProduct, exists := prevMap[p.key()]
		if !exists {
			// 케이스 1: 신규 상품 발견
			// 이전 스냅샷에 없던 상품이므로 diffs에 추가합니다.
			diffs = append(diffs, productDiff{
				Type:    productEventNew,
				Product: p,
				Prev:    nil,
			})

			hasChanges = true
		} else {
			// 케이스 2: 기존 상품의 가격 변동 및 내용 변경 확인
			// 실제 가격 변화가 발생한 경우에만 알림(diffs 추가)을 생성합니다.
			if p.LowPrice != prevProduct.LowPrice {
				diffs = append(diffs, productDiff{
					Type:    productEventPriceChanged,
					Product: p,
					Prev:    prevProduct,
				})

				hasChanges = true
			} else if !p.contentEquals(prevProduct) {
				// 알림 대상은 아니지만, 메타 정보가 변경되었으므로 스냅샷 갱신 필요
				hasChanges = true
			}
		}
	}

	// 4. 상품 삭제 감지(개수 비교) 및 비정상 상황(0건) 방어
	prevLen := 0
	if prev != nil {
		prevLen = len(prev.Products)
	}

	// [중요] 일시적 오류로 인해 검색 결과가 0건이 된 경우 스냅샷 갱신 방지
	//
	// 시나리오: 네이버 측 일시 오류나 네트워크 문제로 0건이 반환될 수 있습니다.
	// 이때 스냅샷을 0건으로 갱신하면, 다음 정상 실행 시 모든 상품이 '신규'로 인식되어
	// 사용자에게 대량 알림(Spam)이 전송되는 참사가 발생합니다.
	//
	// 전략: '이전에 데이터가 있었는데 갑자기 0건이 된 경우'는 비정상으로 간주하여
	// 변경사항이 없다고 판단(false 반환)하고 기존 스냅샷을 유지합니다.
	//
	// [주의 - 한계점]
	// 이 안전 장치로 인해 실제로 모든 상품이 품절되어 0건이 된 경우에도
	// 스냅샷이 갱신되지 않아, DB에 과거 데이터가 남을 수 있습니다.
	// 하지만 잘못된 알림 폭탄(Spam)으로 인한 사용자 경험 저하를 막는 것이
	// 데이터 정합성보다 우선순위가 높다고 판단하여 이 방식을 채택합니다.
	if len(s.Products) == 0 && prevLen > 0 {
		return nil, false
	}

	// [참고] 이 로직은 삭제된 상품을 정확히 식별(누가 삭제되었는지)하지 않고,
	// 단순히 전체 개수가 변동되었는지만 확인하여 스냅샷 갱신(`hasChanges=true`)을 유도합니다.
	if len(s.Products) != prevLen {
		hasChanges = true
	}

	return diffs, hasChanges
}
