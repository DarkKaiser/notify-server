package scraper

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

// eucKrContent Helper to create EUC-KR encoded content
func eucKrContent(s string) string {
	var buf bytes.Buffer
	w := transform.NewWriter(&buf, korean.EUCKR.NewEncoder())
	w.Write([]byte(s))
	w.Close()
	return buf.String()
}

// faultyReader Reader that fails on Read
type faultyReader struct {
	err error
}

func (r *faultyReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

// =================================================================================
// Test Group 1: FetchHTML (High-Level Integration)
// =================================================================================

func TestFetchHTML(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		url         string
		body        io.Reader
		header      http.Header
		options     []Option
		setupMock   func(*mocks.MockFetcher)
		ctxSetup    func() (context.Context, context.CancelFunc)
		wantErr     bool
		errType     apperrors.ErrorType
		errContains []string
		checkLog    func(*testing.T, *test.Hook)
		validate    func(*testing.T, *goquery.Document)
	}{
		{
			name:   "Success - UTF-8 Basic",
			method: http.MethodGet,
			url:    "http://example.com/utf8",
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse(`<html><body><div class="test">Success</div></body></html>`, 200)
				resp.Header.Set("Content-Type", "text/html; charset=utf-8")
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/utf8", nil)
				resp.Request = req

				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == "http://example.com/utf8" &&
						strings.Contains(req.Header.Get("Accept"), "text/html")
				})).Return(resp, nil)
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "Success", doc.Find(".test").Text())
			},
		},
		{
			name:   "Success - Custom Header & POST",
			method: http.MethodPost,
			url:    "http://example.com/post",
			body:   strings.NewReader("key=value"),
			header: http.Header{"X-Custom": []string{"MyValue"}},
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse(`<html></html>`, 200)
				req, _ := http.NewRequest(http.MethodPost, "http://example.com/post", strings.NewReader("key=value"))
				resp.Request = req

				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					buf := new(bytes.Buffer)
					buf.ReadFrom(req.Body)
					return req.Method == http.MethodPost &&
						req.Header.Get("X-Custom") == "MyValue" &&
						buf.String() == "key=value"
				})).Return(resp, nil)
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.NotNil(t, doc)
			},
		},
		{
			name:   "Success - EUC-KR Encoding",
			method: http.MethodGet,
			url:    "http://example.com/euckr",
			setupMock: func(m *mocks.MockFetcher) {
				content := eucKrContent(`<html><body><div class="test">성공</div></body></html>`)
				resp := mocks.NewMockResponse(content, 200)
				resp.Header.Set("Content-Type", "text/html; charset=euc-kr")
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/euckr", nil)
				resp.Request = req
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "성공", doc.Find(".test").Text())
			},
		},
		{
			name:   "Success - Loose Content-Type (Warning Log)",
			method: http.MethodGet,
			url:    "http://example.com/image",
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse(`<html><body>Real HTML</body></html>`, 200)
				resp.Header.Set("Content-Type", "image/png") // Invalid Content-Type for HTML
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/image", nil)
				resp.Request = req
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			checkLog: func(t *testing.T, hook *test.Hook) {
				found := false
				for _, entry := range hook.AllEntries() {
					if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "비표준 Content-Type") {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected warning log for non-standard content type")
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "Real HTML", doc.Find("body").Text())
			},
		},
		{
			name:   "Success - Redirect URL Resolution",
			method: http.MethodGet,
			url:    "http://example.com/initial",
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse(`<html><body><a href="/link">Link</a></body></html>`, 200)
				finalReq, _ := http.NewRequest(http.MethodGet, "http://example.com/final", nil)
				resp.Request = finalReq // Simulate redirect
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "http://example.com/final", doc.Url.String())
				link, exists := doc.Find("a").Attr("href")
				assert.True(t, exists)
				assert.Equal(t, "/link", link)
			},
		},
		{
			name:    "Error - Response Body Too Large (Truncated)",
			method:  http.MethodGet,
			url:     "http://example.com/large",
			options: []Option{WithMaxResponseBodySize(10)},
			setupMock: func(m *mocks.MockFetcher) {
				// To trigger truncation in `executeRequest` -> `readResponseBodyWithLimit`,
				// the response body MUST be larger than limit + 1 (11 bytes).
				// "This body is definitely larger than 10 bytes" is 44 bytes, so it should work.
				// However, if the mock response body reader is not read fully by the code under test, it might fail.
				// `readResponseBodyWithLimit` reads up to limit+1.
				resp := mocks.NewMockResponse("This body is definitely larger than 10 bytes", 200)
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/large", nil)
				resp.Request = req
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.InvalidInput,
			errContains: []string{"응답 본문 크기 초과"},
		},
		{
			name:    "Error - Prepare Body Failed",
			method:  http.MethodPost,
			url:     "http://example.com/fail-read",
			body:    &faultyReader{err: errors.New("read error")},
			wantErr: true,
			// newErrPrepareRequestBody wraps the error with apperrors.Internal or keeps the error type if it's already an AppError.
			// Since faultyReader returns a standard error, it should be wrapped as Internal.
			// We'll check if it contains the original error message.
			errType:     apperrors.Unknown,
			errContains: []string{"read error"},
		},
		{
			name:   "Error - Network Failure",
			method: http.MethodGet,
			url:    "http://example.com/error",
			setupMock: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(nil, errors.New("network error"))
			},
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: []string{"network error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := test.NewGlobal()
			mockFetcher := &mocks.MockFetcher{}
			if tt.setupMock != nil {
				tt.setupMock(mockFetcher)
			}

			var ctx context.Context
			var cancel context.CancelFunc
			if tt.ctxSetup != nil {
				ctx, cancel = tt.ctxSetup()
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()

			s := New(mockFetcher, tt.options...).(*scraper)

			doc, err := s.FetchHTML(ctx, tt.method, tt.url, tt.body, tt.header)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != apperrors.Unknown {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s", tt.errType)
				}
				for _, msg := range tt.errContains {
					assert.Contains(t, err.Error(), msg)
				}
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, doc)
				}
			}

			if tt.checkLog != nil {
				tt.checkLog(t, hook)
			}
			mockFetcher.AssertExpectations(t)
		})
	}
}

