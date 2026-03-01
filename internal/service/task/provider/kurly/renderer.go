package kurly

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

const (
	// estimatedProductDiffSize 단일 상품의 변경 내역(Diff)을 문자열로 렌더링할 때 필요한 예상 버퍼 크기(Byte)입니다.
	estimatedProductDiffSize = 300

	// fallbackProductName CSV 데이터에서 상품명이 누락되었거나 공백일 경우 사용자에게 표시할 대체 텍스트입니다.
	fallbackProductName = "알 수 없는 상품"

	// lowestPriceTimeLayout 최저가 갱신 시각 등을 사람이 읽기 쉬운 문자열로 포맷팅할 때 사용하는 레이아웃입니다.
	lowestPriceTimeLayout = "2006/01/02 15:04"
)

// kstLocation 한국 표준시(KST, UTC+9) 타임존 객체입니다.
var kstLocation = time.FixedZone("KST", 9*60*60)

// renderProduct 단일 상품 정보를 알림 메시지 포맷으로 변환합니다.
//
// 신규 상품 등록이나 단일 상품 현황 조회와 같이 이전 상품 정보와의 비교가 필요 없는 경우에 사용합니다.
// 내부적으로 formatProductItem을 호출하며, 이전 상품 정보 인자(prev)는 nil로 고정하여 전달합니다.
//
// 매개변수:
//   - p: 렌더링할 상품 정보
//   - supportsHTML: 수신 체널(텔레그램 등)의 HTML 태그 지원 여부
//   - m: 상품명 옆에 표시될 상태 마크 (예: 신규 상품인 경우 "🆕")
//
// 반환값: 체널 규격(HTML/텍스트)에 맞게 포맷팅된 상품 정보 문자열
func renderProduct(p *product, supportsHTML bool, m mark.Mark) string {
	return formatProductItem(p, supportsHTML, m, nil)
}

// renderProductDiffs 분석 과정에서 감지된 모든 변동 상품(Diff)을 하나의 통합 알림 메시지로 렌더링합니다.
//
// 매개변수:
//   - diffs: 스냅샷 비교를 통해 추출된 변동 상품 정보 목록
//   - supportsHTML: 수신 체널(텔레그램 등)의 HTML 태그 지원 여부
//
// 반환값: 모든 변동 상품 정보가 결합된 최종 알림 메시지 문자열 (변동 상품이 없으면 빈 문자열을 반환합니다)
func renderProductDiffs(diffs []productDiff, supportsHTML bool) string {
	if len(diffs) == 0 {
		return ""
	}

	var sb strings.Builder

	// 변경된 상품 내역을 렌더링하기 위해 필요한 메모리 크기를 사전에 예측하여 할당합니다.
	// 예측 공식: (변동 사항 수) × (항목당 예상 크기)
	sb.Grow(len(diffs) * estimatedProductDiffSize)

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

		case productEventReappeared:
			needSeparator = true
			sb.WriteString(renderProduct(diff.Product, supportsHTML, mark.New))

		case productEventLowestPriceAchieved:
			needSeparator = true
			sb.WriteString(formatProductItem(diff.Product, supportsHTML, mark.BestPrice, diff.Prev))

		case productEventPriceChanged:
			needSeparator = true
			sb.WriteString(formatProductItem(diff.Product, supportsHTML, mark.Modified, diff.Prev))
		}
	}

	return sb.String()
}

