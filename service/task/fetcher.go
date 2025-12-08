package task

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"golang.org/x/net/html/charset"
)

// Fetcher HTTP 요청을 수행하는 인터페이스
type Fetcher interface {
	Get(url string) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

// HTTPFetcher 타임아웃과 User-Agent가 설정된 HTTP 클라이언트 구현체
type HTTPFetcher struct {
	client *http.Client
}

// NewHTTPFetcher 30초 타임아웃이 설정된 HTTPFetcher를 생성합니다
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Get 지정된 URL로 GET 요청을 전송합니다 (User-Agent 자동 설정)
func (h *HTTPFetcher) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return h.Do(req)
}

// Do HTTP 요청을 전송하며, User-Agent 헤더가 없으면 자동으로 추가합니다
func (h *HTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	// User-Agent가 설정되지 않은 경우 기본값(Chrome) 설정
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	}
	return h.client.Do(req)
}

// NewHTMLDocument 지정된 URL에서 HTML 문서를 가져와 goquery.Document로 파싱합니다
// 인코딩 변환(UTF-8)을 자동으로 처리합니다
func NewHTMLDocument(fetcher Fetcher, url string) (*goquery.Document, error) {
	resp, err := fetcher.Get(url)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrTaskExecutionFailed, fmt.Sprintf("페이지(%s) 접근이 실패하였습니다.", url))
	}
	defer resp.Body.Close() // 응답을 받은 즉시 defer 설정하여 메모리 누수 방지

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.New(apperrors.ErrTaskExecutionFailed, fmt.Sprintf("페이지(%s) 접근이 실패하였습니다.(%s)", url, resp.Status))
	}

	// Content-Type 헤더를 기반으로 인코딩을 UTF-8로 변환
	utf8Reader, err := charset.NewReader(resp.Body, resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrTaskExecutionFailed, fmt.Sprintf("페이지(%s)의 인코딩 변환이 실패하였습니다.", url))
	}

	doc, err := goquery.NewDocumentFromReader(utf8Reader)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrTaskExecutionFailed, fmt.Sprintf("불러온 페이지(%s)의 데이터 파싱이 실패하였습니다.", url))
	}

	return doc, nil
}

// NewHTMLDocumentSelection HTML 문서에서 CSS 선택자로 요소를 찾아 반환합니다
// 선택자에 해당하는 요소가 없으면 에러를 반환합니다
func NewHTMLDocumentSelection(fetcher Fetcher, url string, selector string) (*goquery.Selection, error) {
	doc, err := NewHTMLDocument(fetcher, url)
	if err != nil {
		return nil, err
	}

	sel := doc.Find(selector)
	if sel.Length() <= 0 {
		return nil, apperrors.New(apperrors.ErrTaskExecutionFailed, fmt.Sprintf("불러온 페이지(%s)의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요", url))
	}

	return sel, nil
}

// WebScrape HTML 문서에서 CSS 선택자로 요소들을 찾아 각 요소마다 콜백 함수를 실행합니다
func WebScrape(fetcher Fetcher, url string, selector string, f func(int, *goquery.Selection) bool) error {
	sel, err := NewHTMLDocumentSelection(fetcher, url, selector)
	if err != nil {
		return err
	}

	sel.EachWithBreak(f)

	return nil
}

// UnmarshalFromResponseJSONData HTTP 요청을 전송하고 응답 JSON을 구조체로 변환합니다
// json.Decoder를 사용하여 스트림 방식으로 처리합니다
func UnmarshalFromResponseJSONData(fetcher Fetcher, method, url string, header map[string]string, body io.Reader, v interface{}) error {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrTaskExecutionFailed, fmt.Sprintf("페이지(%s) 접근이 실패하였습니다.", url))
	}
	for key, value := range header {
		req.Header.Set(key, value)
	}

	resp, err := fetcher.Do(req)
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrTaskExecutionFailed, fmt.Sprintf("페이지(%s) 접근이 실패하였습니다.", url))
	}
	defer resp.Body.Close() // 응답을 받은 즉시 defer 설정하여 메모리 누수 방지

	if resp.StatusCode != http.StatusOK {
		return apperrors.New(apperrors.ErrTaskExecutionFailed, fmt.Sprintf("페이지(%s) 접근이 실패하였습니다.(%s)", url, resp.Status))
	}

	// json.Decoder를 사용하여 스트림 방식으로 JSON 파싱 (메모리 효율적)
	if err = json.NewDecoder(resp.Body).Decode(v); err != nil {
		return apperrors.Wrap(err, apperrors.ErrTaskExecutionFailed, fmt.Sprintf("불러온 페이지(%s) 데이터의 JSON 변환이 실패하였습니다.", url))
	}

	return nil
}
