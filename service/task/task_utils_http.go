package task

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"io/ioutil"
	"net/http"
)

//noinspection GoUnhandledErrorResult
func newHTMLDocument(url string) (*goquery.Document, error) {
	resp, err := http.Get(url)
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

func newHTMLDocumentSelection(url string, selector string) (*goquery.Selection, error) {
	doc, err := newHTMLDocument(url)
	if err != nil {
		return nil, err
	}

	sel := doc.Find(selector)
	if sel.Length() <= 0 {
		return nil, fmt.Errorf("불러온 페이지(%s)의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요.", url)
	}

	return sel, nil
}

func webScrape(url string, selector string, f func(int, *goquery.Selection) bool) error {
	sel, err := newHTMLDocumentSelection(url, selector)
	if err != nil {
		return err
	}

	sel.EachWithBreak(f)

	return nil
}

//noinspection GoUnhandledErrorResult
func unmarshalFromResponseJSONData(method, url string, header map[string]string, body io.Reader, v interface{}) error {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("페이지(%s) 접근이 실패하였습니다.(error:%s)", url, err)
	}
	for key, value := range header {
		req.Header.Set(key, value)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("페이지(%s) 접근이 실패하였습니다.(error:%s)", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("페이지(%s) 접근이 실패하였습니다.(%s)", url, resp.Status)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("불러온 페이지(%s) 데이터를 읽을 수 없습니다.(error:%s)", url, err)
	}

	if err = json.Unmarshal(bodyBytes, v); err != nil {
		return fmt.Errorf("불러온 페이지(%s) 데이터의 JSON 변환이 실패하였습니다.(error:%s)", url, err)
	}

	return nil
}