// formatProductItem 단일 상품 알림 메시지를 조립하는 핵심 내부 렌더러입니다.
//
// renderProduct와 renderProductDiffs 양쪽에서 공통으로 호출되는 포맷팅 공유 함수로,
// 외부에서 직접 호출하는 대신 상위 두 함수를 통해 사용되는 것을 원칙으로 합니다.
// 상품명(하이퍼링크), 현재 가격, 이전 가격, 역대 최저가 순으로 조합하여 출력합니다.
//
// 매개변수:
//   - p: 렌더링할 최신 상품 정보
//   - supportsHTML: 수신 체널(텔레그램 등)의 HTML 태그 지원 여부
//   - m: 상품명 옆에 표시될 상태 마크 (예: 신규 "🆕", 최저가 "🔥", 가격 변동 "🔄")
//   - prev: 가격 비교의 기준이 되는 이전 상품 정보 (nil이면 이전 가격 항목 표시 생략)
//
// 반환값: 채널 규격(HTML/텍스트)에 맞춰 포맷팅된 단일 상품 정보 문자열
func formatProductItem(p *product, supportsHTML bool, m mark.Mark, prev *product) string {
	var sb strings.Builder

	// 예상되는 크기로 버퍼 크기 사전 할당 (메모리 재할당 최소화)
	sb.Grow(estimatedProductDiffSize)

	// 상품 이름 및 링크
	var formattedName string
	if supportsHTML {
		formattedName = fmt.Sprintf("<a href=\"%s\"><b>%s</b></a>", p.pageURL(), template.HTMLEscapeString(p.Name))
	} else {
		formattedName = p.Name
	}

	fmt.Fprintf(&sb, "☞ %s%s", formattedName, m.WithSpace())

	// 현재 가격
	sb.WriteString("\n      • 현재 가격 : ")
	writeFormattedPrice(&sb, p.Price, p.DiscountedPrice, p.DiscountRate, supportsHTML)

	// 이전 가격
	if prev != nil {
		sb.WriteString("\n      • 이전 가격 : ")
		writeFormattedPrice(&sb, prev.Price, prev.DiscountedPrice, prev.DiscountRate, supportsHTML)
	}

	// 최저 가격
	if p.LowestPrice != 0 {
		sb.WriteString("\n      • 최저 가격 : ")
		writeFormattedPrice(&sb, p.LowestPrice, 0, 0, supportsHTML)

		// UTC 시간을 한국 시간(KST, UTC+9)으로 변환하여 표시
		kstTime := p.LowestPriceTimeUTC.In(kstLocation)
		fmt.Fprintf(&sb, " (%s)", kstTime.Format(lowestPriceTimeLayout))
	}

	return sb.String()
}

// writeFormattedPrice 가격 정보를 채널 규격에 맞게 포맷팅하여 문자열 Builder에 직접 씁니다.
//
// 할인가의 유효성을 먼저 검사하고, 유효한 경우에만 할인 표기를 추가합니다.
// 유효하지 않은 경우(아래 방어 조건 참고)에는 정가만 단순 출력하여 사용자 혼란을 방지합니다.
//
// [방어 조건 - 정가만 출력하는 경우]
//   - discountedPrice <= 0: 할인가 정보 자체가 없는 경우 (예: 최저가 이력처럼 할인가가 없는 가격)
//   - discountedPrice >= price: 할인가가 정가 이상인 경우 (데이터 오류 또는 실질적 할인 없음)
//
// 매개변수:
//   - sb: 포맷팅된 결과를 누적할 문자열 Builder (새 문자열을 할당하지 않고 직접 씁니다)
//   - price: 정가 (원 단위)
//   - discountedPrice: 할인가 (원 단위). 0 이하이면 할인 없음으로 간주합니다.
//   - discountRate: 할인율 (퍼센트, 0~100). 0이면 할인율 표기를 생략합니다.
//   - supportsHTML: 수신 채널(텔레그램 등)의 HTML 태그 지원 여부
//
// 출력 예시:
//   - 할인 없음:               "10,000원"
//   - 할인 있음 (HTML):        "<s>10,000원</s> 9,000원 (10%)"
//   - 할인 있음 (일반 텍스트): "10,000원 ⇒ 9,000원 (10%)"
func writeFormattedPrice(sb *strings.Builder, price, discountedPrice, discountRate int, supportsHTML bool) {
	// 방어 조건: 할인 표기가 불가능하거나 무의미한 경우 정가만 출력하고 반환합니다.
	if discountedPrice <= 0 || discountedPrice >= price {
		fmt.Fprintf(sb, "%s원", strutil.Comma(price))
		return
	}

	formattedPrice := strutil.Comma(price)
	formattedDiscountedPrice := strutil.Comma(discountedPrice)

	// 할인율이 유효한 경우에만 문자열 생성 (0% 표시 방지)
	var formattedDiscountRate string
	if discountRate > 0 {
		formattedDiscountRate = fmt.Sprintf(" (%d%%)", discountRate)
	}

	if supportsHTML {
		// 예: <s>10,000원</s> 9,000원 (10%)
		fmt.Fprintf(sb, "<s>%s원</s> %s원%s", formattedPrice, formattedDiscountedPrice, formattedDiscountRate)
		return
	}

	// 예: 10,000원 ⇒ 9,000원 (10%)
	fmt.Fprintf(sb, "%s원 ⇒ %s원%s", formattedPrice, formattedDiscountedPrice, formattedDiscountRate)
}

