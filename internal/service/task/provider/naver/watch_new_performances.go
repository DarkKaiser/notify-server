package naver

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

const (
	// searchAPIBaseURL 네이버 모바일 통합검색의 내부 API 엔드포인트입니다.
	//
	// [목적]
	//  - 공연 정보를 JSON 형태로 비동기 수집(AJAX)하는 데 사용됩니다.
	//  - "https://m.search.naver.com" 도메인을 사용하여 모바일 환경에 최적화된 데이터를 응답받습니다.
	searchAPIBaseURL = "https://m.search.naver.com/p/csearch/content/nqapirender.nhn"

	// allocSizePerPerformance 알림 메시지 생성 시, 단일 공연 정보를 렌더링하는 데 필요한 예상 버퍼 크기(Byte)입니다.
	//
	// 이 상수는 `strings.Builder.Grow()`를 통해 내부 버퍼를 선제적으로 확보(Pre-allocation)하는 데 사용됩니다.
	// 적절한 초기 용량을 설정함으로써, 메시지 조합 과정에서 발생하는 불필요한 슬라이스 재할당(Reallocation)과
	// 데이터 복사(Memory Copy) 비용을 최소화하여 렌더링 성능을 최적화합니다.
	allocSizePerPerformance = 300

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

// performanceEventType 공연 데이터의 상태 변화(변경 유형)를 식별하기 위한 열거형입니다.
type performanceEventType int

const (
	eventNone           performanceEventType = iota
	eventNewPerformance                      // 신규 공연 등록
)

// performanceDiff 공연 데이터의 변경 사항(신규 등록 등)을 표현하는 중간 객체입니다.
type performanceDiff struct {
	Type        performanceEventType
	Performance *performance
}

// keywordMatchers 문자열 기반의 필터 설정을 반복 사용에 최적화된 Matcher 객체로 변환하여 캡슐화한 구조체입니다.
// (매우 빈번하게 호출되는 필터링 로직에서 문자열 분할 비용을 제거하기 위함)
type keywordMatchers struct {
	TitleMatcher *strutil.KeywordMatcher
	PlaceMatcher *strutil.KeywordMatcher
}

// executeWatchNewPerformances 작업을 실행하여 신규 공연 정보를 확인합니다.
func (t *task) executeWatchNewPerformances(ctx context.Context, commandSettings *watchNewPerformancesSettings, prevSnapshot *watchNewPerformancesSnapshot, supportsHTML bool) (message string, changedTaskResultData interface{}, err error) {
	// 1. 최신 공연 정보를 수집한다.
	currentPerformances, err := t.fetchPerformances(ctx, commandSettings)
	if err != nil {
		return "", nil, err
	}

	currentSnapshot := &watchNewPerformancesSnapshot{
		Performances: currentPerformances,
	}

	// 2, 빠른 조회를 위해 이전 공연 목록을 Map(Set)으로 변환한다.
	prevPerformancesSet := make(map[string]bool)
	if prevSnapshot != nil {
		for _, p := range prevSnapshot.Performances {
			prevPerformancesSet[p.Key()] = true
		}
	}

	// 3. 신규 정보 확인 및 알림 메시지 생성
	message, shouldSave := t.analyzeAndReport(currentSnapshot, prevPerformancesSet, supportsHTML)

	if shouldSave {
		// "변경 사항이 있다면(shouldSave=true), 반드시 알림 메시지도 존재해야 한다"는 규칙을 확인합니다.
		// 만약 메시지 없이 데이터만 갱신되면, 사용자는 변경 사실을 영영 모르게 될 수 있습니다.
		// 이를 방지하기 위해, 이런 비정상적인 상황에서는 저장을 차단하고 즉시 로그를 남깁니다.
		if message == "" {
			t.LogWithContext("task.naver", applog.WarnLevel, "변경 사항 감지 후 저장 프로세스를 시도했으나, 알림 메시지가 비어있습니다 (저장 건너뜀)", nil, nil)
			return "", nil, nil
		}

		return message, currentSnapshot, nil
	}

	return message, nil, nil
}

// fetchPerformances 네이버 검색 API를 호출하여 조건에 맞는 공연 목록을 수집합니다.
func (t *task) fetchPerformances(ctx context.Context, commandSettings *watchNewPerformancesSettings) ([]*performance, error) {
	// 매 페이지 순회 시마다 문자열 분할 연산이 반복되는 것을 방지하기 위해,
	// 루프 진입 전 1회만 수행하여 불변(Invariant) 데이터를 최적화된 Matcher 형태로 변환합니다.
	matchers := &keywordMatchers{
		TitleMatcher: strutil.NewKeywordMatcher(
			strutil.SplitClean(commandSettings.Filters.Title.IncludedKeywords, ","),
			strutil.SplitClean(commandSettings.Filters.Title.ExcludedKeywords, ","),
		),
		PlaceMatcher: strutil.NewKeywordMatcher(
			strutil.SplitClean(commandSettings.Filters.Place.IncludedKeywords, ","),
			strutil.SplitClean(commandSettings.Filters.Place.ExcludedKeywords, ","),
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
			t.LogWithContext("task.naver", applog.WarnLevel, "작업 취소 요청이 감지되어 공연 정보 수집 프로세스를 중단합니다", applog.Fields{
				"page_index":      pageIndex,
				"collected_count": len(currentPerformances),
				"fetched_count":   totalFetchedCount,
			}, nil)

			return nil, nil
		}

		if pageIndex > commandSettings.MaxPages {
			t.LogWithContext("task.naver", applog.WarnLevel, "설정된 최대 페이지 수집 제한에 도달하여 프로세스를 조기 종료합니다", applog.Fields{
				"limit_max_pages": commandSettings.MaxPages,
				"current_page":    pageIndex,
				"collected_count": len(currentPerformances),
				"fetched_count":   totalFetchedCount,
			}, nil)

			break
		}

		t.LogWithContext("task.naver", applog.DebugLevel, "네이버 공연 검색 API 페이지를 요청합니다", applog.Fields{
			"query":      commandSettings.Query,
			"page_index": pageIndex,
		}, nil)

		// API 요청 URL 생성
		searchAPIURL := buildSearchAPIURL(commandSettings.Query, pageIndex)

		var pageContent = &searchResponse{}
		err := t.GetScraper().FetchJSON(ctx, "GET", searchAPIURL, nil, nil, pageContent)
		if err != nil {
			return nil, err
		}

		// API로부터 수신한 비정형 HTML 데이터를 DOM 파싱하여 정형화된 공연 객체 리스트로 변환합니다.
		pagePerformances, rawCount, err := t.parsePerformancesFromHTML(ctx, pageContent.HTML, matchers)
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
			t.LogWithContext("task.naver", applog.DebugLevel, "페이지네이션 종료 조건(데이터 없음)에 도달하여 수집 프로세스를 정상 종료합니다", applog.Fields{
				"last_visited_page": pageIndex - 1,
				"collected_count":   len(currentPerformances),
				"fetched_count":     totalFetchedCount,
			}, nil)

			break
		}

		time.Sleep(time.Duration(commandSettings.PageFetchDelay) * time.Millisecond)
	}

	t.LogWithContext("task.naver", applog.InfoLevel, "공연 정보 수집 및 키워드 매칭 프로세스가 완료되었습니다", applog.Fields{
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
func (t *task) parsePerformancesFromHTML(ctx context.Context, html string, matchers *keywordMatchers) ([]*performance, int, error) {
	doc, err := t.GetScraper().ParseHTML(ctx, strings.NewReader(html), "", "")
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
			// t.LogWithContext("task.naver", applog.TraceLevel, "키워드 매칭 조건에 의해 제외되었습니다", applog.Fields{"title": perf.Title}, nil)
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
		return nil, scraper.NewErrHTMLStructureChanged("", fmt.Sprintf("공연 제목 추출 실패 (선택자: %s, 발견된 요소 수: %d)", selectorTitle, titleSelection.Length()))
	}
	title := strings.TrimSpace(titleSelection.Text())
	if title == "" {
		return nil, scraper.NewErrHTMLStructureChanged("", fmt.Sprintf("공연 제목이 비어있습니다 (선택자: %s)", selectorTitle))
	}

	// 장소
	placeSelection := s.Find(selectorPlace)
	if placeSelection.Length() != 1 {
		return nil, scraper.NewErrHTMLStructureChanged("", fmt.Sprintf("공연 장소 추출 실패 (선택자: %s, 발견된 요소 수: %d)", selectorPlace, placeSelection.Length()))
	}
	place := strings.TrimSpace(placeSelection.Text())
	if place == "" {
		return nil, scraper.NewErrHTMLStructureChanged("", fmt.Sprintf("공연 장소가 비어있습니다 (선택자: %s)", selectorPlace))
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

// analyzeAndReport 수집된 데이터를 분석하여 사용자에게 보낼 알림 메시지를 생성합니다.
//
// [주요 동작]
// 1. 변화 확인: 이전 데이터와 비교해 새로운 공연 정보가 등록되었는지 확인합니다.
// 2. 메시지 작성: 발견된 신규 공연을 보기 좋게 포맷팅합니다.
// 3. 알림 결정:
//   - 스케줄러 실행: 신규 정보가 있을 때만 알림을 보냅니다. (조용히 모니터링)
//   - 사용자 실행: 신규 정보가 없어도 "변경 없음"이라고 알려줍니다. (확실한 피드백)
func (t *task) analyzeAndReport(currentSnapshot *watchNewPerformancesSnapshot, prevPerformancesSet map[string]bool, supportsHTML bool) (message string, shouldSave bool) {
	// 신규 공연을 식별합니다.
	diffs := t.calculatePerformanceDiffs(currentSnapshot, prevPerformancesSet)

	// 식별된 신규 공연 데이터를 알림 메시지로 변환합니다.
	diffMessage := t.renderPerformanceDiffs(diffs, supportsHTML)

	if len(diffs) > 0 {
		return "새로운 공연정보가 등록되었습니다.\n\n" + diffMessage, true
	}

	// 스케줄러(Scheduler)에 의한 자동 실행이 아닌, 사용자 요청에 의한 수동 실행인 경우입니다.
	//
	// 자동 실행 시에는 변경 사항이 없으면 불필요한 알림(Noise)을 방지하기 위해 침묵하지만,
	// 수동 실행 시에는 "변경 없음"이라는 명시적인 피드백을 제공하여 시스템이 정상 동작 중임을 사용자가 인지할 수 있도록 합니다.
	if t.GetRunBy() == contract.TaskRunByUser {
		if len(currentSnapshot.Performances) == 0 {
			return "등록된 공연정보가 존재하지 않습니다.", false
		}

		var sb strings.Builder

		// 예상 메시지 크기로 초기 용량 할당 (공연당 약 300바이트 추정)
		sb.Grow(len(currentSnapshot.Performances) * allocSizePerPerformance)

		for i, p := range currentSnapshot.Performances {
			if i > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(p.Render(supportsHTML, ""))
		}
		return "신규로 등록된 공연정보가 없습니다.\n\n현재 등록된 공연정보는 아래와 같습니다:\n\n" + sb.String(), false
	}

	return "", false
}

// calculatePerformanceDiffs 현재 스냅샷과 이전 스냅샷을 비교하여 신규 공연을 찾아냅니다.
// 즉, 이전에 없던 새로운 공연이 발견되면 이를 결과 목록에 담아 반환합니다.
func (t *task) calculatePerformanceDiffs(currentSnapshot *watchNewPerformancesSnapshot, prevPerformancesSet map[string]bool) []performanceDiff {
	var diffs []performanceDiff

	for _, p := range currentSnapshot.Performances {
		// 이전에 수집된 목록에 존재하지 않는다면 신규 공연으로 판단한다.
		if !prevPerformancesSet[p.Key()] {
			diffs = append(diffs, performanceDiff{
				Type:        eventNewPerformance,
				Performance: p,
			})
		}
	}

	return diffs
}

// renderPerformanceDiffs 찾아낸 신규 공연 목록을 사용자가 보기 편한 알림 메시지로 변환합니다.
func (t *task) renderPerformanceDiffs(diffs []performanceDiff, supportsHTML bool) string {
	if len(diffs) == 0 {
		return ""
	}

	var sb strings.Builder

	// 예상 메시지 크기로 초기 용량 할당 (공연당 약 300바이트 추정)
	sb.Grow(len(diffs) * allocSizePerPerformance)

	for i, diff := range diffs {
		if i > 0 {
			sb.WriteString("\n\n")
		}

		if diff.Type == eventNewPerformance {
			sb.WriteString(diff.Performance.Render(supportsHTML, mark.New))
		}
	}

	return sb.String()
}
