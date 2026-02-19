package naver

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
)

const (
	// performanceSearchEndpoint 네이버 모바일 통합검색의 내부 API 엔드포인트입니다.
	// 이 URL은 공개 API가 아니며, 모바일 검색 페이지(m.search.naver.com)에서
	// 내부적으로 공연/전시 검색 결과 HTML을 비동기로 가져오는 데 사용하는 주소입니다.
	performanceSearchEndpoint = "https://m.search.naver.com/p/csearch/content/nqapirender.nhn"

	// ------------------------------------------------------------------------------------------------
	// CSS 셀렉터 정의
	// 아래 선택자들은 네이버 모바일 검색 결과 페이지(HTML)의 DOM 구조를 분석하여 도출된 값입니다.
	// ------------------------------------------------------------------------------------------------

	// selectorPerformanceItem 검색 결과 리스트에서 개별 공연 항목을 식별합니다.
	selectorPerformanceItem = "li:has(.title_box)"

	// selectorTitle 공연 항목 내부의 공연명 텍스트를 추출합니다.
	selectorTitle = ".title_box .name"

	// selectorPlace 공연 항목 내부의 공연장(장소) 텍스트를 추출합니다.
	selectorPlace = ".title_box .sub_text"

	// selectorThumbnail 공연 항목 내부의 포스터 이미지 요소를 식별합니다.
	selectorThumbnail = ".thumb img"

	// selectorNoResult 검색 결과가 없을 때 네이버가 렌더링하는 '결과 없음' 안내 영역을 식별합니다.
	selectorNoResult = ".api_no_result"

	// ------------------------------------------------------------------------------------------------
	// API 쿼리 파라미터 키 정의
	//
	// 네이버 모바일 통합검색 내부 API는 'u1', 'u2'와 같이 의미를 알 수 없는 축약된 파라미터 키를 사용합니다.
	// 코드의 가독성을 높이고 유지보수를 용이하게 하기 위해, 각 파라미터의 역할을 나타내는 상수로 매핑하여 사용합니다.
	// ------------------------------------------------------------------------------------------------

	paramQuery       = "u1" // 검색어
	paramGenre       = "u2" // 장르 필터
	paramDateRange   = "u3" // 날짜 범위
	paramStatus      = "u4" // 공연 상태
	paramSort        = "u5" // 정렬 기준
	paramAdult       = "u6" // 성인 공연 포함 여부
	paramPage        = "u7" // 요청할 페이지 번호 (1-based)
	paramDetailGenre = "u8" // 세부 장르 필터
)

// fetchPagePerformances 네이버 통합검색 API를 호출하여 특정 페이지의 공연 목록을 가져옵니다.
//
// 매개변수:
//   - ctx: HTTP 요청의 타임아웃 및 취소 신호를 전파하기 위한 컨텍스트
//   - query: 검색어 (예: "뮤지컬", "콘서트")
//   - pageNumber: 요청할 페이지 번호 (1부터 시작)
//
// 반환값:
//   - []*performance: 파싱에 성공한 공연 목록
//   - int: HTML에서 발견된 공연 항목의 원시(raw) 개수
//   - error: 네트워크 오류, JSON 파싱 실패, 예상치 못한 HTML 구조 감지 시 반환
func (t *task) fetchPagePerformances(ctx context.Context, query string, pageNumber int) ([]*performance, int, error) {
	searchAPIURL := buildPerformanceSearchURL(query, pageNumber)

	// 네이버 내부 API의 JSON 응답 구조체입니다.
	// 이 API는 검색 결과 HTML 조각을 JSON 객체의 'html' 필드에 담아 반환합니다.
	// 'html' 필드를 *string으로 선언하여, 필드 자체가 누락(null)된 경우와 빈 문자열("")인 경우를 명확히 구분합니다.
	type performanceSearchResponse struct {
		HTML *string `json:"html"`
	}

	var resp = &performanceSearchResponse{}
	err := t.Scraper().FetchJSON(ctx, "GET", searchAPIURL, nil, nil, resp)
	if err != nil {
		return nil, 0, err
	}

	if resp.HTML == nil {
		// API 응답에서 'html' 필드 자체가 누락(null)되었습니다.
		// 이는 네이버 측 API 스키마가 변경되었을 가능성이 높으므로, 즉시 에러로 처리하여 운영자가 인지할 수 있도록 합니다.
		// (참고: 검색 결과가 없는 경우라도 'html' 필드는 빈 문자열("")로 존재해야 합니다.)
		return nil, 0, scraper.NewErrHTMLStructureChanged(searchAPIURL, "API 응답 형식이 유효하지 않습니다: 필수 필드 'html'이 누락되었습니다.")
	}

	html := *resp.HTML

	// 수신한 HTML 문자열을 DOM으로 파싱하여 구조화된 공연 목록으로 변환합니다.
	return t.parsePerformancesFromHTML(ctx, html, searchAPIURL, pageNumber)
}

