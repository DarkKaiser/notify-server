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

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	"github.com/darkkaiser/notify-server/service/task"
)

const (
	TidKurly task.ID = "KURLY" // 마켓컬리

	TcidKurlyWatchProductPrice task.CommandID = "WatchProductPrice" // 마켓컬리 가격 확인
)

const (
	kurlyBaseURL = "https://www.kurly.com/"
)

// watchProductColumn 감시할 상품 목록의 헤더
type watchProductColumn uint

// 감시할 상품 목록의 헤더 컬럼
const (
	WatchProductColumnNo          watchProductColumn = iota // 상품 코드 컬럼
	WatchProductColumnName                                  // 상품 이름 컬럼
	WatchProductColumnWatchStatus                           // 감시 대상인지에 대한 활성/비활성 컬럼
)

// 감시 대상인지에 대한 활성/비활성 컬럼의 값
const (
	WatchStatusEnabled  = "1"
	WatchStatusDisabled = "0"
)

type kurlyWatchProductPriceCommandData struct {
	WatchProductsFile string `json:"watch_products_file"`
}

func (d *kurlyWatchProductPriceCommandData) validate() error {
	if d.WatchProductsFile == "" {
		return apperrors.New(apperrors.ErrInvalidInput, "상품 목록이 저장된 파일이 입력되지 않았습니다")
	}
	if strings.HasSuffix(strings.ToLower(d.WatchProductsFile), ".csv") == false {
		return apperrors.New(apperrors.ErrInvalidInput, "상품 목록이 저장된 파일은 .CSV 파일만 사용할 수 있습니다")
	}
	return nil
}

type kurlyProduct struct {
	No               int       `json:"no"`                 // 상품 코드
	Name             string    `json:"name"`               // 상품 이름
	Price            int       `json:"price"`              // 가격
	DiscountedPrice  int       `json:"discounted_price"`   // 할인 가격
	DiscountRate     int       `json:"discount_rate"`      // 할인율
	LowestPrice      int       `json:"lowest_price"`       // 최저 가격
	LowestPriceTime  time.Time `json:"lowest_price_time"`  // 최저 가격이 등록된 시간
	IsUnknownProduct bool      `json:"is_unknown_product"` // 알 수 없는 상품인지에 대한 여부(상품 코드가 존재하지 않거나, 이전에는 판매를 하였지만 현재는 판매하고 있지 않는 상품)
}

func (p *kurlyProduct) String(messageTypeHTML bool, mark string, previousProduct *kurlyProduct) string {
	// 가격 및 할인 가격을 문자열로 반환하는 함수
	formatPrice := func(price, discountedPrice, discountRate int) string {
		// 할인 가격이 없거나 가격과 동일하면 그냥 가격을 반환한다.
		if discountedPrice == 0 || discountedPrice == price {
			return fmt.Sprintf("%s원", strutil.FormatCommas(price))
		}

		if messageTypeHTML == true {
			return fmt.Sprintf("<s>%s원</s> %s원 (%d%%)", strutil.FormatCommas(price), strutil.FormatCommas(discountedPrice), discountRate)
		}
		return fmt.Sprintf("%s원 ⇒ %s원 (%d%%)", strutil.FormatCommas(price), strutil.FormatCommas(discountedPrice), discountRate)
	}

	// 상품 이름
	var name string
	if messageTypeHTML == true {
		name = fmt.Sprintf("☞ <a href=\"%sgoods/%d\"><b>%s</b></a>%s", kurlyBaseURL, p.No, template.HTMLEscapeString(p.Name), mark)
	} else {
		name = fmt.Sprintf("☞ %s%s", template.HTMLEscapeString(p.Name), mark)
	}

	// 상품의 이전 가격 문자열을 구한다.
	var previousPriceString string
	if previousProduct != nil {
		previousPriceString = fmt.Sprintf("\n      • 이전 가격 : %s", formatPrice(previousProduct.Price, previousProduct.DiscountedPrice, previousProduct.DiscountRate))
	}

	// 상품의 최저 가격 문자열을 구한다.
	var lowestPriceString string
	if p.LowestPrice != 0 {
		lowestPriceString = fmt.Sprintf("\n      • 최저 가격 : %s (%s)", formatPrice(p.LowestPrice, 0, 0), p.LowestPriceTime.Format("2006/01/02 15:04"))
	}

	return fmt.Sprintf("%s\n      • 현재 가격 : %s%s%s", name, formatPrice(p.Price, p.DiscountedPrice, p.DiscountRate), previousPriceString, lowestPriceString)
}

