package task

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"golang.org/x/net/html/charset"
)

// Fetcher HTTP 요청을 수행하는 인터페이스
type Fetcher interface {
	Get(url string) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

// FetchHTMLDocument 지정된 URL로 HTTP 요청을 보내 HTML 문서를 가져오고, goquery.Document로 파싱합니다.
// 응답 헤더의 Content-Type을 분석하여, 비 UTF-8 인코딩(예: EUC-KR) 페이지도 자동으로 UTF-8로 변환하여 처리합니다.
func FetchHTMLDocument(fetcher Fetcher, url string) (*goquery.Document, error) {
	resp, err := fetcher.Get(url)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.Unavailable, fmt.Sprintf("HTML 페이지(%s) 요청 중 네트워크 또는 클라이언트 에러가 발생했습니다.", url))
	}
	defer resp.Body.Close() // 응답을 받은 즉시 defer 설정하여 메모리 누수 방지

	if resp.StatusCode != http.StatusOK {
		errType := apperrors.ExecutionFailed
		// 5xx (Server Error) or 429 (Too Many Requests) -> Unavailable
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			errType = apperrors.Unavailable
		}
		return nil, apperrors.New(errType, fmt.Sprintf("HTML 페이지(%s) 요청이 실패했습니다. 상태 코드: %s", url, resp.Status))
	}

	// Content-Type 헤더를 기반으로 인코딩을 UTF-8로 변환
	utf8Reader, err := charset.NewReader(resp.Body, resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("페이지(%s)의 인코딩 변환이 실패하였습니다.", url))
	}

	doc, err := goquery.NewDocumentFromReader(utf8Reader)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("불러온 페이지(%s)의 데이터 파싱이 실패하였습니다.", url))
	}

	return doc, nil
}

// FetchHTMLSelection 지정된 URL의 HTML 문서에서 CSS 선택자(selector)에 해당하는 요소를 찾습니다.
// 선택된 요소가 없으면 에러를 반환하여, 변경된 웹 페이지 구조를 조기에 감지할 수 있도록 돕습니다.
func FetchHTMLSelection(fetcher Fetcher, url string, selector string) (*goquery.Selection, error) {
	doc, err := FetchHTMLDocument(fetcher, url)
	if err != nil {
		return nil, err
	}

	sel := doc.Find(selector)
	if sel.Length() <= 0 {
		return nil, NewErrHTMLStructureChanged(url, "")
	}

	return sel, nil
}

// FetchJSON HTTP 요청을 수행하고 응답 본문(JSON)을 지정된 구조체(v)로 디코딩합니다.
func FetchJSON(fetcher Fetcher, method, url string, header map[string]string, body io.Reader, v interface{}) error {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return apperrors.Wrap(err, apperrors.Internal, fmt.Sprintf("JSON 요청 생성에 실패했습니다. (URL: %s)", url))
	}
	for key, value := range header {
		req.Header.Set(key, value)
	}

	resp, err := fetcher.Do(req)
	if err != nil {
		return apperrors.Wrap(err, apperrors.Unavailable, fmt.Sprintf("JSON API(%s) 요청 전송 중 에러가 발생했습니다.", url))
	}
	defer resp.Body.Close() // 응답을 받은 즉시 defer 설정하여 메모리 누수 방지

	if resp.StatusCode != http.StatusOK {
		errType := apperrors.ExecutionFailed
		// 5xx (Server Error) or 429 (Too Many Requests) -> Unavailable
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			errType = apperrors.Unavailable
		}
		return apperrors.New(errType, fmt.Sprintf("JSON API(%s) 요청이 실패했습니다. 상태 코드: %s", url, resp.Status))
	}

	// json.Decoder를 사용하여 스트림 방식으로 JSON 파싱 (메모리 효율적)
	if err = json.NewDecoder(resp.Body).Decode(v); err != nil {
		return apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("불러온 페이지(%s) 데이터의 JSON 변환이 실패하였습니다.", url))
	}

	return nil
}

// ScrapeHTML 지정된 URL의 HTML 문서에서 CSS 선택자에 해당하는 모든 요소를 순회하며 콜백 함수를 실행합니다.
func ScrapeHTML(fetcher Fetcher, url string, selector string, f func(int, *goquery.Selection) bool) error {
	sel, err := FetchHTMLSelection(fetcher, url, selector)
	if err != nil {
		return err
	}

	sel.EachWithBreak(f)

	return nil
}