// buildPerformanceSearchURL 네이버 모바일 통합검색 내부 API를 호출하기 위한 완전한 URL을 조립하여 반환합니다.
//
// 네이버 공연/전시 검색 결과는 공개 API가 아닌 모바일 검색 페이지의 내부 API를 통해 제공됩니다.
// 이 함수는 해당 API가 요구하는 고정 파라미터(key, pkid, where)와
// 검색 조건 파라미터(검색어, 장르, 정렬 등)를 조합하여 최종 요청 URL을 생성합니다.
//
// 매개변수:
//   - query: 검색어 (예: "뮤지컬", "콘서트")
//   - pageNumber: 요청할 페이지 번호 (1부터 시작)
//
// 반환값:
//   - string: 완성된 API 요청 URL 문자열
func buildPerformanceSearchURL(query string, pageNumber int) string {
	params := url.Values{}

	// API가 요구하는 고정 파라미터
	params.Set("key", "kbList")     // 지식베이스(Knowledge Base) 리스트 식별자
	params.Set("pkid", "269")       // 공연/전시 카테고리 식별자
	params.Set("where", "nexearch") // 검색 영역 (nexearch: 네이버 통합검색)

	// 검색 조건 파라미터
	params.Set(paramQuery, query)                   // 검색어 (예: "뮤지컬", "콘서트")
	params.Set(paramGenre, "all")                   // 장르 필터 ("all": 전체 장르)
	params.Set(paramDateRange, "")                  // 날짜 범위 (빈 값: 전체 기간)
	params.Set(paramStatus, "ingplan")              // 공연 상태 ("ingplan": 진행 중 또는 예정인 공연만 포함)
	params.Set(paramSort, "date")                   // 정렬 기준 ("date": 최신 등록순, "rank": 인기순)
	params.Set(paramAdult, "N")                     // 성인 공연 포함 여부 ("N": 제외)
	params.Set(paramPage, strconv.Itoa(pageNumber)) // 요청할 페이지 번호 (1-based)
	params.Set(paramDetailGenre, "all")             // 세부 장르 필터 ("all": 전체 세부 장르)

	return fmt.Sprintf("%s?%s", performanceSearchEndpoint, params.Encode())
}

