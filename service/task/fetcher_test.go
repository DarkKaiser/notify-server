package task

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

// TestHTTPFetcher_Methods_Table consolidates generic HTTPFetcher method tests (Do, Get, User-Agent behavior)
func TestHTTPFetcher_Methods_Table(t *testing.T) {
	// Setup a test server that validates User-Agent
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		if userAgent == "" || !strings.Contains(userAgent, "Mozilla/5.0") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	fetcher := NewHTTPFetcher()

	tests := []struct {
		name        string
		action      func() (*http.Response, error)
		expectError bool
	}{
		{
			name: "Do Request (Automatic User-Agent)",
			action: func() (*http.Response, error) {
				req, _ := http.NewRequest("GET", ts.URL, nil)
				return fetcher.Do(req)
			},
			expectError: false,
		},
		{
			name: "Get Request (Automatic User-Agent)",
			action: func() (*http.Response, error) {
				return fetcher.Get(ts.URL)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.action()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if resp != nil {
					assert.Equal(t, http.StatusOK, resp.StatusCode)
				}
			}
		})
	}
}

// TestHTTPFetcher_Timeout checks initialization (Basic check)
func TestHTTPFetcher_Timeout(t *testing.T) {
	fetcher := NewHTTPFetcher()
	assert.NotNil(t, fetcher)
}

func TestFetchHTMLDocument_Table(t *testing.T) {
	eucKrContent := func(s string) string {
		var buf bytes.Buffer
		w := transform.NewWriter(&buf, korean.EUCKR.NewEncoder())
		w.Write([]byte(s))
		w.Close()
		return buf.String()
	}

	tests := []struct {
		name        string
		url         string
		setupMock   func(*TestMockFetcher)
		wantErr     bool
		errContains string
		validateDoc func(*testing.T, *goquery.Document)
	}{
		{
			name: "Success - UTF-8",
			url:  "http://example.com/utf8",
			setupMock: func(m *TestMockFetcher) {
				htmlContent := `<html><body><div class="test">안녕</div></body></html>`
				resp := NewMockResponse(htmlContent, 200)
				resp.Header.Set("Content-Type", "text/html; charset=utf-8")
				m.On("Get", "http://example.com/utf8").Return(resp, nil)
			},
			validateDoc: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "안녕", doc.Find(".test").Text())
			},
		},
		{
			name: "Success - EUC-KR",
			url:  "http://example.com/euckr",
			setupMock: func(m *TestMockFetcher) {
				content := eucKrContent(`<html><body><div class="test">안녕</div></body></html>`)
				resp := NewMockResponse(content, 200)
				resp.Header.Set("Content-Type", "text/html; charset=euc-kr")
				m.On("Get", "http://example.com/euckr").Return(resp, nil)
			},
			validateDoc: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "안녕", doc.Find(".test").Text())
			},
		},
		{
			name: "Fetcher Error",
			url:  "http://example.com/error",
			setupMock: func(m *TestMockFetcher) {
				m.On("Get", "http://example.com/error").Return(nil, errors.New("network error"))
			},
			wantErr:     true,
			errContains: "HTML 페이지(http://example.com/error) 요청 중 네트워크 또는 클라이언트 에러가 발생했습니다.",
		},
		{
			name: "HTTP 500 Error",
			url:  "http://example.com/500",
			setupMock: func(m *TestMockFetcher) {
				resp := NewMockResponse("", 500)
				resp.Status = "500 Internal Server Error"
				m.On("Get", "http://example.com/500").Return(resp, nil)
			},
			wantErr:     true,
			errContains: "HTML 페이지(http://example.com/500) 요청이 실패했습니다. 상태 코드: 500 Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &TestMockFetcher{}
			if tt.setupMock != nil {
				tt.setupMock(mockFetcher)
			}

			doc, err := FetchHTMLDocument(mockFetcher, tt.url)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, doc)
			} else {
				assert.NoError(t, err)
				if tt.validateDoc != nil {
					tt.validateDoc(t, doc)
				}
			}
			mockFetcher.AssertExpectations(t)
		})
	}
}

