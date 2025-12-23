package kurly

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/mark"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
)

var (
	// reExtractNextData 마켓컬리 상품 페이지의 핵심 데이터가 담긴 <script> 태그 내용을 추출합니다.
	// (페이지 소스에 포함된 초기 데이터를 직접 긁어와서, 별도의 API 호출 없이도 상품 정보를 얻을 수 있게 해줍니다)
	reExtractNextData = regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*>([\s\S]*?)</script>`)

	// reDetectUnavailable 추출한 데이터에서 "상품 정보 없음(null)" 패턴이 있는지 검사합니다.
	// 이 패턴이 발견되면 '판매 중지'되거나 '삭제된 상품'으로 판단하여 불필요한 알림을 보내지 않도록 합니다.
	reDetectUnavailable = regexp.MustCompile(`"product":\s*null`)
)

// csvColumnIndex CSV 파일에서 상품 정보를 파싱할 때 사용되는 컬럼 인덱스를 정의하는 타입입니다.
type csvColumnIndex int

const (
	// CSV 파일의 헤더 순서에 따른 컬럼 인덱스 상수입니다.
	//
	// [주의]
	// 이 상수의 순서는 실제 CSV 파일의 헤더 순서와 **엄격하게 일치**해야 합니다.
	// 파일 포맷이 변경될 경우, 이 상수의 정의도 반드시 함께 수정되어야 합니다.
	csvColumnNo     csvColumnIndex = iota // [0] 상품 코드
	csvColumnName                         // [1] 상품 이름
	csvColumnStatus                       // [2] 감시 활성화 여부

	// CSV 파일의 '감시 활성화 여부' 컬럼에 사용되는 상태값 상수입니다.
	//
	// [설명]
	// CSV 파일에서 읽어온 데이터는 문자열(string) 타입이므로, 비교의 정확성을 위해
	// 정수형(1) 대신 문자열 상수("1")를 정의하여 사용합니다. ('1'이 아닌 모든 값은 비활성 상태로 간주합니다)
	csvStatusEnabled = "1" // 감시 활성화
)

type watchProductPriceSettings struct {
	WatchProductsFile string `json:"watch_products_file"`
}

func (s *watchProductPriceSettings) validate() error {
	s.WatchProductsFile = strings.TrimSpace(s.WatchProductsFile)
	if s.WatchProductsFile == "" {
		return apperrors.New(apperrors.InvalidInput, "watch_products_file이 입력되지 않았거나 공백입니다")
	}
	if !strings.HasSuffix(strings.ToLower(s.WatchProductsFile), ".csv") {
		return apperrors.New(apperrors.InvalidInput, "watch_products_file 설정에는 .csv 확장자를 가진 파일 경로만 지정할 수 있습니다")
	}
	return nil
}

// watchProductPriceSnapshot 가격 변동을 감지하기 위한 상품 데이터의 스냅샷입니다.
type watchProductPriceSnapshot struct {
	Products []*product `json:"products"`
}

// @@@@@
func (t *task) executeWatchProductPrice(commandSettings *watchProductPriceSettings, prevSnapshot *watchProductPriceSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {
	//
	// 감시할 상품 목록을 읽어들인다.
	//
	f, err := os.Open(commandSettings.WatchProductsFile)
	if err != nil {
		return "", nil, apperrors.Wrap(err, apperrors.InvalidInput, "상품 목록이 저장된 파일을 불러올 수 없습니다. 파일이 존재하는지와 경로가 올바른지 확인해 주세요")
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return "", nil, apperrors.Wrap(err, apperrors.InvalidInput, "상품 목록을 불러올 수 없습니다")
	}

	// 감시할 상품 목록의 헤더를 제거한다.
	records = records[1:]

	// 감시할 상품 목록에서 중복된 상품을 정규화한다.
	records, duplicateRecords := t.normalizeDuplicateProducts(records)

	//
	// 읽어들인 상품들의 가격 및 상태를 확인한다.
	//
	currentSnapshot := &watchProductPriceSnapshot{
		Products: make([]*product, 0, len(records)),
	}

	for _, record := range records {
		if record[csvColumnStatus] != csvStatusEnabled {
			continue
		}

		// 상품 코드를 숫자로 변환한다.
		id, err := strconv.Atoi(record[csvColumnNo])
		if err != nil {
			return "", nil, apperrors.Wrap(err, apperrors.InvalidInput, "상품 코드의 숫자 변환이 실패하였습니다")
		}

		// 상품 페이지를 읽어들인다.
		productDetailPageURL := fmt.Sprintf(productPageURLFormat, id)
		doc, err := tasksvc.FetchHTMLDocument(t.GetFetcher(), productDetailPageURL)
		if err != nil {
			return "", nil, err
		}

		// 읽어들인 페이지에서 상품 데이터가 JSON 포맷으로 저장된 자바스크립트 구문을 추출한다.
		html, err := doc.Html()
		if err != nil {
			return "", nil, apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("불러온 페이지(%s)에서 HTML 추출이 실패하였습니다", productDetailPageURL))
		}
		match := reExtractNextData.FindStringSubmatch(html)
		if len(match) < 2 {
			return "", nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("불러온 페이지(%s)에서 상품에 대한 JSON 데이터 추출이 실패하였습니다.(error:%s)", productDetailPageURL, err))
		}
		jsonProductData := match[1]

		var product = &product{
			ID:                 id,
			Name:               "",
			Price:              0,
			DiscountedPrice:    0,
			DiscountRate:       0,
			LowestPrice:        0,
			LowestPriceTimeUTC: time.Time{},
			IsUnavailable:      false,
		}

		// 알 수 없는 상품(현재 판매중이지 않은 상품)인지 확인한다.
		if reDetectUnavailable.MatchString(jsonProductData) {
			product.IsUnavailable = true
		}

		if !product.IsUnavailable {
			sel := doc.Find("#product-atf > section.css-1ua1wyk")
			if sel.Length() != 1 {
				return "", nil, tasksvc.NewErrHTMLStructureChanged(productDetailPageURL, "상품정보 섹션 추출 실패")
			}

			// 상품 이름을 확인한다.
			ps := sel.Find("div.css-84rb3h > div.css-6zfm8o > div.css-o3fjh7 > h1")
			if ps.Length() != 1 {
				return "", nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 이름 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productDetailPageURL))
			}
			product.Name = strutil.NormalizeSpaces(ps.Text())

			// 상품 가격을 추출한다.
			ps = sel.Find("h2.css-xrp7wx > span.css-8h3us8")
			if ps.Length() == 0 /* 가격, 단위(원) */ {
				ps = sel.Find("h2.css-xrp7wx > div.css-o2nlqt > span")
				if ps.Length() != 2 /* 가격 + 단위(원) */ {
					return "", nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productDetailPageURL))
				}

				// 가격
				product.Price, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), ",", ""))
				if err != nil {
					return "", nil, apperrors.Wrap(err, apperrors.ExecutionFailed, "상품 가격의 숫자 변환이 실패하였습니다")
				}
			} else if ps.Length() == 1 /* 할인율, 할인 가격, 단위(원) */ {
				// 할인율
				product.DiscountRate, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), "%", ""))
				if err != nil {
					return "", nil, apperrors.Wrap(err, apperrors.ExecutionFailed, "상품 할인율의 숫자 변환이 실패하였습니다")
				}

				// 할인 가격
				ps = sel.Find("h2.css-xrp7wx > div.css-o2nlqt > span")
				if ps.Length() != 2 /* 가격 + 단위(원) */ {
					return "", nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productDetailPageURL))
				}

				product.DiscountedPrice, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), ",", ""))
				if err != nil {
					return "", nil, apperrors.Wrap(err, apperrors.ExecutionFailed, "상품 할인 가격의 숫자 변환이 실패하였습니다")
				}

				// 가격
				ps = sel.Find("span.css-1s96j0s > span")
				if ps.Length() != 1 /* 가격 + 단위(원) */ {
					return "", nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productDetailPageURL))
				}
				product.Price, _ = strconv.Atoi(strings.ReplaceAll(strings.ReplaceAll(ps.Text(), ",", ""), "원", ""))
			} else {
				return "", nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(1) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productDetailPageURL))
			}
		}

		currentSnapshot.Products = append(currentSnapshot.Products, product)
	}

	return t.diffAndNotify(records, duplicateRecords, currentSnapshot, prevSnapshot, supportsHTML)
}

// @@@@@
// diffAndNotify는 현재 수집된 상품 정보와 이전 스냅샷을 비교하여 변동 사항을 분석합니다.
// 가격 변동, 품절 상태 변경, 신규 상품 등록 등의 이벤트를 감지하고,
// 사용자에게 발송할 포맷팅된 알림 메시지와 갱신된 작업 결과 데이터를 생성합니다.
func (t *task) diffAndNotify(records, duplicateRecords [][]string, currentSnapshot, prevSnapshot *watchProductPriceSnapshot, supportsHTML bool) (string, interface{}, error) {
	//
	// 상품들의 변경된 가격 및 상태를 확인한다.
	//
	var sb strings.Builder
	sb.Grow(1024)

	lineSpacing := "\n\n"
	if supportsHTML {
		lineSpacing = "\n"
	}
	// 비교를 위해 이전 스냅샷의 상품들을 Map으로 변환합니다 (ID -> Product).
	// O(N) 조회를 통해 성능을 최적화합니다.
	prevProductMap := make(map[int]*product, len(prevSnapshot.Products))
	for _, p := range prevSnapshot.Products {
		prevProductMap[p.ID] = p
	}

	for _, actualityProduct := range currentSnapshot.Products {
		originProduct, exists := prevProductMap[actualityProduct.ID]

		// 1. 이전 정보가 없거나(신규), 이전에는 알 수 없는 상품이었던 경우
		if !exists || (originProduct.IsUnavailable && !actualityProduct.IsUnavailable) {
			// 알 수 없는 상품인 경우 상품에 대한 정보를 사용자에게 알리지 않는다.
			if actualityProduct.IsUnavailable {
				continue
			}

			// 최저 가격 갱신 (신규 상품 취급)
			actualityProduct.updateLowestPrice()

			if sb.Len() > 0 {
				sb.WriteString(lineSpacing)
			}
			sb.WriteString(actualityProduct.Render(supportsHTML, mark.New, nil))
			continue
		}

		// 2. 상품이 판매 중이었다가 알 수 없는 상품(판매 중지)으로 변경된 경우
		if !originProduct.IsUnavailable && actualityProduct.IsUnavailable {
			continue // 변경 내역 렌더링 생략
		}

		// 3. 기존 상품 가격/정보 변경 비교
		// 이전 최저가 정보를 승계
		actualityProduct.LowestPrice = originProduct.LowestPrice
		actualityProduct.LowestPriceTimeUTC = originProduct.LowestPriceTimeUTC

		// 현재 가격 기준으로 최저가 갱신 시도
		actualityProduct.updateLowestPrice()

		// 가격이나 할인율 등이 변경되었는지 확인
		if actualityProduct.Price != originProduct.Price ||
			actualityProduct.DiscountedPrice != originProduct.DiscountedPrice ||
			actualityProduct.DiscountRate != originProduct.DiscountRate {

			if sb.Len() > 0 {
				sb.WriteString(lineSpacing)
			}
			sb.WriteString(actualityProduct.Render(supportsHTML, mark.Change, originProduct))
		}
	}

	//
	// 조건에 따라 상품 정보 변경 사항을 처리하고 메시지를 생성한다.
	//
	duplicateProductsMessage := t.buildDuplicateProductsMessage(duplicateRecords, supportsHTML)
	unknownProductsMessage := t.buildUnknownProductsMessage(currentSnapshot.Products, records, supportsHTML)

	var message string
	var changedTaskResultData interface{}

	if sb.Len() > 0 || len(duplicateProductsMessage) > 0 || len(unknownProductsMessage) > 0 {
		if sb.Len() > 0 {
			message = fmt.Sprintf("상품 정보가 변경되었습니다.\n\n%s\n\n", sb.String())
		} else {
			message = "상품 정보가 변경되었습니다.\n\n"
		}
		if len(duplicateProductsMessage) > 0 {
			message += fmt.Sprintf("중복으로 등록된 상품 목록:\n%s\n\n", duplicateProductsMessage)
		}
		if len(unknownProductsMessage) > 0 {
			message += fmt.Sprintf("알 수 없는 상품 목록:\n%s\n\n", unknownProductsMessage)
		}

		changedTaskResultData = currentSnapshot
	} else {
		if t.GetRunBy() == tasksvc.RunByUser {
			if len(currentSnapshot.Products) == 0 {
				message = "등록된 상품 정보가 존재하지 않습니다."
			} else {
				for _, actualityProduct := range currentSnapshot.Products {
					if sb.Len() > 0 {
						sb.WriteString(lineSpacing)
					}
					sb.WriteString(actualityProduct.Render(supportsHTML, "", nil))
				}

				message = fmt.Sprintf("변경된 상품 정보가 없습니다.\n\n%s현재 등록된 상품 정보는 아래와 같습니다:", sb.String())
			}
		}
	}

	return message, changedTaskResultData, nil
}

// @@@@@
// normalizeDuplicateProducts 함수는 입력된 상품 목록에서 중복된 상품을 제거하고, 중복된 상품을 별도의 목록에 저장한다.
// 반환 값으로는 중복이 제거된 상품 목록과 중복된 상품 목록을 반환한다.
func (t *task) normalizeDuplicateProducts(records [][]string) ([][]string, [][]string) {
	distinctRecords := make([][]string, 0, len(records))
	duplicateRecords := make([][]string, 0, len(records))

	checkedProducts := make(map[string]bool)

	for _, record := range records {
		if len(record) == 0 {
			continue
		}

		productNo := record[csvColumnNo]
		if !checkedProducts[productNo] {
			checkedProducts[productNo] = true
			distinctRecords = append(distinctRecords, record)
		} else {
			duplicateRecords = append(duplicateRecords, record)
		}
	}

	return distinctRecords, duplicateRecords
}

// @@@@@
// buildDuplicateProductsMessage 중복으로 등록된 상품 목록에 대한 알림 메시지를 생성합니다.
func (t *task) buildDuplicateProductsMessage(duplicateRecords [][]string, supportsHTML bool) string {
	if len(duplicateRecords) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, record := range duplicateRecords {
		if i > 0 {
			sb.WriteString("\n")
		}

		productNo := strings.TrimSpace(record[csvColumnNo])
		productName := template.HTMLEscapeString(strings.TrimSpace(record[csvColumnName]))

		if supportsHTML {
			sb.WriteString(fmt.Sprintf("      • <a href=\"%s\"><b>%s</b></a>", fmt.Sprintf(productPageURLFormat, productNo), productName))
		} else {
			sb.WriteString(fmt.Sprintf("      • %s(%s)", productName, productNo))
		}
	}
	return sb.String()
}

// @@@@@
// buildUnknownProductsMessage 알 수 없는 상품(판매 중지 등) 목록에 대한 알림 메시지를 생성합니다.
func (t *task) buildUnknownProductsMessage(products []*product, records [][]string, supportsHTML bool) string {
	var sb strings.Builder

	for _, product := range products {
		if !product.IsUnavailable {
			continue
		}

		// CSV 레코드에서 해당 상품의 정보를 찾습니다.
		for _, record := range records {
			if record[csvColumnNo] == strconv.Itoa(product.ID) {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}

				productNo := strings.TrimSpace(record[csvColumnNo])
				productName := template.HTMLEscapeString(strings.TrimSpace(record[csvColumnName]))

				if supportsHTML {
					sb.WriteString(fmt.Sprintf("      • <a href=\"%s\"><b>%s</b></a>", fmt.Sprintf(productPageURLFormat, productNo), productName))
				} else {
					sb.WriteString(fmt.Sprintf("      • %s(%s)", productName, productNo))
				}
				break
			}
		}
	}
	return sb.String()
}
