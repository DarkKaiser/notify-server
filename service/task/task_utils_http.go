package task

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
)

// Fetcher interface for network requests
type Fetcher interface {
	Get(url string) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

// HTTPFetcher implementation using http.DefaultClient
type HTTPFetcher struct{}

func (h *HTTPFetcher) Get(url string) (*http.Response, error) {
	return http.Get(url)
}

func (h *HTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

// noinspection GoUnhandledErrorResult
func newHTMLDocument(fetcher Fetcher, url string) (*goquery.Document, error) {
	resp, err := fetcher.Get(url)
	if err != nil {
		return nil, fmt.Errorf("페이지(%s) 접근이 실패하였습니다.(error:%s)", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("페이지(%s) 접근이 실패하였습니다.(%s)", url, resp.Status)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("불러온 페이지(%s)의 데이터 파싱이 실패하였습니다.(error:%s)", url, err)
	}

	return doc, nil
}

func newHTMLDocumentSelection(fetcher Fetcher, url string, selector string) (*goquery.Selection, error) {
	doc, err := newHTMLDocument(fetcher, url)
	if err != nil {
		return nil, err
	}

	sel := doc.Find(selector)
	if sel.Length() <= 0 {
		return nil, fmt.Errorf("불러온 페이지(%s)의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요", url)
	}

	return sel, nil
}

func webScrape(fetcher Fetcher, url string, selector string, f func(int, *goquery.Selection) bool) error {
	sel, err := newHTMLDocumentSelection(fetcher, url, selector)
	if err != nil {
		return err
	}

	sel.EachWithBreak(f)

	return nil
}

// noinspection GoUnhandledErrorResult
func unmarshalFromResponseJSONData(fetcher Fetcher, method, url string, header map[string]string, body io.Reader, v interface{}) error {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("페이지(%s) 접근이 실패하였습니다.(error:%s)", url, err)
	}
	for key, value := range header {
		req.Header.Set(key, value)
	}

	resp, err := fetcher.Do(req)
	if err != nil {
		return fmt.Errorf("페이지(%s) 접근이 실패하였습니다.(error:%s)", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("페이지(%s) 접근이 실패하였습니다.(%s)", url, resp.Status)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("불러온 페이지(%s) 데이터를 읽을 수 없습니다.(error:%s)", url, err)
	}

	if err = json.Unmarshal(bodyBytes, v); err != nil {
		return fmt.Errorf("불러온 페이지(%s) 데이터의 JSON 변환이 실패하였습니다.(error:%s)", url, err)
	}

	return nil
}
