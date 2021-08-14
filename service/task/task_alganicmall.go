package task

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/encoding/korean"
	"strconv"
	"strings"
)

const (
	// TaskID
	TidAlganicMall TaskID = "ALGANICMALL" // 엘가닉몰(http://www.alganicmall.com/)

	// TaskCommandID
	TcidAlganicMallWatchNewEvents TaskCommandID = "WatchNewEvents" // 엘가닉몰 신규 이벤트 확인
	TcidAlganicMallWatchAtoCream  TaskCommandID = "WatchAtoCream"  // 엘가닉몰 아토크림 정보 변경 확인
)

const (
	alganicmallBaseUrl = "https://www.alganicmall.com/"
)

type alganicmallWatchNewEventsResultData struct {
	Events []struct {
		Name string `json:"name"`
		Url  string `json:"url"`
	} `json:"events"`
}

type alganicmallWatchAtoCreamResultData struct {
	Products []struct {
		Name  string `json:"name"`
		Price int    `json:"price"`
		Url   string `json:"url"`
	} `json:"products"`
}

func init() {
	supportedTasks[TidAlganicMall] = &supportedTaskConfig{
		commandConfigs: []*supportedTaskCommandConfig{{
			taskCommandID: TcidAlganicMallWatchNewEvents,

			allowMultipleInstances: true,

			newTaskResultDataFn: func() interface{} { return &alganicmallWatchNewEventsResultData{} },
		}, {
			taskCommandID: TcidAlganicMallWatchAtoCream,

			allowMultipleInstances: true,

			newTaskResultDataFn: func() interface{} { return &alganicmallWatchAtoCreamResultData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData, config *g.AppConfig) (taskHandler, error) {
			if taskRunData.taskID != TidAlganicMall {
				return nil, errors.New("등록되지 않은 작업입니다.😱")
			}

			task := &alganicMallTask{
				task: task{
					id:         taskRunData.taskID,
					commandID:  taskRunData.taskCommandID,
					instanceID: instanceID,

					notifierID: taskRunData.notifierID,

					canceled: false,

					runBy: taskRunData.taskRunBy,
				},
			}

			task.runFn = func(taskResultData interface{}, isSupportedHTMLMessage bool) (string, interface{}, error) {
				switch task.CommandID() {
				case TcidAlganicMallWatchNewEvents:
					return task.runWatchNewEvents(taskResultData, isSupportedHTMLMessage)

				case TcidAlganicMallWatchAtoCream:
					return task.runWatchAtoCream(taskResultData, isSupportedHTMLMessage)
				}

				return "", nil, ErrNoImplementationForTaskCommand
			}

			return task, nil
		},
	}
}

type alganicMallTask struct {
	task
}

func (t *alganicMallTask) runWatchNewEvents(taskResultData interface{}, isSupportedHTMLMessage bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*alganicmallWatchNewEventsResultData)
	if ok == false {
		log.Panic("TaskResultData의 타입 변환이 실패하였습니다.")
	}

	// 이벤트 페이지를 읽어서 정보를 추출한다.
	var err0 error
	var euckrDecoder = korean.EUCKR.NewDecoder()
	var actualityTaskResultData = &alganicmallWatchNewEventsResultData{}
	err = scrapeHTMLDocument(fmt.Sprintf("%sboard/board.html?code=alganic_image1", alganicmallBaseUrl), "div.bbs-table-list > div.fixed-img-collist > ul > li > a", func(i int, s *goquery.Selection) bool {
		name, _err_ := euckrDecoder.String(s.Text())
		if _err_ != nil {
			err0 = fmt.Errorf("이벤트명의 문자열 변환(EUC-KR to UTF-8)이 실패하였습니다.(error:%s)", _err_)
			return false
		}

		url, exists := s.Attr("href")
		if exists == false {
			err0 = errors.New("이벤트 상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
			return false
		}

		actualityTaskResultData.Events = append(actualityTaskResultData.Events, struct {
			Name string `json:"name"`
			Url  string `json:"url"`
		}{
			Name: utils.CleanString(name),
			Url:  fmt.Sprintf("%s%s", alganicmallBaseUrl, url),
		})

		return true
	})
	if err != nil {
		return "", nil, err
	}
	if err0 != nil {
		return "", nil, err0
	}

	// 신규 이벤트 정보를 확인한다.
	m := ""
	existsNewEvents := false
	for _, actualityEvent := range actualityTaskResultData.Events {
		isNewEvent := true
		for _, originEvent := range originTaskResultData.Events {
			if actualityEvent.Name == originEvent.Name && actualityEvent.Url == originEvent.Url {
				isNewEvent = false
				break
			}
		}

		if isNewEvent == true {
			existsNewEvents = true

			if isSupportedHTMLMessage == true {
				if m != "" {
					m += "\n"
				}
				m = fmt.Sprintf("%s☞ <a href=\"%s\"><b>%s</b></a> 🆕", m, actualityEvent.Url, actualityEvent.Name)
			} else {
				if m != "" {
					m += "\n\n"
				}
				m = fmt.Sprintf("%s☞ %s 🆕\n%s", m, actualityEvent.Name, actualityEvent.Url)
			}
		}
	}

	if existsNewEvents == true {
		message = fmt.Sprintf("신규 이벤트가 발생하였습니다.\n\n%s", m)
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.Events) == 0 {
				message = "등록된 이벤트가 존재하지 않습니다."
			} else {
				message = "신규 이벤트가 없습니다.\n\n현재 진행중인 이벤트는 아래와 같습니다:"

				if isSupportedHTMLMessage == true {
					message += "\n"
					for _, actualityEvent := range actualityTaskResultData.Events {
						message = fmt.Sprintf("%s\n☞ <a href=\"%s\"><b>%s</b></a>", message, actualityEvent.Url, actualityEvent.Name)
					}
				} else {
					for _, actualityEvent := range actualityTaskResultData.Events {
						message = fmt.Sprintf("%s\n\n☞ %s\n%s", message, actualityEvent.Name, actualityEvent.Url)
					}
				}
			}
		}
	}

	return message, changedTaskResultData, nil
}

