package naver

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
)

const (
	// TaskID
	ID tasksvc.ID = "NAVER" // 네이버

	// CommandID
	WatchNewPerformancesCommand tasksvc.CommandID = "WatchNewPerformances" // 네이버 신규 공연정보 확인
)

type watchNewPerformancesCommandConfig struct {
	Query   string `json:"query"`
	Filters struct {
		Title struct {
			IncludedKeywords string `json:"included_keywords"`
			ExcludedKeywords string `json:"excluded_keywords"`
		} `json:"title"`
		Place struct {
			IncludedKeywords string `json:"included_keywords"`
			ExcludedKeywords string `json:"excluded_keywords"`
		} `json:"place"`
	} `json:"filters"`
}

func (c *watchNewPerformancesCommandConfig) validate() error {
	if c.Query == "" {
		return apperrors.New(apperrors.ErrInvalidInput, "query가 입력되지 않았습니다")
	}
	return nil
}

type performanceSearchResponse struct {
	HTML string `json:"html"`
}

type performance struct {
	Title     string `json:"title"`
	Place     string `json:"place"`
	Thumbnail string `json:"thumbnail"`
}

func (p *performance) String(messageTypeHTML bool, mark string) string {
	if messageTypeHTML == true {
		return fmt.Sprintf("☞ <a href=\"https://search.naver.com/search.naver?query=%s\"><b>%s</b></a>%s\n      • 장소 : %s", url.QueryEscape(p.Title), template.HTMLEscapeString(p.Title), mark, p.Place)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s%s\n      • 장소 : %s", template.HTMLEscapeString(p.Title), mark, p.Place))
}

type watchNewPerformancesSnapshot struct {
	Performances []*performance `json:"performances"`
}

func init() {
	tasksvc.Register(ID, &tasksvc.Config{
		Commands: []*tasksvc.CommandConfig{{
			ID: WatchNewPerformancesCommand,

			AllowMultiple: true,

			NewSnapshot: func() interface{} { return &watchNewPerformancesSnapshot{} },
		}},

		NewTask: newTask,
	})
}

func newTask(instanceID tasksvc.InstanceID, req *tasksvc.SubmitRequest, appConfig *config.AppConfig) (tasksvc.Handler, error) {
	fetcher := tasksvc.NewRetryFetcherFromConfig(appConfig.HTTPRetry.MaxRetries, appConfig.HTTPRetry.RetryDelay)

	return createTask(instanceID, req, appConfig, fetcher)
}

func createTask(instanceID tasksvc.InstanceID, req *tasksvc.SubmitRequest, appConfig *config.AppConfig, fetcher tasksvc.Fetcher) (tasksvc.Handler, error) {
	if req.TaskID != ID {
		return nil, tasksvc.ErrTaskUnregistered
	}

	tTask := &task{
		Task: tasksvc.NewBaseTask(req.TaskID, req.CommandID, instanceID, req.NotifierID, req.RunBy),

		appConfig: appConfig,
	}

	tTask.SetFetcher(fetcher)

	// CommandID에 따른 실행 함수를 미리 바인딩합니다 (Fail Fast)
	switch req.CommandID {
	case WatchNewPerformancesCommand:
		tTask.SetExecute(func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
			for _, t := range tTask.appConfig.Tasks {
				if tTask.GetID() == tasksvc.ID(t.ID) {
					for _, c := range t.Commands {
						if tTask.GetCommandID() == tasksvc.CommandID(c.ID) {
							commandConfig := &watchNewPerformancesCommandConfig{}
							if err := tasksvc.DecodeMap(commandConfig, c.Data); err != nil {
								return "", nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
							}
							if err := commandConfig.validate(); err != nil {
								return "", nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
							}

							originTaskResultData, ok := previousSnapshot.(*watchNewPerformancesSnapshot)
							if ok == false {
								return "", nil, tasksvc.NewErrTypeAssertionFailed("TaskResultData", &watchNewPerformancesSnapshot{}, previousSnapshot)
							}

							return tTask.executeWatchNewPerformances(commandConfig, originTaskResultData, supportsHTML)
						}
					}
					break
				}
			}
			return "", nil, apperrors.New(apperrors.ErrInternal, "Command configuration not found")
		})
	default:
		return nil, apperrors.New(apperrors.ErrInvalidInput, "지원하지 않는 명령입니다: "+string(req.CommandID))
	}

	return tTask, nil
}

type task struct {
	tasksvc.Task

	appConfig *config.AppConfig
}