func TestFetchHTMLDocument(t *testing.T) {
	// Simple Helper Wrapper Test
	mockFetcher := &mocks.MockFetcher{}
	resp := mocks.NewMockResponse("<html></html>", 200)
	resp.Request, _ = http.NewRequest("GET", "http://example.com", nil)
	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := New(mockFetcher)
	doc, err := s.FetchHTMLDocument(context.Background(), "http://example.com", nil)

	assert.NoError(t, err)
	assert.NotNil(t, doc)
	mockFetcher.AssertExpectations(t)
}

// =================================================================================
// Test Group 2: ParseHTML (Logic & Encoding)
// =================================================================================

func TestParseHTML_Comprehensive(t *testing.T) {
	tests := []struct {
		name        string
		input       io.Reader
		url         string
		contentType string
		options     []Option
		wantErr     bool
		errType     apperrors.ErrorType
		errContains []string
		validate    func(*testing.T, *goquery.Document)
	}{
		{
			name:        "Success - Simple UTF-8",
			input:       strings.NewReader(`<html><head><title>Hello</title></head></html>`),
			contentType: "text/html",
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "Hello", doc.Find("title").Text())
			},
		},
		{
			name:        "Success - EUC-KR with Meta Tag",
			input:       strings.NewReader(eucKrContent(`<html><head><meta charset="euc-kr"><title>한글</title></head></html>`)),
			contentType: "text/html",
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "한글", doc.Find("title").Text())
			},
		},
		{
			name:        "Error - Nil Reader",
			input:       nil,
			wantErr:     true,
			errType:     apperrors.Internal,
			errContains: []string{"nil"},
		},
		{
			name: "Error - Typed Nil Reader",
			input: func() io.Reader {
				var buf *bytes.Buffer
				return buf
			}(),
			wantErr:     true,
			errType:     apperrors.Internal,
			errContains: []string{"Typed Nil"},
		},
		{
			name: "Success - UTF-8 BOM Handling",
			input: func() io.Reader {
				// UTF-8 BOM + Content
				bom := []byte{0xEF, 0xBB, 0xBF}
				return io.MultiReader(bytes.NewReader(bom), strings.NewReader("<html><body>BOM Test</body></html>"))
			}(),
			validate: func(t *testing.T, doc *goquery.Document) {
				text := doc.Find("body").Text()
				assert.True(t, strings.Contains(text, "BOM Test"))
			},
		},
		{
			name:        "Warning - Unknown Encoding Fallback (Should Succeed)",
			input:       strings.NewReader(`<html><body>Unknown</body></html>`),
			contentType: "text/html; charset=unknown-xyz", // Unknown charset
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "Unknown", doc.Find("body").Text())
			},
		},
		{
			name:  "Robustness - Invalid Base URL string",
			input: strings.NewReader("<html></html>"),
			url:   "://invalid-url",
			validate: func(t *testing.T, doc *goquery.Document) {
				// Should ignore invalid URL and parse successfully
				assert.NotNil(t, doc)
			},
		},
		{
			name: "Error - Size Limit Exceeded",
			input: func() io.Reader {
				large := strings.Repeat("A", 11)
				return strings.NewReader(large)
			}(),
			options:     []Option{WithMaxResponseBodySize(10)},
			wantErr:     true,
			errType:     apperrors.InvalidInput,
			errContains: []string{"크기 초과"},
		},
		{
			name:        "Error - Read Failed",
			input:       &faultyReader{err: errors.New("read error")},
			wantErr:     true,
			errType:     apperrors.Unavailable, // newErrReadHTMLInput uses Unavailable
			errContains: []string{"read error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.options
			if len(opts) == 0 {
				opts = []Option{WithMaxResponseBodySize(1024)}
			}
			s := New(&mocks.MockFetcher{}, opts...).(*scraper)

			doc, err := s.ParseHTML(context.Background(), tt.input, tt.url, tt.contentType)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != apperrors.Unknown {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s", tt.errType)
				}
				for _, msg := range tt.errContains {
					assert.Contains(t, err.Error(), msg)
				}
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, doc)
				}
			}
		})
	}
}

