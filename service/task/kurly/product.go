package kurly

import (
	"fmt"
	"html/template"
	"time"

	"github.com/darkkaiser/notify-server/pkg/strutil"
)

const (
	// productPageURLFormat 마켓컬리 상품 상세 페이지의 URL을 생성하기 위한 포맷 문자열입니다.
	//
	// 사용 예시:
	//
	//  url := fmt.Sprintf(productPageURLFormat, 12345) // "https://www.kurly.com/goods/12345"
	productPageURLFormat = "https://www.kurly.com/goods/%v"
)

// product 마켓컬리 상품 상세 페이지에서 조회된 개별 상품 정보를 담는 도메인 모델입니다.
type product struct {
	ID              int       `json:"no"`                 // 상품 코드
	Name            string    `json:"name"`               // 상품 이름
	Price           int       `json:"price"`              // 가격
	DiscountedPrice int       `json:"discounted_price"`   // 할인 가격
	DiscountRate    int       `json:"discount_rate"`      // 할인율
	LowestPrice     int       `json:"lowest_price"`       // 최저 가격
	LowestPriceTime time.Time `json:"lowest_price_time"`  // 최저 가격이 등록된 시간
	IsUnavailable   bool      `json:"is_unknown_product"` // 상품 정보를 불러올 수 없는지에 대한 여부(상품 코드가 존재하지 않거나, 판매를 하고 있지 않는 상품)
}

// updateLowestPrice 현재 상품의 가격(정가 또는 할인가)과 기존 최저가를 비교하여,
// 더 낮은 가격이 발견되면 최저가 및 갱신 시간을 업데이트합니다.
//
// [동작 상세]
// 1. 현재 상품의 유효 가격(Effective Price)을 결정합니다. (할인가 존재 시 할인가 우선)
// 2. 유효 가격이 기존 최저가보다 낮거나, 기존 최저가 정보가 없는 경우 갱신합니다.
// 3. 갱신 시점의 시간을 고정하여 데이터 정합성을 보장합니다.
func (p *product) updateLowestPrice() {
	// 1. 현재 시점의 가장 "낮은 가격"을 먼저 결정
	effectivePrice := p.Price
	if p.DiscountedPrice > 0 && p.DiscountedPrice < p.Price {
		effectivePrice = p.DiscountedPrice
	}

	// 2. 시간 고정
	now := time.Now()

	// 3. 단 한 번의 비교 및 갱신
	if p.LowestPrice == 0 || p.LowestPrice > effectivePrice {
		p.LowestPrice = effectivePrice
		p.LowestPriceTime = now
	}
}

// @@@@@
// Render 상품 정보를 알림 메시지 포맷으로 렌더링하여 반환합니다.
func (p *product) Render(supportsHTML bool, mark string, previousProduct *product) string {
	// 상품 이름
	var name string
	if supportsHTML {
		name = fmt.Sprintf("☞ <a href=\"%s\"><b>%s</b></a>%s", fmt.Sprintf(productPageURLFormat, p.ID), template.HTMLEscapeString(p.Name), mark)
	} else {
		name = fmt.Sprintf("☞ %s%s", template.HTMLEscapeString(p.Name), mark)
	}

	// 상품의 이전 가격 문자열을 구한다.
	var previousPriceString string
	if previousProduct != nil {
		previousPriceString = fmt.Sprintf("\n      • 이전 가격 : %s", formatPrice(previousProduct.Price, previousProduct.DiscountedPrice, previousProduct.DiscountRate, supportsHTML))
	}

	// 상품의 최저 가격 문자열을 구한다.
	var lowestPriceString string
	if p.LowestPrice != 0 {
		lowestPriceString = fmt.Sprintf("\n      • 최저 가격 : %s (%s)", formatPrice(p.LowestPrice, 0, 0, supportsHTML), p.LowestPriceTime.Format("2006/01/02 15:04"))
	}

	return fmt.Sprintf("%s\n      • 현재 가격 : %s%s%s", name, formatPrice(p.Price, p.DiscountedPrice, p.DiscountRate, supportsHTML), previousPriceString, lowestPriceString)
}

// @@@@@
// formatPrice 가격 정보를 포맷팅하여 반환합니다.
func formatPrice(price, discountedPrice, discountRate int, supportsHTML bool) string {
	// 할인 가격이 없거나 가격과 동일하면 그냥 가격을 반환한다.
	if discountedPrice == 0 || discountedPrice == price {
		return fmt.Sprintf("%s원", strutil.FormatCommas(price))
	}

	if supportsHTML {
		return fmt.Sprintf("<s>%s원</s> %s원 (%d%%)", strutil.FormatCommas(price), strutil.FormatCommas(discountedPrice), discountRate)
	}
	return fmt.Sprintf("%s원 ⇒ %s원 (%d%%)", strutil.FormatCommas(price), strutil.FormatCommas(discountedPrice), discountRate)
}
