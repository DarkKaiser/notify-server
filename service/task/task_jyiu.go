package task

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/utils"
	log "github.com/sirupsen/logrus"
	"strings"
)

const (
	// TaskID
	TidJyiu TaskID = "JYIU" // 전남여수산학융합원(https://www.jyiu.or.kr/)

	// TaskCommandID
	TcidJyiuWatchNewNotice    TaskCommandID = "WatchNewNotice"    // 전남여수산학융합원 공지사항 새글 확인
	TcidJyiuWatchNewEducation TaskCommandID = "WatchNewEducation" // 전남여수산학융합원 신규 교육프로그램 확인
)

const (
	jyiuBaseUrl = "https://www.jyiu.or.kr/"
)

type jyiuWatchNewNoticeResultData struct {
	Notice []struct {
		Title string `json:"title"`
		Date  string `json:"date"`
		Url   string `json:"url"`
	} `json:"notice"`
}

type jyiuWatchNewEducationResultData struct {
	Education []struct {
		Title            string `json:"title"`
		TrainingPeriod   string `json:"training_period"`
		AcceptancePeriod string `json:"acceptance_period"`
		Url              string `json:"url"`
	} `json:"education"`
}

func init() {
	supportedTasks[TidJyiu] = &supportedTaskConfig{
		commandConfigs: []*supportedTaskCommandConfig{{
			taskCommandID: TcidJyiuWatchNewNotice,

			allowMultipleInstances: true,

			newTaskResultDataFn: func() interface{} { return &jyiuWatchNewNoticeResultData{} },
		}, {
			taskCommandID: TcidJyiuWatchNewEducation,

			allowMultipleInstances: true,

			newTaskResultDataFn: func() interface{} { return &jyiuWatchNewEducationResultData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData, config *g.AppConfig) (taskHandler, error) {
			if taskRunData.taskID != TidJyiu {
				return nil, errors.New("등록되지 않은 작업입니다.😱")
			}

			task := &jyiuTask{
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
				case TcidJyiuWatchNewNotice:
					return task.runWatchNewNotice(taskResultData, isSupportedHTMLMessage)

				case TcidJyiuWatchNewEducation:
					return task.runWatchNewEducation(taskResultData, isSupportedHTMLMessage)
				}

				return "", nil, ErrNoImplementationForTaskCommand
			}

			return task, nil
		},
	}
}

type jyiuTask struct {
	task
}

func (t *jyiuTask) runWatchNewNotice(taskResultData interface{}, isSupportedHTMLMessage bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*jyiuWatchNewNoticeResultData)
	if ok == false {
		log.Panic("TaskResultData의 타입 변환이 실패하였습니다.")
	}

	// 공지사항 페이지를 읽어온다.
	document, err := httpWebPageDocument(fmt.Sprintf("%sgms_005001/", jyiuBaseUrl))
	if err != nil {
		return "", nil, err
	}
	if document.Find("#contents table.bbsList > tbody > tr").Length() <= 0 {
		return "Web 페이지의 구조가 변경되었습니다. CSS셀렉터를 수정하세요.", nil, nil
	}

	// 읽어온 공지사항 페이지에서 이벤트 정보를 추출한다.
	actualityTaskResultData := &jyiuWatchNewNoticeResultData{}
	document.Find("#contents table.bbsList > tbody > tr").EachWithBreak(func(i int, s *goquery.Selection) bool {
		// 공지사항 컬럼 개수를 확인한다.
		as := s.Find("td")
		if as.Length() != 5 {
			err = errors.New(fmt.Sprintf("공지사항 데이터 파싱이 실패하였습니다. CSS셀렉터를 확인하세요.(공지사항 컬럼 개수 불일치:%d)", as.Length()))
			return false
		}

		id, exists := as.Eq(1).Find("a").Attr("onclick")
		if exists == false {
			err = errors.New(fmt.Sprint("공지사항 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요."))
			return false
		}
		pos1 := strings.Index(id, "(")
		pos2 := strings.LastIndex(id, ")")
		if pos1 == -1 || pos2 == -1 || pos1 == pos2 {
			err = errors.New(fmt.Sprint("공지사항 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요."))
			return false
		}
		id = id[pos1+1 : pos2]

		actualityTaskResultData.Notice = append(actualityTaskResultData.Notice, struct {
			Title string `json:"title"`
			Date  string `json:"date"`
			Url   string `json:"url"`
		}{
			Title: utils.CleanString(as.Eq(1).Text()),
			Date:  utils.CleanString(as.Eq(3).Text()),
			Url:   fmt.Sprintf("%sgms_005001/view?id=%s", jyiuBaseUrl, id),
		})

		return true
	})
	if err != nil {
		return "", nil, err
	}

	// 공지사항 새로운 글 정보를 확인한다.
	m := ""
	existsNewNotice := false
	for _, actualityNotice := range actualityTaskResultData.Notice {
		isNewNotice := true
		for _, originNotice := range originTaskResultData.Notice {
			if actualityNotice.Title == originNotice.Title && actualityNotice.Date == originNotice.Date && actualityNotice.Url == originNotice.Url {
				isNewNotice = false
				break
			}
		}

		if isNewNotice == true {
			existsNewNotice = true

			if isSupportedHTMLMessage == true {
				if m != "" {
					m += "\n"
				}
				m = fmt.Sprintf("%s☞ <a href=\"%s\"><b>%s</b></a> 🆕", m, actualityNotice.Url, actualityNotice.Title)
			} else {
				if m != "" {
					m += "\n\n"
				}
				m = fmt.Sprintf("%s☞ %s 🆕\n%s", m, actualityNotice.Title, actualityNotice.Url)
			}
		}
	}

	if existsNewNotice == true {
		message = fmt.Sprintf("새로운 공지사항이 등록되었습니다.\n\n%s", m)
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.Notice) == 0 {
				message = "등록된 공지사항이 존재하지 않습니다."
			} else {
				message = "새로 등록된 공지사항이 없습니다.\n\n현재 등록된 공지사항은 아래와 같습니다:"

				if isSupportedHTMLMessage == true {
					message += "\n"
					for _, actualityNotice := range actualityTaskResultData.Notice {
						message = fmt.Sprintf("%s\n☞ <a href=\"%s\"><b>%s</b></a>", message, actualityNotice.Url, actualityNotice.Title)
					}
				} else {
					for _, actualityNotice := range actualityTaskResultData.Notice {
						message = fmt.Sprintf("%s\n\n☞ %s\n%s", message, actualityNotice.Title, actualityNotice.Url)
					}
				}
			}
		}
	}

	return message, changedTaskResultData, nil
}