// 만약 이전에 저장된 최저 가격이 없다면, 가격과 할인 가격에서 더 낮은 가격을 최저 가격으로 변경한다.
// 만약 이전에 저장된 최저 가격이 있다면, 가격 또는 할인 가격과 이전에 저장된 최저 가격을 비교하여 더 낮은 가격을 최저 가격으로 변경한다.
func (p *kurlyProduct) updateLowestPrice() {
	setLowestPrice := func(price int) {
		if p.LowestPrice == 0 || p.LowestPrice > price {
			// 최저 가격이 저장되어 있지 않거나, 새로운 가격이 더 낮다면 최저 가격을 업데이트하고 현재 시간을 기록한다.
			p.LowestPrice = price
			p.LowestPriceTime = time.Now()
		}
	}

	// 할인 가격이 존재하면 최저 가격을 업데이트한다.
	if p.DiscountedPrice != 0 {
		setLowestPrice(p.DiscountedPrice)
	}
	// 현재 가격으로 최저 가격을 업데이트한다.
	setLowestPrice(p.Price)
}

type kurlyWatchProductPriceResultData struct {
	Products []*kurlyProduct `json:"products"`
}

func init() {
	task.Register(TidKurly, &task.Config{
		Commands: []*task.CommandConfig{{
			ID: TcidKurlyWatchProductPrice,

			AllowMultiple: true,

			NewSnapshot: func() interface{} { return &kurlyWatchProductPriceResultData{} },
		}},

		NewTask: func(instanceID task.InstanceID, req *task.RunRequest, appConfig *config.AppConfig) (task.Handler, error) {
			if req.TaskID != TidKurly {
				return nil, apperrors.New(task.ErrTaskNotFound, "등록되지 않은 작업입니다.😱")
			}

			tTask := &kurlyTask{
				Task: task.Task{
					ID:         req.TaskID,
					CommandID:  req.CommandID,
					InstanceID: instanceID,

					NotifierID: req.NotifierID,

					Canceled: false,

					RunBy: req.RunBy,

					Fetcher: nil,
				},
				appConfig: appConfig,
			}

			retryDelay, err := time.ParseDuration(appConfig.HTTPRetry.RetryDelay)
			if err != nil {
				retryDelay, _ = time.ParseDuration(config.DefaultRetryDelay)
			}
			tTask.Fetcher = task.NewRetryFetcher(task.NewHTTPFetcher(), appConfig.HTTPRetry.MaxRetries, retryDelay, 30*time.Second)

			tTask.Execute = func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {

				switch tTask.GetCommandID() {
				case TcidKurlyWatchProductPrice:
					for _, t := range tTask.appConfig.Tasks {
						if tTask.GetID() == task.ID(t.ID) {
							for _, c := range t.Commands {
								if tTask.GetCommandID() == task.CommandID(c.ID) {
									commandData := &kurlyWatchProductPriceCommandData{}
									if err := task.FillCommandDataFromMap(commandData, c.Data); err != nil {
										return "", nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
									}
									if err := commandData.validate(); err != nil {
										return "", nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
									}

									originTaskResultData, ok := previousSnapshot.(*kurlyWatchProductPriceResultData)
									if ok == false {
										return "", nil, apperrors.New(apperrors.ErrInternal, fmt.Sprintf("TaskResultData의 타입 변환이 실패하였습니다 (expected: *kurlyWatchProductPriceResultData, got: %T)", previousSnapshot))
									}

									return tTask.executeWatchProductPrice(commandData, originTaskResultData, supportsHTML)
								}
							}
							break
						}
					}
				}
				return "", nil, task.ErrCommandNotImplemented
			}

			return tTask, nil
		},
	})
}

type kurlyTask struct {
	task.Task

	appConfig *config.AppConfig
}