// =================================================================================
// Test Group 3: VerifyHTMLContentType (Internal Logic)
// =================================================================================

func TestVerifyHTMLContentType(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		contentType string
		wantLog     bool
		logMsg      string
	}{
		{
			name:        "Standard HTML",
			statusCode:  200,
			contentType: "text/html",
			wantLog:     false,
		},
		{
			name:        "Standard HTML with Charset",
			statusCode:  200,
			contentType: "text/html; charset=utf-8",
			wantLog:     false,
		},
		{
			name:        "XHTML",
			statusCode:  200,
			contentType: "application/xhtml+xml",
			wantLog:     false,
		},
		{
			name:        "Warning - JSON Content Type",
			statusCode:  200,
			contentType: "application/json",
			wantLog:     true,
			logMsg:      "비표준 Content-Type",
		},
		{
			name:        "Warning - Plain Text",
			statusCode:  200,
			contentType: "text/plain",
			wantLog:     true,
			logMsg:      "비표준 Content-Type",
		},
		{
			name:        "Warning - Empty Content Type",
			statusCode:  200,
			contentType: "",
			wantLog:     true,
			logMsg:      "비표준 Content-Type",
		},
		{
			name:        "Ignore - 204 No Content",
			statusCode:  204,
			contentType: "", // Irrelevant
			wantLog:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := test.NewGlobal()
			s := New(&mocks.MockFetcher{}).(*scraper)
			logger := applog.WithContext(context.Background())

			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     http.Header{"Content-Type": []string{tt.contentType}},
			}

			// verifyHTMLContentType returns error generally nil (just logs warning), but we check logs.
			err := s.verifyHTMLContentType(resp, "http://test.url", logger)
			assert.NoError(t, err)

			found := false
			for _, entry := range hook.AllEntries() {
				if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, tt.logMsg) {
					found = true
				}
			}

			if tt.wantLog {
				assert.True(t, found, "Expected warning log")
			} else {
				assert.False(t, found, "Expected no warning log")
			}
		})
	}
}

// =================================================================================
// Test Group 4: Context Cancellation
// =================================================================================

func TestParseHTML_ContextCancel(t *testing.T) {
	hugeHTML := "<html><body>" + strings.Repeat("<div>test</div>", 10000) + "</body></html>"
	reader := strings.NewReader(hugeHTML)

	ctx, cancel := context.WithCancel(context.Background())
	s := New(&mocks.MockFetcher{}).(*scraper)

	cancel() // Cancel immediately

	_, err := s.ParseHTML(ctx, reader, "", "")
	require.Error(t, err)
	// Check if it returns the specific wrapped error for cancellation
	assert.True(t, apperrors.Is(err, apperrors.Unavailable) || errors.Is(err, context.Canceled), "Should return cancellation error")
}

func init() {
	applog.SetLevel(applog.DebugLevel)
}
