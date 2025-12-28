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
// 주로 단일 상품 상태 조회와 같이 비교 대상이 없는 경우에 사용됩니다.
func (p *product) Render(supportsHTML bool, mark string) string {
	return p.renderInternal(supportsHTML, mark, nil)
}

// RenderDiff 현재 상품 상태와 과거 상태를 비교하여 변경 사항을 강조한 알림 메시지를 생성합니다.
func (p *product) RenderDiff(supportsHTML bool, mark string, prev *product) string {
	return p.renderInternal(supportsHTML, mark, prev)
}

// renderInternal 상품 알림 메시지를 생성하는 핵심 내부 구현체입니다.
func (p *product) renderInternal(supportsHTML bool, mark string, prev *product) string {
	var sb strings.Builder

	// 예상 버퍼 크기 할당
	sb.Grow(512)

	if supportsHTML {
		const htmlFormat = `☞ <a href="%s"><b>%s</b></a> (%s) %s원`

		fmt.Fprintf(&sb,
			htmlFormat,
			p.Link,
			p.Title,
			p.MallName,
			strutil.FormatCommas(p.LowPrice),
		)
	} else {
		const textFormat = `☞ %s (%s) %s원`

		fmt.Fprintf(&sb,
			textFormat,
			p.Title,
			p.MallName,
			strutil.FormatCommas(p.LowPrice),
		)
	}

	// 이전 가격 정보 추가 (HTML/Text 공통)
	if prev != nil {
		if p.LowPrice != prev.LowPrice {
			fmt.Fprintf(&sb, " (이전: %s원)", strutil.FormatCommas(prev.LowPrice))
		}
	}

	// Mark 추가
	sb.WriteString(mark)

	// Text 모드일 경우 줄바꿈 후 링크 추가
	if !supportsHTML {
		fmt.Fprintf(&sb, "\n%s", p.Link)
	}

	return sb.String()
}
