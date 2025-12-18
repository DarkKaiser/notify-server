package naver

import (
	"fmt"
	"html/template"
	"net/url"
	"strconv"
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

// executeWatchNewPerformances 작업을 실행하여 신규 공연 정보를 확인합니다.
func (t *task) executeWatchNewPerformances(commandConfig *watchNewPerformancesCommandConfig, prevSnapshot *watchNewPerformancesSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {
	// 1. 최신 공연 정보 수집
	newPerformances, err := t.fetchPerformances(commandConfig)
	if err != nil {
		return "", nil, err
	}

	actualityTaskResultData := &watchNewPerformancesSnapshot{
		Performances: newPerformances,
	}

	// 2. 신규 정보 확인 및 알림 메시지 생성
	return t.diffAndNotify(actualityTaskResultData, prevSnapshot, supportsHTML)
}

// fetchPerformances 네이버 검색 페이지를 순회하며 공연 정보를 수집합니다.
func (t *task) fetchPerformances(commandConfig *watchNewPerformancesCommandConfig) ([]*performance, error) {
	var performances []*performance
	titleIncludedKeywords := strutil.SplitAndTrim(commandConfig.Filters.Title.IncludedKeywords, ",")
	titleExcludedKeywords := strutil.SplitAndTrim(commandConfig.Filters.Title.ExcludedKeywords, ",")
	placeIncludedKeywords := strutil.SplitAndTrim(commandConfig.Filters.Place.IncludedKeywords, ",")
	placeExcludedKeywords := strutil.SplitAndTrim(commandConfig.Filters.Place.ExcludedKeywords, ",")

	searchPerformancePageIndex := 1
	for {
		var searchResultData = &performanceSearchResponse{}
		baseURL := "https://m.search.naver.com/p/csearch/content/nqapirender.nhn"
		params := url.Values{}
		params.Set("key", "kbList")
		params.Set("pkid", "269")
		params.Set("where", "nexearch")
		params.Set("u1", commandConfig.Query)
		params.Set("u2", "all")
		params.Set("u3", "")
		params.Set("u4", "ingplan")
		params.Set("u5", "date")
		params.Set("u6", "N")
		params.Set("u7", strconv.Itoa(searchPerformancePageIndex))
		params.Set("u8", "all")

		err := tasksvc.FetchJSON(t.GetFetcher(), "GET", fmt.Sprintf("%s?%s", baseURL, params.Encode()), nil, nil, searchResultData)
		if err != nil {
			return nil, err
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(searchResultData.HTML))
		if err != nil {
			return nil, apperrors.Wrap(err, apperrors.ExecutionFailed, "불러온 페이지의 데이터 파싱이 실패하였습니다")
		}

		// 읽어온 페이지에서 공연정보를 추출한다.
		ps := doc.Find("ul > li")
		ps.EachWithBreak(func(i int, s *goquery.Selection) bool {
			p, parseErr := parsePerformance(s)
			if parseErr != nil {
				err = parseErr
				return false
			}

			if !tasksvc.Filter(p.Title, titleIncludedKeywords, titleExcludedKeywords) || !tasksvc.Filter(p.Place, placeIncludedKeywords, placeExcludedKeywords) {
				return true
			}

			performances = append(performances, p)
			return true
		})
		if err != nil {
			return nil, err
		}

		searchPerformancePageIndex += 1

		// 불러온 데이터가 없는 경우, 모든 공연정보를 불러온 것으로 인식한다.
		if ps.Length() == 0 {
			break
		}

		time.Sleep(pageFetchDelay)
	}

	return performances, nil
}

// parsePerformance 단일 공연 정보를 파싱합니다.
func parsePerformance(s *goquery.Selection) (*performance, error) {
	// 제목
	pis := s.Find("div.item > div.title_box > strong.name")
	if pis.Length() != 1 {
		return nil, tasksvc.NewErrHTMLStructureChanged("", "공연 제목 추출이 실패하였습니다")
	}
	title := strings.TrimSpace(pis.Text())

	// 장소
	pis = s.Find("div.item > div.title_box > span.sub_text")
	if pis.Length() != 1 {
		return nil, tasksvc.NewErrHTMLStructureChanged("", "공연 장소 추출이 실패하였습니다")
	}
	place := strings.TrimSpace(pis.Text())

	// 썸네일 이미지
	pis = s.Find("div.item > div.thumb > img")
	if pis.Length() != 1 {
		return nil, tasksvc.NewErrHTMLStructureChanged("", "공연 썸네일 이미지 추출이 실패하였습니다")
	}
	thumbnailSrc, exists := pis.Attr("src")
	if !exists {
		return nil, tasksvc.NewErrHTMLStructureChanged("", "공연 썸네일 이미지 추출이 실패하였습니다")
	}
	thumbnail := fmt.Sprintf(`<img src="%s">`, thumbnailSrc)

	return &performance{
		Title:     title,
		Place:     place,
		Thumbnail: thumbnail,
	}, nil
}

// diffAndNotify 이전 스냅샷과 비교하여 변경 사항을 알림 메시지로 생성합니다.
func (t *task) diffAndNotify(currentSnapshot, prevSnapshot *watchNewPerformancesSnapshot, supportsHTML bool) (string, interface{}, error) {
	m := ""
	lineSpacing := "\n\n"
	err := tasksvc.EachSourceElementIsInTargetElementOrNot(currentSnapshot.Performances, prevSnapshot.Performances, func(selem, telem interface{}) (bool, error) {
		actualityPerformance, ok1 := selem.(*performance)
		originPerformance, ok2 := telem.(*performance)
		if !ok1 || !ok2 {
			return false, tasksvc.NewErrTypeAssertionFailed("selm/telm", &performance{}, selem)
		}
		if actualityPerformance.Title == originPerformance.Title && actualityPerformance.Place == originPerformance.Place {
			return true, nil
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
		return "새로운 공연정보가 등록되었습니다.\n\n" + m, currentSnapshot, nil
	}

	if t.GetRunBy() == tasksvc.RunByUser {
		if len(currentSnapshot.Performances) == 0 {
			return "등록된 공연정보가 존재하지 않습니다.", nil, nil
		}

		for _, actualityPerformance := range currentSnapshot.Performances {
			if m != "" {
				m += lineSpacing
			}
			m += actualityPerformance.String(supportsHTML, "")
		}
		return "신규로 등록된 공연정보가 없습니다.\n\n현재 등록된 공연정보는 아래와 같습니다:\n\n" + m, nil, nil
	}

	return "", nil, nil
}
