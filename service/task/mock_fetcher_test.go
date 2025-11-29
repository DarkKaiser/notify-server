package task

import (
	"bytes"
	"io"
	"net/http"
)

// MockFetcher is a mock implementation of the Fetcher interface
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

// NewMockResponse creates a new http.Response with the given body and status code
func NewMockResponse(body string, statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

// NewMockResponseWithJSON creates a new http.Response with the given JSON body and status code
func NewMockResponseWithJSON(jsonBody string, statusCode int) *http.Response {
	resp := NewMockResponse(jsonBody, statusCode)
	resp.Header.Set("Content-Type", "application/json")
	return resp
}
