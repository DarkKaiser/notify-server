package scraper_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
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
		setupMock   func(*mocks.MockFetcher)
		wantErr     bool
		errType     apperrors.ErrorType
		errContains string
		validateDoc func(*testing.T, *goquery.Document)
	}{
		{
			name: "Success - UTF-8 Explicit Header",
			url:  "http://example.com/utf8",
			setupMock: func(m *mocks.MockFetcher) {
				htmlContent := `<html><body><div class="test">안녕</div></body></html>`
				resp := mocks.NewMockResponse(htmlContent, 200)
				resp.Header.Set("Content-Type", "text/html; charset=utf-8")
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/utf8", nil)
				resp.Request = req
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == "http://example.com/utf8"
				})).Return(resp, nil)
			},
			validateDoc: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "안녕", doc.Find(".test").Text())
			},
		},
		{
			name: "Success - EUC-KR Explicit Header",
			url:  "http://example.com/euckr",
			setupMock: func(m *mocks.MockFetcher) {
				content := eucKrContent(`<html><body><div class="test">안녕</div></body></html>`)
				resp := mocks.NewMockResponse(content, 200)
				resp.Header.Set("Content-Type", "text/html; charset=euc-kr")
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/euckr", nil)
				resp.Request = req
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == "http://example.com/euckr"
				})).Return(resp, nil)
			},
			validateDoc: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "안녕", doc.Find(".test").Text())
			},
		},
		{
			name: "Success - Missing Charset Header (Auto Detection)",
			url:  "http://example.com/auto",
			setupMock: func(m *mocks.MockFetcher) {
				// No charset in header, but content is valid UTF-8
				htmlContent := `<html><head><meta charset="utf-8"></head><body><div class="test">안녕</div></body></html>`
				resp := mocks.NewMockResponse(htmlContent, 200)
				resp.Header.Set("Content-Type", "text/html") // No charset
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/auto", nil)
				resp.Request = req
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == "http://example.com/auto"
				})).Return(resp, nil)
			},
			validateDoc: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "안녕", doc.Find(".test").Text())
			},
		},
		{
			name: "Fetcher Error - Network Failure",
			url:  "http://example.com/error",
			setupMock: func(m *mocks.MockFetcher) {
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == "http://example.com/error"
				})).Return(nil, errors.New("network error"))
			},
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: "HTTP 페이지(http://example.com/error) 요청 중 네트워크 또는 클라이언트 에러가 발생했습니다",
		},
		{
			name: "HTTP 500 Error",
			url:  "http://example.com/500",
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse("Internal Server Error Details", 500)
				resp.Status = "500 Internal Server Error"
				resp.Header.Set("Content-Type", "text/html; charset=utf-8") // Essential for validation
				// Request is needed for URL redaction in error message
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/500", nil)
				resp.Request = req
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == "http://example.com/500"
				})).Return(resp, nil)
			},
			wantErr:     true,
			errContains: "Internal Server Error Details",
		},
		{
			name: "HTTP 404 Error (Client Error)",
			url:  "http://example.com/404",
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse("Not Found Details", 404)
				resp.Status = "404 Not Found"
				resp.Header.Set("Content-Type", "text/html; charset=utf-8") // Essential for validation
				// Request is needed for URL redaction in error message
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/404", nil)
				resp.Request = req
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == "http://example.com/404"
				})).Return(resp, nil)
			},
			wantErr:     true,
			errContains: "Not Found Details",
		},
		{
			name: "Success - XHTML Content Type",
			url:  "http://example.com/xhtml",
			setupMock: func(m *mocks.MockFetcher) {
				htmlContent := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd"><html xmlns="http://www.w3.org/1999/xhtml"><body><div class="test">XHTML</div></body></html>`
				resp := mocks.NewMockResponse(htmlContent, 200)
				resp.Header.Set("Content-Type", "application/xhtml+xml; charset=utf-8")
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/xhtml", nil)
				resp.Request = req
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == "http://example.com/xhtml"
				})).Return(resp, nil)
			},
			validateDoc: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "XHTML", doc.Find(".test").Text())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &mocks.MockFetcher{}
			if tt.setupMock != nil {
				tt.setupMock(mockFetcher)
			}

			s := scraper.New(mockFetcher)
			doc, err := s.FetchHTMLDocument(context.Background(), tt.url, nil)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				if tt.errType != apperrors.Unknown {
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

func TestFetchHTML(t *testing.T) {
	t.Run("Success - POST Request with Body", func(t *testing.T) {
		mockFetcher := &mocks.MockFetcher{}
		htmlContent := `<html><body><div class="result">Success</div></body></html>`
		resp := mocks.NewMockResponse(htmlContent, 200)
		resp.Header.Set("Content-Type", "text/html; charset=utf-8")

		url := "http://example.com/post"
		req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader("key=value"))
		resp.Request = req

		mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			if req.Method != http.MethodPost || req.URL.String() != url {
				return false
			}
			// Verify Body
			buf := new(bytes.Buffer)
			buf.ReadFrom(req.Body)
			return buf.String() == "key=value"
		})).Return(resp, nil)

		s := scraper.New(mockFetcher)

		doc, err := s.FetchHTML(context.Background(), http.MethodPost, url, strings.NewReader("key=value"), nil)

		assert.NoError(t, err)
		assert.NotNil(t, doc)
		assert.Equal(t, "Success", doc.Find(".result").Text())

		// Verify URL injection for relative links
		// goquery Document Url field should be set
		assert.Equal(t, url, doc.Url.String())
	})

	t.Run("Success - Verify Relative Link Resolution", func(t *testing.T) {
		mockFetcher := &mocks.MockFetcher{}
		// HTML with a relative link
		htmlContent := `<html><body><a href="/login">Login</a></body></html>`
		resp := mocks.NewMockResponse(htmlContent, 200)
		resp.Header.Set("Content-Type", "text/html; charset=utf-8")

		url := "http://example.com/base"
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		resp.Request = req

		mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.URL.String() == url
		})).Return(resp, nil)

		s := scraper.New(mockFetcher)
		doc, err := s.FetchHTML(context.Background(), http.MethodGet, url, nil, nil)

		assert.NoError(t, err)

		// Find the link and resolve it
		sel := doc.Find("a")
		_, exists := sel.Attr("href")
		assert.True(t, exists)

		assert.Equal(t, url, doc.Url.String())
	})
}

