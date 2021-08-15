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

type jyiuNotice struct {
	Title string `json:"title"`
	Date  string `json:"date"`
	Url   string `json:"url"`
}

type jyiuWatchNewNoticeResultData struct {
	Notices []*jyiuNotice `json:"notices"`
}

type jyiuEducation struct {
	Title            string `json:"title"`
	TrainingPeriod   string `json:"training_period"`
	AcceptancePeriod string `json:"acceptance_period"`
	Url              string `json:"url"`
}

type jyiuWatchNewEducationResultData struct {
	Educations []*jyiuEducation `json:"educations"`
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

	// 공지사항 페이지를 읽어서 정보를 추출한다.
	var err0 error
	var actualityTaskResultData = &jyiuWatchNewNoticeResultData{}
	err = scrapeHTMLDocument(fmt.Sprintf("%sgms_005001/", jyiuBaseUrl), "#contents table.bbsList > tbody > tr", func(i int, s *goquery.Selection) bool {
		// 공지사항 컬럼 개수를 확인한다.
		as := s.Find("td")
		if as.Length() != 5 {
			err0 = fmt.Errorf("불러온 페이지의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요.(컬럼 개수 불일치:%d)", as.Length())
			return false
		}

		id, exists := as.Eq(1).Find("a").Attr("onclick")
		if exists == false {
			err0 = errors.New("상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
			return false
		}
		pos1 := strings.Index(id, "(")
		pos2 := strings.LastIndex(id, ")")
		if pos1 == -1 || pos2 == -1 || pos1 == pos2 {
			err0 = errors.New("상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
			return false
		}
		id = id[pos1+1 : pos2]

		actualityTaskResultData.Notices = append(actualityTaskResultData.Notices, &jyiuNotice{
			Title: utils.CleanString(as.Eq(1).Find("a").Text()),
			Date:  utils.CleanString(as.Eq(3).Text()),
			Url:   fmt.Sprintf("%sgms_005001/view?id=%s", jyiuBaseUrl, id),
		})

		return true
	})
	if err != nil {
		return "", nil, err
	}
	if err0 != nil {
		return "", nil, err0
	}

	// 신규로 등록된 공지사항이 존재하는지 확인한다.
	m := ""
	existsNewNotice := false
	for _, actualityNotice := range actualityTaskResultData.Notices {
		isNewNotice := true
		for _, originNotice := range originTaskResultData.Notices {
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
		message = fmt.Sprintf("새 공지사항이 등록되었습니다.\n\n%s", m)
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.Notices) == 0 {
				message = "등록된 공지사항이 존재하지 않습니다."
			} else {
				message = "신규로 등록된 공지사항이 없습니다.\n\n현재 등록된 공지사항은 아래와 같습니다:"

				if isSupportedHTMLMessage == true {
					message += "\n"
					for _, actualityNotice := range actualityTaskResultData.Notices {
						message = fmt.Sprintf("%s\n☞ <a href=\"%s\"><b>%s</b></a>", message, actualityNotice.Url, actualityNotice.Title)
					}
				} else {
					for _, actualityNotice := range actualityTaskResultData.Notices {
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

	// 교육프로그램 페이지를 읽어서 정보를 추출한다.
	var err0 error
	var actualityTaskResultData = &jyiuWatchNewEducationResultData{}
	err = scrapeHTMLDocument(fmt.Sprintf("%sgms_003001/experienceList", jyiuBaseUrl), "div.gms_003001 table.bbsList > tbody > tr", func(i int, s *goquery.Selection) bool {
		// 교육프로그램 컬럼 개수를 확인한다.
		as := s.Find("td")
		if as.Length() != 6 {
			err0 = fmt.Errorf("불러온 페이지의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요.(컬럼 개수 불일치:%d)", as.Length())
			return false
		}

		url, exists := s.Attr("onclick")
		if exists == false {
			err0 = errors.New("상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
			return false
		}
		pos1 := strings.Index(url, "'")
		pos2 := strings.LastIndex(url, "'")
		if pos1 == -1 || pos2 == -1 || pos1 == pos2 {
			err0 = errors.New("상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
			return false
		}
		url = url[pos1+1 : pos2]

		actualityTaskResultData.Educations = append(actualityTaskResultData.Educations, &jyiuEducation{
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
	if err0 != nil {
		return "", nil, err0
	}

	// 교육프로그램 새로운 글 정보를 확인한다.
	m := ""
	existsNewEducation := false
	for _, actualityEducation := range actualityTaskResultData.Educations {
		isNewEducation := true
		for _, originEducation := range originTaskResultData.Educations {
			if actualityEducation.Title == originEducation.Title && actualityEducation.TrainingPeriod == originEducation.TrainingPeriod && actualityEducation.AcceptancePeriod == originEducation.AcceptancePeriod && actualityEducation.Url == originEducation.Url {
				isNewEducation = false
				break
			}
		}

		if isNewEducation == true {
			existsNewEducation = true

			if isSupportedHTMLMessage == true {
				if m != "" {
					m += "\n\n"
				}
				m = fmt.Sprintf("%s☞ <a href=\"%s\"><b>%s</b></a> 🆕\n      • 교육기간 : %s\n      • 접수기간 : %s", m, actualityEducation.Url, actualityEducation.Title, actualityEducation.TrainingPeriod, actualityEducation.AcceptancePeriod)
			} else {
				if m != "" {
					m += "\n\n"
				}
				m = fmt.Sprintf("%s☞ %s 🆕\n%s", m, actualityEducation.Title, actualityEducation.Url)
			}
		}
	}

	if existsNewEducation == true {
		message = fmt.Sprintf("새 교육프로그램이 등록되었습니다.\n\n%s", m)
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.Educations) == 0 {
				message = "등록된 교육프로그램이 존재하지 않습니다."
			} else {
				message = "신규로 등록된 교육프로그램이 없습니다.\n\n현재 등록된 교육프로그램은 아래와 같습니다:"

				if isSupportedHTMLMessage == true {
					for _, actualityEducation := range actualityTaskResultData.Educations {
						message = fmt.Sprintf("%s\n\n☞ <a href=\"%s\"><b>%s</b></a>\n      • 교육기간 : %s\n      • 접수기간 : %s", message, actualityEducation.Url, actualityEducation.Title, actualityEducation.TrainingPeriod, actualityEducation.AcceptancePeriod)
					}
				} else {
					for _, actualityEducation := range actualityTaskResultData.Educations {
						message = fmt.Sprintf("%s\n\n☞ %s\n%s", message, actualityEducation.Title, actualityEducation.Url)
					}
				}
			}
		}
	}

	return message, changedTaskResultData, nil
}
