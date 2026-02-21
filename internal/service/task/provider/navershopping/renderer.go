package navershopping

import (
	"fmt"
	"strings"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

const (
	// estimatedProductMsgSize 단일 상품 정보를 렌더링할 때 필요한 예상 버퍼 크기(Byte)입니다.
	estimatedProductMsgSize = 300
)

// renderProduct 단일 상품 정보를 알림 메시지 포맷에 맞게 렌더링합니다.
//
// 상품의 제목, 판매처, 가격 정보를 조합하여 사용자에게 보여줄 최종 텍스트를 생성합니다.
// 내부적으로 formatProductItem을 호출하며, 이전 가격과의 비교가 필요 없는 일반적인 출력 상황에서 활용됩니다.
//
// 매개변수:
//   - p: 렌더링할 최신 상품 정보
//   - supportsHTML: 수신 채널(텔레그램 등)의 HTML 태그 지원 여부
//   - m: 상품명 옆에 표시될 상태 마크 (예: 신규 상품인 경우 "🆕")
//
// 반환값: 각 채널 규격에 맞춰 포맷팅이 완료된 상품 정보 문자열
func renderProduct(p *product, supportsHTML bool, m mark.Mark) string {
	return formatProductItem(p, supportsHTML, m, nil)
}

// renderProductDiffs 수집 과정에서 발견된 모든 변동 상품(신규 등록, 가격 변동)을 하나의 통합 메시지로 렌더링합니다.
//
// 개별 상품들의 정보를 순차적으로 조합하며, 가독성을 위해 상품 사이에는 적절한 구분(빈 줄)을 추가합니다.
// 스냅샷 비교 결과물인 diffs 슬라이스를 순회하면서 각 상품의 상태(신규/변동)에 맞는 상세 렌더러를 호출합니다.
//
// 매개변수:
//   - diffs: 분석을 통해 추출된 변동 상품 정보 목록
//   - supportsHTML: 수신 채널(텔레그램 등)의 HTML 태그 지원 여부
//
// 반환값: 모든 변동 상품 정보가 결합된 최종 알림 메시지 문자열 (변동 상품이 없으면 빈 문자열을 반환합니다)
func renderProductDiffs(diffs []productDiff, supportsHTML bool) string {
	if len(diffs) == 0 {
		return ""
	}

	var sb strings.Builder

	// 상품 개수 x 평균 메시지 크기로 버퍼 크기 사전 할당 (메모리 재할당 최소화)
	sb.Grow(len(diffs) * estimatedProductMsgSize)

	needSeparator := false
	for _, diff := range diffs {
		// 첫 번째 렌더링 항목이 아니면 구분을 위해 빈 줄 추가
		if needSeparator {
			sb.WriteString("\n\n")
		}

		switch diff.Type {
		case productEventNew:
			needSeparator = true
			sb.WriteString(renderProduct(diff.Product, supportsHTML, mark.New))

		case productEventPriceChanged:
			needSeparator = true
			sb.WriteString(formatProductItem(diff.Product, supportsHTML, mark.Modified, diff.Prev))
		}
	}

	return sb.String()
}

// formatProductItem 단일 상품 정보를 알림 메시지 포맷에 맞게 조립하는 핵심 내부 렌더러입니다.
//
// 상품명, 판매처, 현재 가격을 기본으로 표시하며, 이전 상품 정보가 제공된 경우 가격 변동 내역을 추가하여
// 사용자가 한눈에 변화를 인지할 수 있도록 돕습니다.
// renderProduct와 renderProductDiffs 양쪽에서 공통으로 호출되는 포맷팅 공유 함수로,
// 외부에서 직접 호출하는 대신 두 상위 함수를 통해 사용되는 것을 원칙으로 합니다.
//
// 매개변수:
//   - p: 렌더링할 최신 상품 정보
//   - supportsHTML: 수신 채널(텔레그램 등)의 HTML 태그 지원 여부
//   - m: 상품명 옆에 표시될 상태 마크 (예: 신규 상품 "🆕", 가격 변동 "🔄")
//   - prev: 가격 비교의 기준이 되는 이전 상품 정보 (nil인 경우 가격 변동 내역 표시 생략)
//
// 반환값: 채널 규격(HTML/텍스트)에 맞춰 포맷팅된 단일 상품 정보 문자열
//   - HTML: 상품명에 하이퍼링크와 볼드 처리를 적용하여 가독성을 높입니다.
//   - 텍스트: 상품 정보를 한 줄로 나열하고, 하단에 URL을 별도로 표시합니다.
func formatProductItem(p *product, supportsHTML bool, m mark.Mark, prev *product) string {
	var sb strings.Builder

	// 상품 평균 메시지 크기로 버퍼 크기 사전 할당 (메모리 재할당 최소화)
	sb.Grow(estimatedProductMsgSize)

	if supportsHTML {
		const htmlFormat = `☞ <a href="%s"><b>%s</b></a> (%s) %s원`

		fmt.Fprintf(&sb,
			htmlFormat,
			p.Link,
			p.Title,
			p.MallName,
			strutil.Comma(p.LowPrice),
		)
	} else {
		const textFormat = `☞ %s (%s) %s원`

		fmt.Fprintf(&sb,
			textFormat,
			p.Title,
			p.MallName,
			strutil.Comma(p.LowPrice),
		)
	}

	// 가격 변동이 있는 경우 이전 가격 표시 (HTML/Text 공통)
	if prev != nil {
		if p.LowPrice != prev.LowPrice {
			fmt.Fprintf(&sb, " (이전: %s원)", strutil.Comma(prev.LowPrice))
		}
	}

	// 상품 상태를 나타내는 마크 (예: "🆕", "🔄")
	sb.WriteString(m.WithSpace())

	// 텍스트 모드에서는 상품명 아래 줄에 링크를 별도로 표시
	if !supportsHTML {
		fmt.Fprintf(&sb, "\n%s", p.Link)
	}

	return sb.String()
}

// renderCurrentStatus 현재 스냅샷에 기록된 전체 감시 상품 목록을 하나의 통합 메시지로 렌더링합니다.
//
// 사용자가 수동으로 작업을 실행했으나 이전 대비 변경 사항이 없을 때,
// "현재 감시 중인 상품들의 최신 상태"를 한눈에 브리핑하기 위해 analyzeAndReport에서 호출됩니다.
//
// 매개변수:
//   - snapshot: 현재 시점에 수집된 전체 상품 정보 스냅샷
//   - supportsHTML: 알림을 수신할 메신저 채널(예: 텔레그램)의 HTML 서식 지원 여부
//
// 반환값:
//   - 전체 감시 상품 목록이 포함된 렌더링된 메시지 문자열
//   - 스냅샷이 nil이거나 상품이 0건인 경우 빈 문자열을 반환합니다.
func renderCurrentStatus(snapshot *watchPriceSnapshot, supportsHTML bool) string {
	if snapshot == nil || len(snapshot.Products) == 0 {
		return ""
	}

	var sb strings.Builder

	// 상품 개수 x 평균 메시지 크기로 버퍼 크기 사전 할당 (메모리 재할당 최소화)
	sb.Grow(len(snapshot.Products) * estimatedProductMsgSize)

	for i, p := range snapshot.Products {
		// 첫 번째 상품이 아니면 구분을 위해 빈 줄 추가
		if i > 0 {
			sb.WriteString("\n\n")
		}

		sb.WriteString(renderProduct(p, supportsHTML, ""))
	}

	return sb.String()
}

// renderSearchConditionsSummary 사용자가 설정한 조회 조건을 알림 메시지에 삽입할 요약 문자열로 렌더링합니다.
//
// 검색 키워드, 상품명 포함/제외 키워드, 가격 상한선을 하나의 통합된 텍스트 블록으로 조합합니다.
// 변경 감지 여부와 무관하게 알림 메시지의 앞부분에 "조회 조건 안내"로 항상 포함되어,
// 사용자가 어떤 기준으로 모니터링이 수행되고 있는지를 한눈에 확인할 수 있도록 돕습니다.
//
// 매개변수:
//   - settings: 렌더링에 사용할 조회 조건 값들입니다.
//
// 반환값: 여러 조건 항목이 글머리 기호(•)로 정리된 사람이 읽기 쉬운(Human-Readable) 형식의 문자열
func renderSearchConditionsSummary(settings *watchPriceSettings) string {
	return fmt.Sprintf(`조회 조건은 아래와 같습니다:

  • 검색 키워드 : %s
  • 상품명 포함 키워드 : %s
  • 상품명 제외 키워드 : %s
  • %s원 미만의 상품`,
		settings.Query,
		settings.Filters.IncludedKeywords,
		settings.Filters.ExcludedKeywords,
		strutil.Comma(settings.Filters.PriceLessThan),
	)
}
