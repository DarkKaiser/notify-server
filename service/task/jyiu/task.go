package jyiu

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
)

const (
	// TaskID
	ID tasksvc.ID = "JYIU" // 전남여수산학융합원(https://www.jyiu.or.kr/)

	// CommandID
	WatchNewNoticeCommand    tasksvc.CommandID = "WatchNewNotice"    // 전남여수산학융합원 공지사항 새글 확인
	WatchNewEducationCommand tasksvc.CommandID = "WatchNewEducation" // 전남여수산학융합원 신규 교육프로그램 확인
)

const (
	baseURL = "https://www.jyiu.or.kr/"
)

type notice struct {
	Title string `json:"title"`
	Date  string `json:"date"`
	URL   string `json:"url"`
}

func (n *notice) String(messageTypeHTML bool, mark string) string {
	if messageTypeHTML == true {
		return fmt.Sprintf("☞ <a href=\"%s\"><b>%s</b></a>%s", n.URL, n.Title, mark)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s%s\n%s", n.Title, mark, n.URL))
}

type watchNewNoticeSnapshot struct {
	Notices []*notice `json:"notices"`
}

type education struct {
	Title            string `json:"title"`
	TrainingPeriod   string `json:"training_period"`
	AcceptancePeriod string `json:"acceptance_period"`
	URL              string `json:"url"`
}

func (e *education) String(messageTypeHTML bool, mark string) string {
	if messageTypeHTML == true {
		return fmt.Sprintf("☞ <a href=\"%s\"><b>%s</b></a>%s\n      • 교육기간 : %s\n      • 접수기간 : %s", e.URL, e.Title, mark, e.TrainingPeriod, e.AcceptancePeriod)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s%s\n%s", e.Title, mark, e.URL))
}

type watchNewEducationSnapshot struct {
	Educations []*education `json:"educations"`
}

func init() {
	tasksvc.Register(ID, &tasksvc.Config{
		Commands: []*tasksvc.CommandConfig{{
			ID: WatchNewNoticeCommand,

			AllowMultiple: true,

			NewSnapshot: func() interface{} { return &watchNewNoticeSnapshot{} },
		}, {
			ID: WatchNewEducationCommand,

			AllowMultiple: true,

			NewSnapshot: func() interface{} { return &watchNewEducationSnapshot{} },
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
		return nil, apperrors.New(tasksvc.ErrTaskNotFound, "등록되지 않은 작업입니다.😱")
	}

	tTask := &task{
		Task: tasksvc.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),
	}

	tTask.SetFetcher(fetcher)

	// CommandID에 따른 실행 함수를 미리 바인딩합니다 (Fail Fast)
	switch req.CommandID {
	case WatchNewNoticeCommand:
		tTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			originTaskResultData, ok := previousSnapshot.(*watchNewNoticeSnapshot)
			if ok == false {
				return "", nil, apperrors.New(apperrors.ErrInternal, fmt.Sprintf("TaskResultData의 타입 변환이 실패하였습니다 (expected: *watchNewNoticeSnapshot, got: %T)", previousSnapshot))
			}

			return tTask.executeWatchNewNotice(originTaskResultData, supportsHTML)
		})

	case WatchNewEducationCommand:
		tTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			originTaskResultData, ok := previousSnapshot.(*watchNewEducationSnapshot)
			if ok == false {
				return "", nil, apperrors.New(apperrors.ErrInternal, fmt.Sprintf("TaskResultData의 타입 변환이 실패하였습니다 (expected: *watchNewEducationSnapshot, got: %T)", previousSnapshot))
			}

			return tTask.executeWatchNewEducation(originTaskResultData, supportsHTML)
		})

	default:
		return nil, apperrors.New(apperrors.ErrInvalidInput, "지원하지 않는 명령입니다: "+string(req.CommandID))
	}

	return tTask, nil
}

type task struct {
	tasksvc.Task
}

