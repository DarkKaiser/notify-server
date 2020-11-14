package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/utils"
	log "github.com/sirupsen/logrus"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const (
	// TaskID
	TidNaver TaskID = "NAVER" // 네이버

	// TaskCommandID
	TcidNaverWatchNewPerformances TaskCommandID = "WatchNewPerformances" // 네이버 신규 공연정보 확인
)

type naverWatchNewPerformancesSearchResultData struct {
	Total int `json:"total"`
	List  []struct {
		Html string `json:"html"`
	} `json:"list"`
}

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
		return errors.New("query가 입력되지 않았습니다")
	}
	return nil
}

type naverWatchNewPerformancesResultData struct {
	Performances []struct {
		Title     string `json:"title"`
		Period    string `json:"period"`
		Place     string `json:"place"`
		Thumbnail string `json:"thumbnail"`
	} `json:"performances"`
}

func init() {
	supportedTasks[TidNaver] = &supportedTaskConfig{
		commandConfigs: []*supportedTaskCommandConfig{{
			taskCommandID: TcidNaverWatchNewPerformances,

			allowMultipleIntances: true,

			newTaskResultDataFn: func() interface{} { return &naverWatchNewPerformancesResultData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData, config *g.AppConfig) (taskHandler, error) {
			if taskRunData.taskID != TidNaver {
				return nil, errors.New("등록되지 않은 작업입니다.😱")
			}

			task := &naverTask{
				task: task{
					id:         taskRunData.taskID,
					commandID:  taskRunData.taskCommandID,
					instanceID: instanceID,

					notifierID: taskRunData.notifierID,

					canceled: false,

					runBy: taskRunData.taskRunBy,
				},

				config: config,
			}

			task.runFn = func(taskResultData interface{}, isSupportedHTMLMessage bool) (string, interface{}, error) {
				switch task.CommandID() {
				case TcidNaverWatchNewPerformances:
					for _, t := range task.config.Tasks {
						if task.ID() == TaskID(t.ID) {
							for _, c := range t.Commands {
								if task.CommandID() == TaskCommandID(c.ID) {
									taskCommandData := &naverWatchNewPerformancesTaskCommandData{}
									if err := fillTaskCommandDataFromMap(taskCommandData, c.Data); err != nil {
										return "", nil, errors.New(fmt.Sprintf("작업 커맨드 데이터가 유효하지 않습니다.(error:%s)", err))
									}
									if err := taskCommandData.validate(); err != nil {
										return "", nil, errors.New(fmt.Sprintf("작업 커맨드 데이터가 유효하지 않습니다.(error:%s)", err))
									}

									return task.runWatchNewPerformances(taskCommandData, taskResultData, isSupportedHTMLMessage)
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

	config *g.AppConfig
}

//noinspection GoUnhandledErrorResult,GoErrorStringFormat
func (t *naverTask) runWatchNewPerformances(taskCommandData *naverWatchNewPerformancesTaskCommandData, taskResultData interface{}, isSupportedHTMLMessage bool) (message string, changedTaskResultData interface{}, err error) {
	originTaskResultData, ok := taskResultData.(*naverWatchNewPerformancesResultData)
	if ok == false {
		log.Panic("TaskResultData의 타입 변환이 실패하였습니다.")
	}

	actualityTaskResultData := &naverWatchNewPerformancesResultData{}
	titleIncludedKeywords := utils.SplitExceptEmptyItems(taskCommandData.Filters.Title.IncludedKeywords, ",")
	titleExcludedKeywords := utils.SplitExceptEmptyItems(taskCommandData.Filters.Title.ExcludedKeywords, ",")
	placeIncludedKeywords := utils.SplitExceptEmptyItems(taskCommandData.Filters.Place.IncludedKeywords, ",")
	placeExcludedKeywords := utils.SplitExceptEmptyItems(taskCommandData.Filters.Place.ExcludedKeywords, ",")

	// 전라도 지역 공연정보를 읽어온다.
	searchStartPerformancePos := 1
	for {
		resp, err0 := http.Get(fmt.Sprintf("https://m.search.naver.com/p/csearch/content/qapirender.nhn?key=PerformListAPI&where=nexearch&pkid=269&q=%s&so=&start=%d", url.QueryEscape(taskCommandData.Query), searchStartPerformancePos))
		if err0 != nil {
			return "", nil, fmt.Errorf("Web 페이지 접근이 실패하였습니다.(error:%s)", err0)
		}
		if resp.StatusCode != http.StatusOK {
			return "", nil, fmt.Errorf("Web 페이지 접근이 실패하였습니다.(%s)", resp.Status)
		}

		bodyBytes, err0 := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err0 != nil {
			return "", nil, fmt.Errorf("응답 데이터 읽기가 실패하였습니다.(error:%s)", err0)
		}

		var searchResultData naverWatchNewPerformancesSearchResultData
		err0 = json.Unmarshal(bodyBytes, &searchResultData)
		if err0 != nil {
			return "", nil, fmt.Errorf("응답 데이터의 JSON 변환이 실패하였습니다.(error:%s)", err0)
		}

		document, err0 := goquery.NewDocumentFromReader(strings.NewReader(searchResultData.List[0].Html))
		if err0 != nil {
			return "", nil, fmt.Errorf("HTML 파싱이 실패하였습니다.(error:%s)", err0)
		}

		// 읽어온 페이지에서 공연정보를 추출한다.
		ps := document.Find("ul > li")
		ps.EachWithBreak(func(i int, s *goquery.Selection) bool {
			// 제목
			pis := s.Find("div.list_title a.tit")
			if pis.Length() != 1 {
				err = errors.New("공연 제목 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
				return false
			}
			title := strings.TrimSpace(pis.Text())

			// 기간
			pis = s.Find("div.list_title > span.period")
			if pis.Length() != 1 {
				err = errors.New("공연 기간 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
				return false
			}
			period := strings.TrimSpace(pis.Text())

			period = strings.Replace(period, ".", "년 ", 1)
			period = strings.Replace(period, ".", "월 ", 1)
			period = strings.Replace(period, ".", "일", 1)
			period = strings.Replace(period, ".", "년 ", 1)
			period = strings.Replace(period, ".", "월 ", 1)
			period = strings.Replace(period, ".", "일", 1)
			period = strings.Replace(period, "~", " ~ ", 1)

			// 장소
			pis = s.Find("div.list_title > span.list_cate")
			if pis.Length() != 1 {
				err = errors.New("공연 장소 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
				return false
			}
			place := strings.TrimSpace(pis.Text())

			// 썸네일 이미지
			pis = s.Find("div.list_thumb > a > img")
			if pis.Length() != 1 {
				err = errors.New("공연 썸네일 이미지 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
				return false
			}
			thumbnailSrc, exists := pis.Attr("src")
			if exists == false {
				err = errors.New("공연 썸네일 이미지 추출이 실패하였습니다. CSS셀렉터를 확인하세요.")
				return false
			}
			thumbnail := fmt.Sprintf(`<img src="%s">`, thumbnailSrc)

			if filter(title, titleIncludedKeywords, titleExcludedKeywords) == false || filter(place, placeIncludedKeywords, placeExcludedKeywords) == false {
				return true
			}

			actualityTaskResultData.Performances = append(actualityTaskResultData.Performances, struct {
				Title     string `json:"title"`
				Period    string `json:"period"`
				Place     string `json:"place"`
				Thumbnail string `json:"thumbnail"`
			}{
				Title:     title,
				Period:    period,
				Place:     place,
				Thumbnail: thumbnail,
			})

			return true
		})
		if err != nil {
			return "", nil, err
		}

		searchStartPerformancePos += ps.Length()
		if searchStartPerformancePos > searchResultData.Total || ps.Length() == 0 {
			break
		}
	}

	// 신규 공연정보를 확인한다.
	var m = ""
	var existsNewPerformances = false
	for _, actualityPerformance := range actualityTaskResultData.Performances {
		for _, originPerformance := range originTaskResultData.Performances {
			if actualityPerformance.Title == originPerformance.Title && actualityPerformance.Period == originPerformance.Period && actualityPerformance.Place == originPerformance.Place {
				goto NEXTITEM
			}
		}

		existsNewPerformances = true

		if isSupportedHTMLMessage == true {
			if m != "" {
				m += "\n\n"
			}
			m = fmt.Sprintf("%s☞ <a href=\"https://search.naver.com/search.naver?query=%s\"><b>%s</b></a> 🆕\n      • 일정 : %s\n      • 장소 : %s", m, url.QueryEscape(actualityPerformance.Title), template.HTMLEscapeString(actualityPerformance.Title), actualityPerformance.Period, actualityPerformance.Place)
		} else {
			if m != "" {
				m += "\n\n"
			}
			m = fmt.Sprintf("%s☞ %s 🆕\n      • 일정 : %s\n      • 장소 : %s", m, template.HTMLEscapeString(actualityPerformance.Title), actualityPerformance.Period, actualityPerformance.Place)
		}

	NEXTITEM:
	}

	if existsNewPerformances == true {
		message = fmt.Sprintf("신규 공연정보가 등록되었습니다.\n\n%s", m)
		changedTaskResultData = actualityTaskResultData
	} else {
		if t.runBy == TaskRunByUser {
			if len(actualityTaskResultData.Performances) == 0 {
				message = "등록된 공연정보가 존재하지 않습니다."
			} else {
				message = "신규 공연정보가 없습니다.\n\n현재 진행중인 공연정보는 아래와 같습니다:"

				if isSupportedHTMLMessage == true {
					for _, actualityPerformance := range actualityTaskResultData.Performances {
						message = fmt.Sprintf("%s\n\n☞ <a href=\"https://search.naver.com/search.naver?query=%s\"><b>%s</b></a>\n      • 일정 : %s\n      • 장소 : %s", message, url.QueryEscape(actualityPerformance.Title), template.HTMLEscapeString(actualityPerformance.Title), actualityPerformance.Period, actualityPerformance.Place)
					}
				} else {
					for _, actualityPerformance := range actualityTaskResultData.Performances {
						message = fmt.Sprintf("%s\n\n☞ %s\n      • 일정 : %s\n      • 장소 : %s", message, template.HTMLEscapeString(actualityPerformance.Title), actualityPerformance.Period, actualityPerformance.Place)
					}
				}
			}
		}
	}

	return message, changedTaskResultData, nil
}
