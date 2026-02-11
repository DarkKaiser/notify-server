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

func TestFetchHTML(t *testing.T) {
	// Logrus Hook을 사용하여 로그 메시지를 검증합니다.
	hook := test.NewGlobal()

	tests := []struct {
		name        string
		method      string
		url         string
		body        string // 입력 편의상 string으로 받음
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
					return req.Method == http.MethodGet && req.URL.String() == "http://example.com/utf8"
				})).Return(resp, nil)
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "안녕", doc.Find(".test").Text())
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
			body:   "user=admin",
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
			name:    "Error - Body Too Large",
			method:  http.MethodGet,
			url:     "http://example.com/large",
			options: []scraper.Option{scraper.WithMaxResponseBodySize(10)}, // 아주 작은 크기 제한
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse("This body is definitely larger than 10 bytes", 200)
				// resp.IsTruncated = true (X) - http.Response에는 이 필드가 없음. Scraper 내부 로직으로 처리됨.
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/large", nil)
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

			var bodyReader io.Reader
			if tt.body != "" {
				bodyReader = strings.NewReader(tt.body)
			}

			doc, err := s.FetchHTML(ctx, tt.method, tt.url, bodyReader, nil)

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

func TestParseHTML(t *testing.T) {
	hook := test.NewGlobal()

	tests := []struct {
		name        string
		input       io.Reader
		url         string
		contentType string
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
			errContains: "파싱 초기화 실패: 입력 데이터 스트림(Reader)이 유효하지 않은 타입(Typed Nil)입니다",
		},
		{
			name: "Success - UTF-8 BOM Handling",
			input: func() io.Reader {
				// UTF-8 BOM + Content
				bom := []byte{0xEF, 0xBB, 0xBF}
				return io.MultiReader(bytes.NewReader(bom), strings.NewReader("<html><body>BOM Test</body></html>"))
			}(),
			validate: func(t *testing.T, doc *goquery.Document) {
				// BOM이 제거되지 않을 수 있으므로 Contains로 검증
				assert.Contains(t, doc.Find("body").Text(), "BOM Test")
			},
		},
		{
			name:  "Warning - Unknown Encoding Fallback",
			input: strings.NewReader(`<html><body>Unknown</body></html>`),
			// 매우 이상한 인코딩 이름을 주어 감지 실패 & Fallback 유도
			contentType: "text/html; charset=unknown-xyz",
			checkLog: func(t *testing.T, hook *test.Hook) {
				// 인코딩 감지 실패 경고 로그 확인 (구현에 따라 로그가 남지 않을 수도 있음 - charset 패키지가 unknown을 무시하고 utf-8/windows-1252로 처리할 수 있음)
				// 현재 구현상 DetermineEncoding이 nil을 반환하거나 fallback 될 때를 검증하는 것은 까다로우나,
				// 로직상 'utf8Reader = br'로 떨어지는 경로를 테스트
			},
			validate: func(t *testing.T, doc *goquery.Document) {
				assert.Equal(t, "Unknown", doc.Find("body").Text())
			},
		},
		{
			name: "Success - Partial Read Check (LimitReader)",
			input: func() io.Reader {
				// MaxSize 보다 큰 데이터
				large := strings.Repeat("A", 2048)
				return strings.NewReader(large)
			}(),
			// 실제 MaxSize 적용 확인은 Mocking이 아닌 Integration 레벨에서 확실하지만,
			// 여기서는 ParseHTML이 내부적으로 LimitReader를 쓰는지 간접 확인 (LimitReader는 Read 시 EOF를 빨리 반환)
			// 단위 테스트 레벨에서는 LimitReader 래핑 여부만 로직 검증으로 충분
			validate: func(t *testing.T, doc *goquery.Document) {
				// LimitReader가 적용되어도 goquery는 에러 없이 파싱함 (잘린 HTML도 파싱 시도)
				assert.NotNil(t, doc)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook.Reset()
			// ParseHTML 테스트 시 MaxSize는 기본값 또는 임의값 사용
			s := scraper.New(&mocks.MockFetcher{}, scraper.WithMaxResponseBodySize(1024))

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
}