// renderDuplicateRecords 이번 수집 사이클에서 처음으로 감지된 중복 등록 상품 목록을
// 사용자에게 발송할 알림 포맷에 맞게 렌더링합니다.
//
// 매개변수:
//   - newDuplicateRecords: 이번 사이클에 새롭게 중복이 확인된 레코드 목록 (CSV 원시 행 배열)
//     이미 알림이 발송된 상품은 이 함수에 도달하기 전에 제외됩니다.
//   - supportsHTML: 수신 채널(텔레그램 등)의 HTML 태그 지원 여부
//
// 반환값: 중복 상품 목록이 담긴 알림 메시지 문자열 (전달된 목록이 없으면 빈 문자열을 반환합니다)
func renderDuplicateRecords(newDuplicateRecords [][]string, supportsHTML bool) string {
	if len(newDuplicateRecords) == 0 {
		return ""
	}

	var sb strings.Builder

	// 중복 레코드 수 기준으로 버퍼 크기를 사전 예약하여 루프 중 불필요한 메모리 재할당을 방지합니다.
	sb.Grow(len(newDuplicateRecords) * 150)

	for i, record := range newDuplicateRecords {
		// 두 번째 항목부터는 항목 사이에 줄바꿈을 삽입하여 목록을 구분합니다.
		if i > 0 {
			sb.WriteString("\n")
		}

		// CSV 레코드에서 상품 ID와 상품명을 추출합니다.
		productID := strings.TrimSpace(record[columnID])
		productName := strings.TrimSpace(record[columnName])

		// CSV의 상품명 칼럼이 비어있는 경우, 알림 메시지에 공백이 그대로 노출되지 않도록 대체 텍스트를 사용합니다.
		if productName == "" {
			productName = fallbackProductName
		}

		// 글머리 기호(•)와 함께 상품 링크를 렌더링합니다.
		sb.WriteString("      • ")
		sb.WriteString(renderProductLink(productID, productName, supportsHTML))
	}

	return sb.String()
}

// renderUnavailableProducts 이번 수집 사이클에서 새롭게 판매 불가(단종·접근 불가) 상태로 전이된 상품 목록을
// 사용자에게 발송할 알림 포맷에 맞게 렌더링합니다.
//
// 매개변수:
//   - newlyUnavailableProducts: 이번 사이클에 처음으로 판매 불가로 전이된 상품의 ID·Name 슬라이스
//     이전 사이클에서 이미 판매 불가였던 상품은 이 함수에 도달하기 전에 제외됩니다.
//   - supportsHTML: 수신 채널(텔레그램 등)의 HTML 태그 지원 여부
//
// 반환값: 판매 불가 상품 목록이 담긴 알림 메시지 문자열 (전달된 목록이 없으면 빈 문자열을 반환합니다)
func renderUnavailableProducts(newlyUnavailableProducts []struct{ ID, Name string }, supportsHTML bool) string {
	if len(newlyUnavailableProducts) == 0 {
		return ""
	}

	var sb strings.Builder

	// 판매 불가 상품 수 기준으로 버퍼 크기를 사전 예약하여 루프 중 불필요한 메모리 재할당을 방지합니다.
	sb.Grow(len(newlyUnavailableProducts) * 150)

	for i, p := range newlyUnavailableProducts {
		// 두 번째 항목부터는 항목 사이에 줄바꿈을 삽입하여 목록을 구분합니다.
		if i > 0 {
			sb.WriteString("\n")
		}

		// 글머리 기호(•)와 함께 상품 링크를 렌더링합니다.
		sb.WriteString("      • ")
		sb.WriteString(renderProductLink(p.ID, p.Name, supportsHTML))
	}

	return sb.String()
}

// renderProductLink 상품 ID와 상품명을 채널 규격에 맞는 링크 문자열로 렌더링합니다.
//
// 매개변수:
//   - id: 상품 ID
//   - name: 상품명
//   - supportsHTML: 수신 채널(텔레그램 등)의 HTML 태그 지원 여부
//
// 반환값: 채널 규격에 맞게 포맷팅된 상품 링크 문자열
func renderProductLink(id, name string, supportsHTML bool) string {
	if supportsHTML {
		escapedName := template.HTMLEscapeString(name)
		return fmt.Sprintf("<a href=\"%s\"><b>%s</b></a>", productPageURL(id), escapedName)
	}

	return fmt.Sprintf("%s(%s)", name, id)
}

