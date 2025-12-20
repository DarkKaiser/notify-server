package naver_shopping

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
	watchPriceCommandIDPrefix string = "WatchPrice_"

	// 네이버쇼핑 검색 URL
	searchURL = "https://openapi.naver.com/v1/search/shop.json"
)

type watchPriceCommandSettings struct {
	Query   string `json:"query"`
	Filters struct {
		IncludedKeywords string `json:"included_keywords"`
		ExcludedKeywords string `json:"excluded_keywords"`
		PriceLessThan    int    `json:"price_less_than"`
	} `json:"filters"`
}

func (c *watchPriceCommandSettings) validate() error {
	if c.Query == "" {
		return apperrors.New(apperrors.InvalidInput, "query가 입력되지 않았습니다")
	}
	if c.Filters.PriceLessThan <= 0 {
		return apperrors.New(apperrors.InvalidInput, "price_less_than에 0 이하의 값이 입력되었습니다")
	}
	return nil
}

type searchResponse struct {
	Total   int `json:"total"`
	Start   int `json:"start"`
	Display int `json:"display"`
	Items   []struct {
		Title       string `json:"title"`
		Link        string `json:"link"`
		LowPrice    string `json:"lprice"`
		MallName    string `json:"mallName"`
		ProductID   string `json:"productId"`
		ProductType string `json:"productType"`
	} `json:"items"`
}

type product struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	LowPrice    int    `json:"lprice"`
	ProductID   string `json:"productId"`
	ProductType string `json:"productType"`
}

func (p *product) String(supportsHTML bool, mark string) string {
	if supportsHTML == true {
		return fmt.Sprintf("☞ <a href=\"%s\"><b>%s</b></a> %s원%s", p.Link, p.Title, strutil.FormatCommas(p.LowPrice), mark)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s %s원%s\n%s", p.Title, strutil.FormatCommas(p.LowPrice), mark, p.Link))
}

type watchPriceSnapshot struct {
	Products []*product `json:"products"`
}

// noinspection GoUnhandledErrorResult
func (t *task) executeWatchPrice(commandSettings *watchPriceCommandSettings, originTaskResultData *watchPriceSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {

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
		err = tasksvc.FetchJSON(t.GetFetcher(), "GET", fmt.Sprintf("%s?query=%s&display=100&start=%d&sort=sim", searchURL, url.QueryEscape(commandSettings.Query), searchResultItemStartNo), header, nil, _searchResultData_)
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
		if tasksvc.Filter(item.Title, includedKeywords, excludedKeywords) == false {
			goto NEXTITEM
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

	NEXTITEM:
	}

	//
	// 필터링 된 상품 정보를 확인한다.
	//
	m := ""
	lineSpacing := "\n\n"
	if supportsHTML == true {
		lineSpacing = "\n"
	}
	err = tasksvc.EachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Products, originTaskResultData.Products, func(selem, telem interface{}) (bool, error) {
		actualityProduct, ok1 := selem.(*product)
		originProduct, ok2 := telem.(*product)
		if ok1 == false || ok2 == false {
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
			if m != "" {
				m += lineSpacing
			}
			m += originProduct.String(supportsHTML, fmt.Sprintf(" ⇒ %s원 🔁", strutil.FormatCommas(actualityProduct.LowPrice)))
		}
	}, func(selem interface{}) {
		actualityProduct := selem.(*product)

		if m != "" {
			m += lineSpacing
		}
		m += actualityProduct.String(supportsHTML, " 🆕")
	})
	if err != nil {
		return "", nil, err
	}

	filtersDescription := fmt.Sprintf("조회 조건은 아래와 같습니다:\n• 검색 키워드 : %s\n• 상풍명 포함 키워드 : %s\n• 상품명 제외 키워드 : %s\n• %s원 미만의 상품", commandSettings.Query, commandSettings.Filters.IncludedKeywords, commandSettings.Filters.ExcludedKeywords, strutil.FormatCommas(commandSettings.Filters.PriceLessThan))

	if m != "" {
		message = fmt.Sprintf("조회 조건에 해당되는 상품의 정보가 변경되었습니다.\n\n%s\n\n%s", filtersDescription, m)
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.GetRunBy() == tasksvc.RunByUser {
			if len(actualityTaskResultData.Products) == 0 {
				message = fmt.Sprintf("조회 조건에 해당되는 상품이 존재하지 않습니다.\n\n%s", filtersDescription)
			} else {
				for _, actualityProduct := range actualityTaskResultData.Products {
					if m != "" {
						m += lineSpacing
					}
					m += actualityProduct.String(supportsHTML, "")
				}

				message = fmt.Sprintf("조회 조건에 해당되는 상품의 변경된 정보가 없습니다.\n\n%s\n\n조회 조건에 해당되는 상품은 아래와 같습니다:\n\n%s", filtersDescription, m)
			}
		}
	}

	return message, changedTaskResultData, nil
}