func (t *task) executeWatchNewNotice(originTaskResultData *watchNewNoticeSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {

	// 공지사항 페이지를 읽어서 정보를 추출한다.
	var err0 error
	var actualityTaskResultData = &watchNewNoticeSnapshot{}
	err = tasksvc.ScrapeHTML(t.GetFetcher(), fmt.Sprintf("%sgms_005001/", baseURL), "#contents table.bbsList > tbody > tr", func(i int, s *goquery.Selection) bool {
		// 공지사항 컬럼 개수를 확인한다.
		as := s.Find("td")
		if as.Length() != 5 {
			err0 = apperrors.New(tasksvc.ErrTaskExecutionFailed, fmt.Sprintf("불러온 페이지의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요.(컬럼 개수 불일치:%d)", as.Length()))
			return false
		}

		id, exists := as.Eq(1).Find("a").Attr("onclick")
		if exists == false {
			err0 = apperrors.New(tasksvc.ErrTaskExecutionFailed, "상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		pos1 := strings.Index(id, "(")
		pos2 := strings.LastIndex(id, ")")
		if pos1 == -1 || pos2 == -1 || pos1 == pos2 {
			err0 = apperrors.New(tasksvc.ErrTaskExecutionFailed, "상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		id = id[pos1+1 : pos2]

		title := strutil.NormalizeSpaces(as.Eq(1).Find("a").Text())
		if utf8.ValidString(title) == false {
			title0 := ""
			for _, v := range title {
				if utf8.ValidString(string(v)) == false {
					break
				}
				title0 += string(v)
			}
			title = title0
		}

		actualityTaskResultData.Notices = append(actualityTaskResultData.Notices, &notice{
			Title: title,
			Date:  strutil.NormalizeSpaces(as.Eq(3).Text()),
			URL:   fmt.Sprintf("%sgms_005001/view?id=%s", baseURL, id),
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
	if supportsHTML == true {
		lineSpacing = "\n"
	}
	err = tasksvc.EachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Notices, originTaskResultData.Notices, func(selem, telem interface{}) (bool, error) {
		actualityNotice, ok1 := selem.(*notice)
		originNotice, ok2 := telem.(*notice)
		if ok1 == false || ok2 == false {
			return false, apperrors.New(apperrors.ErrInternal, "selem/telem의 타입 변환이 실패하였습니다")
		} else {
			if actualityNotice.Title == originNotice.Title && actualityNotice.Date == originNotice.Date && actualityNotice.URL == originNotice.URL {
				return true, nil
			}
		}
		return false, nil
	}, nil, func(selem interface{}) {
		actualityNotice := selem.(*notice)

		if m != "" {
			m += lineSpacing
		}
		m += actualityNotice.String(supportsHTML, " 🆕")
	})
	if err != nil {
		return "", nil, err
	}

	if m != "" {
		message = "새로운 공지사항이 등록되었습니다.\n\n" + m
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.GetRunBy() == tasksvc.RunByUser {
			if len(actualityTaskResultData.Notices) == 0 {
				message = "등록된 공지사항이 존재하지 않습니다."
			} else {
				for _, actualityNotice := range actualityTaskResultData.Notices {
					if m != "" {
						m += lineSpacing
					}
					m += actualityNotice.String(supportsHTML, "")
				}

				message = "신규로 등록된 공지사항이 없습니다.\n\n현재 등록된 공지사항은 아래와 같습니다:\n\n" + m
			}
		}
	}

	return message, changedTaskResultData, nil
}

func (t *task) executeWatchNewEducation(originTaskResultData *watchNewEducationSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {

	// 교육프로그램 페이지를 읽어서 정보를 추출한다.
	var err0 error
	var actualityTaskResultData = &watchNewEducationSnapshot{}
	err = tasksvc.ScrapeHTML(t.GetFetcher(), fmt.Sprintf("%sgms_003001/experienceList", baseURL), "div.gms_003001 table.bbsList > tbody > tr", func(i int, s *goquery.Selection) bool {
		// 교육프로그램 컬럼 개수를 확인한다.
		as := s.Find("td")
		if as.Length() != 6 {
			err0 = apperrors.New(tasksvc.ErrTaskExecutionFailed, fmt.Sprintf("불러온 페이지의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요.(컬럼 개수 불일치:%d)", as.Length()))
			return false
		}

		url, exists := s.Attr("onclick")
		if exists == false {
			err0 = apperrors.New(tasksvc.ErrTaskExecutionFailed, "상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		pos1 := strings.Index(url, "'")
		pos2 := strings.LastIndex(url, "'")
		if pos1 == -1 || pos2 == -1 || pos1 == pos2 {
			err0 = apperrors.New(tasksvc.ErrTaskExecutionFailed, "상세페이지 URL 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
			return false
		}
		url = url[pos1+1 : pos2]

		title := strutil.NormalizeSpaces(as.Eq(2).Text())
		if utf8.ValidString(title) == false {
			title0 := ""
			for _, v := range title {
				if utf8.ValidString(string(v)) == false {
					break
				}
				title0 += string(v)
			}
			title = title0
		}

		actualityTaskResultData.Educations = append(actualityTaskResultData.Educations, &education{
			Title:            title,
			TrainingPeriod:   strutil.NormalizeSpaces(as.Eq(4).Text()),
			AcceptancePeriod: strutil.NormalizeSpaces(as.Eq(5).Text()),
			URL:              fmt.Sprintf("%s%s", baseURL, url),
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
	err = tasksvc.EachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Educations, originTaskResultData.Educations, func(selem, telem interface{}) (bool, error) {
		actualityEducation, ok1 := selem.(*education)
		originEducation, ok2 := telem.(*education)
		if ok1 == false || ok2 == false {
			return false, apperrors.New(apperrors.ErrInternal, "selem/telem의 타입 변환이 실패하였습니다")
		} else {
			if actualityEducation.Title == originEducation.Title && actualityEducation.TrainingPeriod == originEducation.TrainingPeriod && actualityEducation.AcceptancePeriod == originEducation.AcceptancePeriod && actualityEducation.URL == originEducation.URL {
				return true, nil
			}
		}
		return false, nil
	}, nil, func(selem interface{}) {
		actualityEducation := selem.(*education)

		if m != "" {
			m += lineSpacing
		}
		m += actualityEducation.String(supportsHTML, " 🆕")
	})
	if err != nil {
		return "", nil, err
	}

	if m != "" {
		message = "새로운 교육프로그램이 등록되었습니다.\n\n" + m
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.GetRunBy() == tasksvc.RunByUser {
			if len(actualityTaskResultData.Educations) == 0 {
				message = "등록된 교육프로그램이 존재하지 않습니다."
			} else {
				for _, actualityEducation := range actualityTaskResultData.Educations {
					if m != "" {
						m += lineSpacing
					}
					m += actualityEducation.String(supportsHTML, "")
				}

				message = "신규로 등록된 교육프로그램이 없습니다.\n\n현재 등록된 교육프로그램은 아래와 같습니다:\n\n" + m
			}
		}
	}

	return message, changedTaskResultData, nil
}