func TestFetchHTML_NoContent(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	resp := mocks.NewMockResponse("", 204)
	resp.Status = "204 No Content"
	// No Content-Type needed for 204
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/204-html", nil)
	resp.Request = req

	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)
	doc, err := s.FetchHTML(context.Background(), "GET", "http://example.com/204-html", nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, "", doc.Text())
}

func TestFetchHTML_RedirectUrl(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}

	// Simulation of a scenario where a redirect occurred.
	// In the real http.Client, the final response.Request field points to the last request (the redirected one).
	// We simulate this by setting resp.Request to the final URL.
	finalUrl := "http://example.com/final"
	htmlContent := `<html><body>Redirected</body></html>`
	resp := mocks.NewMockResponse(htmlContent, 200)
	resp.Header.Set("Content-Type", "text/html; charset=utf-8")

	finalReq, _ := http.NewRequest(http.MethodGet, finalUrl, nil)
	resp.Request = finalReq

	// The scraper calls Fetcher.Do with the INITIAL URL.
	initialUrl := "http://example.com/initial"
	mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == initialUrl
	})).Return(resp, nil)

	s := scraper.New(mockFetcher)
	doc, err := s.FetchHTML(context.Background(), http.MethodGet, initialUrl, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, doc)

	// doc.Url should be the FINAL URL, not the initial one.
	assert.Equal(t, finalUrl, doc.Url.String())
}