// noinspection GoUnhandledErrorResult,GoErrorStringFormat
func (t *kurlyTask) executeWatchProductPrice(commandData *kurlyWatchProductPriceCommandData, originTaskResultData *kurlyWatchProductPriceResultData, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {

	//
	// 감시할 상품 목록을 읽어들인다.
	//
	f, err := os.Open(commandData.WatchProductsFile)
	if err != nil {
		return "", nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "상품 목록이 저장된 파일을 불러올 수 없습니다. 파일이 존재하는지와 경로가 올바른지 확인해 주세요")
	}
	defer f.Close()

	r := csv.NewReader(f)
	watchProducts, err := r.ReadAll()
	if err != nil {
		return "", nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "상품 목록을 불러올 수 없습니다")
	}

	// 감시할 상품 목록의 헤더를 제거한다.
	watchProducts = watchProducts[1:]

	// 감시할 상품 목록에서 중복된 상품을 정규화한다.
	watchProducts, duplicateWatchProducts := t.normalizeDuplicateProducts(watchProducts)

	//
	// 읽어들인 상품들의 가격 및 상태를 확인한다.
	//
	actualityTaskResultData := &kurlyWatchProductPriceResultData{}

	// 읽어들인 상품 페이지에서 상품 데이터가 JSON 포맷으로 저장된 자바스크립트 구문을 추출하기 위한 정규표현식
	re1 := regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*>([\s\S]*?)</script>`)

	// 읽어들인 상품 페이지의 상품 데이터에서 판매중인 상품이 아닌지 확인하고자 하는 정규표현식
	re2 := regexp.MustCompile(`"product":\s*null`)

	for _, watchProduct := range watchProducts {
		if watchProduct[WatchProductColumnWatchStatus] != WatchStatusEnabled {
			continue
		}

		// 상품 코드를 숫자로 변환한다.
		no, err := strconv.Atoi(watchProduct[WatchProductColumnNo])
		if err != nil {
			return "", nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "상품 코드의 숫자 변환이 실패하였습니다")
		}

		// 상품 페이지를 읽어들인다.
		productDetailPageURL := fmt.Sprintf("%sgoods/%d", kurlyBaseURL, no)
		doc, err := task.FetchHTMLDocument(t.Fetcher, productDetailPageURL)
		if err != nil {
			return "", nil, err
		}

		// 읽어들인 페이지에서 상품 데이터가 JSON 포맷으로 저장된 자바스크립트 구문을 추출한다.
		html, err := doc.Html()
		if err != nil {
			return "", nil, apperrors.Wrap(err, task.ErrTaskExecutionFailed, fmt.Sprintf("불러온 페이지(%s)에서 HTML 추출이 실패하였습니다", productDetailPageURL))
		}
		match := re1.FindStringSubmatch(html)
		if len(match) < 2 {
			return "", nil, apperrors.New(task.ErrTaskExecutionFailed, fmt.Sprintf("불러온 페이지(%s)에서 상품에 대한 JSON 데이터 추출이 실패하였습니다.(error:%s)", productDetailPageURL, err))
		}
		jsonProductData := match[1]

		var product = &kurlyProduct{
			No:               no,
			Name:             "",
			Price:            0,
			DiscountedPrice:  0,
			DiscountRate:     0,
			LowestPrice:      0,
			LowestPriceTime:  time.Time{},
			IsUnknownProduct: false,
		}

		// 알 수 없는 상품(현재 판매중이지 않은 상품)인지 확인한다.
		if re2.MatchString(jsonProductData) == true {
			product.IsUnknownProduct = true
		}

		if product.IsUnknownProduct == false {
			sel := doc.Find("#product-atf > section.css-1ua1wyk")
			if sel.Length() != 1 {
				return "", nil, apperrors.New(task.ErrTaskExecutionFailed, fmt.Sprintf("불러온 페이지(%s)의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요.(상품정보 섹션 추출 실패)", productDetailPageURL))
			}

			// 상품 이름을 확인한다.
			ps := sel.Find("div.css-84rb3h > div.css-6zfm8o > div.css-o3fjh7 > h1")
			if ps.Length() != 1 {
				return "", nil, apperrors.New(task.ErrTaskExecutionFailed, fmt.Sprintf("상품 이름 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productDetailPageURL))
			}
			product.Name = strutil.NormalizeSpaces(ps.Text())

			// 상품 가격을 추출한다.
			ps = sel.Find("h2.css-xrp7wx > span.css-8h3us8")
			if ps.Length() == 0 /* 가격, 단위(원) */ {
				ps = sel.Find("h2.css-xrp7wx > div.css-o2nlqt > span")
				if ps.Length() != 2 /* 가격 + 단위(원) */ {
					return "", nil, apperrors.New(task.ErrTaskExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productDetailPageURL))
				}

				// 가격
				product.Price, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), ",", ""))
				if err != nil {
					return "", nil, apperrors.Wrap(err, task.ErrTaskExecutionFailed, "상품 가격의 숫자 변환이 실패하였습니다")
				}
			} else if ps.Length() == 1 /* 할인율, 할인 가격, 단위(원) */ {
				// 할인율
				product.DiscountRate, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), "%", ""))
				if err != nil {
					return "", nil, apperrors.Wrap(err, task.ErrTaskExecutionFailed, "상품 할인율의 숫자 변환이 실패하였습니다")
				}

				// 할인 가격
				ps = sel.Find("h2.css-xrp7wx > div.css-o2nlqt > span")
				if ps.Length() != 2 /* 가격 + 단위(원) */ {
					return "", nil, apperrors.New(task.ErrTaskExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productDetailPageURL))
				}

				product.DiscountedPrice, err = strconv.Atoi(strings.ReplaceAll(ps.Eq(0).Text(), ",", ""))
				if err != nil {
					return "", nil, apperrors.Wrap(err, task.ErrTaskExecutionFailed, "상품 할인 가격의 숫자 변환이 실패하였습니다")
				}

				// 가격
				ps = sel.Find("span.css-1s96j0s > span")
				if ps.Length() != 1 /* 가격 + 단위(원) */ {
					return "", nil, apperrors.New(task.ErrTaskExecutionFailed, fmt.Sprintf("상품 가격(0) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productDetailPageURL))
				}
				product.Price, _ = strconv.Atoi(strings.ReplaceAll(strings.ReplaceAll(ps.Text(), ",", ""), "원", ""))
			} else {
				return "", nil, apperrors.New(task.ErrTaskExecutionFailed, fmt.Sprintf("상품 가격(1) 추출이 실패하였습니다. CSS셀렉터를 확인하세요.(%s)", productDetailPageURL))
			}
		}

		actualityTaskResultData.Products = append(actualityTaskResultData.Products, product)
	}

	//
	// 상품들의 변경된 가격 및 상태를 확인한다.
	//
	m := ""
	lineSpacing := "\n\n"
	if supportsHTML == true {
		lineSpacing = "\n"
	}
	err = task.EachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Products, originTaskResultData.Products, func(selem, telem interface{}) (bool, error) {
		actualityProduct, ok1 := selem.(*kurlyProduct)
		originProduct, ok2 := telem.(*kurlyProduct)
		if ok1 == false || ok2 == false {
			return false, apperrors.New(apperrors.ErrInternal, "selem/telem의 타입 변환이 실패하였습니다")
		} else {
			if actualityProduct.No == originProduct.No {
				return true, nil
			}
		}
		return false, nil
	}, func(selem, telem interface{}) {
		actualityProduct := selem.(*kurlyProduct)
		originProduct := telem.(*kurlyProduct)

		// 상품이 원래는 판매 중이었지만, 이제는 알 수 없는 상품으로 변경된 경우...
		if originProduct.IsUnknownProduct == false && actualityProduct.IsUnknownProduct == true {
			return
		}
		// 상품이 원래는 알 수 없는 상품이었지만, 이제는 판매 중인 상품으로 변경된 경우...
		if originProduct.IsUnknownProduct == true && actualityProduct.IsUnknownProduct == false {
			// 최저 가격을 업데이트한다.
			actualityProduct.updateLowestPrice()

			if m != "" {
				m += lineSpacing
			}
			m += actualityProduct.String(supportsHTML, " 🆕", nil)

			return
		}

		// 상품의 이전 최저 가격과 해당 시간 정보를 현재 상품 정보에 반영합니다.
		actualityProduct.LowestPrice = originProduct.LowestPrice
		actualityProduct.LowestPriceTime = originProduct.LowestPriceTime

		// 최저 가격을 업데이트한다.
		actualityProduct.updateLowestPrice()

		if actualityProduct.Price != originProduct.Price || actualityProduct.DiscountedPrice != originProduct.DiscountedPrice || actualityProduct.DiscountRate != originProduct.DiscountRate {
			if m != "" {
				m += lineSpacing
			}
			m += actualityProduct.String(supportsHTML, " 🔁", originProduct)
		}
	}, func(selem interface{}) {
		actualityProduct := selem.(*kurlyProduct)

		// 알 수 없는 상품인 경우에는 상품에 대한 정보를 사용자에게 알리지 않는다.
		if actualityProduct.IsUnknownProduct == true {
			return
		}

		// 최저 가격을 업데이트한다.
		actualityProduct.updateLowestPrice()

		if m != "" {
			m += lineSpacing
		}
		m += actualityProduct.String(supportsHTML, " 🆕", nil)
	})
	if err != nil {
		return "", nil, err
	}

	//
	// 읽어들인 상품 목록에서 중복된 상품 및 현재 판매중이지 않은 상품을 확인하고, 각각에 대해 상품들의 정보를 추출한다.
	//

	// 읽어들인 상품 목록에서 중복으로 등록된 상품들의 정보를 추출한다.
	var duplicateProductsBuilder strings.Builder
	for i, product := range duplicateWatchProducts {
		if i > 0 {
			duplicateProductsBuilder.WriteString("\n")
		}

		productNo := strings.TrimSpace(product[WatchProductColumnNo])
		productName := template.HTMLEscapeString(strings.TrimSpace(product[WatchProductColumnName]))

		if supportsHTML == true {
			duplicateProductsBuilder.WriteString(fmt.Sprintf("      • <a href=\"%sgoods/%s\"><b>%s</b></a>", kurlyBaseURL, productNo, productName))
		} else {
			duplicateProductsBuilder.WriteString(fmt.Sprintf("      • %s(%s)", productName, productNo))
		}
	}

	// 읽어들인 상품 목록에서 알 수 없는 상품들의 정보를 추출한다.
	var unknownProductsBuilder strings.Builder
	for _, product := range actualityTaskResultData.Products {
		if product.IsUnknownProduct == true {
			for _, watchProduct := range watchProducts {
				if watchProduct[WatchProductColumnNo] == strconv.Itoa(product.No) {
					if unknownProductsBuilder.Len() != 0 {
						unknownProductsBuilder.WriteString("\n")
					}

					productNo := strings.TrimSpace(watchProduct[WatchProductColumnNo])
					productName := template.HTMLEscapeString(strings.TrimSpace(watchProduct[WatchProductColumnName]))

					if supportsHTML == true {
						unknownProductsBuilder.WriteString(fmt.Sprintf("      • <a href=\"%sgoods/%s\"><b>%s</b></a>", kurlyBaseURL, productNo, productName))
					} else {
						unknownProductsBuilder.WriteString(fmt.Sprintf("      • %s(%s)", productName, productNo))
					}
					break
				}
			}
		}
	}

	//
	// 조건에 따라 상품 정보 변경 사항을 처리하고 메시지를 생성한다.
	//
	if m != "" || duplicateProductsBuilder.Len() > 0 || unknownProductsBuilder.Len() > 0 {
		if m != "" {
			message = fmt.Sprintf("상품 정보가 변경되었습니다.\n\n%s\n\n", m)
		} else {
			message = "상품 정보가 변경되었습니다.\n\n"
		}
		if duplicateProductsBuilder.Len() > 0 {
			message += fmt.Sprintf("중복으로 등록된 상품 목록:\n%s\n\n", duplicateProductsBuilder.String())
		}
		if unknownProductsBuilder.Len() > 0 {
			message += fmt.Sprintf("알 수 없는 상품 목록:\n%s\n\n", unknownProductsBuilder.String())
		}

		changedTaskResultData = actualityTaskResultData
	} else {
		if t.RunBy == task.RunByUser {
			if len(actualityTaskResultData.Products) == 0 {
				message = "등록된 상품 정보가 존재하지 않습니다."
			} else {
				for _, actualityProduct := range actualityTaskResultData.Products {
					if m != "" {
						m += lineSpacing
					}
					m += actualityProduct.String(supportsHTML, "", nil)
				}

				message = fmt.Sprintf("변경된 상품 정보가 없습니다.\n\n%s현재 등록된 상품 정보는 아래와 같습니다:", m)
			}
		}
	}

	return message, changedTaskResultData, nil
}

// normalizeDuplicateProducts 함수는 입력된 상품 목록에서 중복된 상품을 제거하고, 중복된 상품을 별도의 목록에 저장한다.
// 반환 값으로는 중복이 제거된 상품 목록과 중복된 상품 목록을 반환한다.
func (t *kurlyTask) normalizeDuplicateProducts(products [][]string) ([][]string, [][]string) {
	var distinctProducts [][]string
	var duplicateProducts [][]string

	checkedProducts := make(map[string]bool)

	for _, product := range products {
		if len(product) == 0 {
			continue
		}

		productNo := product[WatchProductColumnNo]
		if !checkedProducts[productNo] {
			checkedProducts[productNo] = true
			distinctProducts = append(distinctProducts, product)
		} else {
			duplicateProducts = append(duplicateProducts, product)
		}
	}

	return distinctProducts, duplicateProducts
}
