package kurly

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

const (
	// allocSizePerProductDiff 단일 상품의 변경 내역(Diff)을 문자열로 렌더링할 때 필요한 예상 메모리 크기(Byte)입니다.
	allocSizePerProductDiff = 300
)

const (
	// fallbackProductName CSV 데이터에서 상품명이 없거나 공백일 경우 사용자에게 표시할 대체 텍스트입니다.
	fallbackProductName = "알 수 없는 상품"
)

// renderProductDiffs 감지된 상품 변동 내역(Diffs)을 최종 사용자가 읽기 편한 알림 메시지로 변환합니다.
func renderProductDiffs(diffs []productDiff, supportsHTML bool) string {
	if len(diffs) == 0 {
		return ""
	}

	var sb strings.Builder

	// 변경된 상품 내역을 렌더링하기 위해 필요한 메모리 크기를 사전에 예측하여 할당합니다.
	// 예측 공식: (변동 사항 수) × (항목당 예상 크기)
	sb.Grow(len(diffs) * allocSizePerProductDiff)

	lineSpacing := "\n\n"
	for i, diff := range diffs {
		if i > 0 {
			sb.WriteString(lineSpacing)
		}

		switch diff.Type {
		case eventNewProduct:
			sb.WriteString(diff.Product.Render(supportsHTML, mark.New))
		case eventRestocked:
			sb.WriteString(diff.Product.Render(supportsHTML, mark.New))
		case eventLowestPriceRenewed:
			sb.WriteString(diff.Product.RenderDiff(supportsHTML, mark.BestPrice, diff.Prev))
		case eventPriceChanged:
			sb.WriteString(diff.Product.RenderDiff(supportsHTML, mark.Modified, diff.Prev))
		}
	}

	return sb.String()
}

// buildDuplicateRecordsMessage 감시 대상 파일(CSV)에 중복 기입된 상품(레코드) 목록을 사용자 알림용 메시지로 포맷팅합니다.
//
// [설명]
// 사용자가 실수로 동일한 상품을 여러 번 입력한 경우, 이를 파싱 단계에서 별도의 목록으로 분리합니다.
// 이 함수는 중복 기입된 상품(레코드) 목록을 순회하며, 알림 메시지 하단에 경고성 정보로 표시할 문자열을 생성합니다.
//
// [매개변수]
//   - duplicateRecords: CSV 파일에서 읽어온 중복 기입된 상품(레코드) 목록입니다.
//   - supportsHTML: HTML 형식의 메시지 지원 여부입니다.
//
// [반환값]
//   - string: 포맷팅된 중복 상품 메시지입니다.
func buildDuplicateRecordsMessage(duplicateRecords [][]string, supportsHTML bool) string {
	if len(duplicateRecords) == 0 {
		return ""
	}

	var sb strings.Builder

	// 예상되는 문자열 크기만큼 미리 할당하여 메모리 복사 비용 방지 (라인당 약 150바이트 예상)
	sb.Grow(len(duplicateRecords) * 150)

	for i, record := range duplicateRecords {
		if i > 0 {
			sb.WriteString("\n")
		}

		productID := strings.TrimSpace(record[csvColumnID])
		productName := strings.TrimSpace(record[csvColumnName])

		// 상품명이 비어있는 경우 대체 텍스트 제공
		if productName == "" {
			productName = fallbackProductName
		}

		sb.WriteString("      • ")
		sb.WriteString(renderProductLink(productID, productName, supportsHTML))
	}

	return sb.String()
}

// buildUnavailableProductsMessage 판매 중지 또는 상품 정보 삭제 등으로 인해 정보를 수집할 수 없는 상품 목록을 포맷팅합니다.
//
// [설명]
// 크롤링 과정에서 `IsUnavailable` 상태로 플래그가 설정된 상품들을 필터링하여 사용자에게 보고합니다.
// 이는 품절이나 상품 정보 삭제와 같은 비즈니스적으로 중요한 상태 변화를 사용자가 즉시 인지할 수 있도록 돕습니다.
//
// [매개변수]
//   - products: 크롤링 과정에서 수집된 상품 목록입니다.
//   - records: CSV 파일에서 읽어온 감시 대상 상품(레코드) 목록입니다.
//   - supportsHTML: HTML 형식의 메시지 지원 여부입니다.
//
// [반환값]
//   - string: 포맷팅된 정보를 수집할 수 없는 상품 메시지입니다.
func buildUnavailableProductsMessage(products []*product, records [][]string, supportsHTML bool) string {
	if len(products) == 0 {
		return ""
	}

	// CSV 레코드를 Map으로 인덱싱하여 검색 속도 향상
	recordMap := make(map[string]string, len(records))
	for _, record := range records {
		if len(record) > int(csvColumnName) {
			id := strings.TrimSpace(record[csvColumnID])
			name := strings.TrimSpace(record[csvColumnName])
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

// buildNotificationMessage 수집된 변경 내역과 부가 정보를 조합하여 최종 사용자 알림 메시지를 생성합니다.
//
// [설계 의도]
// 변경 사항이 존재할 경우, 해당 내역을 상세히 브리핑하는 메시지를 우선하여 생성합니다.
// 만약 변경 사항이 없더라도 사용자가 명시적 의도로 작업을(RunByUser) 실행한 경우에는, 시스템이 정상 동작 중임을
// 안심시키기 위해 현재 스냅샷을 기반으로 한 요약 리포트(Fallback Mode)를 제공합니다.
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
			sb.WriteString(actualityProduct.Render(supportsHTML, ""))
		}

		return sb.String()
	}

	return ""
}

// renderProductLink 상품 ID와 이름을 조합하여 알림 메시지에 사용할 포맷팅된 링크 문자열을 생성합니다.
//
// [매개변수]
//   - productID: 상품 고유 식별자 (URL 생성에 사용)
//   - productName: 화면에 표시될 상품 이름
//   - supportsHTML: HTML 태그 포함 여부
//
// [반환값]
//   - string: 포맷팅된 링크 문자열
func renderProductLink(productID, productName string, supportsHTML bool) string {
	if supportsHTML {
		escapedName := template.HTMLEscapeString(productName)
		return fmt.Sprintf("<a href=\"%s\"><b>%s</b></a>", formatProductPageURL(productID), escapedName)
	}
	return fmt.Sprintf("%s(%s)", productName, productID)
}
