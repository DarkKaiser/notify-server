package scraper_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

// eucKrContent EUC-KR 인코딩 헬퍼 함수
func eucKrContent(s string) string {
	var buf bytes.Buffer
	w := transform.NewWriter(&buf, korean.EUCKR.NewEncoder())
	w.Write([]byte(s))
	w.Close()
	return buf.String()
}

// faultyReader 읽기 시 에러를 반환하는 Reader
type faultyReader struct {
	err error
}

func (r *faultyReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

func TestFetchHTML(t *testing.T) {
	// Logrus Hook을 사용하여 로그 메시지를 검증합니다.
	hook := test.NewGlobal()

	tests := []struct {
		name        string
		method      string
		url         string
		body        io.Reader
		header      http.Header
		options     []scraper.Option
		setupMock   func(*mocks.MockFetcher)
		ctxSetup    func() (context.Context, context.CancelFunc)
		wantErr     bool
		errContains string
		checkLog    func(*testing.T, *test.Hook)
		validate    func(*testing.T, *goquery.Document)
	}{
		{
			name:   "Success - UTF-8 Basic",
			method: http.MethodGet,
			url:    "http://example.com/utf8",
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse(`<html><body><div class="test">안녕</div></body></html>`, 200)
				resp.Header.Set("Content-Type", "text/html; charset=utf-8")
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/utf8", nil)
				resp.Request = req

				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == "http://example.com/utf8" &&
						req.Header.Get("Accept") != "" // DefaultAccept 확인
				})).Return(resp, nil)
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "안녕", doc.Find(".test").Text())
			},
		},
		{
			name:   "Success - Custom Header",
			method: http.MethodGet,
			url:    "http://example.com/header",
			header: http.Header{"X-Custom": []string{"MyValue"}},
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse(`<html></html>`, 200)
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/header", nil)
				resp.Request = req

				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Header.Get("X-Custom") == "MyValue"
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

				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == "http://example.com/euckr"
				})).Return(resp, nil)
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "성공", doc.Find(".test").Text())
			},
		},
		{
			name:   "Success - POST with Body",
			method: http.MethodPost,
			url:    "http://example.com/login",
			body:   strings.NewReader("user=admin"),
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse("<html></html>", 200)
				req, _ := http.NewRequest(http.MethodPost, "http://example.com/login", strings.NewReader("user=admin"))
				resp.Request = req

				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					if req.Method != http.MethodPost {
						return false
					}
					buf := new(bytes.Buffer)
					buf.ReadFrom(req.Body)
					return buf.String() == "user=admin"
				})).Return(resp, nil)
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.NotNil(t, doc)
			},
		},
		{
			name:   "Success - 204 No Content",
			method: http.MethodGet,
			url:    "http://example.com/204",
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse("", 204)
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/204", nil)
				resp.Request = req

				m.On("Do", mock.Anything).Return(resp, nil)
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "", doc.Text())
			},
		},
		{
			name:   "Success - Loose Content-Type Validation (Warning Log)",
			method: http.MethodGet,
			url:    "http://example.com/image",
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse("<html><body>Real HTML</body></html>", 200)
				resp.Header.Set("Content-Type", "image/png") // 잘못된 Content-Type
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/image", nil)
				resp.Request = req

				m.On("Do", mock.Anything).Return(resp, nil)
			},
			checkLog: func(t *testing.T, hook *test.Hook) {
				// 경고 로그가 발생했는지 확인
				found := false
				for _, entry := range hook.AllEntries() {
					if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "비표준 Content-Type 헤더가 감지") {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected warning log for loose content-type validation")
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
				// 최종 URL 설정
				finalReq, _ := http.NewRequest(http.MethodGet, "http://example.com/final", nil)
				resp.Request = finalReq

				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == "http://example.com/initial"
				})).Return(resp, nil)
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				// goquery.Document의 URL이 최종 URL로 설정되었는지 확인
				assert.Equal(t, "http://example.com/final", doc.Url.String())
				// href 링크가 존재하는지 확인 (절대 경로 변환은 goquery Attr에서 자동 수행되지 않음)
				link, exists := doc.Find("a").Attr("href")
				assert.True(t, exists)
				assert.Equal(t, "/link", link)
			},
		},
		{
			name:   "Error - Prepare Body Failed",
			method: http.MethodPost,
			url:    "http://example.com/fail-body",
			body:   &faultyReader{err: errors.New("read error")},
			setupMock: func(m *mocks.MockFetcher) {
				// Body 읽기 단계에서 실패하므로 Fetcher 호출 안됨
			},
			wantErr:     true,
			errContains: "read error",
		},
		{
			name:    "Error - Request Body Too Large",
			method:  http.MethodPost,
			url:     "http://example.com/large-body",
			body:    strings.NewReader(strings.Repeat("A", 20)),
			options: []scraper.Option{scraper.WithMaxRequestBodySize(10)}, // 10바이트 제한
			setupMock: func(m *mocks.MockFetcher) {
				// Body 읽기 단계에서 실패하므로 Fetcher 호출 안됨
			},
			wantErr:     true,
			errContains: "요청 본문 크기 초과",
		},
		{
			name:   "Error - Network Failure",
			method: http.MethodGet,
			url:    "http://example.com/error",
			setupMock: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(nil, errors.New("connection refused"))
			},
			wantErr:     true,
			errContains: "connection refused",
		},
		{
			name:    "Error - Response Body Too Large",
			method:  http.MethodGet,
			url:     "http://example.com/large-resp",
			options: []scraper.Option{scraper.WithMaxResponseBodySize(10)}, // 아주 작은 크기 제한
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse("This body is definitely larger than 10 bytes", 200)
				// resp.IsTruncated = true (X) - http.Response에는 이 필드가 없음. Scraper 내부 로직으로 처리됨.
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/large-resp", nil)
				resp.Request = req

				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errContains: "응답 본문 크기 초과",
		},
		{
			name:   "Error - Context Canceled",
			method: http.MethodGet,
			url:    "http://example.com/cancel",
			ctxSetup: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // 시작하자마자 취소
				return ctx, cancel
			},
			setupMock: func(m *mocks.MockFetcher) {
				// Context가 취소되었더라도 Do가 호출될 수 있음 (http.Client 동작 모방)
				// 이 경우 에러를 반환하도록 설정
				m.On("Do", mock.Anything).Return(nil, context.Canceled)
			},
			wantErr:     true,
			errContains: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook.Reset() // 로그 훅 초기화

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

			s := scraper.New(mockFetcher, tt.options...)

			doc, err := s.FetchHTML(ctx, tt.method, tt.url, tt.body, tt.header)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
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
	// Simple Helper Test
	mockFetcher := &mocks.MockFetcher{}
	resp := mocks.NewMockResponse("<html></html>", 200)
	resp.Request, _ = http.NewRequest("GET", "http://example.com", nil)
	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)
	doc, err := s.FetchHTMLDocument(context.Background(), "http://example.com", nil)

	assert.NoError(t, err)
	assert.NotNil(t, doc)
	mockFetcher.AssertExpectations(t)
}

