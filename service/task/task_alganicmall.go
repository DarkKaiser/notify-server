package task

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/korean"
	"net/http"
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
	// @@@@@
	Events []struct {
		Title string `json:"title"`
		Link  string `json:"link"`
	} `json:"events"`
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

			task.runFn = func(taskData interface{}, taskNotificationSender TaskNotificationSender, taskCtx TaskContext) (message string, changedTaskData interface{}, err error) {
				switch task.CommandID() {
				case TcidAlganicMallWatchNewEvents:
					message, changedTaskData = task.runWatchNewEvents(taskData, taskNotificationSender, taskCtx)

				case TcidAlganicMallWatchAtoCream:
					message, changedTaskData = task.runWatchAtoCream(taskData, taskNotificationSender, taskCtx)

				default:
					err = ErrNoImplementationForTaskCommand
				}

				return message, changedTaskData, err
			}

			return task
		},
	}
}

type alganicMallTask struct {
	task
}

func (t *alganicMallTask) runWatchNewEvents(taskData interface{}, taskNotificationSender TaskNotificationSender, taskCtx TaskContext) (message string, changedTaskData interface{}) {
	var orignTaskData, ok = taskData.(*alganicmallWatchNewEventsData)
	if ok == false {
		// @@@@@
	}
	println(orignTaskData)
	var currentTaskData = &alganicmallWatchNewEventsData{}
	println(currentTaskData)

	newEventsPageUrl := fmt.Sprintf("%sboard/board.html?code=alganic_image1", alganicmallBaseUrl)
	res, err := http.Get(newEventsPageUrl)
	if err != nil {
		//log.Fatal(err)
		taskCtx.WithError()
		return
	}
	if res.StatusCode != 200 {
		//log.Fatal("Request failed with Status:", res.StatusCode)
		taskCtx.WithError()
		//t.notifyError(taskNotificationSender, "작업 진행중 오류가 발생하여 작업이 실패하였습니다.\n\n- 작업데이터 생성이 실패하였습니다.", taskCtx)
		return
	}

	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		//log.Fatal(err)
		taskCtx.WithError()
		return
	}

	euckrDec := korean.EUCKR.NewDecoder()
	doc.Find("td.bl_subject > a").Each(func(i int, s *goquery.Selection) {
		attr, _ := s.Attr("href")
		s2, _ := euckrDec.String(s.Text())

		currentTaskData.Events = append(currentTaskData.Events, struct {
			Name string `json:"name"`
			Url  string `json:"url"`
		}{s2, fmt.Sprintf("%sboard/%s", alganicmallBaseUrl, attr)})
	})

	var changed bool
	for _, event := range currentTaskData.Events {
		find := false
		for _, s := range orignTaskData.Events {
			if event.Name == s.Name && event.Url == s.Url {
				find = true
				break
			}
		}
		if find == false {
			message = fmt.Sprintf("%s\n\n%s\n%s", message, event.Name, event.Url)
			changed = true
		}
	}

	if changed == true {
		changedTaskData = &currentTaskData
	}

	if len(message) == 0 {
		message = "신규 이벤트가 없습니다."
	}

	if t.canceled == true {
		return "", nil
	}

	return message, changedTaskData
}

func (t *alganicMallTask) runWatchAtoCream(taskData interface{}, taskNotificationSender TaskNotificationSender, taskCtx TaskContext) (message string, changedTaskData interface{}) {
	//$("table.product_table")
	// 제목 : <font class="brandbrandname"> 아토크림 10개 세트<span class="braddname"></span></font>
	// 가격 : <span class="brandprice"><span class="mk_price">190,000원</span></span>

	var config = taskData.(*alganicmallWatchAtoCreamData)
	println(config)

	// @@@@@
	fmt.Print("$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$")
	if t.canceled == true {
		return
	}

	return "", nil
}
