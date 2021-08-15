package task

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net/http"
)

//noinspection GoUnhandledErrorResult
func getHTMLDocument(url string) (*goquery.Document, error) {
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

func getHTMLDocumentSelection(url string, selector string) (*goquery.Selection, error) {
	doc, err := getHTMLDocument(url)
	if err != nil {
		return nil, err
	}

	sel := doc.Find(selector)
	if sel.Length() <= 0 {
		return nil, fmt.Errorf("불러온 페이지(%s)의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요.", url)
	}

	return sel, nil
}

func scrapeHTMLDocument(url string, selector string, f func(int, *goquery.Selection) bool) error {
	sel, err := getHTMLDocumentSelection(url, selector)
	if err != nil {
		return err
	}

	sel.EachWithBreak(f)

	return nil
}
