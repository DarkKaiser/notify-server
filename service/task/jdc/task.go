package jdc

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
)

const (
	// TaskID
	ID tasksvc.ID = "JDC" // 전남디지털역량교육(http://전남디지털역량.com/)

	// CommandID
	WatchNewOnlineEducationCommand tasksvc.CommandID = "WatchNewOnlineEducation" // 신규 비대면 온라인 특별/정규교육 확인
)

const (
	baseURL = "http://전남디지털역량.com/"
)

type onlineEducationCourse struct {
	Title1         string `json:"title1"`
	Title2         string `json:"title2"`
	TrainingPeriod string `json:"training_period"`
	URL            string `json:"url"`
	Err            error
}

func (c *onlineEducationCourse) String(messageTypeHTML bool, mark string) string {
	if messageTypeHTML == true {
		return fmt.Sprintf("☞ <a href=\"%s\"><b>%s &gt; %s</b></a>%s\n      • 교육기간 : %s", c.URL, c.Title1, c.Title2, mark, c.TrainingPeriod)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s > %s%s\n%s", c.Title1, c.Title2, mark, c.URL))
}

type watchNewOnlineEducationSnapshot struct {
	OnlineEducationCourses []*onlineEducationCourse `json:"online_education_courses"`
}

func init() {
	tasksvc.Register(ID, &tasksvc.Config{
		Commands: []*tasksvc.CommandConfig{{
			ID: WatchNewOnlineEducationCommand,

			AllowMultiple: true,

			NewSnapshot: func() interface{} { return &watchNewOnlineEducationSnapshot{} },
		}},

		NewTask: newTask,
	})
}

func newTask(instanceID tasksvc.InstanceID, req *tasksvc.SubmitRequest, appConfig *config.AppConfig) (tasksvc.Handler, error) {
	fetcher := tasksvc.NewRetryFetcherFromConfig(appConfig.HTTPRetry.MaxRetries, appConfig.HTTPRetry.RetryDelay)
	return createTask(instanceID, req, fetcher)
}

func createTask(instanceID tasksvc.InstanceID, req *tasksvc.SubmitRequest, fetcher tasksvc.Fetcher) (tasksvc.Handler, error) {
	if req.TaskID != ID {
		return nil, tasksvc.ErrTaskUnregistered
	}

	t := &task{
		Task: tasksvc.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),
	}

	t.SetFetcher(fetcher)

	// CommandID에 따른 실행 함수를 미리 바인딩합니다 (Fail Fast)
	switch req.CommandID {
	case WatchNewOnlineEducationCommand:
		t.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			originTaskResultData, ok := previousSnapshot.(*watchNewOnlineEducationSnapshot)
			if ok == false {
				return "", nil, tasksvc.NewErrTypeAssertionFailed("TaskResultData", &watchNewOnlineEducationSnapshot{}, previousSnapshot)
			}

			return t.executeWatchNewOnlineEducation(originTaskResultData, supportsHTML)
		})
	default:
		return nil, apperrors.New(apperrors.ErrInvalidInput, "지원하지 않는 명령입니다: "+string(req.CommandID))
	}

	return t, nil
}

type task struct {
	tasksvc.Task
}

