package task

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
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

type alganicmallWatchNewEventsData struct {
	Events []struct {
		Name string `json:"name"`
		Url  string `json:"url"`
	} `json:"events"`
}

type alganicmallWatchAtoCreamData struct {
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

			allowMultipleIntances: true,

			newTaskDataFn: func() interface{} { return &alganicmallWatchNewEventsData{} },
		}, {
			taskCommandID: TcidAlganicMallWatchAtoCream,

			allowMultipleIntances: true,

			newTaskDataFn: func() interface{} { return &alganicmallWatchAtoCreamData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData) taskHandler {
			if taskRunData.taskID != TidAlganicMall {
				return nil
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

			task.runFn = func(taskData interface{}) (string, interface{}, error) {
				switch task.CommandID() {
				case TcidAlganicMallWatchNewEvents:
					return task.runWatchNewEvents(taskData)

				case TcidAlganicMallWatchAtoCream:
					return task.runWatchAtoCream(taskData)
				}

				return "", nil, ErrNoImplementationForTaskCommand
			}

			return task
		},
	}
}

type alganicMallTask struct {
	task
}

func (t *alganicMallTask) runWatchNewEvents(taskData interface{}) (message string, changedTaskData interface{}, err error) {
	originTaskData, ok := taskData.(*alganicmallWatchNewEventsData)
	if ok == false {
		log.Panic("TaskData의 타입 변환이 실패하였습니다.")
	}

	// 이벤트 페이지를 읽어온다.
	document, err := httpWebPageDocument(fmt.Sprintf("%sboard/board.html?code=alganic_image1", alganicmallBaseUrl))
	if err != nil {
		return "", nil, err
	}

	// @@@@@ css가 바뀌어도 알수가 없음
	// 읽어온 이벤트 페이지에서 이벤트 정보를 추출한다.
	euckrDecoder := korean.EUCKR.NewDecoder()
	actualityTaskData := &alganicmallWatchNewEventsData{}
	document.Find("#bl_table #bl_list td.bl_subject > a").EachWithBreak(func(i int, s *goquery.Selection) bool {
		name, err0 := euckrDecoder.String(s.Text())
		if err0 != nil {
			err = errors.New(fmt.Sprintf("이벤트 이름의 문자열 변환(EUC-KR to UTF-8)이 실패하였습니다. (error:%s)", err0))
			return false
		}

		url, exists := s.Attr("href")
		if exists == false {
			err = errors.New(fmt.Sprint("이벤트 URL 추출이 실패하였습니다."))
			return false
		}

		actualityTaskData.Events = append(actualityTaskData.Events, struct {
			Name string `json:"name"`
			Url  string `json:"url"`
		}{
			Name: utils.CleanString(name),
			Url:  fmt.Sprintf("%sboard/%s", alganicmallBaseUrl, url),
		})

		return true
	})
	if err != nil {
		return "", nil, err
	}

	// 신규 이벤트 정보를 확인한다.
	m := ""
	existsNewEvents := false
	for _, actualityEvent := range actualityTaskData.Events {
		existsOriginEvent := false
		for _, originEvent := range originTaskData.Events {
			if actualityEvent.Name == originEvent.Name && actualityEvent.Url == originEvent.Url {
				existsOriginEvent = true
				break
			}
		}

		if existsOriginEvent == false {
			existsNewEvents = true

			if len(m) > 0 {
				m = fmt.Sprintf("%s\n\n☞ %s\n%s", m, actualityEvent.Name, actualityEvent.Url)
			} else {
				m = fmt.Sprintf("%s☞ %s\n%s", m, actualityEvent.Name, actualityEvent.Url)
			}
		}
	}

	if existsNewEvents == true {
		message = fmt.Sprintf("신규 이벤트가 발생하였습니다.\n\n%s", m)
		changedTaskData = actualityTaskData
	} else {
		if t.runBy == TaskRunByUser {
			message = "신규 이벤트가 없습니다.\n\n현재 진행중인 이벤트는 다음과 같습니다:"
			for _, actualityEvent := range actualityTaskData.Events {
				message = fmt.Sprintf("%s\n\n☞ %s\n%s", message, actualityEvent.Name, actualityEvent.Url)
			}
		}
	}

	if t.IsCanceled() == true {
		return "", nil, nil
	}

	return message, changedTaskData, nil
}

