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
	// searchAPIBaseURL 네이버 모바일 통합검색의 내부 API 엔드포인트입니다.
	//
	// [목적]
	//  - 공연 정보를 JSON 형태로 비동기 수집(AJAX)하는 데 사용됩니다.
	//  - "https://m.search.naver.com" 도메인을 사용하여 모바일 환경에 최적화된 데이터를 응답받습니다.
	searchAPIBaseURL = "https://m.search.naver.com/p/csearch/content/nqapirender.nhn"

	// searchResultPageURL 사용자에게 제공할 '검색 결과 페이지'의 기본 URL입니다.
	//
	// [목적]
	//  - 알림 메시지에서 공연명을 클릭했을 때 이동할 하이퍼링크(Target URL)를 생성하는 데 사용됩니다.
	//  - 쿼리 파라미터(?query=...)를 추가하여 사용자가 해당 공연의 상세 검색 결과를 즉시 확인할 수 있도록 돕습니다.
	searchResultPageURL = "https://search.naver.com/search.naver"

	// newPerformanceMark 신규 공연 알림 메시지에 표시될 강조 마크입니다.
	newPerformanceMark = " 🆕"

	// ------------------------------------------------------------------------------------------------
	// CSS Selectors
	// ------------------------------------------------------------------------------------------------
	// 네이버 공연 검색 결과 페이지의 DOM 구조 변경에 대응하기 위한 CSS 선택자 상수를 정의합니다.
	// 각 선택자는 페이지의 특정 요소를 정확히 식별하고, 불필요한 요소(광고, 추천 목록 등)를 배제하도록 설계되었습니다.
	// ------------------------------------------------------------------------------------------------

	// selectorPerformanceItem 검색 결과 리스트에서 개별 공연 카드를 식별합니다.
	// 이 선택자로 추출된 각 요소를 순회하며 Title, Place, Thumbnail 정보를 파싱합니다.
	selectorPerformanceItem = "li:has(.title_box)"

	// selectorTitle 공연 카드 내부의 '공연명'을 추출합니다.
	selectorTitle = ".title_box .name"

	// selectorPlace 공연 카드 내부의 '장소/공연장' 정보를 추출합니다.
	selectorPlace = ".title_box .sub_text"

	// selectorThumbnail 공연 카드 내부의 공연 포스터 이미지의 URL을 추출합니다.
	selectorThumbnail = ".thumb img"
)

type watchNewPerformancesSettings struct {
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

	// 선택적 설정 (Optional Configuration)
	// 값이 제공되지 않을 경우 validate() 메서드에서 기본값이 자동으로 적용됩니다.
	MaxPages       int `json:"max_pages"`           // 최대 수집 페이지 수
	PageFetchDelay int `json:"page_fetch_delay_ms"` // 페이지 수집 간 대기 시간 (ms)
}

func (s *watchNewPerformancesSettings) validate() error {
	s.Query = strings.TrimSpace(s.Query)
	if s.Query == "" {
		return apperrors.New(apperrors.InvalidInput, "query가 입력되지 않았거나 공백입니다")
	}

	// 기본 설정값 적용
	if s.MaxPages <= 0 {
		s.MaxPages = 50
	}
	if s.PageFetchDelay <= 0 {
		s.PageFetchDelay = 100
	}

	return nil
}

// watchNewPerformancesSnapshot 신규 공연을 식별하기 위한 공연 데이터의 스냅샷입니다.
type watchNewPerformancesSnapshot struct {
	Performances []*performance `json:"performances"`
}

// performance 크롤링된 공연 정보를 담는 도메인 모델입니다.
type performance struct {
	Title     string `json:"title"`
	Place     string `json:"place"`
	Thumbnail string `json:"thumbnail"`
}

func (p *performance) Equals(other *performance) bool {
	if p == nil || other == nil {
		return false
	}
	return p.Title == other.Title && p.Place == other.Place
}

