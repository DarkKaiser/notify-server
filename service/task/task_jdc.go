package task

import (
	"errors"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/utils"
	log "github.com/sirupsen/logrus"
)

const (
	// TaskID
	TidJdc TaskID = "JDC" // 전남디지털역량교육(http://전남디지털역량.com/)

	// TaskCommandID
	TcidJdcWatchNewOnlineEducation TaskCommandID = "WatchNewOnlineEducation" // 신규 비대면 온라인 특별/정규교육 확인
)

const (
	jdcBaseURL = "http://전남디지털역량.com/"
)

type jdcOnlineEducationCourse struct {
	Title1         string `json:"title1"`
	Title2         string `json:"title2"`
	TrainingPeriod string `json:"training_period"`
	URL            string `json:"url"`
	Err            error
}

func (c *jdcOnlineEducationCourse) String(messageTypeHTML bool, mark string) string {
	if messageTypeHTML == true {
		return fmt.Sprintf("☞ <a href=\"%s\"><b>%s &gt; %s</b></a>%s\n      • 교육기간 : %s", c.URL, c.Title1, c.Title2, mark, c.TrainingPeriod)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s > %s%s\n%s", c.Title1, c.Title2, mark, c.URL))
}

type jdcWatchNewOnlineEducationResultData struct {
	OnlineEducationCourses []*jdcOnlineEducationCourse `json:"online_education_courses"`
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

					fetcher: &HTTPFetcher{},
				},
			}

			task.runFn = func(taskResultData interface{}, messageTypeHTML bool) (string, interface{}, error) {
				switch task.CommandID() {
				case TcidJdcWatchNewOnlineEducation:
					return task.runWatchNewOnlineEducation(taskResultData, messageTypeHTML)
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

func (t *jdcTask) runWatchNewOnlineEducation(taskResultData interface{}, messageTypeHTML bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*jdcWatchNewOnlineEducationResultData)
	if ok == false {
		log.Panic("TaskResultData의 타입 변환이 실패하였습니다.")
	}

	actualityTaskResultData := &jdcWatchNewOnlineEducationResultData{}

	// 등록된 비대면 온라인 특별교육/정규교육 강의 정보를 읽어온다.
	scrapedOnlineEducationCourses, err := t.scrapeOnlineEducationCourses(fmt.Sprintf("%sproduct/list?type=digital_edu", jdcBaseURL))
	if err != nil {
		return "", nil, err
	}
	actualityTaskResultData.OnlineEducationCourses = append(actualityTaskResultData.OnlineEducationCourses, scrapedOnlineEducationCourses...)

	scrapedOnlineEducationCourses, err = t.scrapeOnlineEducationCourses(fmt.Sprintf("%sproduct/list?type=untact_edu", jdcBaseURL))
	if err != nil {
		return "", nil, err
	}
	actualityTaskResultData.OnlineEducationCourses = append(actualityTaskResultData.OnlineEducationCourses, scrapedOnlineEducationCourses...)

	// 새로운 강의 정보를 확인한다.
	m := ""
	lineSpacing := "\n\n"
	err = eachSourceElementIsInTargetElementOrNot(actualityTaskResultData.OnlineEducationCourses, originTaskResultData.OnlineEducationCourses, func(selem, telem interface{}) (bool, error) {
		actualityEducationCourse, ok1 := selem.(*jdcOnlineEducationCourse)
		originEducationCourse, ok2 := telem.(*jdcOnlineEducationCourse)
		if ok1 == false || ok2 == false {
			return false, errors.New("selem/telem의 타입 변환이 실패하였습니다")
		} else {
			if actualityEducationCourse.Title1 == originEducationCourse.Title1 && actualityEducationCourse.Title2 == originEducationCourse.Title2 && actualityEducationCourse.TrainingPeriod == originEducationCourse.TrainingPeriod {
				return true, nil
			}
		}
		return false, nil
	}, nil, func(selem interface{}) {
		actualityEducationCourse := selem.(*jdcOnlineEducationCourse)

		if m != "" {
			m += lineSpacing
		}
		m += actualityEducationCourse.String(messageTypeHTML, " 🆕")
	})
	if err != nil {
		return "", nil, err
	}

	if m != "" {
		message = "새로운 온라인교육 강의가 등록되었습니다.\n\n" + m
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.OnlineEducationCourses) == 0 {
				message = "등록된 온라인교육 강의가 존재하지 않습니다."
			} else {
				for _, actualityEducationCourse := range actualityTaskResultData.OnlineEducationCourses {
					if m != "" {
						m += lineSpacing
					}
					m += actualityEducationCourse.String(messageTypeHTML, "")
				}

				message = "신규로 등록된 온라인교육 강의가 없습니다.\n\n현재 등록된 온라인교육 강의는 아래와 같습니다:\n\n" + m
			}
		}
	}

	return message, changedTaskResultData, nil
}