func (t *jyiuTask) runWatchNewEducation(taskResultData interface{}, isSupportedHTMLMessage bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*jyiuWatchNewEducationResultData)
	if ok == false {
		log.Panic("TaskResultData의 타입 변환이 실패하였습니다.")
	}

	// 교육프로그램 페이지를 읽어온다.
	document, err := httpWebPageDocument(fmt.Sprintf("%sgms_003001/experienceList", jyiuBaseUrl))
	if err != nil {
		return "", nil, err
	}
	if document.Find("div.gms_003001 table.bbsList > tbody > tr").Length() <= 0 {
		return "Web 페이지의 구조가 변경되었습니다. CSS셀렉터를 수정하세요.", nil, nil
	}

	// 읽어온 교육프로그램 페이지에서 이벤트 정보를 추출한다.
	actualityTaskResultData := &jyiuWatchNewEducationResultData{}
	document.Find("div.gms_003001 table.bbsList > tbody > tr").EachWithBreak(func(i int, s *goquery.Selection) bool {
		// 교육프로그램 컬럼 개수를 확인한다.
		as := s.Find("td")
		if as.Length() != 6 {
			err = errors.New(fmt.Sprintf("교육프로그램 데이터 파싱이 실패하였습니다. CSS셀렉터를 확인하세요.(교육프로그램 컬럼 개수 불일치:%d)", as.Length()))
			return false
		}

		url, exists := s.Attr("onclick")
		if exists == false {
			err = errors.New(fmt.Sprint("교육프로그램 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요."))
			return false
		}
		pos1 := strings.Index(url, "'")
		pos2 := strings.LastIndex(url, "'")
		if pos1 == -1 || pos2 == -1 || pos1 == pos2 {
			err = errors.New(fmt.Sprint("교육프로그램 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요."))
			return false
		}
		url = url[pos1+1 : pos2]

		actualityTaskResultData.Education = append(actualityTaskResultData.Education, struct {
			Title            string `json:"title"`
			TrainingPeriod   string `json:"training_period"`
			AcceptancePeriod string `json:"acceptance_period"`
			Url              string `json:"url"`
		}{
			Title:            utils.CleanString(as.Eq(2).Text()),
			TrainingPeriod:   utils.CleanString(as.Eq(4).Text()),
			AcceptancePeriod: utils.CleanString(as.Eq(5).Text()),
			Url:              fmt.Sprintf("%s%s", jyiuBaseUrl, url),
		})

		return true
	})
	if err != nil {
		return "", nil, err
	}

	// 교육프로그램 새로운 글 정보를 확인한다.
	m := ""
	existsNewEducation := false
	for _, actualityEducation := range actualityTaskResultData.Education {
		isNewEducation := true
		for _, originEducation := range originTaskResultData.Education {
			if actualityEducation.Title == originEducation.Title && actualityEducation.TrainingPeriod == originEducation.TrainingPeriod && actualityEducation.AcceptancePeriod == originEducation.AcceptancePeriod && actualityEducation.Url == originEducation.Url {
				isNewEducation = false
				break
			}
		}

		if isNewEducation == true {
			existsNewEducation = true

			if isSupportedHTMLMessage == true {
				if m != "" {
					m += "\n"
				}
				m = fmt.Sprintf("%s☞ <a href=\"%s\"><b>%s</b></a> 🆕", m, actualityEducation.Url, actualityEducation.Title)
			} else {
				if m != "" {
					m += "\n\n"
				}
				m = fmt.Sprintf("%s☞ %s 🆕\n%s", m, actualityEducation.Title, actualityEducation.Url)
			}
		}
	}

	if existsNewEducation == true {
		message = fmt.Sprintf("새로운 교육프로그램이 등록되었습니다.\n\n%s", m)
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.Education) == 0 {
				message = "등록된 교육프로그램이 존재하지 않습니다."
			} else {
				message = "새로 등록된 교육프로그램이 없습니다.\n\n현재 등록된 교육프로그램은 아래와 같습니다:"

				if isSupportedHTMLMessage == true {
					message += "\n"
					for _, actualityEducation := range actualityTaskResultData.Education {
						message = fmt.Sprintf("%s\n☞ <a href=\"%s\"><b>%s</b></a>", message, actualityEducation.Url, actualityEducation.Title)
					}
				} else {
					for _, actualityEducation := range actualityTaskResultData.Education {
						message = fmt.Sprintf("%s\n\n☞ %s\n%s", message, actualityEducation.Title, actualityEducation.Url)
					}
				}
			}
		}
	}

	return message, changedTaskResultData, nil
}