// parsePerformancesFromHTML 네이버 모바일 통합검색 API로부터 수신한 HTML 문자열을 파싱하여 구조화된 []*performance 목록으로 변환합니다.
//
// HTML 내의 각 공연 항목 요소를 순회하며 제목, 장소, 썸네일을 추출합니다.
// 파싱 도중 하나라도 오류가 발생하면 즉시 중단하고 에러를 반환합니다.
//
// 매개변수:
//   - ctx: HTML 파싱 작업의 취소 신호를 전파하기 위한 컨텍스트
//   - html: 네이버 API가 JSON 응답의 'html' 필드에 담아 반환한 HTML 조각 문자열
//   - pageURL: API 요청 URL. 에러 메타데이터 및 썸네일 상대 경로를 절대 경로로 변환하는 기준 URL로 사용됩니다.
//   - pageNumber: 요청 중인 페이지 번호 (1부터 시작)
//
// 반환값:
//   - []*performance: 파싱에 성공한 공연 목록
//   - int: HTML에서 발견된 공연 항목의 원시(raw) 개수
//   - error: HTML 파싱에 실패했거나, 필수 요소(제목·장소) 누락 등 페이지 구조 변경이 감지된 경우 반환
func (t *task) parsePerformancesFromHTML(ctx context.Context, html string, pageURL string, pageNumber int) ([]*performance, int, error) {
	// 수신한 HTML 문자열을 goquery가 탐색할 수 있는 DOM 트리로 변환합니다.
	doc, err := t.Scraper().ParseHTML(ctx, strings.NewReader(html), pageURL, "")
	if err != nil {
		return nil, 0, err
	}

	// 파싱된 DOM에서 공연 항목 요소 전체를 선택합니다.
	performanceItems := doc.Find(selectorPerformanceItem)

	// rawCount는 파싱·필터링 전 HTML에서 발견된 공연 항목의 전체 개수로,
	// 최종 반환되는 performances 슬라이스의 길이와 구분하기 위해 별도로 보존합니다.
	rawCount := performanceItems.Length()

	// 공연 항목이 하나도 발견되지 않았다면, 정상적인 '결과 없음'인지 HTML 구조 변경인지를 판별합니다.
	if rawCount == 0 {
		// 공연 항목이 하나도 없을 때의 처리 분기:
		//
		// - 1페이지에서 결과가 없다면, 검색어 자체에 해당하는 공연이 없거나 HTML 구조가 변경된 것입니다.
		//   이 경우 네이버는 '결과 없음' 안내 배너(selectorNoResult)를 렌더링하므로,
		//   해당 배너가 존재하면 정상적인 '검색 결과 없음'으로 처리합니다.
		//   배너조차 없다면 HTML 구조가 변경된 것으로 판단하여 에러를 반환합니다.
		//
		// - 2페이지 이상에서 결과가 없다면, 페이지네이션이 끝난 것입니다.
		//   이 경우 네이버는 배너를 렌더링하지 않을 수 있으므로, 에러 없이 정상 종료합니다.
		if pageNumber == 1 {
			noResultElement := doc.Find(selectorNoResult)
			if noResultElement.Length() == 0 {
				// '결과 없음' 배너도 없고 HTML 자체가 너무 짧다면, 빈 응답이 내려온 것으로 판단합니다.
				if len(strings.TrimSpace(html)) < 50 {
					return nil, 0, scraper.NewErrHTMLStructureChanged(pageURL, "API 응답 HTML 데이터가 유효하지 않습니다: 내용이 비정상적으로 짧습니다")
				}

				return nil, 0, scraper.NewErrHTMLStructureChanged(pageURL, fmt.Sprintf("HTML 구조 변경 감지: 공연 목록과 '결과 없음' 안내 요소가 모두 존재하지 않습니다 (참조 선택자: %s)", selectorNoResult))
			}
		}

		return []*performance{}, 0, nil
	}

	// 공연 항목 수만큼 슬라이스를 미리 할당하여, 순회 중 불필요한 메모리 재할당을 방지합니다.
	performances := make([]*performance, 0, rawCount)

	// 각 공연 항목 요소를 순차적으로 순회하며 구조화된 *performance 객체로 변환합니다.
	var parseErr error
	performanceItems.EachWithBreak(func(_ int, s *goquery.Selection) bool {
		perf, err := parsePerformance(s, pageURL)
		if err != nil {
			parseErr = err
			return false // 순회 즉시 중단
		}
		performances = append(performances, perf)
		return true
	})
	if parseErr != nil {
		return nil, rawCount, parseErr
	}

	return performances, rawCount, nil
}

