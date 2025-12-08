package task

import (
	"bytes"
	"io"
	"net/http"
)

// MockFetcher Fetcher 인터페이스의 Mock 구현체입니다.
type MockFetcher struct {
	Response *http.Response
	Err      error
}

func (m *MockFetcher) Get(url string) (*http.Response, error) {
	return m.Response, m.Err
}

func (m *MockFetcher) Do(req *http.Request) (*http.Response, error) {
	return m.Response, m.Err
}

// NewMockResponse 주어진 body와 status code를 가진 새로운 http.Response를 생성합니다.
func NewMockResponse(body string, statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

// NewMockResponseWithJSON 주어진 JSON body와 status code를 가진 새로운 http.Response를 생성합니다.
func NewMockResponseWithJSON(jsonBody string, statusCode int) *http.Response {
	resp := NewMockResponse(jsonBody, statusCode)
	resp.Header.Set("Content-Type", "application/json")
	return resp
}