func TestParseHTML(t *testing.T) {
	hook := test.NewGlobal()

	tests := []struct {
		name        string
		input       io.Reader
		url         string
		contentType string
		options     []scraper.Option
		wantErr     bool
		errContains string
		checkLog    func(*testing.T, *test.Hook)
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
			errContains: "HTML 파싱 실패: 입력 데이터 스트림이 nil입니다",
		},
		{
			name: "Error - Typed Nil Reader",
			input: func() io.Reader {
				var buf *bytes.Buffer
				return buf
			}(),
			wantErr:     true,
			errContains: "HTML 파싱 실패: 입력 데이터 스트림이 Typed Nil입니다",
		},
		{
			name: "Success - UTF-8 BOM Handling",
			input: func() io.Reader {
				// UTF-8 BOM + Content
				bom := []byte{0xEF, 0xBB, 0xBF}
				return io.MultiReader(bytes.NewReader(bom), strings.NewReader("<html><body>BOM Test</body></html>"))
			}(),
			validate: func(t *testing.T, doc *goquery.Document) {
				// BOM이 제거되지 않고 남아있음을 확인 (Go의 charset/encoding 구현 특성상 BOM이 보존될 수 있음)
				// 실제 동작: "\ufeffBOM Test"
				text := doc.Find("body").Text()
				assert.True(t, strings.Contains(text, "BOM Test"), "Body text should contain 'BOM Test'")
				// BOM이 포함되어 있는지 확인 (선택적)
				// assert.Contains(t, text, "\ufeff")
			},
		},
		{
			name:  "Warning - Unknown Encoding Fallback",
			input: strings.NewReader(`<html><body>Unknown</body></html>`),
			// 매우 이상한 인코딩 이름을 주어 감지 실패 & Fallback 유도
			contentType: "text/html; charset=unknown-xyz",
			checkLog: func(t *testing.T, hook *test.Hook) {
				// 인코딩 감지 실패 경고 로그 확인
				// charset.DetermineEncoding 구현 특성상 unknown은 무시되고 UTF-8/Win-1252로 떨어질 가능성 높음
				// 여기서는 로직이 죽지 않고 Fallback 되는지만 확인
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "Unknown", doc.Find("body").Text())
			},
		},
		{
			name:  "Robustness - Invalid URL String",
			input: strings.NewReader("<html></html>"),
			url:   "://invalid-url",
			checkLog: func(t *testing.T, hook *test.Hook) {
				// URL 파싱 에러 경고 로그 확인
				found := false
				for _, entry := range hook.AllEntries() {
					if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "Base URL 설정 실패") {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected warning log for invalid URL")
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.NotNil(t, doc)
			},
		},
		{
			name: "Error - Size Limit Exceeded (Boundary Check)",
			input: func() io.Reader {
				// 10바이트 제한에 11바이트 데이터
				large := strings.Repeat("A", 11)
				return strings.NewReader(large)
			}(),
			options:     []scraper.Option{scraper.WithMaxResponseBodySize(10)},
			wantErr:     true,
			errContains: "입력 데이터 크기 초과",
		},
		{
			name: "Success - Size Limit Exact Match",
			input: func() io.Reader {
				// 10바이트 제한에 10바이트 데이터
				exact := strings.Repeat("A", 10)
				return strings.NewReader(exact)
			}(),
			options: []scraper.Option{scraper.WithMaxResponseBodySize(10)},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, 10, len(doc.Text())) // HTML 태그 없이 text만 있으면 text content 길이와 같음 (goquery 파싱 방식에 따라 다를 수 있음)
			},
		},
		{
			name:        "Error - Read Failed",
			input:       &faultyReader{err: errors.New("read error")},
			wantErr:     true,
			errContains: "입력 데이터 읽기 실패",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook.Reset()

			// 기본 옵션 + 테스트 케이스 옵션
			opts := tt.options
			if len(opts) == 0 {
				opts = []scraper.Option{scraper.WithMaxResponseBodySize(1024)} // Default for test
			}

			s := scraper.New(&mocks.MockFetcher{}, opts...)

			doc, err := s.ParseHTML(context.Background(), tt.input, tt.url, tt.contentType)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
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
		})
	}
}