// Key 공연을 고유하게 식별하기 위한 문자열 키를 반환합니다.
//
// 반환값은 "제목|장소" 형식으로, 파이프(|) 문자를 구분자로 사용하여 제목과 장소를 결합합니다.
// 이 키는 Map 기반 중복 제거나 빠른 조회(O(1))가 필요한 상황에서 사용됩니다.
//
// [중요] 이 메서드의 비교 기준(Title + Place)은 Equals() 메서드와 반드시 일치해야 합니다.
// 만약 두 공연이 Equals()로 동일하다면, Key()도 동일한 값을 반환해야 합니다.
func (p *performance) Key() string {
	return fmt.Sprintf("%s|%s", p.Title, p.Place)
}

// String 수집된 공연 정보를 사용자에게 발송하기 위한 알림 메시지 포맷으로 변환합니다.
func (p *performance) String(supportsHTML bool, mark string) string {
	if supportsHTML {
		const htmlFormat = `☞ <a href="%s?query=%s"><b>%s</b></a>%s
      • 장소 : %s`

		return fmt.Sprintf(
			htmlFormat,
			searchResultPageURL,
			url.QueryEscape(p.Title),
			template.HTMLEscapeString(p.Title),
			mark,
			p.Place,
		)
	}

	const textFormat = `☞ %s%s
      • 장소 : %s`

	return strings.TrimSpace(fmt.Sprintf(textFormat, p.Title, mark, p.Place))
}

// keywordMatchers 문자열 기반의 필터 설정을 최적화된 Matcher로 변환한 필터 데이터입니다.
type keywordMatchers struct {
	TitleMatcher *strutil.KeywordMatcher
	PlaceMatcher *strutil.KeywordMatcher
}

