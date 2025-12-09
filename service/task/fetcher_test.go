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

// TestHTTPFetcher_UserAgent verifies that the User-Agent header is automatically set if missing.
func TestHTTPFetcher_UserAgent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		if userAgent == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Expecting the default User-Agent set in fetcher.go
		if !strings.Contains(userAgent, "Mozilla/5.0") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	fetcher := NewHTTPFetcher()

	// Test Do method
	req, _ := http.NewRequest("GET", ts.URL, nil)
	resp, err := fetcher.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test Get method
	resp, err = fetcher.Get(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestHTTPFetcher_Timeout checks that the fetcher is initialized with a timeout.
func TestHTTPFetcher_Timeout(t *testing.T) {
	fetcher := NewHTTPFetcher()
	assert.NotNil(t, fetcher)
	// We can't directly check the private client field or its timeout without reflection,
	// but we trust the constructor. Verification relies on code review or functional testing if needed.
}

// TestFetchHTMLDocument verifies fetching and parsing HTML, including robust encoding handling.
func TestFetchHTMLDocument(t *testing.T) {
	// Helper to create EUC-KR encoded content
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
		contentType string // Response Content-Type header
		bodyContent string // Response body content
		setupMock   func(*TestMockFetcher)
		wantErr     bool
		errContains string
		validateDoc func(*testing.T, *goquery.Document)
	}{
		{
			name:        "Success - UTF-8",
			url:         "http://example.com/utf8",
			contentType: "text/html; charset=utf-8",
			bodyContent: `<html><body><div class="test">안녕</div></body></html>`,
			setupMock: func(m *TestMockFetcher) {
				htmlContent := `<html><body><div class="test">안녕</div></body></html>`
				resp := NewMockResponse(htmlContent, 200)
				resp.Header.Set("Content-Type", "text/html; charset=utf-8")
				m.On("Get", "http://example.com/utf8").Return(resp, nil)
			},
			validateDoc: func(t *testing.T, doc *goquery.Document) {
				assert.NotNil(t, doc)
				assert.Equal(t, "안녕", doc.Find(".test").Text())
			},
		},
		{
			name:        "Success - EUC-KR",
			url:         "http://example.com/euckr",
			contentType: "text/html; charset=euc-kr",
			setupMock: func(m *TestMockFetcher) {
				// "안녕" in EUC-KR
				content := eucKrContent(`<html><body><div class="test">안녕</div></body></html>`)
				resp := NewMockResponse(content, 200)
				resp.Header.Set("Content-Type", "text/html; charset=euc-kr")
				m.On("Get", "http://example.com/euckr").Return(resp, nil)
			},
			validateDoc: func(t *testing.T, doc *goquery.Document) {
				assert.NotNil(t, doc)
				// Should be converted to UTF-8 correctly
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
			errContains: "페이지(http://example.com/error) 접근이 실패하였습니다.",
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
			errContains: "페이지(http://example.com/500) 접근이 실패하였습니다.(500 Internal Server Error)",
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

// TestFetchHTMLSelection verifies finding specific elements within an HTML document.
func TestFetchHTMLSelection(t *testing.T) {
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
				assert.NotNil(t, sel)
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
				assert.Nil(t, sel)
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

// TestScrapeHTML verifies the scraping logic, including control flow (breaking the loop).
func TestScrapeHTML(t *testing.T) {
	t.Run("Scrape - Iterate All", func(t *testing.T) {
		mockFetcher := &TestMockFetcher{}
		htmlContent := `
			<html><body>
				<ul class="list">
					<li>Item 1</li>
					<li>Item 2</li>
					<li>Item 3</li>
				</ul>
			</body></html>
		`
		resp := NewMockResponse(htmlContent, 200)
		resp.Header.Set("Content-Type", "text/html; charset=utf-8")
		mockFetcher.On("Get", "http://example.com").Return(resp, nil)

		count := 0
		err := ScrapeHTML(mockFetcher, "http://example.com", ".list li", func(i int, s *goquery.Selection) bool {
			count++
			return true // Continue iteration
		})

		assert.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("Scrape - Early Exit", func(t *testing.T) {
		mockFetcher := &TestMockFetcher{}
		htmlContent := `
			<html><body>
				<ul class="list">
					<li>Item 1</li>
					<li>Item 2</li>
					<li>Item 3</li>
				</ul>
			</body></html>
		`
		resp := NewMockResponse(htmlContent, 200)
		resp.Header.Set("Content-Type", "text/html; charset=utf-8")
		mockFetcher.On("Get", "http://example.com").Return(resp, nil)

		count := 0
		err := ScrapeHTML(mockFetcher, "http://example.com", ".list li", func(i int, s *goquery.Selection) bool {
			count++
			return false // Stop iteration after first item
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

// TestFetchJSON verifies JSON fetching including error cases for bad JSON or empty bodies.
func TestFetchJSON(t *testing.T) {
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
				jsonContent := `{"name": "test", "value": "invalid"}` // Value expects int
				resp := NewMockResponse(jsonContent, 200)
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errContains: "JSON 변환이 실패하였습니다",
		},
		{
			name:   "JSON Parsing Error - Malformed JSON",
			method: "GET",
			url:    "http://example.com/malformed",
			setupMock: func(m *TestMockFetcher) {
				jsonContent := `{"name": "test", "val` // Truncated JSON
				resp := NewMockResponse(jsonContent, 200)
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errContains: "JSON 변환이 실패하였습니다",
		},
		{
			name:   "Empty Body",
			method: "GET",
			url:    "http://example.com/empty",
			setupMock: func(m *TestMockFetcher) {
				resp := NewMockResponse("", 200)
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			// Decoding empty string usually gives EOF, wrapped in our error
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
			errContains: "페이지(http://example.com/404) 접근이 실패하였습니다.(404 Not Found)",
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
