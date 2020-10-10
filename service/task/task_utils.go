package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net/http"
)

func httpWebPageDocument(url string) (*goquery.Document, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Web 페이지 접근이 실패하였습니다.(error:%s)", err))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Web 페이지 접근이 실패하였습니다.(%s)", resp.Status))
	}
	defer resp.Body.Close()

	document, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("HTML 파싱이 실패하였습니다.(error:%s)", err))
	}

	return document, nil
}

func convertMapTypeToStructType(from map[string]interface{}, to interface{}) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, to); err != nil {
		return err
	}
	return nil
}
