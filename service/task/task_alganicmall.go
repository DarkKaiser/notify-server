package task

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/utils"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
	"net/http"
)

const (
	// TaskID
	TidAlganicMall TaskID = "ALGANICMALL" // 엘가닉몰(http://www.alganicmall.com/)

	// TaskCommandID
	TcidAlganicMallWatchNewEvents TaskCommandID = "WatchNewEvents" // 엘가닉몰 신규 이벤트 확인
	TcidAlganicMallWatchAtoCream  TaskCommandID = "WatchAtoCream"  // 엘가닉몰 아토크림 정보 변경 확인
)

type alganicmallWatchNewEventsData struct {
	// @@@@@
	Events []struct {
		Title string `json:"title"`
		Link  string `json:"link"`
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

			newTaskData: func() interface{} { return &alganicmallWatchNewEventsData{} },
		}, {
			taskCommandID: TcidAlganicMallWatchAtoCream,

			allowMultipleIntances: true,

			newTaskData: func() interface{} { return &alganicmallWatchAtoCreamData{} },
		}},

		newTask: func(instanceID TaskInstanceID, taskRunData *taskRunData) taskHandler {
			if taskRunData.taskID != TidAlganicMall {
				return nil
			}

			task := &alganicMallTask{
				task: task{
					id:         taskRunData.taskID,
					commandID:  taskRunData.taskCommandID,
					instanceID: instanceID,

					notifierID: taskRunData.notifierID,

					cancel: false,
				},
			}

			task.run = func(taskData interface{}, taskNotificationSender TaskNotificationSender, taskCtx context.Context) (message string, changedTaskData interface{}, err error) {
				switch task.CommandID() {
				case TcidAlganicMallWatchNewEvents:
					message, changedTaskData = task.runWatchNewEvents(taskData, taskNotificationSender, taskCtx)

				case TcidAlganicMallWatchAtoCream:
					message, changedTaskData = task.runWatchAtoCream(taskData, taskNotificationSender, taskCtx)

				default:
					err = errors.New("no find task command")
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

func (t *alganicMallTask) runWatchNewEvents(taskData interface{}, taskNotificationSender TaskNotificationSender, taskCtx context.Context) (message string, changedTaskData interface{}) {
	var orignTaskData, ok = taskData.(*alganicmallWatchNewEventsData)
	println(ok)

	// @@@@@
	in := len(orignTaskData.Events)
	println(in)
	for _, s := range orignTaskData.Events {
		println(s.Title)
	}

	err := t.writeTaskDataToFile(&orignTaskData)
	if err != nil {

	}

	// http://suapapa.github.io/blog//post/handling_cp949_in_go/
	// https://m.blog.naver.com/PostView.nhn?blogId=nersion&logNo=220884742148&proxyReferer=https:%2F%2Fwww.google.com%2F

	clPageUrl := fmt.Sprintf("https://www.alganicmall.com/board/board.html?code=alganic_image1")

	res, err := http.Get(clPageUrl)
	utils.CheckErr(err)
	utils.CheckStatusCode(res)

	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	utils.CheckErr(err)

	doc.Find("td.bl_subject > a").Each(func(i int, s *goquery.Selection) {
		//	https://medium.com/@SlackBeck/%EC%9C%84%EC%B1%97-%EB%AF%B8%EB%8B%88%ED%94%84%EB%A1%9C%EA%B7%B8%EB%9E%A8%EC%97%90%EC%84%9C-%EC%9C%84%EC%B1%97-%ED%8E%98%EC%9D%B4-%EC%97%B0%EB%8F%99-%EC%82%BD%EC%A7%88%EA%B8%B0-%EB%B6%80%EC%A0%9C-golang%EC%97%90%EC%84%9C-euc-kr-%EC%84%9C%EB%B2%84%EC%99%80-http-%ED%86%B5%EC%8B%A0%ED%95%98%EA%B8%B0-8dbbeca13c9
		euckrDec := korean.EUCKR.NewDecoder()
		s2, err := euckrDec.String(s.Text())
		if err != nil {
			panic(err)
		}
		println(s2)

		// 인코딩 변환 필요
		var bufs bytes.Buffer
		wr := transform.NewWriter(&bufs, korean.EUCKR.NewDecoder())
		wr.Write([]byte(s.Text()))
		wr.Close()

		convVal := bufs.String()
		println(convVal)

		fmt.Print(s.Text())
	})

	// @@@@@

	//for i := 0; i < 500; i++ {
	//	log.Info("&&&&&&&&&&&&&&&&&&& alganicMallTask running.. ")
	//	time.Sleep(1 * time.Second)
	//
	//	if t.cancel == true {
	//		// 종료처리필요
	//		log.Info("==============================취소==========================================")
	//		break
	//	}
	//}

	if t.cancel == false {
		message = "태스크가 완료되었습니다."

		return message, changedTaskData
	}

	return "", nil
}

func (t *alganicMallTask) runWatchAtoCream(taskData interface{}, taskNotificationSender TaskNotificationSender, taskCtx context.Context) (message string, changedTaskData interface{}) {
	var config = taskData.(*alganicmallWatchAtoCreamData)
	println(config)

	// @@@@@
	fmt.Print("$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$$")
	if t.cancel == true {
		return
	}

	return "", nil
}
