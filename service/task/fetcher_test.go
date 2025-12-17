package task

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

// TestFetchHTMLDocument_Table covers FetchHTMLDocument with expert scenarios:
// - UTF-8 vs EUC-KR encoding detection (via Header).
// - Fallback or Meta tag detection scenarios handled by `charset.NewReader` (implicit).
// - Error propagation network/5xx.
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
		errType     apperrors.ErrorType
		errContains string
		validateDoc func(*testing.T, *goquery.Document)
	}{
		{
			name: "Success - UTF-8 Explicit Header",
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
			name: "Success - EUC-KR Explicit Header",
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
			name: "Success - Missing Charset Header (Auto Detection)",
			url:  "http://example.com/auto",
			setupMock: func(m *TestMockFetcher) {
				// No charset in header, but content is valid UTF-8
				htmlContent := `<html><head><meta charset="utf-8"></head><body><div class="test">안녕</div></body></html>`
				resp := NewMockResponse(htmlContent, 200)
				resp.Header.Set("Content-Type", "text/html") // No charset
				m.On("Get", "http://example.com/auto").Return(resp, nil)
			},
			validateDoc: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "안녕", doc.Find(".test").Text())
			},
		},
		{
			name: "Fetcher Error - Network Failure",
			url:  "http://example.com/error",
			setupMock: func(m *TestMockFetcher) {
				m.On("Get", "http://example.com/error").Return(nil, errors.New("network error"))
			},
			wantErr:     true,
			errType:     apperrors.Unavailable,
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
			errType:     apperrors.Unavailable,
			errContains: "HTML 페이지(http://example.com/500) 요청이 실패했습니다. 상태 코드: 500 Internal Server Error",
		},
		{
			name: "HTTP 404 Error (Client Error)",
			url:  "http://example.com/404",
			setupMock: func(m *TestMockFetcher) {
				resp := NewMockResponse("", 404)
				resp.Status = "404 Not Found"
				m.On("Get", "http://example.com/404").Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.ExecutionFailed, // 4xx is ExecutionFailed (as per refactor plan)
			errContains: "HTML 페이지(http://example.com/404) 요청이 실패했습니다. 상태 코드: 404 Not Found",
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
				if tt.errType != "" {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s, got %v", tt.errType, err)
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
			name:     "Success - Element Found",
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
			name:     "Error - Selection Not Found (Wait for structure change detection)",
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
		{
			name:     "Error - Underlying Fetch Error",
			url:      "http://example.com/error",
			selector: ".target",
			setupMock: func(m *TestMockFetcher) {
				m.On("Get", "http://example.com/error").Return(nil, errors.New("connection reset"))
			},
			wantErr:     true,
			errContains: "connection reset",
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
		body        interface{} // Object to serialize to JSON for body
		setupMock   func(*TestMockFetcher)
		wantErr     bool
		errType     apperrors.ErrorType
		errContains string
		validateRes func(*testing.T, TestData)
	}{
		{
			name:   "Success - POST with Body and Header",
			method: "POST",
			url:    "http://example.com",
			header: map[string]string{"X-Custom": "HeaderVal"},
			body:   map[string]string{"input": "data"},
			setupMock: func(m *TestMockFetcher) {
				jsonContent := `{"name": "test", "value": 123}`
				resp := NewMockResponse(jsonContent, 200)
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					// Verify Request Properties
					if req.Method != "POST" || req.URL.String() != "http://example.com" {
						return false
					}
					if req.Header.Get("X-Custom") != "HeaderVal" {
						return false
					}

					// Verify Body Read
					if req.Body == nil {
						return false
					}
					bodyBytes, _ := io.ReadAll(req.Body)
					req.Body.Close() // Important to close/reset for further use if needed, but here it's consumed
					return strings.Contains(string(bodyBytes), `"input":"data"`)
				})).Return(resp, nil)
			},
			validateRes: func(t *testing.T, res TestData) {
				assert.Equal(t, "test", res.Name)
				assert.Equal(t, 123, res.Value)
			},
		},
		{
			name:   "Error - JSON Parsing (Invalid Type)",
			method: "GET",
			url:    "http://example.com",
			setupMock: func(m *TestMockFetcher) {
				jsonContent := `{"name": "test", "value": "invalid"}`
				resp := NewMockResponse(jsonContent, 200)
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.ExecutionFailed, // Parsing error
			errContains: "JSON 변환이 실패하였습니다",
		},
		{
			name:   "Error - HTTP 404 Status",
			method: "GET",
			url:    "http://example.com/404",
			setupMock: func(m *TestMockFetcher) {
				resp := NewMockResponse(`{"error": "not found"}`, 404)
				resp.Status = "404 Not Found"
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.ExecutionFailed, // Client Error
			errContains: "JSON API(http://example.com/404) 요청이 실패했습니다. 상태 코드: 404 Not Found",
		},
		{
			name:   "Error - HTTP 500 Status (Unavailable)",
			method: "GET",
			url:    "http://example.com/500",
			setupMock: func(m *TestMockFetcher) {
				resp := NewMockResponse(`error`, 500)
				resp.Status = "500 Internal Server Error"
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: "JSON API(http://example.com/500) 요청이 실패했습니다. 상태 코드: 500 Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &TestMockFetcher{}
			if tt.setupMock != nil {
				tt.setupMock(mockFetcher)
			}

			// Prepare Body Reader
			var bodyReader io.Reader
			if tt.body != nil {
				jsonBytes, _ := json.Marshal(tt.body)
				bodyReader = bytes.NewReader(jsonBytes)
			}

			var result TestData
			err := FetchJSON(mockFetcher, tt.method, tt.url, tt.header, bodyReader, &result)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				if tt.errType != "" {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s, got %v", tt.errType, err)
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

func TestScrapeHTML(t *testing.T) {
	t.Run("Scrape - Iterate All Elements", func(t *testing.T) {
		mockFetcher := &TestMockFetcher{}
		htmlContent := `<html><body><ul class="list"><li>Item 1</li><li>Item 2</li><li>Item 3</li></ul></body></html>`
		resp := NewMockResponse(htmlContent, 200)
		mockFetcher.On("Get", "http://example.com").Return(resp, nil)

		var items []string
		err := ScrapeHTML(mockFetcher, "http://example.com", ".list li", func(i int, s *goquery.Selection) bool {
			items = append(items, s.Text())
			return true
		})

		assert.NoError(t, err)
		assert.Equal(t, 3, len(items))
		assert.Equal(t, []string{"Item 1", "Item 2", "Item 3"}, items)
	})

	t.Run("Scrape - Early Exit", func(t *testing.T) {
		mockFetcher := &TestMockFetcher{}
		htmlContent := `<html><body><ul class="list"><li>Item 1</li><li>Item 2</li><li>Item 3</li></ul></body></html>`
		resp := NewMockResponse(htmlContent, 200)
		mockFetcher.On("Get", "http://example.com").Return(resp, nil)

		count := 0
		err := ScrapeHTML(mockFetcher, "http://example.com", ".list li", func(i int, s *goquery.Selection) bool {
			count++
			return count < 2 // 2가 되면 false 반환, 3번째 아이템 스킵
		})

		assert.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("Scrape - Fetch Error", func(t *testing.T) {
		mockFetcher := &TestMockFetcher{}
		mockFetcher.On("Get", "http://example.com").Return(nil, errors.New("scrape error"))

		err := ScrapeHTML(mockFetcher, "http://example.com", ".list li", func(i int, s *goquery.Selection) bool {
			return true
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "scrape error")
		// FetchHTMLDocument returns Unavailable for network errors
		assert.True(t, apperrors.Is(err, apperrors.Unavailable))
	})
}
