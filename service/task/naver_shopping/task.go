package naver_shopping

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/strutils"
	"github.com/darkkaiser/notify-server/service/task"
)

const (
	naverShoppingWatchPriceTaskCommandIDPrefix string = "WatchPrice_"

	// TaskID
	TidNaverShopping task.TaskID = "NS" // 네이버쇼핑(https://shopping.naver.com/)

	// TaskCommandID
	TcidNaverShoppingWatchPriceAny = task.TaskCommandID(naverShoppingWatchPriceTaskCommandIDPrefix + "*") // 네이버쇼핑 가격 확인

	// 네이버쇼핑 검색 URL
	naverShoppingSearchURL = "https://openapi.naver.com/v1/search/shop.json"
)

type naverShoppingTaskData struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func (d *naverShoppingTaskData) validate() error {
	if d.ClientID == "" {
		return apperrors.New(apperrors.ErrInvalidInput, "client_id가 입력되지 않았습니다")
	}
	if d.ClientSecret == "" {
		return apperrors.New(apperrors.ErrInvalidInput, "client_secret이 입력되지 않았습니다")
	}
	return nil
}

type naverShoppingWatchPriceTaskCommandData struct {
	Query   string `json:"query"`
	Filters struct {
		IncludedKeywords string `json:"included_keywords"`
		ExcludedKeywords string `json:"excluded_keywords"`
		PriceLessThan    int    `json:"price_less_than"`
	} `json:"filters"`
}

func (d *naverShoppingWatchPriceTaskCommandData) validate() error {
	if d.Query == "" {
		return apperrors.New(apperrors.ErrInvalidInput, "query가 입력되지 않았습니다")
	}
	if d.Filters.PriceLessThan <= 0 {
		return apperrors.New(apperrors.ErrInvalidInput, "price_less_than에 0 이하의 값이 입력되었습니다")
	}
	return nil
}

type naverShoppingWatchPriceSearchResultData struct {
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

type naverShoppingProduct struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	LowPrice    int    `json:"lprice"`
	ProductID   string `json:"productId"`
	ProductType string `json:"productType"`
}