func TestFetchHTMLSelection_Table(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		selector    string
		setupMock   func(*TestMockFetcher)
		wantErr     bool
		errContains string
		validateSel func(*testing.T, *goquery.Selection)
	}{
		{
			name:     "Success",
			url:      "http://example.com/success",
			selector: ".target",
			setupMock: func(m *TestMockFetcher) {
				htmlContent := `<html><body><div class="target">Found Me</div></body></html>`
				resp := NewMockResponse(htmlContent, 200)
				resp.Header.Set("Content-Type", "text/html; charset=utf-8")
				m.On("Get", "http://example.com/success").Return(resp, nil)
			},
			validateSel: func(t *testing.T, sel *goquery.Selection) {
				assert.Equal(t, "Found Me", sel.Text())
			},
		},
		{
			name:     "Selection Not Found",
			url:      "http://example.com/missing",
			selector: ".target",
			setupMock: func(m *TestMockFetcher) {
				htmlContent := `<html><body><div class="other">Not Me</div></body></html>`
				resp := NewMockResponse(htmlContent, 200)
				resp.Header.Set("Content-Type", "text/html; charset=utf-8")
				m.On("Get", "http://example.com/missing").Return(resp, nil)
			},
			wantErr:     true,
			errContains: "CSS셀렉터를 확인하세요",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &TestMockFetcher{}
			if tt.setupMock != nil {
				tt.setupMock(mockFetcher)
			}

			sel, err := FetchHTMLSelection(mockFetcher, tt.url, tt.selector)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				if tt.validateSel != nil {
					tt.validateSel(t, sel)
				}
			}
			mockFetcher.AssertExpectations(t)
		})
	}
}

func TestScrapeHTML(t *testing.T) {
	// ScrapeHTML Logic flow test, kept as is or slightly refined
	t.Run("Scrape - Iterate All", func(t *testing.T) {
		mockFetcher := &TestMockFetcher{}
		htmlContent := `<html><body><ul class="list"><li>Item 1</li><li>Item 2</li><li>Item 3</li></ul></body></html>`
		resp := NewMockResponse(htmlContent, 200)
		mockFetcher.On("Get", "http://example.com").Return(resp, nil)

		count := 0
		err := ScrapeHTML(mockFetcher, "http://example.com", ".list li", func(i int, s *goquery.Selection) bool {
			count++
			return true
		})

		assert.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("Scrape - Early Exit", func(t *testing.T) {
		mockFetcher := &TestMockFetcher{}
		htmlContent := `<html><body><ul class="list"><li>Item 1</li><li>Item 2</li></ul></body></html>`
		resp := NewMockResponse(htmlContent, 200)
		mockFetcher.On("Get", "http://example.com").Return(resp, nil)

		count := 0
		err := ScrapeHTML(mockFetcher, "http://example.com", ".list li", func(i int, s *goquery.Selection) bool {
			count++
			return false
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

func TestFetchJSON_Table(t *testing.T) {
	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name        string
		method      string
		url         string
		header      map[string]string
		setupMock   func(*TestMockFetcher)
		wantErr     bool
		errContains string
		validateRes func(*testing.T, TestData)
	}{
		{
			name:   "Success",
			method: "POST",
			url:    "http://example.com",
			header: map[string]string{"X-Custom": "HeaderVal"},
			setupMock: func(m *TestMockFetcher) {
				jsonContent := `{"name": "test", "value": 123}`
				resp := NewMockResponse(jsonContent, 200)
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == "POST" &&
						req.URL.String() == "http://example.com" &&
						req.Header.Get("X-Custom") == "HeaderVal"
				})).Return(resp, nil)
			},
			validateRes: func(t *testing.T, res TestData) {
				assert.Equal(t, "test", res.Name)
				assert.Equal(t, 123, res.Value)
			},
		},
		{
			name:   "JSON Parsing Error - Invalid Type",
			method: "GET",
			url:    "http://example.com",
			setupMock: func(m *TestMockFetcher) {
				jsonContent := `{"name": "test", "value": "invalid"}`
				resp := NewMockResponse(jsonContent, 200)
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errContains: "JSON 변환이 실패하였습니다",
		},
		{
			name:   "HTTP Error Status",
			method: "GET",
			url:    "http://example.com/404",
			setupMock: func(m *TestMockFetcher) {
				resp := NewMockResponse(`{"error": "not found"}`, 404)
				resp.Status = "404 Not Found"
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errContains: "JSON API(http://example.com/404) 요청이 실패했습니다. 상태 코드: 404 Not Found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &TestMockFetcher{}
			if tt.setupMock != nil {
				tt.setupMock(mockFetcher)
			}

			var result TestData
			err := FetchJSON(mockFetcher, tt.method, tt.url, tt.header, nil, &result)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				if tt.validateRes != nil {
					tt.validateRes(t, result)
				}
			}
			mockFetcher.AssertExpectations(t)
		})
	}
}
