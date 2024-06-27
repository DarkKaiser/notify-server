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

func (n *jyiuNotice) String(messageTypeHTML bool, mark string) string {
	if messageTypeHTML == true {
		return fmt.Sprintf("☞ <a href=\"%s\"><b>%s</b></a>%s", n.Url, n.Title, mark)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s%s\n%s", n.Title, mark, n.Url))
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

func (e *jyiuEducation) String(messageTypeHTML bool, mark string) string {
	if messageTypeHTML == true {
		return fmt.Sprintf("☞ <a href=\"%s\"><b>%s</b></a>%s\n      • 교육기간 : %s\n      • 접수기간 : %s", e.Url, e.Title, mark, e.TrainingPeriod, e.AcceptancePeriod)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s%s\n%s", e.Title, mark, e.Url))
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

			task.runFn = func(taskResultData interface{}, messageTypeHTML bool) (string, interface{}, error) {
				switch task.CommandID() {
				case TcidJyiuWatchNewNotice:
					return task.runWatchNewNotice(taskResultData, messageTypeHTML)

				case TcidJyiuWatchNewEducation:
					return task.runWatchNewEducation(taskResultData, messageTypeHTML)
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

func (t *jyiuTask) runWatchNewNotice(taskResultData interface{}, messageTypeHTML bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*jyiuWatchNewNoticeResultData)
	if ok == false {
		log.Panic("TaskResultData의 타입 변환이 실패하였습니다.")
	}

	// 공지사항 페이지를 읽어서 정보를 추출한다.
	var err0 error
	var actualityTaskResultData = &jyiuWatchNewNoticeResultData{}
	err = webScrape(fmt.Sprintf("%sgms_005001/", jyiuBaseUrl), "#contents table.bbsList > tbody > tr", func(i int, s *goquery.Selection) bool {
		// 공지사항 컬럼 개수를 확인한다.
		as := s.Find("td")
		if as.Length() != 5 {
			err0 = fmt.Errorf("불러온 페이지의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요.(컬럼 개수 불일치:%d)", as.Length())
			return false
		}

		id, exists := as.Eq(1).Find("a").Attr("onclick")
		if exists == false {
			err0 = errors.New("상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		pos1 := strings.Index(id, "(")
		pos2 := strings.LastIndex(id, ")")
		if pos1 == -1 || pos2 == -1 || pos1 == pos2 {
			err0 = errors.New("상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		id = id[pos1+1 : pos2]

		actualityTaskResultData.Notices = append(actualityTaskResultData.Notices, &jyiuNotice{
			Title: utils.Trim(as.Eq(1).Find("a").Text()),
			Date:  utils.Trim(as.Eq(3).Text()),
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
	lineSpacing := "\n\n"
	if messageTypeHTML == true {
		lineSpacing = "\n"
	}
	err = eachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Notices, originTaskResultData.Notices, func(selem, telem interface{}) (bool, error) {
		actualityNotice, ok1 := selem.(*jyiuNotice)
		originNotice, ok2 := telem.(*jyiuNotice)
		if ok1 == false || ok2 == false {
			return false, errors.New("selem/telem의 타입 변환이 실패하였습니다")
		} else {
			if actualityNotice.Title == originNotice.Title && actualityNotice.Date == originNotice.Date && actualityNotice.Url == originNotice.Url {
				return true, nil
			}
		}
		return false, nil
	}, nil, func(selem interface{}) {
		actualityNotice := selem.(*jyiuNotice)

		if m != "" {
			m += lineSpacing
		}
		m += actualityNotice.String(messageTypeHTML, " 🆕")
	})
	if err != nil {
		return "", nil, err
	}

	if m != "" {
		message = "새로운 공지사항이 등록되었습니다.\n\n" + m
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.Notices) == 0 {
				message = "등록된 공지사항이 존재하지 않습니다."
			} else {
				for _, actualityNotice := range actualityTaskResultData.Notices {
					if m != "" {
						m += lineSpacing
					}
					m += actualityNotice.String(messageTypeHTML, "")
				}

				message = "신규로 등록된 공지사항이 없습니다.\n\n현재 등록된 공지사항은 아래와 같습니다:\n\n" + m
			}
		}
	}

	return message, changedTaskResultData, nil
}

func (t *jyiuTask) runWatchNewEducation(taskResultData interface{}, messageTypeHTML bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*jyiuWatchNewEducationResultData)
	if ok == false {
		log.Panic("TaskResultData의 타입 변환이 실패하였습니다.")
	}

	// 교육프로그램 페이지를 읽어서 정보를 추출한다.
	var err0 error
	var actualityTaskResultData = &jyiuWatchNewEducationResultData{}
	err = webScrape(fmt.Sprintf("%sgms_003001/experienceList", jyiuBaseUrl), "div.gms_003001 table.bbsList > tbody > tr", func(i int, s *goquery.Selection) bool {
		// 교육프로그램 컬럼 개수를 확인한다.
		as := s.Find("td")
		if as.Length() != 6 {
			err0 = fmt.Errorf("불러온 페이지의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요.(컬럼 개수 불일치:%d)", as.Length())
			return false
		}

		url, exists := s.Attr("onclick")
		if exists == false {
			err0 = errors.New("상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		pos1 := strings.Index(url, "'")
		pos2 := strings.LastIndex(url, "'")
		if pos1 == -1 || pos2 == -1 || pos1 == pos2 {
			err0 = errors.New("상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		url = url[pos1+1 : pos2]

		actualityTaskResultData.Educations = append(actualityTaskResultData.Educations, &jyiuEducation{
			Title:            utils.Trim(as.Eq(2).Text()),
			TrainingPeriod:   utils.Trim(as.Eq(4).Text()),
			AcceptancePeriod: utils.Trim(as.Eq(5).Text()),
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
	lineSpacing := "\n\n"
	err = eachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Educations, originTaskResultData.Educations, func(selem, telem interface{}) (bool, error) {
		actualityEducation, ok1 := selem.(*jyiuEducation)
		originEducation, ok2 := telem.(*jyiuEducation)
		if ok1 == false || ok2 == false {
			return false, errors.New("selem/telem의 타입 변환이 실패하였습니다")
		} else {
			if actualityEducation.Title == originEducation.Title && actualityEducation.TrainingPeriod == originEducation.TrainingPeriod && actualityEducation.AcceptancePeriod == originEducation.AcceptancePeriod && actualityEducation.Url == originEducation.Url {
				return true, nil
			}
		}
		return false, nil
	}, nil, func(selem interface{}) {
		actualityEducation := selem.(*jyiuEducation)

		if m != "" {
			m += lineSpacing
		}
		m += actualityEducation.String(messageTypeHTML, " 🆕")
	})
	if err != nil {
		return "", nil, err
	}

	if m != "" {
		message = "새로운 교육프로그램이 등록되었습니다.\n\n" + m
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.Educations) == 0 {
				message = "등록된 교육프로그램이 존재하지 않습니다."
			} else {
				for _, actualityEducation := range actualityTaskResultData.Educations {
					if m != "" {
						m += lineSpacing
					}
					m += actualityEducation.String(messageTypeHTML, "")
				}

				message = "신규로 등록된 교육프로그램이 없습니다.\n\n현재 등록된 교육프로그램은 아래와 같습니다:\n\n" + m
			}
		}
	}

	return message, changedTaskResultData, nil
}