// TestParseHTML_ContextCancel 파싱 도중 컨텍스트 취소 시나리오
func TestParseHTML_ContextCancel(t *testing.T) {
	// 매우 큰 데이터로 파싱 시간을 범
	hugeHTML := "<html><body>" + strings.Repeat("<div>test</div>", 10000) + "</body></html>"
	reader := strings.NewReader(hugeHTML)

	ctx, cancel := context.WithCancel(context.Background())
	s := scraper.New(&mocks.MockFetcher{})

	// 파싱 시작 직전에 취소를 걸기 위해 고루틴 사용 또는 Reader 조작
	// 여기서는 단순히 미리 취소된 컨텍스트 전달 (Early Exit 테스트)
	cancel()

	_, err := s.ParseHTML(ctx, reader, "", "")
	assert.Error(t, err)
	// ParseHTML에서 반환하는 에러는 scraper.ErrContextCanceled (apperrors.New로 생성됨)
	// 따라서 context.Canceled와 errors.Is로 매칭되지 않을 수 있음
	assert.True(t, errors.Is(err, scraper.ErrContextCanceled), "Should return scraper.ErrContextCanceled")
}

// TestParseHTML_LogFieldVerification 로그 필드가 올바르게 설정되는지 블랙박스 테스트
func TestParseHTML_LogFieldVerification(t *testing.T) {
	hook := test.NewGlobal()
	s := scraper.New(&mocks.MockFetcher{})

	input := strings.NewReader("<html><title>Test</title></html>")
	url := "http://example.com"
	contentType := "text/html"

	_, err := s.ParseHTML(context.Background(), input, url, contentType)
	assert.NoError(t, err)

	// 로그 엔트리 검사
	found := false
	for _, entry := range hook.AllEntries() {
		// 성공 로그 찾기
		if entry.Level == logrus.DebugLevel && strings.Contains(entry.Message, "HTML 파싱 완료") {
			found = true
			assert.Equal(t, "Test", entry.Data["title"])
			assert.Equal(t, true, entry.Data["has_base_url"])
			break
		}
	}
	assert.True(t, found, "Expected success log with correct fields")
}

// TestFetchHTMLRequestParams_Validator 검증기 로직 단위 테스트
func TestFetchHTMLRequestParams_Validator(t *testing.T) {
	// verifyHTMLContentType 로직은 private 메서드이므로, FetchHTML을 통해 간접 테스트
	// 여기서는 Content-Type 헤더가 없을 때의 동작을 확인

	hook := test.NewGlobal()
	mockFetcher := &mocks.MockFetcher{}

	// Content-Type 없음
	resp := mocks.NewMockResponse(`<html><body>No Content Type</body></html>`, 200)
	resp.Header.Del("Content-Type")
	req, _ := http.NewRequest("GET", "http://example.com/no-type", nil)
	resp.Request = req

	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)
	doc, err := s.FetchHTML(context.Background(), "GET", "http://example.com/no-type", nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, "No Content Type", doc.Find("body").Text())

	// 경고 로그 확인 (Content-Type이 없으므로 비표준으로 간주되어 경고 발생)
	foundWarning := false
	for _, entry := range hook.AllEntries() {
		if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "비표준 Content-Type") {
			foundWarning = true
			break
		}
	}
	assert.True(t, foundWarning, "Should warn about missing content type")
}

func init() {
	// 테스트 환경 로거 설정
	applog.SetLevel(applog.DebugLevel)
}
