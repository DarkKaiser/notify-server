package kurly

import (
	"fmt"
	"html/template"
	"strconv"
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
// 반환값: 체널 규격(HTML/텍스트)에 맞게 포맷팅된 상품 알림 라인 문자열
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

// @@@@@ 함수를 분리해야 한다고 함
func buildDuplicateRecordsMessage(duplicateRecords [][]string, duplicateNotifiedIDs []string, supportsHTML bool) (string, []string) {
	if len(duplicateRecords) == 0 {
		return "", nil
	}

	// 빠른 조회를 위해 이미 알림이 나간 ID 목록을 Set으로 변환
	reportedMap := make(map[string]struct{}, len(duplicateNotifiedIDs))
	for _, id := range duplicateNotifiedIDs {
		reportedMap[id] = struct{}{}
	}

	// 현재 회차의 전체 중복 ID 목록 (추후 저장용)
	newReportedIDs := make([]string, 0, len(duplicateRecords))
	// 새로 알림을 내보낼 중복 ID들 (메시지 생성용)
	var newDuplicateRecords [][]string

	for _, record := range duplicateRecords {
		productID := strings.TrimSpace(record[columnID])
		newReportedIDs = append(newReportedIDs, productID)

		if _, alreadyReported := reportedMap[productID]; !alreadyReported {
			newDuplicateRecords = append(newDuplicateRecords, record)
		}
	}

	if len(newDuplicateRecords) == 0 {
		return "", newReportedIDs
	}

	var sb strings.Builder

	// 예상되는 문자열 크기만큼 미리 할당하여 메모리 복사 비용 방지 (라인당 약 150바이트 예상)
	sb.Grow(len(newDuplicateRecords) * 150)

	for i, record := range newDuplicateRecords {
		if i > 0 {
			sb.WriteString("\n")
		}

		productID := strings.TrimSpace(record[columnID])
		productName := strings.TrimSpace(record[columnName])

		// 상품명이 비어있는 경우 대체 텍스트 제공
		if productName == "" {
			productName = fallbackProductName
		}

		sb.WriteString("      • ")
		sb.WriteString(renderProductLink(productID, productName, supportsHTML))
	}

	return sb.String(), newReportedIDs
}

// @@@@@
func buildUnavailableProductsMessage(products []*product, prevProductsMap map[int]*product, records [][]string, supportsHTML bool) string {
	if len(products) == 0 {
		return ""
	}

	// CSV 레코드를 Map으로 인덱싱하여 검색 속도 향상
	recordMap := make(map[string]string, len(records))
	for _, record := range records {
		if len(record) > int(columnName) {
			id := strings.TrimSpace(record[columnID])
			name := strings.TrimSpace(record[columnName])
			recordMap[id] = name
		}
	}

	var sb strings.Builder

	// 예상되는 문자열 크기만큼 미리 할당하여 메모리 복사 비용 방지 (라인당 약 150바이트 예상)
	sb.Grow(len(products) * 150)

	for _, p := range products {
		if !p.IsUnavailable {
			continue
		}

		// 무한 스팸 방지: 기존에도 판매 중지(Unavailable) 상태였던 상품이면 스킵 (Status transition 판별)
		if prevProductsMap != nil {
			if prevProduct, exists := prevProductsMap[p.ID]; exists && prevProduct.IsUnavailable {
				continue
			}
		}

		productID := strconv.Itoa(p.ID)
		productName, found := recordMap[productID]
		if !found {
			// 감시 대상 상품(레코드) 목록에 없는 상품은 보고 대상에서 제외합니다
			continue
		}

		// 상품명이 비어있는 경우 대체 텍스트 제공
		if productName == "" {
			productName = fallbackProductName
		}

		if sb.Len() > 0 {
			sb.WriteString("\n")
		}

		sb.WriteString("      • ")
		sb.WriteString(renderProductLink(productID, productName, supportsHTML))
	}

	return sb.String()
}

// @@@@@
func buildNotificationMessage(runBy contract.TaskRunBy, currentSnapshot *watchProductPriceSnapshot, productsDiffMessage, duplicateRecordsMessage, unavailableProductsMessage string, supportsHTML bool) string {
	// [메시지 조합 여부 판단 (Change Detection)]
	// 개별 메시지들(가격 변동, 중복, 식별 불가) 중에서 유효한 내용이 단 하나라도 존재하는지 검사합니다.
	// 이는 알림의 성격을 단순 '현황 보고'에서 유의미한 '이벤트 알림'으로 전환하는 기준이 됩니다.
	hasChanges := strutil.AnyContent(productsDiffMessage, duplicateRecordsMessage, unavailableProductsMessage)
	if hasChanges {
		var sb strings.Builder

		// 예상되는 최소 용량을 미리 할당하여 메모리 재할당 비용 최적화
		expectedSize := len(productsDiffMessage) + len(duplicateRecordsMessage) + len(unavailableProductsMessage) + 100
		sb.Grow(expectedSize)

		if len(productsDiffMessage) > 0 {
			sb.WriteString("상품 정보가 변경되었습니다.\n\n")
			sb.WriteString(productsDiffMessage)
			sb.WriteString("\n\n")
		}
		if len(duplicateRecordsMessage) > 0 {
			sb.WriteString("중복으로 등록된 상품 목록:\n\n")
			sb.WriteString(duplicateRecordsMessage)
			sb.WriteString("\n\n")
		}
		if len(unavailableProductsMessage) > 0 {
			sb.WriteString("알 수 없는 상품 목록:\n\n")
			sb.WriteString(unavailableProductsMessage)
			sb.WriteString("\n\n")
		}

		return sb.String()
	}

	// 변경 사항이 없더라도, 사용자가 명시적 의도로 작업(RunByUser)을 실행한 경우에는 침묵하지 않고 현재 상태를 보고합니다.
	// 이는 시스템이 정상 동작 중임을 사용자에게 확신시켜 주기 위한 중요한 UX 장치입니다.
	if runBy == contract.TaskRunByUser {
		if len(currentSnapshot.Products) == 0 {
			return "등록된 상품 정보가 존재하지 않습니다."
		}

		var sb strings.Builder

		// 예상되는 최소 용량을 미리 할당하여 메모리 재할당 비용 최적화
		sb.Grow(len(currentSnapshot.Products)*400 + 100)

		sb.WriteString("변경된 상품 정보가 없습니다.\n\n")
		sb.WriteString("현재 등록된 상품 정보는 아래와 같습니다:\n\n")

		lineSpacing := "\n\n"
		for i, actualityProduct := range currentSnapshot.Products {
			if i > 0 {
				sb.WriteString(lineSpacing)
			}
			sb.WriteString(renderProduct(actualityProduct, supportsHTML, ""))
		}

		return sb.String()
	}

	return ""
}

// @@@@@
func renderProductLink(productID, productName string, supportsHTML bool) string {
	if supportsHTML {
		escapedName := template.HTMLEscapeString(productName)
		return fmt.Sprintf("<a href=\"%s\"><b>%s</b></a>", buildProductPageURL(productID), escapedName)
	}
	return fmt.Sprintf("%s(%s)", productName, productID)
}