func (t *jdcTask) scrapeOnlineEducationCourses(url string) ([]*jdcOnlineEducationCourse, error) {
	// 온라인교육 강의 목록페이지 URL 정보를 추출한다.
	var err, err0 error
	var courseURLs = make([]string, 0)
	err = webScrape(t.fetcher, url, "#content > ul.prdt-list2 > li > a.link", func(i int, s *goquery.Selection) bool {
		courseURL, exists := s.Attr("href")
		if exists == false {
			err0 = errors.New("강의 목록페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}

		courseURLs = append(courseURLs, courseURL)

		return true
	})
	if err != nil {
		// 온라인교육 강의 데이터가 없는지 확인한다.
		if sel, _ := newHTMLDocumentSelection(t.fetcher, url, "#content > div.no-data2"); sel != nil {
			return nil, nil
		}

		return nil, err
	}
	if err0 != nil {
		return nil, err0
	}

	// 온라인교육 강의의 커리큘럼을 추출한다.
	curriculumWebScrapeDoneC := make(chan []*jdcOnlineEducationCourse, 50)
	for _, courseURL := range courseURLs {
		go t.scrapeOnlineEducationCourseCurriculums(courseURL, curriculumWebScrapeDoneC)
	}

	scrapeOnlineEducationCourses := make([]*jdcOnlineEducationCourse, 0)
	for i := 0; i < len(courseURLs); i++ {
		onlineEducationCourseCurriculums := <-curriculumWebScrapeDoneC

		// 스크랩중에 오류가 발생하였는지 확인한다.
		for _, curriculum := range onlineEducationCourseCurriculums {
			if curriculum.Err != nil {
				return nil, err
			}
		}

		scrapeOnlineEducationCourses = append(scrapeOnlineEducationCourses, onlineEducationCourseCurriculums...)
	}

	return scrapeOnlineEducationCourses, nil
}

func (t *jdcTask) scrapeOnlineEducationCourseCurriculums(url string, curriculumWebScrapeDoneC chan<- []*jdcOnlineEducationCourse) {
	var err0 error
	var onlineEducationCourseCurriculums = make([]*jdcOnlineEducationCourse, 0)

	err := webScrape(t.fetcher, fmt.Sprintf("%sproduct/%s", jdcBaseURL, url), "table.prdt-tbl > tbody > tr", func(i int, s *goquery.Selection) bool {
		// 강의목록 컬럼 개수를 확인한다.
		as := s.Find("td")
		if as.Length() != 3 {
			if utils.Trim(as.Text()) == "정보가 없습니다" {
				return true
			}

			err0 = fmt.Errorf("불러온 페이지의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요.(컬럼 개수 불일치:%d)", as.Length())
			return false
		}

		title1Selection := as.Eq(0).Find("a")
		if title1Selection.Length() != 1 {
			err0 = errors.New("교육과정_제목1 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		title2Selection := as.Eq(0).Find("p")
		if title2Selection.Length() != 1 {
			err0 = errors.New("교육과정_제목2 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}

		courseDetailURL, exists := title1Selection.Attr("href")
		if exists == false {
			err0 = errors.New("강의 상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		// '마감되었습니다', '정원이 초과 되었습니다' 등의 알림창이 뜨도록 되어있는 경우인지 확인한다.
		if !strings.Contains(courseDetailURL, "javascript:alert('") {
			courseDetailURL = fmt.Sprintf("%sproduct/%s", jdcBaseURL, courseDetailURL)
		} else {
			courseDetailURL = ""
		}

		onlineEducationCourseCurriculums = append(onlineEducationCourseCurriculums, &jdcOnlineEducationCourse{
			Title1:         utils.Trim(title1Selection.Text()),
			Title2:         utils.Trim(title2Selection.Text()),
			TrainingPeriod: utils.Trim(as.Eq(1).Text()),
			URL:            courseDetailURL,
			Err:            nil,
		})

		return true
	})
	if err != nil {
		onlineEducationCourseCurriculums = append(onlineEducationCourseCurriculums, &jdcOnlineEducationCourse{Err: err})
	}
	if err0 != nil {
		onlineEducationCourseCurriculums = append(onlineEducationCourseCurriculums, &jdcOnlineEducationCourse{Err: err0})
	}

	curriculumWebScrapeDoneC <- onlineEducationCourseCurriculums
}
