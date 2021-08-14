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
	TidJdc TaskID = "JDC" // 전남디지털역량교육(http://전남디지털역량.com/)

	// TaskCommandID
	TcidJdcWatchNewOnlineEducation TaskCommandID = "WatchNewOnlineEducation" // 신규 비대면 온라인 특별/정규교육 확인
)

const (
	jdcBaseUrl = "http://전남디지털역량.com/"
)

type onlineEducationCourse struct {
	Title1         string `json:"title1"`
	Title2         string `json:"title2"`
	TrainingPeriod string `json:"training_period"`
	Url            string `json:"url"`
}

type jdcWatchNewOnlineEducationResultData struct {
	OnlineEducationCourse []onlineEducationCourse `json:"online_education_course"`
}

func init() {
	supportedTasks[TidJdc] = &supportedTaskConfig{
		commandConfigs: []*supportedTaskCommandConfig{{
			taskCommandID: TcidJdcWatchNewOnlineEducation,

			allowMultipleInstances: true,

			newTaskResultDataFn: func() interface{} { return &jdcWatchNewOnlineEducationResultData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData, config *g.AppConfig) (taskHandler, error) {
			if taskRunData.taskID != TidJdc {
				return nil, errors.New("등록되지 않은 작업입니다.😱")
			}

			task := &jdcTask{
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
				case TcidJdcWatchNewOnlineEducation:
					return task.runWatchNewOnlineEducation(taskResultData, isSupportedHTMLMessage)
				}

				return "", nil, ErrNoImplementationForTaskCommand
			}

			return task, nil
		},
	}
}

type jdcTask struct {
	task
}

func (t *jdcTask) runWatchNewOnlineEducation(taskResultData interface{}, isSupportedHTMLMessage bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*jdcWatchNewOnlineEducationResultData)
	if ok == false {
		log.Panic("TaskResultData의 타입 변환이 실패하였습니다.")
	}

	actualityTaskResultData := jdcWatchNewOnlineEducationResultData{}

	// 등록된 비대면 온라인 특별교육/정규교육 강의 정보를 읽어온다.
	scrapedOnlineEducationCourse, err := t.scrapeOnlineEducationCourse(fmt.Sprintf("%sproduct/list?type=digital_edu", jdcBaseUrl))
	if err != nil {
		return "", nil, err
	}
	actualityTaskResultData.OnlineEducationCourse = append(actualityTaskResultData.OnlineEducationCourse, scrapedOnlineEducationCourse...)

	scrapedOnlineEducationCourse, err = t.scrapeOnlineEducationCourse(fmt.Sprintf("%sproduct/list?type=untact_edu", jdcBaseUrl))
	if err != nil {
		return "", nil, err
	}
	actualityTaskResultData.OnlineEducationCourse = append(actualityTaskResultData.OnlineEducationCourse, scrapedOnlineEducationCourse...)

	// 새로운 강의 정보를 확인한다.
	m := ""
	existsNewCourse := false
	for _, actualityEducationCourse := range actualityTaskResultData.OnlineEducationCourse {
		isNewCourse := true
		for _, originEducationCourse := range originTaskResultData.OnlineEducationCourse {
			if actualityEducationCourse.Title1 == originEducationCourse.Title1 && actualityEducationCourse.Title2 == originEducationCourse.Title2 && actualityEducationCourse.TrainingPeriod == originEducationCourse.TrainingPeriod {
				isNewCourse = false
				break
			}
		}

		if isNewCourse == true {
			existsNewCourse = true

			if isSupportedHTMLMessage == true {
				if m != "" {
					m += "\n\n"
				}
				m = fmt.Sprintf("%s☞ <a href=\"%s\"><b>%s &gt; %s</b></a> 🆕\n      • 교육기간 : %s", m, actualityEducationCourse.Url, actualityEducationCourse.Title1, actualityEducationCourse.Title2, actualityEducationCourse.TrainingPeriod)
			} else {
				if m != "" {
					m += "\n\n"
				}
				m = fmt.Sprintf("%s☞ %s > %s 🆕\n%s", m, actualityEducationCourse.Title1, actualityEducationCourse.Title2, actualityEducationCourse.Url)
			}
		}
	}

	if existsNewCourse == true {
		message = fmt.Sprintf("새로운 온라인교육 강의가 등록되었습니다.\n\n%s", m)
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.OnlineEducationCourse) == 0 {
				message = "등록된 온라인교육 강의가 존재하지 않습니다."
			} else {
				message = "새롭게 등록된 온라인교육 강의가 없습니다.\n\n현재 등록된 온라인교육 강의는 아래와 같습니다:"

				if isSupportedHTMLMessage == true {
					for _, actualityEducationCourse := range actualityTaskResultData.OnlineEducationCourse {
						message = fmt.Sprintf("%s\n\n☞ <a href=\"%s\"><b>%s &gt; %s</b></a>\n      • 교육기간 : %s", message, actualityEducationCourse.Url, actualityEducationCourse.Title1, actualityEducationCourse.Title2, actualityEducationCourse.TrainingPeriod)
					}
				} else {
					for _, actualityEducationCourse := range actualityTaskResultData.OnlineEducationCourse {
						message = fmt.Sprintf("%s\n\n☞ %s > %s\n%s", message, actualityEducationCourse.Title1, actualityEducationCourse.Title2, actualityEducationCourse.Url)
					}
				}
			}
		}
	}

	return message, changedTaskResultData, nil
}

