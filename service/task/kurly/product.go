package kurly

import (
	"fmt"
	"html/template"
	"strings"
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

	// timeLayout 최저가 갱신 시간 등을 표시할 때 사용하는 날짜/시간 포맷입니다.
	timeLayout = "2006/01/02 15:04"
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

// URL 상품 상세 페이지의 전체 URL을 반환합니다.
func (p *product) URL() string {
	return fmt.Sprintf(productPageURLFormat, p.ID)
}

// IsOnSale 상품이 현재 할인 중인지 여부를 반환합니다.
// 할인가가 존재하고(0보다 크고), 정가보다 저렴해야 할인 중으로 간주합니다.
func (p *product) IsOnSale() bool {
	return p.DiscountedPrice > 0 && p.DiscountedPrice < p.Price
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
	if p.IsOnSale() {
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

// Render 상품 정보를 알림 메시지 포맷으로 렌더링하여 반환합니다.
func (p *product) Render(supportsHTML bool, mark string, prev *product) string {
	var sb strings.Builder
	sb.Grow(256) // 예상되는 문자열 크기만큼 미리 할당 (최적화)

	// 1. 상품 이름 및 링크
	safeName := template.HTMLEscapeString(p.Name)
	if supportsHTML {
		fmt.Fprintf(&sb, "☞ <a href=\"%s\"><b>%s</b></a>%s", p.URL(), safeName, mark)
	} else {
		// 기존 로직 유지: Text 모드에서도 HTMLEscapeString 적용
		fmt.Fprintf(&sb, "☞ %s%s", safeName, mark)
	}

	// 2. 현재 가격
	sb.WriteString("\n      • 현재 가격 : ")
	writePrice(&sb, p.Price, p.DiscountedPrice, p.DiscountRate, supportsHTML)

	// 3. 이전 가격
	if prev != nil {
		sb.WriteString("\n      • 이전 가격 : ")
		writePrice(&sb, prev.Price, prev.DiscountedPrice, prev.DiscountRate, supportsHTML)
	}

	// 4. 최저 가격
	if p.LowestPrice != 0 {
		sb.WriteString("\n      • 최저 가격 : ")
		writePrice(&sb, p.LowestPrice, 0, 0, supportsHTML)

		timeStr := p.LowestPriceTime.Format(timeLayout)
		fmt.Fprintf(&sb, " (%s)", timeStr)
	}

	return sb.String()
}

// writePrice 정가, 할인가, 할인율 정보를 조합하여 사용자 친화적인 가격 문자열을 생성하고 빌더에 기록합니다.
//
// [기능 상세]
// 1. 할인이 적용되지 않은 경우: 정가만 표시 (예: "10,000원")
// 2. 할인이 적용된 경우:
//   - HTML 모드: 정가에 취소선(<s>) 적용 + 할인가 + 할인율 (예: "<s>10,000원</s> 9,000원 (10%)")
//   - Text 모드: 정가 ⇒ 할인가 + 할인율 (예: "10,000원 ⇒ 9,000원 (10%)")
//
// [매개변수]
//   - sb: 결과를 기록할 strings.Builder 포인터
//   - price: 할인 전 원래 가격 (정가)
//   - discountedPrice: 할인 후 가격 (0 또는 price와 같으면 할인 없음으로 간주)
//   - discountRate: 할인율 (정수 퍼센트)
//   - supportsHTML: HTML 태그 포함 여부 (Telegram 등 리치 텍스트 지원 클라이언트용)
func writePrice(sb *strings.Builder, price, discountedPrice, discountRate int, supportsHTML bool) {
	// [방어적 코드]
	// 1. discountedPrice <= 0: 할인가 정보 없음
	// 2. discountedPrice >= price: 할인가가 정가보다 비싸거나 같음 (데이터 오류 또는 할인 없음)
	// 위 경우에는 할인 표기를 하지 않고 '정가'만 노출하여 혼란을 방지합니다.
	if discountedPrice <= 0 || discountedPrice >= price {
		fmt.Fprintf(sb, "%s원", strutil.FormatCommas(price))
		return
	}

	formattedPrice := strutil.FormatCommas(price)
	formattedDiscountedPrice := strutil.FormatCommas(discountedPrice)

	// 할인율이 유효한 경우에만 문자열 생성 (0% 표시 방지)
	var rateStr string
	if discountRate > 0 {
		rateStr = fmt.Sprintf(" (%d%%)", discountRate)
	}

	if supportsHTML {
		// 예: <s>10,000원</s> 9,000원 (10%)
		fmt.Fprintf(sb, "<s>%s원</s> %s원%s", formattedPrice, formattedDiscountedPrice, rateStr)
		return
	}

	// 예: 10,000원 ⇒ 9,000원 (10%)
	fmt.Fprintf(sb, "%s원 ⇒ %s원%s", formattedPrice, formattedDiscountedPrice, rateStr)
}