// parsePerformance 개별 공연 항목의 HTML 요소(goquery.Selection)를 받아
// 제목, 장소, 썸네일 URL을 추출하고 *performance 구조체로 반환합니다.
//
// 제목과 장소는 필수 필드로, 요소가 정확히 1개 존재하지 않거나 텍스트가 비어있으면
// HTML 구조 변경으로 간주하여 ErrHTMLStructureChanged 에러를 반환합니다.
// 썸네일은 선택적 필드로, 이미지 요소가 없거나 URL 해결에 실패해도 에러를 반환하지 않습니다.
//
// 매개변수:
//   - s: 파싱할 개별 공연 항목의 HTML 요소 (goquery.Selection)
//   - pageURL: 에러 메타데이터 기록 및 썸네일 상대 경로를 절대 URL로 변환하는 기준 URL
//
// 반환값:
//   - *performance: 제목·장소·썸네일이 채워진 공연 정보 객체
//   - error: 필수 필드 누락 또는 HTML 구조 변경 감지 시 반환
func parsePerformance(s *goquery.Selection, pageURL string) (*performance, error) {
	// ------------------------------------------------------------------------------------------------
	// 공연 제목 추출 (필수)
	// ------------------------------------------------------------------------------------------------

	// 제목 요소가 정확히 1개가 아니면 HTML 구조가 변경된 것으로 판단합니다.
	titleElement := s.Find(selectorTitle)
	if titleElement.Length() != 1 {
		return nil, scraper.NewErrHTMLStructureChanged(pageURL, fmt.Sprintf("HTML 구조 변경 감지: 공연 제목 요소가 예상과 다릅니다 (참조 선택자: %s, 발견된 요소 수: %d)", selectorTitle, titleElement.Length()))
	}
	title := strings.TrimSpace(titleElement.Text())
	if title == "" {
		return nil, scraper.NewErrHTMLStructureChanged(pageURL, fmt.Sprintf("데이터 유효성 검사 실패: 공연 제목 텍스트가 비어있습니다 (참조 선택자: %s)", selectorTitle))
	}

	// ------------------------------------------------------------------------------------------------
	// 공연 장소 추출 (필수)
	// ------------------------------------------------------------------------------------------------

	// 장소 요소가 정확히 1개가 아니면 HTML 구조가 변경된 것으로 판단합니다.
	placeElement := s.Find(selectorPlace)
	if placeElement.Length() != 1 {
		return nil, scraper.NewErrHTMLStructureChanged(pageURL, fmt.Sprintf("HTML 구조 변경 감지: 공연 장소 요소가 예상과 다릅니다 (참조 선택자: %s, 발견된 요소 수: %d)", selectorPlace, placeElement.Length()))
	}
	place := strings.TrimSpace(placeElement.Text())
	if place == "" {
		return nil, scraper.NewErrHTMLStructureChanged(pageURL, fmt.Sprintf("데이터 유효성 검사 실패: 공연 장소 텍스트가 비어있습니다 (참조 선택자: %s)", selectorPlace))
	}

	// ------------------------------------------------------------------------------------------------
	// 썸네일 URL 추출 (선택적)
	// ------------------------------------------------------------------------------------------------

	// 썸네일이 없어도 공연 정보로서 유효하므로, 추출 실패 시 에러를 반환하지 않습니다.
	var thumbnailURL string
	thumbnailElement := s.Find(selectorThumbnail)
	if thumbnailElement.Length() > 0 {
		if src, exists := thumbnailElement.Attr("src"); exists {
			// 네이버 이미지 URL은 프로토콜 생략형('//Example.com/...') 또는 상대 경로('/Example.png')일 수 있습니다.
			// 이를 pageURL을 기준으로 온전한 절대 경로로 변환합니다.
			if resolved, err := resolveAbsoluteURL(pageURL, src); err == nil {
				thumbnailURL = resolved
			} else {
				// URL 변환에 실패한 경우, 원본 src 값이라도 보존합니다.
				thumbnailURL = src
			}
		}
	}

	// ------------------------------------------------------------------------------------------------
	// 공연 정보 객체 생성
	// ------------------------------------------------------------------------------------------------

	perf := &performance{
		Title:     title,
		Place:     place,
		Thumbnail: thumbnailURL,
	}

	return perf, nil
}

// resolveAbsoluteURL baseURL을 기준으로 targetURL을 해석하여 완전한 절대 URL 문자열을 반환합니다.
//
// 표준 라이브러리의 url.URL.ResolveReference(RFC 3986)를 사용하므로,
// 입력 형식에 따라 다음과 같이 처리합니다:
//
//   - 절대 URL
//     "https://example.com/img.jpg" → 그대로 반환
//   - 프로토콜 생략
//     "//example.com/img.jpg" → baseURL의 스킴(https)을 적용
//   - 절대 경로
//     "/path/to/img.jpg" → baseURL의 호스트를 적용
//   - 상대 경로
//     "../img.jpg" → baseURL 기준으로 경로를 계산
func resolveAbsoluteURL(baseURL, targetURL string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	target, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}

	return base.ResolveReference(target).String(), nil
}
