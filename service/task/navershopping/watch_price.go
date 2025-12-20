package navershopping

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
)

const (
	// watchPriceAnyCommandPrefix는 동적 커맨드 라우팅을 위한 식별자 접두어입니다.
	//
	// 이 접두어로 시작하는 모든 CommandID는 `executeWatchPrice` 핸들러로 라우팅되어 처리됩니다.
	// 이를 통해 사용자는 "WatchPrice_Apple", "WatchPrice_Samsung" 등과 같이
	// 하나의 로직으로 처리되는 다수의 커맨드를 유연하게 생성할 수 있습니다.
	watchPriceAnyCommandPrefix = "WatchPrice_"

	// searchAPIURL은 네이버 쇼핑 상품 검색을 위한 OpenAPI 엔드포인트입니다.
	// 공식 문서: https://developers.naver.com/docs/serviceapi/search/shopping/shopping.md
	searchAPIURL = "https://openapi.naver.com/v1/search/shop.json"
)

type watchPriceSettings struct {
	Query   string `json:"query"`
	Filters struct {
		IncludedKeywords string `json:"included_keywords"`
		ExcludedKeywords string `json:"excluded_keywords"`
		PriceLessThan    int    `json:"price_less_than"`
	} `json:"filters"`
}

func (s *watchPriceSettings) validate() error {
	if strings.TrimSpace(s.Query) == "" {
		return apperrors.New(apperrors.InvalidInput, "query가 입력되지 않았거나 공백입니다")
	}
	if s.Filters.PriceLessThan <= 0 {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("price_less_than은 0보다 커야 합니다 (입력값: %d)", s.Filters.PriceLessThan))
	}
	return nil
}

// watchPriceSnapshot 가격 변동을 감지하기 위한 상품 데이터의 스냅샷입니다.
type watchPriceSnapshot struct {
	Products []*product `json:"products"`
}

// product 검색 API를 통해 조회된 개별 상품 정보를 담는 도메인 모델입니다.
type product struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	LowPrice    int    `json:"lprice"`
	ProductID   string `json:"productId"`
	ProductType string `json:"productType"`
}

// String 상품 정보를 사용자에게 발송하기 위한 알림 메시지 포맷으로 변환합니다.
func (p *product) String(supportsHTML bool, mark string) string {
	if supportsHTML {
		const htmlFormat = `☞ <a href="%s"><b>%s</b></a> %s원%s`

		return fmt.Sprintf(
			htmlFormat,
			p.Link,
			p.Title,
			strutil.FormatCommas(p.LowPrice),
			mark,
		)
	}

	const textFormat = `☞ %s %s원%s
%s`

	return strings.TrimSpace(fmt.Sprintf(textFormat, p.Title, strutil.FormatCommas(p.LowPrice), mark, p.Link))
}

type searchResponseItem struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	LowPrice    string `json:"lprice"`
	MallName    string `json:"mallName"`
	ProductID   string `json:"productId"`
	ProductType string `json:"productType"`
}

type searchResponse struct {
	Total   int                   `json:"total"`
	Start   int                   `json:"start"`
	Display int                   `json:"display"`
	Items   []*searchResponseItem `json:"items"`
}

