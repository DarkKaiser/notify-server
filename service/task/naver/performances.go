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
	"github.com/sirupsen/logrus"
)

const (
	// searchBaseURL 네이버 검색 API의 엔드포인트 URL입니다.
	searchBaseURL = "https://m.search.naver.com/p/csearch/content/nqapirender.nhn"

	// CSS Selectors
	// selectorPerformanceItem 네이버 공연 검색 결과의 리스트 컨테이너(ul) 내에서
	// 개별 공연 정보 카드(li)를 식별하여 순회하기 위한 최상위 선택자입니다.
	selectorPerformanceItem = "ul > li"

	// selectorTitle 공연 정보 카드 내 타이틀 영역(div.title_box)에 위치한
	// 실제 공연명 텍스트(strong.name)를 추출하기 위한 선택자입니다.
	selectorTitle = "div.item > div.title_box > strong.name"

	// selectorPlace 타이틀 영역 하단에 위치하며, 공연 장소 정보(span.sub_text)를
	// 텍스트 형태로 포함하고 있는 요소를 지칭합니다.
	selectorPlace = "div.item > div.title_box > span.sub_text"

	// selectorThumbnail 공연 정보 카드의 좌측 썸네일 영역(div.thumb) 내에 존재하는
	// 이미지 태그(img)를 선택하여 src 속성을 추출하기 위해 사용됩니다.
	selectorThumbnail = "div.item > div.thumb > img"
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

	// Optional Configuration (기본값 제공됨)
	MaxPages       int `json:"max_pages"`           // 최대 수집 페이지 수
	PageFetchDelay int `json:"page_fetch_delay_ms"` // 페이지 수집 간 대기 시간 (ms)

	// parsedFilters 필터링 키워드 파싱 결과 캐시 (Eagerly initialized)
	parsedFilters *parsedFilters `json:"-"`
}

type parsedFilters struct {
	TitleIncluded []string
	TitleExcluded []string
	PlaceIncluded []string
	PlaceExcluded []string
}

