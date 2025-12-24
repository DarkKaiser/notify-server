package kurly

import (
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/mark"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
)

const (
	// fallbackProductName CSV 데이터에서 상품명이 없거나 공백일 경우 사용자에게 표시할 대체 텍스트입니다.
	fallbackProductName = "알 수 없는 상품"
)

var (
	// reExtractNextData 마켓컬리 상품 페이지의 핵심 데이터가 담긴 <script> 태그 내용을 추출합니다.
	// (페이지 소스에 포함된 초기 데이터를 직접 긁어와서, 별도의 API 호출 없이도 상품 정보를 얻을 수 있게 해줍니다)
	reExtractNextData = regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*>([\s\S]*?)</script>`)

	// reDetectUnavailable 추출한 데이터에서 "상품 정보 없음(null)" 패턴이 있는지 검사합니다.
	// 이 패턴이 발견되면 '판매 중지'되거나 '삭제된 상품'으로 판단하여 불필요한 알림을 보내지 않도록 합니다.
	reDetectUnavailable = regexp.MustCompile(`"product":\s*null`)
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

// executeWatchProductPrice 감시 대상 상품들의 최신 가격을 조회하고, 이전 상태와 비교하여 변동이 있으면 알림을 생성합니다.
func (t *task) executeWatchProductPrice(loader WatchListLoader, prevSnapshot *watchProductPriceSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {
	// @@@@@
	//
	// 감시할 상품 목록을 읽어들인다. (추상화된 Loader 사용)
	//
	records, err := loader.Load()
	if err != nil {
		return "", nil, err
	}

	// 감시할 상품 목록에서 중복된 상품을 정규화한다.
	records, duplicateRecords := t.extractDuplicateRecords(records)

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
		id, err := strconv.Atoi(record[csvColumnID])
		if err != nil {
			return "", nil, apperrors.Wrap(err, apperrors.InvalidInput, "상품 코드의 숫자 변환이 실패하였습니다")
		}

		// 상품 페이지를 읽어들이고 파싱하여 정보를 추출한다.
		// 상세 페이지에서 상품 정보를 조회 (Fetch + Parse)
		product, err := t.fetchProductInfo(id)
		if err != nil {
			return "", nil, err
		}

		currentSnapshot.Products = append(currentSnapshot.Products, product)
	}

	return t.diffAndNotify(currentSnapshot, prevSnapshot, records, duplicateRecords, supportsHTML)
}

// extractDuplicateRecords 입력된 감시 대상 상품(레코드) 목록에서 중복 기입된 항목을 추출하여 분리합니다.
//
// [설명]
// 감시 대상 상품(레코드) 목록을 순회하며 상품 ID를 기준으로 중복 여부를 검사합니다.
// 처음 등장하는 상품은 `distinctRecords`에 담고, 이미 등장한 상품은 `duplicateRecords`로 추출합니다.
// 이를 통해 핵심 로직에서는 중복 없는 깨끗한 데이터만 처리할 수 있게 됩니다.
//
// [매개변수]
//   - records: CSV 파일에서 읽어온 원본 감시 대상 상품(레코드) 목록입니다.
//
// [반환값]
//   - distinctRecords: 중복이 제거된 유일한 상품(레코드) 목록입니다.
//   - duplicateRecords: 중복으로 판명되어 추출된 상품(레코드) 목록입니다.
func (t *task) extractDuplicateRecords(records [][]string) ([][]string, [][]string) {
	distinctRecords := make([][]string, 0, len(records))
	duplicateRecords := make([][]string, 0, len(records)/2) // 중복 빈도를 고려하여 초기 용량 절반 할당

	// 메모리 효율성을 위해 빈 구조체 사용
	seenProductIDs := make(map[string]struct{}, len(records))

	for _, record := range records {
		// 필수 컬럼(상품 번호) 존재 여부 확인
		if len(record) <= int(csvColumnID) {
			continue
		}

		productID := record[csvColumnID]
		if _, exists := seenProductIDs[productID]; !exists {
			seenProductIDs[productID] = struct{}{}
			distinctRecords = append(distinctRecords, record)
		} else {
			duplicateRecords = append(duplicateRecords, record)
		}
	}

	return distinctRecords, duplicateRecords
}

// fetchProductInfo 상품 상세 페이지(HTML)를 Fetch하여 상품의 최신 상태 및 가격 정보를 조회합니다.
//
// [구현 상세]
// 본 함수는 데이터의 정확성을 위해 '이중 추출(Dual Extraction)' 기법을 사용합니다.
//
//  1. Metadata Parsing (JSON):
//     Next.js가 주입한 `<script id="__NEXT_DATA__">`에서 상품의 판매 상태(Unavailable 여부)를 1차적으로 검증합니다.
//     이는 DOM 렌더링 이전에 원본 데이터의 상태를 확인하여 불필요한 파싱을 방지합니다.
//
//  2. Price Parsing (DOM):
//     HTML DOM 구조를 직접 탐색하여 실제 사용자에게 노출되는 '최종 가격(Pricing)'을 추출합니다.
//     할인 정책(Rate, Discounted)에 따른 동적 구조 변화를 처리합니다.
func (t *task) fetchProductInfo(id int) (*product, error) {
	// @@@@@
	// 상품 페이지를 읽어들인다.
	productPageURL := formatProductPageURL(id)
	doc, err := tasksvc.FetchHTMLDocument(t.GetFetcher(), productPageURL)
	if err != nil {
		return nil, err
	}

	// 읽어들인 페이지에서 상품 데이터가 JSON 포맷으로 저장된 자바스크립트 구문을 추출한다.
	html, err := doc.Html()
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("불러온 페이지(%s)에서 HTML 추출이 실패하였습니다", productPageURL))
	}
	match := reExtractNextData.FindStringSubmatch(html)
	if len(match) < 2 {
		return nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("불러온 페이지(%s)에서 상품에 대한 JSON 데이터 추출이 실패하였습니다.(error:%s)", productPageURL, err))
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
			return nil, tasksvc.NewErrHTMLStructureChanged(productPageURL, "상품정보 섹션 추출 실패")
		}

		// 상품 이름을 확인한다.
		ps := sel.Find("div.css-84rb3h > div.css-6zfm8o > div.css-o3fjh7 > h1")
		if ps.Length() != 1 {
			return nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 이름 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productPageURL))
		}
		product.Name = strutil.NormalizeSpaces(ps.Text())

		// 상품 가격 정보를 추출한다.
		if err := t.extractPriceDetails(sel, product, productPageURL); err != nil {
			return nil, err
		}
	}

	return product, nil
}

// extractPriceDetails HTML DOM에서 가격 상세 정보(정상가, 할인가, 할인율)를 추출하여 Product 구조체에 매핑합니다.
//
// [동작 방식]
// 마켓컬리 상세 페이지의 가격 표시 DOM 구조는 할인 적용 여부에 따라 상이합니다.
// 본 함수는 이 구조적 차이를 식별하여 적절한 필드에 값을 바인딩합니다.
//
//  1. 할인 미적용: 단일 가격 요소(Price)만 존재
//  2. 할인 적용중: 할인율(Rate) + 할인가(Discounted) + 정상가(Price, 취소선) 모두 존재
//
// [매개변수]
//   - sel: 가격 정보가 포함된 DOM Selection
//   - product: 추출된 데이터를 바인딩할 대상 구조체
//   - productPageURL: 에러 발생 시 디버깅을 돕기 위해 로그에 포함할 상품 페이지 URL
func (t *task) extractPriceDetails(sel *goquery.Selection, product *product, productPageURL string) error {
	// @@@@@
	var err error
	ps := sel.Find("h2.css-xrp7wx > span.css-8h3us8")
	if ps.Length() == 0 /* 가격, 단위(원) */ {
		ps = sel.Find("h2.css-xrp7wx > div.css-o2nlqt > span")
		if ps.Length() != 2 /* 가격 + 단위(원) */ {
			return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productPageURL))
		}

		// 가격
		product.Price, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), ",", ""))
		if err != nil {
			return apperrors.Wrap(err, apperrors.ExecutionFailed, "상품 가격의 숫자 변환이 실패하였습니다")
		}
	} else if ps.Length() == 1 /* 할인율, 할인 가격, 단위(원) */ {
		// 할인율
		product.DiscountRate, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), "%", ""))
		if err != nil {
			return apperrors.Wrap(err, apperrors.ExecutionFailed, "상품 할인율의 숫자 변환이 실패하였습니다")
		}

		// 할인 가격
		ps = sel.Find("h2.css-xrp7wx > div.css-o2nlqt > span")
		if ps.Length() != 2 /* 가격 + 단위(원) */ {
			return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productPageURL))
		}

		product.DiscountedPrice, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), ",", ""))
		if err != nil {
			return apperrors.Wrap(err, apperrors.ExecutionFailed, "상품 할인 가격의 숫자 변환이 실패하였습니다")
		}

		// 가격
		ps = sel.Find("span.css-1s96j0s > span")
		if ps.Length() != 1 /* 가격 + 단위(원) */ {
			return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productPageURL))
		}
		product.Price, _ = strconv.Atoi(strings.ReplaceAll(strings.ReplaceAll(ps.Text(), ",", ""), "원", ""))
	} else {
		return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("상품 가격(1) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productPageURL))
	}
	return nil
}

// diffAndNotify 수집된 최신 상품 정보와 이전 상태(Snapshot)를 비교 분석하여 변동 사항을 감지하고 알림 메시지를 생성합니다.
//
// [주요 기능]
//  1. 상품 변경 감지: 가격 변동, 품절 상태 변경, 신규 상품 등록 등을 분석합니다.
//  2. 데이터 정합성 검증: CSV 파일 내 중복 레코드 및 정보 수집 불가 상품을 식별합니다.
//  3. 알림 메시지 조합: 감지된 모든 이벤트(변경, 중복, 불가)를 종합하여 사용자에게 발송할 최종 메시지를 생성합니다.
//  4. 작업 결과 갱신: 유의미한 데이터 변경이 발생한 경우에만 스냅샷을 갱신하여 불필요한 DB 쓰기를 최소화합니다.
//
// [반환값]
//   - string: 사용자에게 발송할 포맷팅된 알림 메시지 (변경 사항이 없으면 문맥에 따라 빈 문자열일 수 있음)
//   - interface{}: DB에 저장할 갱신된 작업 결과 데이터 (변경 사항이 없을 경우 nil)
//   - error: 처리 중 발생한 에러
func (t *task) diffAndNotify(currentSnapshot, prevSnapshot *watchProductPriceSnapshot, records, duplicateRecords [][]string, supportsHTML bool) (string, interface{}, error) {
	// 상품 상태 변화 감지 및 차분(Diff) 렌더링
	// 이전 스냅샷과 현재 상태를 정밀하게 비교하여 유의미한 비즈니스 이벤트(가격 변동, 품절 해제 등)를 포착하고,
	// 이를 사용자가 직관적으로 인지할 수 있는 메시지 포맷으로 변환(Rendering)합니다.
	productsDiffMessage := t.buildProductsDiffMessage(currentSnapshot, prevSnapshot, supportsHTML)

	// 단순한 가격 변동 알림을 넘어, 사용자의 설정 오류(중복 등록)나 외부 요인에 의한 상품 상태 변화(판매 중지)를 식별하여 보고합니다.
	duplicateRecordsMessage := t.buildDuplicateRecordsMessage(duplicateRecords, supportsHTML)
	unavailableProductsMessage := t.buildUnavailableProductsMessage(currentSnapshot.Products, records, supportsHTML)

	// 최종 알림 메시지 조합
	// 앞서 생성된 핵심 변경 내역과 부가 정보들을 하나의 완결된 사용자 메시지로 통합합니다.
	// 이 단계에서는 각 메시지 조각의 유무에 따라 조건부로 포맷팅을 수행하며, 최종적으로 사용자가 받아볼 깔끔하고 가독성 높은 리포트를 완성합니다.
	message := t.buildNotificationMessage(currentSnapshot, productsDiffMessage, duplicateRecordsMessage, unavailableProductsMessage, supportsHTML)

	// 결과 처리 (알림 vs 저장)
	// 알림을 보내는 기준과 데이터를 저장하는 기준을 다르게 적용하여 효율성을 높입니다.
	// - 알림: 사용자가 직접 확인하고 싶어 할 때(RunByUser)는 변경 사항이 없더라도 현재 상태를 리포트하여 안심시켜 줍니다.
	// - 저장: 매번 불필요하게 저장하지 않고, 실제로 가격이나 상태가 변했을 때만 저장하여 시스템 성능을 아낍니다.
	hasChanges := strutil.HasAnyContent(productsDiffMessage, duplicateRecordsMessage, unavailableProductsMessage)
	if hasChanges {
		return message, currentSnapshot, nil
	}

	return message, nil, nil
}

// buildProductsDiffMessage 과거의 가격 정보(Snapshot)와 현재 정보를 꼼꼼히 대조하여, 사용자가 꼭 알아야 할 '진짜 변화'만 추출하여 알림 메시지를 생성합니다.
//
// [핵심 기능]
// 1. 변화 감지: 가격 변동, 할인율 변경, 품절 해제 등 사용자가 반길만한 소식을 찾아냅니다.
// 2. 노이즈 필터링: 이미 알고 있는 정보나 판매 중지된 상품은 굳이 알리지 않아, 알림이 스팸처럼 느껴지지 않게 합니다.
// 3. 메시지 작성: 감지된 변화를 보기 편한 텍스트로 예쁘게 다듬어서(Formatting) 반환합니다.
func (t *task) buildProductsDiffMessage(currentSnapshot, prevSnapshot *watchProductPriceSnapshot, supportsHTML bool) string {
	// @@@@@
	var sb strings.Builder
	// 변경 사항이 발생할 수 있는 최대치(모든 상품 변경 가정)를 고려하여 버퍼를 넉넉히 할당합니다.
	// 상품당 평균 256바이트를 가정합니다. (재할당 오버헤드 최소화)
	estimatedSize := len(currentSnapshot.Products) * 256
	if estimatedSize < 1024 {
		estimatedSize = 1024
	}
	sb.Grow(estimatedSize)

	lineSpacing := "\n\n"
	// 비교를 위해 이전 스냅샷의 상품들을 Map으로 변환합니다 (ID -> Product).
	var prevProductMap map[int]*product
	if prevSnapshot != nil {
		prevProductMap = make(map[int]*product, len(prevSnapshot.Products))
		for _, p := range prevSnapshot.Products {
			prevProductMap[p.ID] = p
		}
	} else {
		prevProductMap = make(map[int]*product)
	}

	for _, currentProduct := range currentSnapshot.Products {
		prevProduct, exists := prevProductMap[currentProduct.ID]

		// 1. 신규 상품이거나, 이전에 알 수 없는 상품이었던 경우
		// 1. 신규 상품(isNewProduct)이거나, 품절 후 재입고(isRestocked)된 경우
		isNewProduct := !exists
		isRestocked := exists && prevProduct.IsUnavailable && !currentProduct.IsUnavailable

		if isNewProduct || isRestocked {
			// 알 수 없는 상품인 경우 상품에 대한 정보를 사용자에게 알리지 않는다.
			if currentProduct.IsUnavailable {
				continue
			}

			// 최저 가격 갱신 (신규 상품 취급)
			currentProduct.updateLowestPrice()

			if sb.Len() > 0 {
				sb.WriteString(lineSpacing)
			}
			sb.WriteString(currentProduct.Render(supportsHTML, mark.New))
			continue
		}

		// 2. 상품이 판매 중이었다가 알 수 없는 상품(판매 중지)으로 변경된 경우
		isDiscontinued := !prevProduct.IsUnavailable && currentProduct.IsUnavailable
		if isDiscontinued {
			continue // 변경 내역 렌더링 생략
		}

		// 3. 기존 상품 변경 내역 비교
		// 이전 최저가 정보를 승계
		currentProduct.LowestPrice = prevProduct.LowestPrice
		currentProduct.LowestPriceTimeUTC = prevProduct.LowestPriceTimeUTC

		// 현재 가격 기준으로 최저가 갱신 시도
		isLowestPriceUpdated := currentProduct.updateLowestPrice()

		// 가격이나 할인율 등이 변경되었는지 확인

		// 가격이나 할인율 등이 변경되었는지 확인
		hasPriceChanged := currentProduct.Price != prevProduct.Price ||
			currentProduct.DiscountedPrice != prevProduct.DiscountedPrice ||
			currentProduct.DiscountRate != prevProduct.DiscountRate

		// [Guard Clause] 변경 사항이 없다면 즉시 다음 상품으로 넘어갑니다.
		if !hasPriceChanged {
			continue
		}

		// [Happy Path] 여기서부터는 "무조건 변경사항이 있는 상태"입니다. 들여쓰기 없이 로직이 진행됩니다.
		if sb.Len() > 0 {
			sb.WriteString(lineSpacing)
		}

		// 최저가가 갱신된 경우 '역대 최저가' 마크를, 단순 변동인 경우 '변경' 마크를 표시
		marker := mark.Change
		if isLowestPriceUpdated {
			marker = mark.BestPrice
		}
		sb.WriteString(currentProduct.RenderDiff(supportsHTML, marker, prevProduct))
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
func (t *task) buildDuplicateRecordsMessage(duplicateRecords [][]string, supportsHTML bool) string {
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
func (t *task) buildUnavailableProductsMessage(products []*product, records [][]string, supportsHTML bool) string {
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
func (t *task) buildNotificationMessage(currentSnapshot *watchProductPriceSnapshot, productsDiffMessage, duplicateRecordsMessage, unavailableProductsMessage string, supportsHTML bool) string {
	// [메시지 조합 여부 판단 (Change Detection)]
	// 개별 메시지들(가격 변동, 중복, 식별 불가) 중에서 유효한 내용이 단 하나라도 존재하는지 검사합니다.
	// 이는 알림의 성격을 단순 '현황 보고'에서 유의미한 '이벤트 알림'으로 전환하는 기준이 됩니다.
	hasChanges := strutil.HasAnyContent(productsDiffMessage, duplicateRecordsMessage, unavailableProductsMessage)
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
	if t.GetRunBy() == tasksvc.RunByUser {
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