func (p *naverShoppingProduct) String(messageTypeHTML bool, mark string) string {
	if messageTypeHTML == true {
		return fmt.Sprintf("☞ <a href=\"%s\"><b>%s</b></a> %s원%s", p.Link, p.Title, strutils.FormatCommas(p.LowPrice), mark)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s %s원%s\n%s", p.Title, strutils.FormatCommas(p.LowPrice), mark, p.Link))
}

type naverShoppingWatchPriceResultData struct {
	Products []*naverShoppingProduct `json:"products"`
}

func init() {
	task.RegisterTask(TidNaverShopping, &task.TaskConfig{
		CommandConfigs: []*task.TaskCommandConfig{{
			TaskCommandID: TcidNaverShoppingWatchPriceAny,

			AllowMultipleInstances: true,

			NewTaskResultDataFn: func() interface{} { return &naverShoppingWatchPriceResultData{} },
		}},

		NewTaskFn: func(instanceID task.TaskInstanceID, taskRunData *task.TaskRunData, appConfig *config.AppConfig) (task.TaskHandler, error) {
			if taskRunData.TaskID != TidNaverShopping {
				return nil, apperrors.New(apperrors.ErrTaskNotFound, "등록되지 않은 작업입니다.😱")
			}

			taskData := &naverShoppingTaskData{}
			for _, t := range appConfig.Tasks {
				if taskRunData.TaskID == task.TaskID(t.ID) {
					if err := task.FillTaskDataFromMap(taskData, t.Data); err != nil {
						return nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 데이터가 유효하지 않습니다")
					}
					break
				}
			}
			if err := taskData.validate(); err != nil {
				return nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 데이터가 유효하지 않습니다")
			}

			tTask := &naverShoppingTask{
				Task: task.Task{
					ID:         taskRunData.TaskID,
					CommandID:  taskRunData.TaskCommandID,
					InstanceID: instanceID,

					NotifierID: taskRunData.NotifierID,

					Canceled: false,

					RunBy: taskRunData.TaskRunBy,

					Fetcher: nil,
				},

				appConfig: appConfig,

				clientID:     taskData.ClientID,
				clientSecret: taskData.ClientSecret,
			}

			retryDelay, err := time.ParseDuration(appConfig.HTTPRetry.RetryDelay)
			if err != nil {
				retryDelay, _ = time.ParseDuration(config.DefaultRetryDelay)
			}
			tTask.Fetcher = task.NewRetryFetcher(&task.HTTPFetcher{}, appConfig.HTTPRetry.MaxRetries, retryDelay)

			tTask.RunFn = func(taskResultData interface{}, messageTypeHTML bool) (string, interface{}, error) {
				// 'WatchPrice_'로 시작되는 명령인지 확인한다.
				if strings.HasPrefix(string(tTask.GetCommandID()), naverShoppingWatchPriceTaskCommandIDPrefix) == true {
					for _, t := range tTask.appConfig.Tasks {
						if tTask.GetID() == task.TaskID(t.ID) {
							for _, c := range t.Commands {
								if tTask.GetCommandID() == task.TaskCommandID(c.ID) {
									taskCommandData := &naverShoppingWatchPriceTaskCommandData{}
									if err := task.FillTaskCommandDataFromMap(taskCommandData, c.Data); err != nil {
										return "", nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
									}
									if err := taskCommandData.validate(); err != nil {
										return "", nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
									}

									return tTask.runWatchPrice(taskCommandData, taskResultData, messageTypeHTML)
								}
							}
							break
						}
					}
				}

				return "", nil, task.ErrNoImplementationForTaskCommand
			}

			return tTask, nil
		},
	})
}

type naverShoppingTask struct {
	task.Task

	appConfig *config.AppConfig

	clientID     string
	clientSecret string
}

// noinspection GoUnhandledErrorResult
func (t *naverShoppingTask) runWatchPrice(taskCommandData *naverShoppingWatchPriceTaskCommandData, taskResultData interface{}, messageTypeHTML bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*naverShoppingWatchPriceResultData)
	if ok == false {
		return "", nil, apperrors.New(apperrors.ErrInternal, fmt.Sprintf("TaskResultData의 타입 변환이 실패하였습니다 (expected: *naverShoppingWatchPriceResultData, got: %T)", taskResultData))
	}

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

		searchResultData = &naverShoppingWatchPriceSearchResultData{}
	)
	for searchResultItemStartNo < searchResultItemTotalCount {
		var _searchResultData_ = &naverShoppingWatchPriceSearchResultData{}
		err = task.UnmarshalFromResponseJSONData(t.Fetcher, "GET", fmt.Sprintf("%s?query=%s&display=100&start=%d&sort=sim", naverShoppingSearchURL, url.QueryEscape(taskCommandData.Query), searchResultItemStartNo), header, nil, _searchResultData_)
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
	actualityTaskResultData := &naverShoppingWatchPriceResultData{}
	includedKeywords := strutils.SplitAndTrim(taskCommandData.Filters.IncludedKeywords, ",")
	excludedKeywords := strutils.SplitAndTrim(taskCommandData.Filters.ExcludedKeywords, ",")

	var lowPrice int
	for _, item := range searchResultData.Items {
		if task.Filter(item.Title, includedKeywords, excludedKeywords) == false {
			goto NEXTITEM
		}

		lowPrice, _ = strconv.Atoi(item.LowPrice)
		if lowPrice > 0 && lowPrice < taskCommandData.Filters.PriceLessThan {
			actualityTaskResultData.Products = append(actualityTaskResultData.Products, &naverShoppingProduct{
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
	if messageTypeHTML == true {
		lineSpacing = "\n"
	}
	err = task.EachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Products, originTaskResultData.Products, func(selem, telem interface{}) (bool, error) {
		actualityProduct, ok1 := selem.(*naverShoppingProduct)
		originProduct, ok2 := telem.(*naverShoppingProduct)
		if ok1 == false || ok2 == false {
			return false, apperrors.New(apperrors.ErrInternal, "selem/telem의 타입 변환이 실패하였습니다")
		} else {
			if actualityProduct.Link == originProduct.Link {
				return true, nil
			}
		}
		return false, nil
	}, func(selem, telem interface{}) {
		actualityProduct := selem.(*naverShoppingProduct)
		originProduct := telem.(*naverShoppingProduct)

		if actualityProduct.LowPrice != originProduct.LowPrice {
			if m != "" {
				m += lineSpacing
			}
			m += originProduct.String(messageTypeHTML, fmt.Sprintf(" ⇒ %s원 🔁", strutils.FormatCommas(actualityProduct.LowPrice)))
		}
	}, func(selem interface{}) {
		actualityProduct := selem.(*naverShoppingProduct)

		if m != "" {
			m += lineSpacing
		}
		m += actualityProduct.String(messageTypeHTML, " 🆕")
	})
	if err != nil {
		return "", nil, err
	}

	filtersDescription := fmt.Sprintf("조회 조건은 아래와 같습니다:\n• 검색 키워드 : %s\n• 상풍명 포함 키워드 : %s\n• 상품명 제외 키워드 : %s\n• %s원 미만의 상품", taskCommandData.Query, taskCommandData.Filters.IncludedKeywords, taskCommandData.Filters.ExcludedKeywords, strutils.FormatCommas(taskCommandData.Filters.PriceLessThan))

	if m != "" {
		message = fmt.Sprintf("조회 조건에 해당되는 상품의 정보가 변경되었습니다.\n\n%s\n\n%s", filtersDescription, m)
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.RunBy == task.TaskRunByUser {
			if len(actualityTaskResultData.Products) == 0 {
				message = fmt.Sprintf("조회 조건에 해당되는 상품이 존재하지 않습니다.\n\n%s", filtersDescription)
			} else {
				for _, actualityProduct := range actualityTaskResultData.Products {
					if m != "" {
						m += lineSpacing
					}
					m += actualityProduct.String(messageTypeHTML, "")
				}

				message = fmt.Sprintf("조회 조건에 해당되는 상품의 변경된 정보가 없습니다.\n\n%s\n\n조회 조건에 해당되는 상품은 아래와 같습니다:\n\n%s", filtersDescription, m)
			}
		}
	}

	return message, changedTaskResultData, nil
}
