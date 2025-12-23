package navershopping

import (
	"fmt"
	"strings"

	"github.com/darkkaiser/notify-server/pkg/strutil"
)

// product 검색 API를 통해 조회된 개별 상품 정보를 담는 도메인 모델입니다.
type product struct {
	ProductID   string `json:"productId"`   // 네이버 쇼핑 상품 ID (상품 고유 식별자)
	ProductType string `json:"productType"` // 상품 유형 (1: 일반, 2: 중고, 3: 단종, 4: 판매예정 등)
	Title       string `json:"title"`       // 상품명 (HTML 태그가 포함될 수 있음)
	Link        string `json:"link"`        // 상품 상세 정보 페이지 URL
	LowPrice    int    `json:"lprice"`      // 판매 최저가 (단위: 원)
	MallName    string `json:"mallName"`    // 판매 쇼핑몰 상호 (예: "네이버", "쿠팡" 등)
}

// Key 상품을 고유하게 식별하기 위한 키를 반환합니다.
func (p *product) Key() string {
	return p.ProductID
}

// Render 상품 정보를 알림 메시지 포맷으로 렌더링하여 반환합니다.
func (p *product) Render(supportsHTML bool, mark string) string {
	if supportsHTML {
		const htmlFormat = `☞ <a href="%s"><b>%s</b></a> (%s) %s원%s`

		return fmt.Sprintf(
			htmlFormat,
			p.Link,
			p.Title,
			p.MallName,
			strutil.FormatCommas(p.LowPrice),
			mark,
		)
	}

	const textFormat = `☞ %s (%s) %s원%s
%s`

	return strings.TrimSpace(fmt.Sprintf(textFormat, p.Title, p.MallName, strutil.FormatCommas(p.LowPrice), mark, p.Link))
}