// buildNotificationMessage 각 렌더러가 생성한 개별 메시지들을 조합하여 최종 알림 메시지를 완성합니다.
//
// [동작 흐름]
// 아래 두 단계를 순서대로 처리합니다.
//
//  1. 변경 사항이 있는 경우 (hasChanges=true)
//     가격 변동·중복 등록·판매 불가 메시지 중 내용이 있는 것만 골라 하나의 문자열로 합칩니다.
//     각 섹션에는 사용자가 읽기 쉽도록 헤더 문장이 앞에 붙습니다.
//
//  2. 변경 사항이 없지만 사용자가 직접 실행한 경우 (runBy=TaskRunByUser)
//     변경 없음을 알리는 문장과 함께 현재 감시 중인 전체 상품 현황을 반환합니다.
//     자동 실행(스케줄러)에 의한 경우에는 빈 문자열을 반환하여 불필요한 알림을 억제합니다.
//
// 매개변수:
//   - runBy: 작업 실행 주체 (사용자 직접 실행 vs 스케줄러 자동 실행)
//   - currentSnapshot: 이번 수집 사이클의 최신 상태 스냅샷 (상품 목록 현황 보고에 사용)
//   - productDiffsMessage: 가격 변동 상품 렌더링 결과 (없으면 빈 문자열)
//   - newDuplicateRecordsMessage: 중복 등록 상품 렌더링 결과 (없으면 빈 문자열)
//   - newlyUnavailableProductsMessage: 판매 불가 전이 상품 렌더링 결과 (없으면 빈 문자열)
//   - supportsHTML: 수신 채널(텔레그램 등)의 HTML 태그 지원 여부
//
// 반환값: 발송할 최종 알림 메시지 문자열. 알림을 보낼 필요가 없으면 빈 문자열을 반환합니다.
func buildNotificationMessage(runBy contract.TaskRunBy, currentSnapshot *watchProductPriceSnapshot, productDiffsMessage, newDuplicateRecordsMessage, newlyUnavailableProductsMessage string, supportsHTML bool) string {
	// 세 개의 개별 메시지(가격 변동·중복 등록·판매 불가) 중 하나라도 내용이 있으면 이벤트 알림 메시지를 조립합니다.
	// 모두 빈 문자열이면 아래 사용자 직접 실행 여부 확인으로 넘어갑니다.
	hasAnyMessage := strutil.AnyContent(productDiffsMessage, newDuplicateRecordsMessage, newlyUnavailableProductsMessage)
	if hasAnyMessage {
		var sb strings.Builder

		// 루프 중 불필요한 메모리 재할당을 줄이기 위해 버퍼 용량을 미리 할당합니다.
		// 세 메시지의 길이 합산에 헤더 문장 오버헤드 100바이트를 더해 예상 크기를 계산합니다.
		sb.Grow(len(productDiffsMessage) + len(newDuplicateRecordsMessage) + len(newlyUnavailableProductsMessage) + 100)

		// 유효한 메시지만 선택적으로 조합합니다. 빈 문자열인 섹션은 자연스럽게 생략됩니다.
		if len(productDiffsMessage) > 0 {
			sb.WriteString("상품 정보가 변경되었습니다.\n\n")
			sb.WriteString(productDiffsMessage)
			sb.WriteString("\n\n")
		}
		if len(newDuplicateRecordsMessage) > 0 {
			sb.WriteString("중복으로 등록된 상품 목록:\n\n")
			sb.WriteString(newDuplicateRecordsMessage)
			sb.WriteString("\n\n")
		}
		if len(newlyUnavailableProductsMessage) > 0 {
			sb.WriteString("알 수 없는 상품 목록:\n\n")
			sb.WriteString(newlyUnavailableProductsMessage)
			sb.WriteString("\n\n")
		}

		return sb.String()
	}

	// 변경 사항이 없더라도, 사용자가 직접 실행(TaskRunByUser)한 경우에는 침묵하지 않고 현재 상태를 보고합니다.
	// 사용자가 직접 실행한 경우에는 "변경 없음 + 현재 상품 현황"을 응답하여, 시스템이 정상 동작 중임을 사용자가 확인할 수 있도록 합니다.
	// 반면 스케줄러 자동 실행 중에는 변경이 없을 때 빈 문자열을 반환하여 불필요한 알림 발송을 억제합니다.
	if runBy == contract.TaskRunByUser {
		if len(currentSnapshot.Products) == 0 {
			return "등록된 상품 정보가 존재하지 않습니다."
		}

		var sb strings.Builder

		// 불필요한 메모리 재할당을 줄이기 위해 버퍼 용량을 미리 할당합니다.
		// 상품당 약 400바이트, 헤더 문장 오버헤드 100바이트를 더해 예상 크기를 계산합니다.
		sb.Grow(len(currentSnapshot.Products)*400 + 100)

		sb.WriteString("변경된 상품 정보가 없습니다.\n\n")
		sb.WriteString("현재 등록된 상품 정보는 아래와 같습니다:\n\n")

		for i, p := range currentSnapshot.Products {
			if i > 0 {
				sb.WriteString("\n\n")
			}

			sb.WriteString(renderProduct(p, supportsHTML, ""))
		}

		return sb.String()
	}

	return ""
}