// noinspection GoUnhandledErrorResult
func (t *task) executeWatchPrice(commandSettings *watchPriceSettings, originTaskResultData *watchPriceSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {

	//
	// 상품에 대한 정보를 검색한다.
	//
	const maxSearchableItemCount = 100 // 한번에 검색 가능한 상품의 최대 갯수
	var (
		header = map[string]string{
			"X-Naver-Client-Id":     t.clientID,
			"X-Naver-Client-Secret": t.clientSecret,
		}
		searchResultItemStartNo    = 1
		searchResultItemTotalCount = math.MaxInt

		searchResultData = &searchResponse{}
	)
	for searchResultItemStartNo < searchResultItemTotalCount {
		var _searchResultData_ = &searchResponse{}

		u, err := url.Parse(searchAPIURL)
		if err != nil {
			return "", nil, apperrors.Wrap(err, apperrors.Internal, "검색 URL 파싱 실패")
		}

		q := u.Query()
		q.Set("query", commandSettings.Query)
		q.Set("display", "100")
		q.Set("start", strconv.Itoa(searchResultItemStartNo))
		q.Set("sort", "sim")
		u.RawQuery = q.Encode()

		err = tasksvc.FetchJSON(t.GetFetcher(), "GET", u.String(), header, nil, _searchResultData_)
		if err != nil {
			return "", nil, err
		}

		if searchResultItemTotalCount == math.MaxInt {
			searchResultData.Total = _searchResultData_.Total
			searchResultData.Start = _searchResultData_.Start
			searchResultData.Display = _searchResultData_.Display

			searchResultItemTotalCount = _searchResultData_.Total

			// 최대 1000건의 데이터를 읽어들이도록 한다.
			if searchResultData.Total > 1000 {
				searchResultData.Total = 1000
				searchResultItemTotalCount = 1000
			}
		}
		searchResultData.Items = append(searchResultData.Items, _searchResultData_.Items...)

		searchResultItemStartNo += maxSearchableItemCount
	}

	//
	// 검색된 상품 목록을 설정된 조건에 맞게 필터링한다.
	//
	actualityTaskResultData := &watchPriceSnapshot{}
	includedKeywords := strutil.SplitAndTrim(commandSettings.Filters.IncludedKeywords, ",")
	excludedKeywords := strutil.SplitAndTrim(commandSettings.Filters.ExcludedKeywords, ",")

	var lowPrice int
	for _, item := range searchResultData.Items {
		if !tasksvc.Filter(item.Title, includedKeywords, excludedKeywords) {
			continue
		}

		lowPrice, _ = strconv.Atoi(item.LowPrice)
		if lowPrice > 0 && lowPrice < commandSettings.Filters.PriceLessThan {
			actualityTaskResultData.Products = append(actualityTaskResultData.Products, &product{
				Title:       item.Title,
				Link:        item.Link,
				LowPrice:    lowPrice,
				ProductID:   item.ProductID,
				ProductType: item.ProductType,
			})
		}
	}

	//
	// 필터링 된 상품 정보를 확인한다.
	//
	//
	// 필터링 된 상품 정보를 확인한다.
	//
	var sb strings.Builder
	lineSpacing := "\n\n"
	if supportsHTML {
		lineSpacing = "\n"
	}
	err = tasksvc.EachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Products, originTaskResultData.Products, func(selem, telem interface{}) (bool, error) {
		actualityProduct, ok1 := selem.(*product)
		originProduct, ok2 := telem.(*product)
		if !ok1 || !ok2 {
			return false, tasksvc.NewErrTypeAssertionFailed("selm/telm", &product{}, selem)
		} else {
			if actualityProduct.Link == originProduct.Link {
				return true, nil
			}
		}
		return false, nil
	}, func(selem, telem interface{}) {
		actualityProduct := selem.(*product)
		originProduct := telem.(*product)

		if actualityProduct.LowPrice != originProduct.LowPrice {
			if sb.Len() > 0 {
				sb.WriteString(lineSpacing)
			}
			sb.WriteString(originProduct.String(supportsHTML, fmt.Sprintf(" ⇒ %s원 🔁", strutil.FormatCommas(actualityProduct.LowPrice))))
		}
	}, func(selem interface{}) {
		actualityProduct := selem.(*product)

		if sb.Len() > 0 {
			sb.WriteString(lineSpacing)
		}
		sb.WriteString(actualityProduct.String(supportsHTML, " 🆕"))
	})
	if err != nil {
		return "", nil, err
	}

	filtersDescription := fmt.Sprintf("조회 조건은 아래와 같습니다:\n• 검색 키워드 : %s\n• 상풍명 포함 키워드 : %s\n• 상품명 제외 키워드 : %s\n• %s원 미만의 상품", commandSettings.Query, commandSettings.Filters.IncludedKeywords, commandSettings.Filters.ExcludedKeywords, strutil.FormatCommas(commandSettings.Filters.PriceLessThan))

	if sb.Len() > 0 {
		message = fmt.Sprintf("조회 조건에 해당되는 상품의 정보가 변경되었습니다.\n\n%s\n\n%s", filtersDescription, sb.String())
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.GetRunBy() == tasksvc.RunByUser {
			if len(actualityTaskResultData.Products) == 0 {
				message = fmt.Sprintf("조회 조건에 해당되는 상품이 존재하지 않습니다.\n\n%s", filtersDescription)
			} else {
				for _, actualityProduct := range actualityTaskResultData.Products {
					if sb.Len() > 0 {
						sb.WriteString(lineSpacing)
					}
					sb.WriteString(actualityProduct.String(supportsHTML, ""))
				}

				message = fmt.Sprintf("조회 조건에 해당되는 상품의 변경된 정보가 없습니다.\n\n%s\n\n조회 조건에 해당되는 상품은 아래와 같습니다:\n\n%s", filtersDescription, sb.String())
			}
		}
	}

	return message, changedTaskResultData, nil
}