// executeWatchNewPerformances 작업을 실행하여 신규 공연 정보를 확인합니다.
func (t *task) executeWatchNewPerformances(commandSettings *watchNewPerformancesSettings, prevSnapshot *watchNewPerformancesSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {
	// 1. 최신 공연 정보 수집
	currentPerformances, err := t.fetchPerformances(commandSettings)
	if err != nil {
		return "", nil, err
	}

	currentSnapshot := &watchNewPerformancesSnapshot{
		Performances: currentPerformances,
	}

	// 2. 신규 정보 확인 및 알림 메시지 생성
	return t.diffAndNotify(currentSnapshot, prevSnapshot, supportsHTML)
}

// fetchPerformances 네이버 통합검색 API를 페이지네이션하여 순회하며 신규 공연 정보를 수집합니다.
func (t *task) fetchPerformances(commandSettings *watchNewPerformancesSettings) ([]*performance, error) {
	// 매 페이지 순회 시마다 문자열 분할 연산이 반복되는 것을 방지하기 위해,
	// 루프 진입 전 1회만 수행하여 불변(Invariant) 데이터를 최적화된 Matcher 형태로 변환합니다.
	matchers := &keywordMatchers{
		TitleMatcher: strutil.NewKeywordMatcher(
			strutil.SplitAndTrim(commandSettings.Filters.Title.IncludedKeywords, ","),
			strutil.SplitAndTrim(commandSettings.Filters.Title.ExcludedKeywords, ","),
		),
		PlaceMatcher: strutil.NewKeywordMatcher(
			strutil.SplitAndTrim(commandSettings.Filters.Place.IncludedKeywords, ","),
			strutil.SplitAndTrim(commandSettings.Filters.Place.ExcludedKeywords, ","),
		),
	}

	// searchResponse 네이버 통합검색 API의 응답을 처리하기 위한 JSON 래퍼(Wrapper)입니다.
	type searchResponse struct {
		HTML string `json:"html"`
	}

	var currentPerformances []*performance

	// 중복 제거를 위한 맵
	// 라이브 서비스 특성상 수집 중 데이터가 밀려서 이전 페이지의 내용이 다음 페이지에 다시 나올 수 있으므로,
	// 세션 내에서 중복을 제거합니다.
	seen := make(map[string]bool)

	pageIndex := 1
	totalFetchedCount := 0
	for {
		// 작업 취소 여부 확인
		if t.IsCanceled() {
			t.LogWithContext("task.naver", logrus.WarnLevel, "작업 취소 요청이 감지되어 공연 정보 수집 프로세스를 중단합니다", logrus.Fields{
				"page_index":      pageIndex,
				"collected_count": len(currentPerformances),
				"fetched_count":   totalFetchedCount,
			}, nil)

			return nil, nil
		}

		if pageIndex > commandSettings.MaxPages {
			t.LogWithContext("task.naver", logrus.WarnLevel, "설정된 최대 페이지 수집 제한에 도달하여 프로세스를 조기 종료합니다", logrus.Fields{
				"limit_max_pages": commandSettings.MaxPages,
				"current_page":    pageIndex,
				"collected_count": len(currentPerformances),
				"fetched_count":   totalFetchedCount,
			}, nil)

			break
		}

		t.LogWithContext("task.naver", logrus.DebugLevel, "네이버 공연 검색 API 페이지를 요청합니다", logrus.Fields{
			"query":      commandSettings.Query,
			"page_index": pageIndex,
		}, nil)

		// API 요청 URL 생성
		searchAPIURL := buildSearchAPIURL(commandSettings.Query, pageIndex)

		var pageContent = &searchResponse{}
		err := tasksvc.FetchJSON(t.GetFetcher(), "GET", searchAPIURL, nil, nil, pageContent)
		if err != nil {
			return nil, err
		}

		// API로부터 수신한 비정형 HTML 데이터를 DOM 파싱하여 정형화된 공연 객체 리스트로 변환합니다.
		pagePerformances, rawCount, err := parsePerformancesFromHTML(pageContent.HTML, matchers)
		if err != nil {
			return nil, err
		}
		totalFetchedCount += rawCount

		// 중복 제거 및 결과 집계
		for _, p := range pagePerformances {
			key := p.Key()
			if seen[key] {
				continue
			}
			seen[key] = true
			currentPerformances = append(currentPerformances, p)
		}

		pageIndex += 1

		// 페이지네이션 종료 감지
		//
		// 현재 페이지에서 탐색된 원본 항목(Raw Count)이 0개라면, 더 이상 제공될 데이터가 없는 상태입니다.
		// 이는 모든 공연 정보를 수집했음을 의미하므로, 불필요한 추가 요청을 방지하기 위해 루프를 정상 종료합니다.
		if rawCount == 0 {
			t.LogWithContext("task.naver", logrus.DebugLevel, "페이지네이션 종료 조건(데이터 없음)에 도달하여 수집 프로세스를 정상 종료합니다", logrus.Fields{
				"last_visited_page": pageIndex - 1,
				"collected_count":   len(currentPerformances),
				"fetched_count":     totalFetchedCount,
			}, nil)

			break
		}

		time.Sleep(time.Duration(commandSettings.PageFetchDelay) * time.Millisecond)
	}

	t.LogWithContext("task.naver", logrus.InfoLevel, "공연 정보 수집 및 키워드 매칭 프로세스가 완료되었습니다", logrus.Fields{
		"collected_count": len(currentPerformances),
		"fetched_count":   totalFetchedCount,
		"request_pages":   pageIndex - 1,
	}, nil)

	return currentPerformances, nil
}

// buildSearchAPIURL 네이버 모바일 통합검색 내부 API 호출을 위한 전체 URL을 생성합니다.
func buildSearchAPIURL(query string, page int) string {
	params := url.Values{}
	params.Set("key", "kbList")     // 지식베이스(Knowledge Base) 리스트 식별자 (고정값)
	params.Set("pkid", "269")       // 공연/전시 정보 식별자 (고정값)
	params.Set("where", "nexearch") // 검색 영역 (통합검색)

	params.Set("u1", query)              // 검색어 (예: "jl")
	params.Set("u2", "all")              // 장르 필터 ("all": 전체)
	params.Set("u3", "")                 // 날짜 범위 ("": 전체 기간)
	params.Set("u4", "ingplan")          // 공연 상태 ("ingplan": 진행중/예정)
	params.Set("u5", "date")             // 정렬 순서 ("date": 최신순, "rank": 인기순)
	params.Set("u6", "N")                // 성인 공연 포함 여부 ("N": 제외)
	params.Set("u7", strconv.Itoa(page)) // 페이지 번호
	params.Set("u8", "all")              // 세부 장르 ("all": 전체)

	return fmt.Sprintf("%s?%s", searchAPIBaseURL, params.Encode())
}

// parsePerformancesFromHTML 수집된 HTML 문서(DOM)를 파싱하여 구조화된 공연 정보 목록으로 변환합니다.
//
// 반환값:
//   - []*performance: 사용자 정의 키워드 조건(Keywords)을 통과하여 최종 선별된 공연 정보 목록
//   - int (rawCount): 키워드 매칭 검사 전 탐색된 원본 항목의 총 개수 (페이지네이션 종료 조건 판별의 기준값)
//   - error: DOM 파싱 실패 또는 필수 요소 누락 등 구조적 변경으로 인한 치명적 에러
func parsePerformancesFromHTML(html string, matchers *keywordMatchers) ([]*performance, int, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, 0, apperrors.Wrap(err, apperrors.ExecutionFailed, "불러온 페이지의 데이터 파싱이 실패하였습니다")
	}

	// 읽어온 페이지에서 공연정보를 추출한다.
	performancesSelection := doc.Find(selectorPerformanceItem)

	// 키워드 매칭 검사 전 탐색된 원본 항목의 개수(Raw Count)입니다.
	// 이 값은 키워드 매칭 결과와는 독립적으로, 현재 페이지에 처리할 데이터가
	// 실제로 존재했는지를 나타내며 페이지네이션 루프의 종료 조건을 결정하는 핵심 지표로 사용됩니다.
	rawCount := performancesSelection.Length()

	// 미리 용량을 최대로 할당하여 메모리 재할당을 최소화한다.
	performances := make([]*performance, 0, rawCount)

	// 각 공연 아이템을 파싱하고 키워드 매칭 여부를 검사한다.
	var parseErr error
	performancesSelection.EachWithBreak(func(_ int, s *goquery.Selection) bool {
		perf, err := parsePerformance(s)
		if err != nil {
			parseErr = err
			return false // 순회 중단
		}

		if !matchers.TitleMatcher.Match(perf.Title) || !matchers.PlaceMatcher.Match(perf.Place) {
			// 키워드 매칭 실패 로깅 (Verbose)
			// t.LogWithContext("task.naver", logrus.TraceLevel, "키워드 매칭 조건에 의해 제외되었습니다", logrus.Fields{"title": perf.Title}, nil)
			return true // 계속 진행
		}

		performances = append(performances, perf)

		return true // 계속 진행
	})
	if parseErr != nil {
		return nil, 0, parseErr
	}

	return performances, rawCount, nil
}

