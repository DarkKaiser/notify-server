package task

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
)

const (
	// TaskID
	TidNaver TaskID = "NAVER" // 네이버

	// TaskCommandID
	TcidNaverWatchNewPerformances TaskCommandID = "WatchNewPerformances" // 네이버 신규 공연정보 확인
)

type naverWatchNewPerformancesTaskCommandData struct {
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

func (d *naverWatchNewPerformancesTaskCommandData) validate() error {
	if d.Query == "" {
		return apperrors.New(apperrors.ErrInvalidInput, "query가 입력되지 않았습니다")
	}
	return nil
}

type naverWatchNewPerformancesSearchResultData struct {
	HTML string `json:"html"`
}

type naverPerformance struct {
	Title     string `json:"title"`
	Place     string `json:"place"`
	Thumbnail string `json:"thumbnail"`
}

func (p *naverPerformance) String(messageTypeHTML bool, mark string) string {
	if messageTypeHTML == true {
		return fmt.Sprintf("☞ <a href=\"https://search.naver.com/search.naver?query=%s\"><b>%s</b></a>%s\n      • 장소 : %s", url.QueryEscape(p.Title), template.HTMLEscapeString(p.Title), mark, p.Place)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s%s\n      • 장소 : %s", template.HTMLEscapeString(p.Title), mark, p.Place))
}

type naverWatchNewPerformancesResultData struct {
	Performances []*naverPerformance `json:"performances"`
}

func init() {
	supportedTasks[TidNaver] = &supportedTaskConfig{
		commandConfigs: []*supportedTaskCommandConfig{{
			taskCommandID: TcidNaverWatchNewPerformances,

			allowMultipleInstances: true,

			newTaskResultDataFn: func() interface{} { return &naverWatchNewPerformancesResultData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData, appConfig *config.AppConfig) (taskHandler, error) {
			if taskRunData.taskID != TidNaver {
				return nil, apperrors.New(apperrors.ErrTaskNotFound, "등록되지 않은 작업입니다.😱")
			}

			task := &naverTask{
				task: task{
					id:         taskRunData.taskID,
					commandID:  taskRunData.taskCommandID,
					instanceID: instanceID,

					notifierID: taskRunData.notifierID,

					canceled: false,

					runBy: taskRunData.taskRunBy,

					fetcher: nil,
				},

				appConfig: appConfig,
			}

			retryDelay, err := time.ParseDuration(appConfig.HTTPRetry.RetryDelay)
			if err != nil {
				retryDelay, _ = time.ParseDuration(config.DefaultRetryDelay)
			}
			task.fetcher = NewRetryFetcher(&HTTPFetcher{}, appConfig.HTTPRetry.MaxRetries, retryDelay)

			task.runFn = func(taskResultData interface{}, messageTypeHTML bool) (string, interface{}, error) {
				switch task.CommandID() {
				case TcidNaverWatchNewPerformances:
					for _, t := range task.appConfig.Tasks {
						if task.ID() == TaskID(t.ID) {
							for _, c := range t.Commands {
								if task.CommandID() == TaskCommandID(c.ID) {
									taskCommandData := &naverWatchNewPerformancesTaskCommandData{}
									if err := fillTaskCommandDataFromMap(taskCommandData, c.Data); err != nil {
										return "", nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
									}
									if err := taskCommandData.validate(); err != nil {
										return "", nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, "작업 커맨드 데이터가 유효하지 않습니다")
									}

									return task.runWatchNewPerformances(taskCommandData, taskResultData, messageTypeHTML)
								}
							}
							break
						}
					}
				}

				return "", nil, ErrNoImplementationForTaskCommand
			}

			return task, nil
		},
	}
}

type naverTask struct {
	task

	appConfig *config.AppConfig
}

// noinspection GoUnhandledErrorResult,GoErrorStringFormat
func (t *naverTask) runWatchNewPerformances(taskCommandData *naverWatchNewPerformancesTaskCommandData, taskResultData interface{}, messageTypeHTML bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*naverWatchNewPerformancesResultData)
	if ok == false {
		return "", nil, apperrors.New(apperrors.ErrInternal, fmt.Sprintf("TaskResultData의 타입 변환이 실패하였습니다 (expected: *naverWatchNewPerformancesResultData, got: %T)", taskResultData))
	}

	actualityTaskResultData := &naverWatchNewPerformancesResultData{}
	titleIncludedKeywords := strutil.SplitExceptEmptyItems(taskCommandData.Filters.Title.IncludedKeywords, ",")
	titleExcludedKeywords := strutil.SplitExceptEmptyItems(taskCommandData.Filters.Title.ExcludedKeywords, ",")
	placeIncludedKeywords := strutil.SplitExceptEmptyItems(taskCommandData.Filters.Place.IncludedKeywords, ",")
	placeExcludedKeywords := strutil.SplitExceptEmptyItems(taskCommandData.Filters.Place.ExcludedKeywords, ",")

	// 전라도 지역 공연정보를 읽어온다.
	searchPerformancePageIndex := 1
	for {
		var searchResultData = &naverWatchNewPerformancesSearchResultData{}
		err = unmarshalFromResponseJSONData(t.fetcher, "GET", fmt.Sprintf("https://m.search.naver.com/p/csearch/content/nqapirender.nhn?key=kbList&pkid=269&where=nexearch&u7=%d&u8=all&u3=&u1=%s&u2=all&u4=ingplan&u6=N&u5=date", searchPerformancePageIndex, url.QueryEscape(taskCommandData.Query)), nil, nil, searchResultData)
		if err != nil {
			return "", nil, err
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(searchResultData.HTML))
		if err != nil {
			return "", nil, apperrors.Wrap(err, apperrors.ErrTaskExecutionFailed, "불러온 페이지의 데이터 파싱이 실패하였습니다")
		}

		// 읽어온 페이지에서 공연정보를 추출한다.
		ps := doc.Find("ul > li")
		ps.EachWithBreak(func(i int, s *goquery.Selection) bool {
			// 제목
			pis := s.Find("div.item > div.title_box > strong.name")
			if pis.Length() != 1 {
				err = apperrors.New(apperrors.ErrTaskExecutionFailed, "공연 제목 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
				return false
			}
			title := strings.TrimSpace(pis.Text())

			// 장소
			pis = s.Find("div.item > div.title_box > span.sub_text")
			if pis.Length() != 1 {
				err = apperrors.New(apperrors.ErrTaskExecutionFailed, "공연 장소 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
				return false
			}
			place := strings.TrimSpace(pis.Text())

			// 썸네일 이미지
			pis = s.Find("div.item > div.thumb > img")
			if pis.Length() != 1 {
				err = apperrors.New(apperrors.ErrTaskExecutionFailed, "공연 썸네일 이미지 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
				return false
			}
			thumbnailSrc, exists := pis.Attr("src")
			if exists == false {
				err = apperrors.New(apperrors.ErrTaskExecutionFailed, "공연 썸네일 이미지 추출이 실패하였습니다. CSS셀렉터를 확인하세요")
				return false
			}
			thumbnail := fmt.Sprintf(`<img src="%s">`, thumbnailSrc)

			if filter(title, titleIncludedKeywords, titleExcludedKeywords) == false || filter(place, placeIncludedKeywords, placeExcludedKeywords) == false {
				return true
			}

			actualityTaskResultData.Performances = append(actualityTaskResultData.Performances, &naverPerformance{
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
	err = eachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Performances, originTaskResultData.Performances, func(selem, telem interface{}) (bool, error) {
		actualityPerformance, ok1 := selem.(*naverPerformance)
		originPerformance, ok2 := telem.(*naverPerformance)
		if ok1 == false || ok2 == false {
			return false, apperrors.New(apperrors.ErrInternal, "selem/telem의 타입 변환이 실패하였습니다")
		} else {
			if actualityPerformance.Title == originPerformance.Title && actualityPerformance.Place == originPerformance.Place {
				return true, nil
			}
		}
		return false, nil
	}, nil, func(selem interface{}) {
		actualityPerformance := selem.(*naverPerformance)

		if m != "" {
			m += lineSpacing
		}
		m += actualityPerformance.String(messageTypeHTML, " 🆕")
	})
	if err != nil {
		return "", nil, err
	}

	if m != "" {
		message = "새로운 공연정보가 등록되었습니다.\n\n" + m
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.Performances) == 0 {
				message = "등록된 공연정보가 존재하지 않습니다."
			} else {
				for _, actualityPerformance := range actualityTaskResultData.Performances {
					if m != "" {
						m += lineSpacing
					}
					m += actualityPerformance.String(messageTypeHTML, "")
				}

				message = "신규로 등록된 공연정보가 없습니다.\n\n현재 등록된 공연정보는 아래와 같습니다:\n\n" + m
			}
		}
	}

	return message, changedTaskResultData, nil
}