func (t *alganicMallTask) runWatchAtoCream(taskResultData interface{}, isSupportedHTMLMessage bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*alganicmallWatchAtoCreamResultData)
	if ok == false {
		log.Panic("TaskResultData의 타입 변환이 실패하였습니다.")
	}

	// 제품 페이지를 읽어서 정보를 추출한다.
	var err0 error
	var euckrDecoder = korean.EUCKR.NewDecoder()
	var priceReplacer = strings.NewReplacer(",", "", "원", "")
	var actualityTaskResultData = &alganicmallWatchAtoCreamResultData{}
	err = scrapeHTMLDocument(fmt.Sprintf("%sshop/shopbrand.html?xcode=020&type=Y", alganicmallBaseUrl), "div.item-wrap > div.item-list > dl.item", func(i int, s *goquery.Selection) bool {
		productSelection := s

		// 제품명
		productNameSelection := productSelection.Find("dd > ul > li:first-child > span")
		if productNameSelection.Length() != 1 {
			err0 = errors.New("제품명 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
			return false
		}
		name, _err_ := euckrDecoder.String(productNameSelection.Text())
		if _err_ != nil {
			err0 = fmt.Errorf("제품명의 문자열 변환(EUC-KR to UTF-8)이 실패하였습니다.(error:%s)", _err_)
			return false
		}
		if strings.Contains(name, "아토크림") == false {
			return true
		}

		// 제품URL
		productLinkSelection := productSelection.Find("dt > a")
		if productLinkSelection.Length() != 1 {
			err0 = errors.New("제품 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
			return false
		}
		url, exists := productLinkSelection.Attr("href")
		if exists == false {
			err0 = errors.New("제품 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
			return false
		}
		// 제품URL의 마지막 파라메터 'GfDT'가 수시로 변경되기 때문에 해당 파라메터를 제거한다.
		pos := strings.LastIndex(url, "&")
		if pos != -1 {
			if url[pos+1:pos+6] == "GfDT=" {
				url = url[:pos]
			}
		}

		// 제품가격
		productPriceSelection := productSelection.Find("dd > ul > li > span.price")
		if productPriceSelection.Length() != 1 {
			err0 = errors.New("제품 가격 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
			return false
		}
		productPriceString, _err_ := euckrDecoder.String(productPriceSelection.Text())
		if _err_ != nil {
			err0 = fmt.Errorf("제품 가격의 문자열 변환(EUC-KR to UTF-8)이 실패하였습니다.(error:%s)", _err_)
			return false
		}
		price, _err_ := strconv.Atoi(utils.CleanString(priceReplacer.Replace(productPriceString)))
		if _err_ != nil {
			err0 = fmt.Errorf("제품 가격의 숫자 변환이 실패하였습니다.(error:%s)", _err_)
			return false
		}

		actualityTaskResultData.Products = append(actualityTaskResultData.Products, struct {
			Name  string `json:"name"`
			Price int    `json:"price"`
			Url   string `json:"url"`
		}{
			Name:  utils.CleanString(name),
			Price: price,
			Url:   fmt.Sprintf("%s%s", alganicmallBaseUrl, url),
		})

		return true
	})
	if err != nil {
		return "", nil, err
	}
	if err0 != nil {
		return "", nil, err0
	}

	// 변경된 제품 정보를 확인한다.
	m := ""
	modifiedProducts := false
	for _, actualityProduct := range actualityTaskResultData.Products {
		isNewProduct := true
		for _, originProduct := range originTaskResultData.Products {
			if actualityProduct.Name == originProduct.Name && actualityProduct.Url == originProduct.Url {
				isNewProduct = false

				// 동일한 제품인데 가격이 변경되었는지 확인한다.
				if actualityProduct.Price != originProduct.Price {
					modifiedProducts = true

					if isSupportedHTMLMessage == true {
						if m != "" {
							m += "\n"
						}
						m = fmt.Sprintf("%s☞ <a href=\"%s\"><b>%s</b></a> %s원 ⇒ %s원 🔁", m, actualityProduct.Url, actualityProduct.Name, utils.FormatCommas(originProduct.Price), utils.FormatCommas(actualityProduct.Price))
					} else {
						if m != "" {
							m += "\n\n"
						}
						m = fmt.Sprintf("%s☞ %s %s원 ⇒ %s원 🔁\n%s", m, actualityProduct.Name, utils.FormatCommas(originProduct.Price), utils.FormatCommas(actualityProduct.Price), actualityProduct.Url)
					}
				}

				break
			}
		}

		if isNewProduct == true {
			modifiedProducts = true

			if isSupportedHTMLMessage == true {
				if m != "" {
					m += "\n"
				}
				m = fmt.Sprintf("%s☞ <a href=\"%s\"><b>%s</b></a> %s원 🆕", m, actualityProduct.Url, actualityProduct.Name, utils.FormatCommas(actualityProduct.Price))
			} else {
				if m != "" {
					m += "\n\n"
				}
				m = fmt.Sprintf("%s☞ %s %s원 🆕\n%s", m, actualityProduct.Name, utils.FormatCommas(actualityProduct.Price), actualityProduct.Url)
			}
		}
	}

	if modifiedProducts == true {
		message = fmt.Sprintf("아토크림에 대한 정보가 변경되었습니다.\n\n%s", m)
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.Products) == 0 {
				message = "아토크림에 대한 정보가 존재하지 않습니다."
			} else {
				message = "아토크림에 대한 변경된 정보가 없습니다.\n\n현재 아토크림에 대한 정보는 아래와 같습니다:"

				if isSupportedHTMLMessage == true {
					message += "\n"
					for _, actualityProduct := range actualityTaskResultData.Products {
						message = fmt.Sprintf("%s\n☞ <a href=\"%s\"><b>%s</b></a> %s원", message, actualityProduct.Url, actualityProduct.Name, utils.FormatCommas(actualityProduct.Price))
					}
				} else {
					for _, actualityProduct := range actualityTaskResultData.Products {
						message = fmt.Sprintf("%s\n\n☞ %s %s원\n%s", message, actualityProduct.Name, utils.FormatCommas(actualityProduct.Price), actualityProduct.Url)
					}
				}
			}
		}
	}

	return message, changedTaskResultData, nil
}
