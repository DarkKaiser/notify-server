package naver

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
)

const (
	// pageFetchDelay 페이지 요청 간 대기 시간 (API Rate Limiting 방지)
	pageFetchDelay = 100 * time.Millisecond
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
		return apperrors.New(apperrors.InvalidInput, "query가 입력되지 않았습니다")
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
	if messageTypeHTML {
		return fmt.Sprintf("☞ <a href=\"https://search.naver.com/search.naver?query=%s\"><b>%s</b></a>%s\n      • 장소 : %s", url.QueryEscape(p.Title), template.HTMLEscapeString(p.Title), mark, p.Place)
	}
	return strings.TrimSpace(fmt.Sprintf("☞ %s%s\n      • 장소 : %s", template.HTMLEscapeString(p.Title), mark, p.Place))
}

type watchNewPerformancesSnapshot struct {
	Performances []*performance `json:"performances"`
}

// noinspection GoUnhandledErrorResult,GoErrorStringFormat
func (t *task) executeWatchNewPerformances(commandConfig *watchNewPerformancesCommandConfig, prevSnapshot *watchNewPerformancesSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {
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
			return "", nil, apperrors.Wrap(err, apperrors.ExecutionFailed, "불러온 페이지의 데이터 파싱이 실패하였습니다")
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
			if !exists {
				err = tasksvc.NewErrHTMLStructureChanged("", "공연 썸네일 이미지 추출이 실패하였습니다")
				return false
			}
			thumbnail := fmt.Sprintf(`<img src="%s">`, thumbnailSrc)

			if !tasksvc.Filter(title, titleIncludedKeywords, titleExcludedKeywords) || !tasksvc.Filter(place, placeIncludedKeywords, placeExcludedKeywords) {
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

		time.Sleep(pageFetchDelay)
	}

	// 신규 공연정보를 확인한다.
	m := ""
	lineSpacing := "\n\n"
	err = tasksvc.EachSourceElementIsInTargetElementOrNot(actualityTaskResultData.Performances, prevSnapshot.Performances, func(selem, telem interface{}) (bool, error) {
		actualityPerformance, ok1 := selem.(*performance)
		originPerformance, ok2 := telem.(*performance)
		if !ok1 || !ok2 {
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
