package kurly

import (
	"fmt"
	"time"
)

const (
	// productPageURLFormat 마켓컬리 상품 상세 페이지 URL을 생성하기 위한 fmt.Sprintf 포맷 문자열입니다.
	//
	// %v 자리에 상품 코드(int)를 대입하면 해당 상품의 상세 페이지 URL이 완성됩니다.
	//
	// 사용 예시:
	//
	//  url := fmt.Sprintf(productPageURLFormat, 12345) // → "https://www.kurly.com/goods/12345"
	productPageURLFormat = "https://www.kurly.com/goods/%v"
)

// product 마켓컬리 상품 상세 페이지에서 수집된 개별 상품 정보를 담는 도메인 모델입니다.
//
// 스냅샷에 포함되어 스토리지에 영속화되며, 다음 수집 사이클에서 가격 변동을 비교하는 기준으로 활용됩니다.
type product struct {
	// ID 마켓컬리에서 부여한 상품 고유 코드입니다. URL의 상품 번호와 동일합니다.
	ID int `json:"no"`

	// Name 상품의 표시 이름입니다.
	Name string `json:"name"`

	// Price 할인 전 정가입니다.
	Price int `json:"price"`

	// DiscountedPrice 할인이 적용된 판매가입니다.
	// 할인이 없는 경우 0 또는 Price와 동일한 값이 됩니다.
	DiscountedPrice int `json:"discounted_price"`

	// DiscountRate 할인율(정수 퍼센트)입니다.
	DiscountRate int `json:"discount_rate"`

	// LowestPrice 이 상품의 역대 최저가입니다.
	// 수집 작업 시마다 tryUpdateLowestPrice()를 호출하여 현재 가격과 비교한 후 갱신합니다.
	// 0이면 아직 최저가가 기록된 적이 없음을 의미합니다.
	LowestPrice int `json:"lowest_price"`

	// LowestPriceTimeUTC LowestPrice가 마지막으로 갱신된 시각(UTC)입니다.
	LowestPriceTimeUTC time.Time `json:"lowest_price_time"`

	// IsUnavailable 상품 정보를 정상적으로 수집할 수 없는 상태인지 여부입니다.
	// 상품 페이지가 존재하지 않거나 판매 중지된 경우 true로 설정됩니다.
	// 또는 FetchFailedCount가 임계치(3회)에 도달할 때도 true로 강제 전이됩니다.
	//
	// [참고] JSON 태그는 하위 호환을 위해 'is_unknown_product'을 유지하지만, 필드명은 의미의 명확성을 위해 'IsUnavailable'을 사용합니다.
	IsUnavailable bool `json:"is_unknown_product"`

	// FetchFailedCount 상품 페이지 크롤링에 연속으로 실패한 횟수입니다.
	// 일시적인 네트워크 장애와 영구적인 상품 소멸을 구별하기 위해 추적합니다.
	// 3회 이상 연속 실패 시 IsUnavailable=true로 전이하여 좀비 데이터 생성을 방지합니다.
	FetchFailedCount int `json:"fetch_failed_count,omitempty"`
}

// productPageURL 상품 ID를 받아 상품 상세 페이지 URL을 반환합니다.
func productPageURL(id any) string {
	return fmt.Sprintf(productPageURLFormat, id)
}

// pageURL 이 상품의 상세 페이지 URL을 반환합니다.
func (p *product) pageURL() string {
	return productPageURL(p.ID)
}

// isOnSale 상품이 현재 할인 중인지 여부를 반환합니다.
//
// 다음 두 조건을 모두 만족해야 할인 중으로 간주합니다.
//   - DiscountedPrice > 0 : 할인가 정보가 존재할 것
//   - DiscountedPrice < Price : 할인가가 정가보다 실제로 저렴할 것
//
// 할인가가 있더라도 정가 이상이면 데이터 오류로 보고 false를 반환합니다.
func (p *product) isOnSale() bool {
	return p.DiscountedPrice > 0 && p.DiscountedPrice < p.Price
}

// effectivePrice 고객이 실제로 지불하는 가격을 반환합니다.
//
// 할인 중(isOnSale)이면 DiscountedPrice를, 그렇지 않으면 Price(정가)를 반환합니다.
func (p *product) effectivePrice() int {
	if p.isOnSale() {
		return p.DiscountedPrice
	}

	return p.Price
}

// hasPriceChangedFrom 현재 상품의 가격 정보가 이전 상품(prev)과 하나라도 달라졌는지 확인합니다.
//
// 정가(Price), 할인가(DiscountedPrice), 할인율(DiscountRate) 중 하나라도 변경되었다면
// 사용자에게 알릴 가치가 있는 변동으로 간주하여 true를 반환합니다.
//
// prev가 nil인 경우(이전 스냅샷이 없는 신규 상품)는 변경된 것으로 간주합니다.
func (p *product) hasPriceChangedFrom(prev *product) bool {
	if prev == nil {
		return true
	}

	return p.Price != prev.Price ||
		p.DiscountedPrice != prev.DiscountedPrice ||
		p.DiscountRate != prev.DiscountRate
}

// tryUpdateLowestPrice 현재 가격이 역대 최저가보다 낮으면 최저가 정보를 갱신합니다.
//
// 갱신이 발생한 경우 true를, 갱신이 불필요하거나 가격이 유효하지 않으면 false를 반환합니다.
//
// [주의] 반드시 상품 변동 사항 비교 이전에 호출해야 합니다.
// 최저가를 먼저 확정해 두어야, 이번 수집 작업의 현재 가격이 역대 최저가 달성인지를 올바르게 판별할 수 있습니다.
func (p *product) tryUpdateLowestPrice() bool {
	// 이번 수집 작업의 가장 "낮은 가격"을 먼저 결정
	effectivePrice := p.effectivePrice()

	// 유효하지 않은 가격(0원 이하)은 최저가로 갱신하지 않습니다.
	if effectivePrice <= 0 {
		return false
	}

	// 기존 최저가가 설정되어 있지 않거나(0), 현재 가격이 기존 최저가보다 낮은 경우 최저가 정보를 갱신합니다.
	if p.LowestPrice == 0 || effectivePrice < p.LowestPrice {
		p.LowestPrice = effectivePrice

		// 서버 환경(TimeZone)에 의존하지 않기 위해 UTC를 명시적으로 사용합니다.
		p.LowestPriceTimeUTC = time.Now().UTC()

		return true
	}

	return false
}