// parsePerformance 단일 공연 정보를 파싱합니다.
func parsePerformance(s *goquery.Selection) (*performance, error) {
	// 제목
	titleSelection := s.Find(selectorTitle)
	if titleSelection.Length() != 1 {
		return nil, tasksvc.NewErrHTMLStructureChanged("", fmt.Sprintf("공연 제목 추출 실패 (선택자: %s, 발견된 요소 수: %d)", selectorTitle, titleSelection.Length()))
	}
	title := strings.TrimSpace(titleSelection.Text())
	if title == "" {
		return nil, tasksvc.NewErrHTMLStructureChanged("", fmt.Sprintf("공연 제목이 비어있습니다 (선택자: %s)", selectorTitle))
	}

	// 장소
	placeSelection := s.Find(selectorPlace)
	if placeSelection.Length() != 1 {
		return nil, tasksvc.NewErrHTMLStructureChanged("", fmt.Sprintf("공연 장소 추출 실패 (선택자: %s, 발견된 요소 수: %d)", selectorPlace, placeSelection.Length()))
	}
	place := strings.TrimSpace(placeSelection.Text())
	if place == "" {
		return nil, tasksvc.NewErrHTMLStructureChanged("", fmt.Sprintf("공연 장소가 비어있습니다 (선택자: %s)", selectorPlace))
	}

	// 썸네일 이미지가 없더라도 제목과 장소 정보가 있다면 수집하는 것이 운영상 유리하므로 에러를 반환하지 않습니다.
	var thumbnailSrc string
	thumbnailSelection := s.Find(selectorThumbnail)
	if thumbnailSelection.Length() > 0 {
		if src, exists := thumbnailSelection.Attr("src"); exists {
			thumbnailSrc = src
		}
	}

	return &performance{
		Title:     title,
		Place:     place,
		Thumbnail: thumbnailSrc,
	}, nil
}