func (t *alganicMallTask) runWatchAtoCream(taskData interface{}) (message string, changedTaskData interface{}, err error) {
	originTaskData, ok := taskData.(*alganicmallWatchAtoCreamData)
	if ok == false {
		log.Panic("TaskData의 타입 변환이 실패하였습니다.")
	}

	// 제품 페이지를 읽어온다.
	document, err := httpWebPageDocument(fmt.Sprintf("%sshop/shopbrand.html?xcode=005&type=X&mcode=002", alganicmallBaseUrl))
	if err != nil {
		return "", nil, err
	}

	// @@@@@ css가 바뀌어도 알수가 없음
	// 읽어온 제품 페이지에서 제품 정보를 추출한다.
	euckrDecoder := korean.EUCKR.NewDecoder()
	actualityTaskData := &alganicmallWatchAtoCreamData{}
	document.Find("table.product_table").EachWithBreak(func(i int, s *goquery.Selection) bool {
		productSelection := s.Find("td")
		// @@@@@ 8개가 아닌것도 있음
		if productSelection.Length() != 8 {
			err = errors.New(fmt.Sprintf("제품의 <TD> 태그의 갯수(%d)가 유효하지 않습니다. CSS셀렉터를 확인하세요.", productSelection.Length()))
			return false
		}

		name, err0 := euckrDecoder.String(productSelection.Eq(3 /* 제품명 */).Text())
		if err0 != nil {
			err = errors.New(fmt.Sprintf("제품 이름의 문자열 변환(EUC-KR to UTF-8)이 실패하였습니다. (error:%s)", err0))
			return false
		}
		if strings.Contains(name, "아토크림") == false {
			return true
		}

		productLinkSelection := productSelection.Eq(3 /* 제품명 */).Find("a")
		if productLinkSelection.Length() != 1 {
			err = errors.New(fmt.Sprintf("제품 이름의 <A> 태그의 갯수(%d)가 유효하지 않습니다. CSS셀렉터를 확인하세요.", productLinkSelection.Length()))
			return false
		}
		url, exists := productLinkSelection.Attr("href")
		if exists == false {
			err = errors.New(fmt.Sprint("제품 URL 추출이 실패하였습니다."))
			return false
		}

		priceString, err0 := euckrDecoder.String(productSelection.Eq(6 /* 제품가격 */).Text())
		if err0 != nil {
			err = errors.New(fmt.Sprintf("제품 가격의 문자열 변환(EUC-KR to UTF-8)이 실패하였습니다. (error:%s)", err0))
			return false
		}
		priceString = utils.CleanString(strings.ReplaceAll(strings.ReplaceAll(priceString, ",", ""), "원", ""))
		price, err0 := strconv.Atoi(priceString)
		if err0 != nil {
			err = errors.New(fmt.Sprintf("제품 가격의 숫자 변환이 실패하였습니다. (error:%s)", err0))
			return false
		}

		actualityTaskData.Products = append(actualityTaskData.Products, struct {
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

	// @@@@@
	// 신규 이벤트 정보를 확인한다.
	m := ""
	existsNewProducts := false
	for _, actualityProduct := range actualityTaskData.Products {
		existsOriginProduct := false
		for _, originProduct := range originTaskData.Products {
			if actualityProduct.Name == originProduct.Name && actualityProduct.Price == originProduct.Price && actualityProduct.Url == originProduct.Url {
				existsOriginProduct = true
				break
			}
		}

		if existsOriginProduct == false {
			existsNewProducts = true

			// @@@@@ 가격만 변경된것은 표현해줘야 됨
			if len(m) > 0 {
				m = fmt.Sprintf("%s\n\n☞ %s\n%s", m, actualityProduct.Name, actualityProduct.Url)
			} else {
				m = fmt.Sprintf("%s☞ %s\n%s", m, actualityProduct.Name, actualityProduct.Url)
			}
		}
	}

	// @@@@@
	if existsNewProducts == true {
		message = fmt.Sprintf("신규 이벤트가 발생하였습니다.\n\n%s", m)
		changedTaskData = actualityTaskData
	} else {
		if t.runBy == TaskRunByUser {
			message = "신규 이벤트가 없습니다.\n\n현재 진행중인 이벤트는 다음과 같습니다:"
			for _, actualityEvent := range actualityTaskData.Products {
				message = fmt.Sprintf("%s\n\n☞ %s\n%s", message, actualityEvent.Name, actualityEvent.Url)
			}
		}
	}

	if t.IsCanceled() == true {
		return "", nil, nil
	}

	return message, changedTaskData, nil
}