func (t *jdcTask) scrapeOnlineEducationCourse(url string) ([]onlineEducationCourse, error) {
	// 강의목록 페이지 URL 정보를 추출한다.
	var err, err0 error
	var courseURLs = make([]string, 0)
	err = scrapeHTMLDocument(url, "#content > ul.prdt-list2 > li > a.link", func(i int, s *goquery.Selection) bool {
		courseURL, exists := s.Attr("href")
		if exists == false {
			err0 = errors.New("강의 목록페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
			return false
		}

		courseURLs = append(courseURLs, courseURL)

		return true
	})
	if err != nil {
		return nil, err
	}
	if err0 != nil {
		return nil, err0
	}

	// 온라인교육 강의의 상세정보를 추출한다.
	var scrapeOnlineEducationCourse = make([]onlineEducationCourse, 0)
	for _, courseURL := range courseURLs {
		err = scrapeHTMLDocument(fmt.Sprintf("%sproduct/%s", jdcBaseUrl, courseURL), "table.prdt-tbl > tbody > tr", func(i int, s *goquery.Selection) bool {
			// 강의목록 컬럼 개수를 확인한다.
			as := s.Find("td")
			if as.Length() != 3 {
				if utils.CleanString(as.Text()) == "정보가 없습니다" {
					return true
				}

				err0 = fmt.Errorf("불러온 페이지의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요.(컬럼 개수 불일치:%d)", as.Length())
				return false
			}

			title1Selection := as.Eq(0).Find("a")
			if title1Selection.Length() != 1 {
				err0 = errors.New("교육과정_제목1 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
				return false
			}
			title2Selection := as.Eq(0).Find("p")
			if title2Selection.Length() != 1 {
				err0 = errors.New("교육과정_제목2 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
				return false
			}

			courseDetailURL, exists := title1Selection.Attr("href")
			if exists == false {
				err0 = errors.New("강의 상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
				return false
			}
			// '마감되었습니다', '정원이 초과 되었습니다' 등의 알림창이 뜨도록 되어있는 경우인지 확인한다.
			if strings.Index(courseDetailURL, "javascript:alert('") == -1 {
				courseDetailURL = fmt.Sprintf("%sproduct/%s", jdcBaseUrl, courseDetailURL)
			} else {
				courseDetailURL = ""
			}

			scrapeOnlineEducationCourse = append(scrapeOnlineEducationCourse, onlineEducationCourse{
				Title1:         utils.CleanString(title1Selection.Text()),
				Title2:         utils.CleanString(title2Selection.Text()),
				TrainingPeriod: utils.CleanString(as.Eq(1).Text()),
				Url:            courseDetailURL,
			})

			return true
		})
		if err != nil {
			return nil, err
		}
		if err0 != nil {
			return nil, err0
		}
	}

	return scrapeOnlineEducationCourse, nil
}