// diffAndNotify 현재 스냅샷과 이전 스냅샷을 비교하여 변경된 공연을 확인하고 알림 메시지를 생성합니다.
func (t *task) diffAndNotify(currentSnapshot, prevSnapshot *watchNewPerformancesSnapshot, supportsHTML bool) (string, interface{}, error) {
	// 예상 메시지 크기로 초기 용량 할당 (공연당 약 300바이트 추정)
	var sb strings.Builder
	if len(currentSnapshot.Performances) > 0 {
		sb.Grow(len(currentSnapshot.Performances) * 300)
	}

	// 최초 실행 시에는 이전 스냅샷이 존재하지 않아 nil 상태일 수 있습니다.
	// 따라서 비교 대상을 명시적으로 nil(또는 빈 슬라이스)로 처리하여,
	// 1. nil 포인터 역참조(Nil Pointer Dereference)로 인한 런타임 패닉을 방지하고 (Safety)
	// 2. 현재 수집된 모든 공연 정보를 '신규'로 식별되도록 유도합니다. (Logic)
	var prevPerformances []*performance
	if prevSnapshot != nil {
		prevPerformances = prevSnapshot.Performances
	}

	// 빠른 조회를 위해 이전 공연 목록을 Map으로 변환한다.
	prevSet := make(map[string]bool, len(prevPerformances))
	for _, p := range prevPerformances {
		prevSet[p.Key()] = true
	}

	// 현재 공연 목록을 순회하며 신규 공연을 식별한다.
	lineSpacing := "\n\n"
	for _, p := range currentSnapshot.Performances {
		// 이전에 수집된 목록에 존재하지 않는다면 신규 공연으로 판단한다.
		if !prevSet[p.Key()] {
			if sb.Len() > 0 {
				sb.WriteString(lineSpacing)
			}
			sb.WriteString(p.String(supportsHTML, newPerformanceMark))
		}
	}
	if sb.Len() > 0 {
		return "새로운 공연정보가 등록되었습니다.\n\n" + sb.String(), currentSnapshot, nil
	}

	// 스케줄러(Scheduler)에 의한 자동 실행이 아닌, 사용자 요청에 의한 수동 실행인 경우입니다.
	//
	// 자동 실행 시에는 변경 사항이 없으면 불필요한 알림(Noise)을 방지하기 위해 침묵하지만,
	// 수동 실행 시에는 "변경 없음"이라는 명시적인 피드백을 제공하여 시스템이 정상 동작 중임을 사용자가 인지할 수 있도록 합니다.
	if t.GetRunBy() == tasksvc.RunByUser {
		if len(currentSnapshot.Performances) == 0 {
			return "등록된 공연정보가 존재하지 않습니다.", nil, nil
		}

		for _, p := range currentSnapshot.Performances {
			if sb.Len() > 0 {
				sb.WriteString(lineSpacing)
			}
			sb.WriteString(p.String(supportsHTML, ""))
		}
		return "신규로 등록된 공연정보가 없습니다.\n\n현재 등록된 공연정보는 아래와 같습니다:\n\n" + sb.String(), nil, nil
	}

	return "", nil, nil
}