func (c *watchNewPerformancesCommandConfig) validate() error {
	if c.Query == "" {
		return apperrors.New(apperrors.InvalidInput, "query가 입력되지 않았습니다")
	}

	// 기본 설정값 적용
	if c.MaxPages <= 0 {
		c.MaxPages = 50
	}
	if c.PageFetchDelay <= 0 {
		c.PageFetchDelay = 100
	}

	// 필터 미리 파싱 (Eager Initialization for Thread Safety)
	c.parsedFilters = &parsedFilters{
		TitleIncluded: strutil.SplitAndTrim(c.Filters.Title.IncludedKeywords, ","),
		TitleExcluded: strutil.SplitAndTrim(c.Filters.Title.ExcludedKeywords, ","),
		PlaceIncluded: strutil.SplitAndTrim(c.Filters.Place.IncludedKeywords, ","),
		PlaceExcluded: strutil.SplitAndTrim(c.Filters.Place.ExcludedKeywords, ","),
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

func (p *performance) Equals(other *performance) bool {
	return p.Title == other.Title && p.Place == other.Place
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
	// 이미 validate() 시점에 파싱된 안전한 필터 사용
	filters := commandConfig.parsedFilters

	searchPerformancePageIndex := 1

	for {
		// 작업 취소 여부 확인
		if t.IsCanceled() {
			logrus.Info("작업이 취소되어 공연 정보 수집을 중단합니다")
			return nil, nil
		}

		if searchPerformancePageIndex > commandConfig.MaxPages {
			logrus.Warnf("최대 페이지 수(%d)를 초과하여 수집을 조기 종료합니다", commandConfig.MaxPages)
			break
		}

		// 페이지네이션 로깅
		logrus.WithFields(logrus.Fields{
			"page":  searchPerformancePageIndex,
			"query": commandConfig.Query,
		}).Debug("공연 정보 페이지를 수집합니다")

		var searchResultData = &performanceSearchResponse{}
		params := url.Values{}
		params.Set("key", "kbList")                                // 지식베이스(Knowledge Base) 리스트 식별자 (고정값)
		params.Set("pkid", "269")                                  // 공연/전시 정보 식별자 (269: 공연/전시)
		params.Set("where", "nexearch")                            // 검색 영역
		params.Set("u1", commandConfig.Query)                      // 검색어 (지역명 등)
		params.Set("u2", "all")                                    // 장르 (all: 전체)
		params.Set("u3", "")                                       // 날짜 범위 (빈 문자열: 전체)
		params.Set("u4", "ingplan")                                // 공연 상태 (ingplan: 진행중/예정)
		params.Set("u5", "date")                                   // 정렬 순서 (date: 최신순)
		params.Set("u6", "N")                                      // 성인 공연 포함 여부 (N: 제외)
		params.Set("u7", strconv.Itoa(searchPerformancePageIndex)) // 페이지 번호
		params.Set("u8", "all")                                    // 세부 장르 (all: 전체)

		err := tasksvc.FetchJSON(t.GetFetcher(), "GET", fmt.Sprintf("%s?%s", searchBaseURL, params.Encode()), nil, nil, searchResultData)
		if err != nil {
			return nil, err
		}

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(searchResultData.HTML))
		if err != nil {
			return nil, apperrors.Wrap(err, apperrors.ExecutionFailed, "불러온 페이지의 데이터 파싱이 실패하였습니다")
		}

		// 읽어온 페이지에서 공연정보를 추출한다.
		ps := doc.Find(selectorPerformanceItem)
		ps.EachWithBreak(func(i int, s *goquery.Selection) bool {
			p, parseErr := parsePerformance(s)
			if parseErr != nil {
				err = parseErr
				return false
			}

			if !tasksvc.Filter(p.Title, filters.TitleIncluded, filters.TitleExcluded) || !tasksvc.Filter(p.Place, filters.PlaceIncluded, filters.PlaceExcluded) {
				// 필터링 로깅 (Verbose)
				// logrus.WithField("title", p.Title).Trace("필터 조건에 의해 제외되었습니다")
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
			logrus.WithField("last_page", searchPerformancePageIndex-1).Debug("더 이상 공연 정보가 없어 수집을 종료합니다")
			break
		}

		time.Sleep(time.Duration(commandConfig.PageFetchDelay) * time.Millisecond)
	}

	logrus.WithField("total_count", len(performances)).Info("공연 정보 수집을 완료했습니다")
	return performances, nil
}

// parsePerformance 단일 공연 정보를 파싱합니다.
func parsePerformance(s *goquery.Selection) (*performance, error) {
	// 제목
	pis := s.Find(selectorTitle)
	if pis.Length() != 1 {
		return nil, tasksvc.NewErrHTMLStructureChanged("", "공연 제목 추출이 실패하였습니다")
	}
	title := strings.TrimSpace(pis.Text())

	// 장소
	pis = s.Find(selectorPlace)
	if pis.Length() != 1 {
		return nil, tasksvc.NewErrHTMLStructureChanged("", "공연 장소 추출이 실패하였습니다")
	}
	place := strings.TrimSpace(pis.Text())

	// 썸네일 이미지
	pis = s.Find(selectorThumbnail)
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
	var sb strings.Builder
	lineSpacing := "\n\n"
	err := tasksvc.EachSourceElementIsInTargetElementOrNot(currentSnapshot.Performances, prevSnapshot.Performances, func(selem, telem interface{}) (bool, error) {
		actualityPerformance, ok1 := selem.(*performance)
		originPerformance, ok2 := telem.(*performance)
		if !ok1 || !ok2 {
			return false, tasksvc.NewErrTypeAssertionFailed("selm/telm", &performance{}, selem)
		}
		if actualityPerformance.Equals(originPerformance) {
			return true, nil
		}
		return false, nil
	}, nil, func(selem interface{}) {
		actualityPerformance := selem.(*performance)

		if sb.Len() > 0 {
			sb.WriteString(lineSpacing)
		}
		sb.WriteString(actualityPerformance.String(supportsHTML, " 🆕"))
	})
	if err != nil {
		return "", nil, err
	}

	if sb.Len() > 0 {
		return "새로운 공연정보가 등록되었습니다.\n\n" + sb.String(), currentSnapshot, nil
	}

	if t.GetRunBy() == tasksvc.RunByUser {
		if len(currentSnapshot.Performances) == 0 {
			return "등록된 공연정보가 존재하지 않습니다.", nil, nil
		}

		for _, actualityPerformance := range currentSnapshot.Performances {
			if sb.Len() > 0 {
				sb.WriteString(lineSpacing)
			}
			sb.WriteString(actualityPerformance.String(supportsHTML, ""))
		}
		return "신규로 등록된 공연정보가 없습니다.\n\n현재 등록된 공연정보는 아래와 같습니다:\n\n" + sb.String(), nil, nil
	}

	return "", nil, nil
}
