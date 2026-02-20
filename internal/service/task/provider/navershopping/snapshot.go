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

// Compare 현재 수집된 상품 스냅샷을 이전 상태와 대조하여 유의미한 변화를 추출합니다.
//
// 매 실행 시 수집된 상품 목록을 다음 기준에 따라 분석합니다:
//  1. 신규 상품 발견: 이전 수집 결과에는 없었으나 새롭게 검색 결과에 등장한 상품을 감지합니다.
//  2. 가격 변동 확인: 이미 알고 있는 상품이지만, 이전보다 저렴해졌거나 가격이 오른 경우를 감지합니다.
//
// 분석된 결과는 가격이 저렴한 순서대로 정렬되어 반환되므로, 호출부에서는 가장 유리한 조건의 상품부터
// 즉시 사용자에게 알림으로 구성하여 보낼 수 있습니다.
//
// 매개변수:
//   - prev: 비교의 기준이 되는 이전 실행 시점의 스냅샷 데이터입니다. (최초 실행 시 nil)
//
// 반환값:
//   - diffs: 신규 등록되었거나 가격이 변한 상품들의 상세 변경 정보 목록입니다.
func (s *watchPriceSnapshot) Compare(prev *watchPriceSnapshot) (diffs []productDiff) {
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
		} else {
			// 케이스 2: 기존 상품의 가격 변동 확인
			// 단순 재수집된 경우는 무시하고, 실제 가격 변화가 발생한 경우에만 알림을 생성합니다.
			if p.LowPrice != prevProduct.LowPrice {
				diffs = append(diffs, productDiff{
					Type:    productEventPriceChanged,
					Product: p,
					Prev:    prevProduct,
				})
			}
		}
	}

	return diffs
}
