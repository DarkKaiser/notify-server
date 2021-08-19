package task

import (
	"errors"
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/utils"
	log "github.com/sirupsen/logrus"
	"net/url"
	"strconv"
	"strings"
)

const (
	naverShoppingWatchPriceTaskCommandIDPrefix string = "WatchPrice_"

	// TaskID
	TidNaverShopping TaskID = "NS" // 네이버쇼핑(https://shopping.naver.com/)

	// TaskCommandID
	TcidNaverShoppingWatchPriceAny = TaskCommandID(naverShoppingWatchPriceTaskCommandIDPrefix + taskCommandIDAnyString) // 네이버쇼핑 가격 확인

	// 네이버쇼핑 검색 URL
	naverShoppingSearchUrl = "https://openapi.naver.com/v1/search/shop.json"
)

type naverShoppingSearchResultData struct {
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

type naverShoppingTaskData struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func (d *naverShoppingTaskData) validate() error {
	if d.ClientID == "" {
		return errors.New("client_id가 입력되지 않았습니다")
	}
	if d.ClientSecret == "" {
		return errors.New("client_secret이 입력되지 않았습니다")
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
		return errors.New("query가 입력되지 않았습니다")
	}
	if d.Filters.PriceLessThan <= 0 {
		return errors.New("price_less_than에 0 이하의 값이 입력되었습니다")
	}
	return nil
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
		return fmt.Sprintf("☞ <a href=\"%s\"><b>%s</b></a> %s원%s", p.Link, p.Title, utils.FormatCommas(p.LowPrice), mark)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s %s원%s\n%s", p.Title, utils.FormatCommas(p.LowPrice), mark, p.Link))
}

type naverShoppingWatchPriceResultData struct {
	Products []*naverShoppingProduct `json:"products"`
}

func init() {
	supportedTasks[TidNaverShopping] = &supportedTaskConfig{
		commandConfigs: []*supportedTaskCommandConfig{{
			taskCommandID: TcidNaverShoppingWatchPriceAny,

			allowMultipleInstances: true,

			newTaskResultDataFn: func() interface{} { return &naverShoppingWatchPriceResultData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData, config *g.AppConfig) (taskHandler, error) {
			if taskRunData.taskID != TidNaverShopping {
				return nil, errors.New("등록되지 않은 작업입니다.😱")
			}

			taskData := &naverShoppingTaskData{}
			for _, t := range config.Tasks {
				if taskRunData.taskID == TaskID(t.ID) {
					if err := fillTaskDataFromMap(taskData, t.Data); err != nil {
						return nil, errors.New(fmt.Sprintf("작업 데이터가 유효하지 않습니다.(error:%s)", err))
					}
					break
				}
			}
			if err := taskData.validate(); err != nil {
				return nil, errors.New(fmt.Sprintf("작업 데이터가 유효하지 않습니다.(error:%s)", err))
			}

			task := &naverShoppingTask{
				task: task{
					id:         taskRunData.taskID,
					commandID:  taskRunData.taskCommandID,
					instanceID: instanceID,

					notifierID: taskRunData.notifierID,

					canceled: false,

					runBy: taskRunData.taskRunBy,
				},

				config: config,

				clientID:     taskData.ClientID,
				clientSecret: taskData.ClientSecret,
			}

			task.runFn = func(taskResultData interface{}, messageTypeHTML bool) (string, interface{}, error) {
				// 'WatchPrice_'로 시작되는 명령인지 확인한다.
				if strings.HasPrefix(string(task.CommandID()), naverShoppingWatchPriceTaskCommandIDPrefix) == true {
					for _, t := range task.config.Tasks {
						if task.ID() == TaskID(t.ID) {
							for _, c := range t.Commands {
								if task.CommandID() == TaskCommandID(c.ID) {
									taskCommandData := &naverShoppingWatchPriceTaskCommandData{}
									if err := fillTaskCommandDataFromMap(taskCommandData, c.Data); err != nil {
										return "", nil, errors.New(fmt.Sprintf("작업 커맨드 데이터가 유효하지 않습니다.(error:%s)", err))
									}
									if err := taskCommandData.validate(); err != nil {
										return "", nil, errors.New(fmt.Sprintf("작업 커맨드 데이터가 유효하지 않습니다.(error:%s)", err))
									}

									return task.runWatchPrice(taskCommandData, taskResultData, messageTypeHTML)
								}
							}
							break
						}
					}
				}

				return "", nil, ErrNoImplementationForTaskCommand
			}

			return task, nil
		},
	}
}

type naverShoppingTask struct {
	task

	config *g.AppConfig

	clientID     string
	clientSecret string
}

//noinspection GoUnhandledErrorResult
func (t *naverShoppingTask) runWatchPrice(taskCommandData *naverShoppingWatchPriceTaskCommandData, taskResultData interface{}, messageTypeHTML bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*naverShoppingWatchPriceResultData)
	if ok == false {
		log.Panic("TaskResultData의 타입 변환이 실패하였습니다.")
	}

	//
	// 상품에 대한 정보를 검색한다.
	//
	var header = map[string]string{
		"X-Naver-Client-Id":     t.clientID,
		"X-Naver-Client-Secret": t.clientSecret,
	}
	var searchResultData = &naverShoppingSearchResultData{}
	err = unmarshalFromResponseJSONData("GET", fmt.Sprintf("%s?query=%s&display=100&start=1&sort=sim", naverShoppingSearchUrl, url.QueryEscape(taskCommandData.Query)), header, nil, searchResultData)
	if err != nil {
		return "", nil, err
	}

	//
	// 검색된 상품 목록을 설정된 조건에 맞게 필터링한다.
	//
	actualityTaskResultData := &naverShoppingWatchPriceResultData{}
	includedKeywords := utils.SplitExceptEmptyItems(taskCommandData.Filters.IncludedKeywords, ",")
	excludedKeywords := utils.SplitExceptEmptyItems(taskCommandData.Filters.ExcludedKeywords, ",")

	var lowPrice int
	for _, item := range searchResultData.Items {
		if filter(item.Title, includedKeywords, excludedKeywords) == false {
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
	err = eachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Products, originTaskResultData.Products, func(selem, telem interface{}) (bool, error) {
		actualityProduct, ok1 := selem.(*naverShoppingProduct)
		originProduct, ok2 := telem.(*naverShoppingProduct)
		if ok1 == false || ok2 == false {
			return false, errors.New("selem/telem의 타입 변환이 실패하였습니다.")
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
			m += originProduct.String(messageTypeHTML, fmt.Sprintf(" ⇒ %s원 🔁", utils.FormatCommas(actualityProduct.LowPrice)))
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

	filtersDescription := fmt.Sprintf("조회 조건은 아래와 같습니다:\n• 검색 키워드 : %s\n• 상풍명 포함 키워드 : %s\n• 상품명 제외 키워드 : %s\n• %s원 미만의 상품", taskCommandData.Query, taskCommandData.Filters.IncludedKeywords, taskCommandData.Filters.ExcludedKeywords, utils.FormatCommas(taskCommandData.Filters.PriceLessThan))

	if m != "" {
		message = fmt.Sprintf("조회 조건에 해당되는 상품의 정보가 변경되었습니다.\n\n%s\n\n%s", filtersDescription, m)
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
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