// noinspection GoUnhandledErrorResult,GoErrorStringFormat
func (t *task) executeWatchNewPerformances(commandConfig *watchNewPerformancesCommandConfig, originTaskResultData *watchNewPerformancesSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {

	actualityTaskResultData := &watchNewPerformancesSnapshot{}
	titleIncludedKeywords := strutil.SplitAndTrim(commandConfig.Filters.Title.IncludedKeywords, ",")
	titleExcludedKeywords := strutil.SplitAndTrim(commandConfig.Filters.Title.ExcludedKeywords, ",")
	placeIncludedKeywords := strutil.SplitAndTrim(commandConfig.Filters.Place.IncludedKeywords, ",")
	placeExcludedKeywords := strutil.SplitAndTrim(commandConfig.Filters.Place.ExcludedKeywords, ",")

	// 전라도 지역 공연정보를 읽어온다.
	searchPerformancePageIndex := 1
	for {
		var searchResultData = &performanceSearchResponse{}
		err = tasksvc.FetchJSON(t.GetFetcher(), "GET", fmt.Sprintf("https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=%d&u8=all&u3=&u1=%s&u2=all&u4=ingplan&u6=N&u5=date", searchPerformancePageIndex, url.QueryEscape(commandConfig.Query)), nil, nil, searchResultData)
		if err != nil {
			return "", nil, err
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(searchResultData.HTML))
		if err != nil {
			return "", nil, apperrors.Wrap(err, apperrors.ErrExecutionFailed, "불러온 페이지의 데이터 파싱이 실패하였습니다")
		}

		// 읽어온 페이지에서 공연정보를 추출한다.
		ps := doc.Find("ul > li")
		ps.EachWithBreak(func(i int, s *goquery.Selection) bool {
			// 제목
			pis := s.Find("div.item > div.title_box > strong.name")
			if pis.Length() != 1 {
				err = tasksvc.NewErrHTMLStructureChanged("", "공연 제목 추출이 실패하였습니다")
				return false
			}
			title := strings.TrimSpace(pis.Text())

			// 장소
			pis = s.Find("div.item > div.title_box > span.sub_text")
			if pis.Length() != 1 {
				err = tasksvc.NewErrHTMLStructureChanged("", "공연 장소 추출이 실패하였습니다")
				return false
			}
			place := strings.TrimSpace(pis.Text())

			// 썸네일 이미지
			pis = s.Find("div.item > div.thumb > img")
			if pis.Length() != 1 {
				err = tasksvc.NewErrHTMLStructureChanged("", "공연 썸네일 이미지 추출이 실패하였습니다")
				return false
			}
			thumbnailSrc, exists := pis.Attr("src")
			if exists == false {
				err = tasksvc.NewErrHTMLStructureChanged("", "공연 썸네일 이미지 추출이 실패하였습니다")
				return false
			}
			thumbnail := fmt.Sprintf(`<img src="%s">`, thumbnailSrc)

			if tasksvc.Filter(title, titleIncludedKeywords, titleExcludedKeywords) == false || tasksvc.Filter(place, placeIncludedKeywords, placeExcludedKeywords) == false {
				return true
			}

			actualityTaskResultData.Performances = append(actualityTaskResultData.Performances, &performance{
				Title:     title,
				Place:     place,
				Thumbnail: thumbnail,
			})

			return true
		})
		if err != nil {
			return "", nil, err
		}

		searchPerformancePageIndex += 1

		// 불러온 데이터가 없는 경우, 모든 공연정보를 불러온 것으로 인식한다.
		if ps.Length() == 0 {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	// 신규 공연정보를 확인한다.
	m := ""
	lineSpacing := "\n\n"
	err = tasksvc.EachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Performances, originTaskResultData.Performances, func(selem, telem interface{}) (bool, error) {
		actualityPerformance, ok1 := selem.(*performance)
		originPerformance, ok2 := telem.(*performance)
		if ok1 == false || ok2 == false {
			return false, tasksvc.NewErrTypeAssertionFailed("selm/telm", &performance{}, selem)
		} else {
			if actualityPerformance.Title == originPerformance.Title && actualityPerformance.Place == originPerformance.Place {
				return true, nil
			}
		}
		return false, nil
	}, nil, func(selem interface{}) {
		actualityPerformance := selem.(*performance)

		if m != "" {
			m += lineSpacing
		}
		m += actualityPerformance.String(supportsHTML, " 🆕")
	})
	if err != nil {
		return "", nil, err
	}

	if m != "" {
		message = "새로운 공연정보가 등록되었습니다.\n\n" + m
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.GetRunBy() == tasksvc.RunByUser {
			if len(actualityTaskResultData.Performances) == 0 {
				message = "등록된 공연정보가 존재하지 않습니다."
			} else {
				for _, actualityPerformance := range actualityTaskResultData.Performances {
					if m != "" {
						m += lineSpacing
					}
					m += actualityPerformance.String(supportsHTML, "")
				}

				message = "신규로 등록된 공연정보가 없습니다.\n\n현재 등록된 공연정보는 아래와 같습니다:\n\n" + m
			}
		}
	}

	return message, changedTaskResultData, nil
}