func (t *task) executeWatchNewOnlineEducation(originTaskResultData *watchNewOnlineEducationSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {

	actualityTaskResultData := &watchNewOnlineEducationSnapshot{}

	// 등록된 비대면 온라인 특별교육/정규교육 강의 정보를 읽어온다.
	scrapedOnlineEducationCourses, err := t.scrapeOnlineEducationCourses(fmt.Sprintf("%sproduct/list?type=digital_edu", baseURL))
	if err != nil {
		return "", nil, err
	}
	actualityTaskResultData.OnlineEducationCourses = append(actualityTaskResultData.OnlineEducationCourses, scrapedOnlineEducationCourses...)

	scrapedOnlineEducationCourses, err = t.scrapeOnlineEducationCourses(fmt.Sprintf("%sproduct/list?type=untact_edu", baseURL))
	if err != nil {
		return "", nil, err
	}
	actualityTaskResultData.OnlineEducationCourses = append(actualityTaskResultData.OnlineEducationCourses, scrapedOnlineEducationCourses...)

	// 새로운 강의 정보를 확인한다.
	m := ""
	lineSpacing := "\n\n"
	err = tasksvc.EachSourceElementIsInTargetElementOrNot(actualityTaskResultData.OnlineEducationCourses, originTaskResultData.OnlineEducationCourses, func(selem, telem interface{}) (bool, error) {
		actualityEducationCourse, ok1 := selem.(*onlineEducationCourse)
		originEducationCourse, ok2 := telem.(*onlineEducationCourse)
		if ok1 == false || ok2 == false {
			return false, tasksvc.NewErrTypeAssertionFailed("selm/telm", &onlineEducationCourse{}, selem)
		} else {
			if actualityEducationCourse.Title1 == originEducationCourse.Title1 && actualityEducationCourse.Title2 == originEducationCourse.Title2 && actualityEducationCourse.TrainingPeriod == originEducationCourse.TrainingPeriod {
				return true, nil
			}
		}
		return false, nil
	}, nil, func(selem interface{}) {
		actualityEducationCourse := selem.(*onlineEducationCourse)

		if m != "" {
			m += lineSpacing
		}
		m += actualityEducationCourse.String(supportsHTML, " 🆕")
	})
	if err != nil {
		return "", nil, err
	}

	if m != "" {
		message = "새로운 온라인교육 강의가 등록되었습니다.\n\n" + m
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.GetRunBy() == tasksvc.RunByUser {
			if len(actualityTaskResultData.OnlineEducationCourses) == 0 {
				message = "등록된 온라인교육 강의가 존재하지 않습니다."
			} else {
				for _, actualityEducationCourse := range actualityTaskResultData.OnlineEducationCourses {
					if m != "" {
						m += lineSpacing
					}
					m += actualityEducationCourse.String(supportsHTML, "")
				}

				message = "신규로 등록된 온라인교육 강의가 없습니다.\n\n현재 등록된 온라인교육 강의는 아래와 같습니다:\n\n" + m
			}
		}
	}

	return message, changedTaskResultData, nil
}

func (t *task) scrapeOnlineEducationCourses(url string) ([]*onlineEducationCourse, error) {
	// 온라인교육 강의 목록페이지 URL 정보를 추출한다.
	var err, err0 error
	var courseURLs = make([]string, 0)
	err = tasksvc.ScrapeHTML(t.GetFetcher(), url, "#content > ul.prdt-list2 > li > a.link", func(i int, s *goquery.Selection) bool {
		courseURL, exists := s.Attr("href")
		if exists == false {
			err0 = apperrors.New(apperrors.ErrExecutionFailed, "강의 목록페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}

		courseURLs = append(courseURLs, courseURL)

		return true
	})
	if err != nil {
		// 온라인교육 강의 데이터가 없는지 확인한다.
		if sel, _ := tasksvc.FetchHTMLSelection(t.GetFetcher(), url, "#content > div.no-data2"); sel != nil {
			return nil, nil
		}

		return nil, err
	}
	if err0 != nil {
		return nil, err0
	}

	// 온라인교육 강의의 커리큘럼을 추출한다.
	curriculumWebScrapeDoneC := make(chan []*onlineEducationCourse, 50)
	for _, courseURL := range courseURLs {
		go t.scrapeOnlineEducationCourseCurriculums(courseURL, curriculumWebScrapeDoneC)
	}

	scrapeOnlineEducationCourses := make([]*onlineEducationCourse, 0)
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

func (t *task) scrapeOnlineEducationCourseCurriculums(url string, curriculumWebScrapeDoneC chan<- []*onlineEducationCourse) {
	var err0 error
	var onlineEducationCourseCurriculums = make([]*onlineEducationCourse, 0)

	err := tasksvc.ScrapeHTML(t.GetFetcher(), fmt.Sprintf("%sproduct/%s", baseURL, url), "table.prdt-tbl > tbody > tr", func(i int, s *goquery.Selection) bool {
		// 강의목록 컬럼 개수를 확인한다.
		as := s.Find("td")
		if as.Length() != 3 {
			if strutil.NormalizeSpaces(as.Text()) == "정보가 없습니다" {
				return true
			}

			err0 = tasksvc.NewErrHTMLStructureChanged("", fmt.Sprintf("목록 컬럼 개수 불일치:%d", as.Length()))
			return false
		}

		title1Selection := as.Eq(0).Find("a")
		if title1Selection.Length() != 1 {
			err0 = apperrors.New(apperrors.ErrExecutionFailed, "교육과정_제목1 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		title2Selection := as.Eq(0).Find("p")
		if title2Selection.Length() != 1 {
			err0 = apperrors.New(apperrors.ErrExecutionFailed, "교육과정_제목2 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}

		courseDetailURL, exists := title1Selection.Attr("href")
		if exists == false {
			err0 = apperrors.New(apperrors.ErrExecutionFailed, "강의 상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		// '마감되었습니다', '정원이 초과 되었습니다' 등의 알림창이 뜨도록 되어있는 경우인지 확인한다.
		if !strings.Contains(courseDetailURL, "javascript:alert('") {
			courseDetailURL = fmt.Sprintf("%sproduct/%s", baseURL, courseDetailURL)
		} else {
			courseDetailURL = ""
		}

		onlineEducationCourseCurriculums = append(onlineEducationCourseCurriculums, &onlineEducationCourse{
			Title1:         strutil.NormalizeSpaces(title1Selection.Text()),
			Title2:         strutil.NormalizeSpaces(title2Selection.Text()),
			TrainingPeriod: strutil.NormalizeSpaces(as.Eq(1).Text()),
			URL:            courseDetailURL,
			Err:            nil,
		})

		return true
	})
	if err != nil {
		onlineEducationCourseCurriculums = append(onlineEducationCourseCurriculums, &onlineEducationCourse{Err: err})
	}
	if err0 != nil {
		onlineEducationCourseCurriculums = append(onlineEducationCourseCurriculums, &onlineEducationCourse{Err: err0})
	}

	curriculumWebScrapeDoneC <- onlineEducationCourseCurriculums
}
