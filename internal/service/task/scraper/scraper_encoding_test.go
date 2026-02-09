package scraper_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

// Helper to create EUC-KR content
func eucKrContent(s string) string {
	var buf bytes.Buffer
	w := transform.NewWriter(&buf, korean.EUCKR.NewEncoder())
	w.Write([]byte(s))
	w.Close()
	return buf.String()
}

func TestParseReader_Encoding(t *testing.T) {
	t.Run("EUC-KR Content with Meta Tag", func(t *testing.T) {
		// "안녕" in EUC-KR with explicit meta charset
		rawHTML := eucKrContent(`<html><head><meta charset="euc-kr"></head><body><div class="test">안녕</div></body></html>`)
		r := strings.NewReader(rawHTML)

		s := scraper.New(&mocks.MockFetcher{})
		doc, err := s.ParseReader(context.Background(), r, "", "text/html; charset=euc-kr")

		assert.NoError(t, err)
		assert.Equal(t, "안녕", doc.Find(".test").Text())
	})

	t.Run("URL Injection", func(t *testing.T) {
		r := strings.NewReader(`<html><body><a href="/login">Login</a></body></html>`)
		s := scraper.New(&mocks.MockFetcher{})

		baseUrl := "http://example.com/base"
		doc, err := s.ParseReader(context.Background(), r, baseUrl, "")

		assert.NoError(t, err)
		assert.NotNil(t, doc.Url)
		assert.Equal(t, baseUrl, doc.Url.String())
	})
}

func TestParseFromReader_EncodingDataLoss(t *testing.T) {
	s := scraper.New(&mocks.MockFetcher{})

	// 1. EUC-KR로 인코딩된 HTML 데이터 생성
	// <title>테스트</title>이 앞부분에 위치하도록 함.
	originalStr := `<html><head><meta charset="euc-kr"><title>테스트</title></head><body>내용</body></html>`
	euckrBytes := []byte(eucKrContent(originalStr))

	// 2. Reader 생성 (Non-seekable)
	r := bytes.NewBuffer(euckrBytes)

	// 3. ParseReader 호출
	doc, err := s.ParseReader(context.Background(), r, "http://example.com", "")
	assert.NoError(t, err)
	assert.NotNil(t, doc)

	// 4. 타이틀 확인
	title := doc.Find("title").Text()
	assert.Equal(t, "테스트", title, "Title should be correctly parsed and decoded")
}

func TestFetchHTML_Encoding_EUCKR(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}

	// EUC-KR content with valid HTML header
	rawHTML := eucKrContent(`<html><body><div class="test">안녕</div></body></html>`)
	resp := mocks.NewMockResponse(rawHTML, 200)
	resp.Header.Set("Content-Type", "text/html; charset=euc-kr")
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/euckr-page", nil)
	resp.Request = req

	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)
	doc, err := s.FetchHTML(context.Background(), http.MethodGet, "http://example.com/euckr-page", nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, "안녕", doc.Find(".test").Text())
}

func TestFetchHTML_CharsetFallback(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	s := scraper.New(mockFetcher)

	// euc-kr라고 주장하지만 실제로는 utf-8인 HTML 데이터 (혹은 그 반대 상황 시뮬레이션)
	htmlContent := `<html><body><h1>Hello World</h1></body></html>`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(htmlContent)),
		Header:     make(http.Header),
		Request:    &http.Request{URL: nil},
	}
	// Invalid charset name causes fallback to original reader
	resp.Header.Set("Content-Type", "text/html; charset=invalid-charset-name-xyz")

	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	doc, err := s.FetchHTML(context.Background(), "GET", "http://example.com", nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, "Hello World", doc.Find("h1").Text())
}

func TestParseReader_LimitBodySize(t *testing.T) {
	const maxBodySize = 10 * 1024 * 1024 // 10MB

	// SpyReader
	type SpyReader struct {
		io.Reader
		ReadBytes int64
	}
	// Note: We need a SpyReader implementation. Defining locally.
	// But SpyReader Read method is needed.
}

// SpyReader for TestParseReader_LimitBodySize
type SpyReader struct {
	io.Reader
	ReadBytes int64
}

func (r *SpyReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	r.ReadBytes += int64(n)
	return n, err
}

// Simple Mock Fetcher for this test
type simpleMockFetcher struct {
	fetcher.Fetcher
}

func TestParseReader_LimitBodySize_Implementation(t *testing.T) {
	const maxBodySize = 10 * 1024 * 1024 // 10MB
	// Note: using &mocks.MockFetcher{} which satisfies Fetcher interface is better than simpleMockFetcher
	s := scraper.New(&mocks.MockFetcher{}, scraper.WithMaxResponseBodySize(maxBodySize))

	inputSize := int64(maxBodySize * 2)
	largeData := bytes.Repeat([]byte("a"), int(inputSize))
	spy := &SpyReader{Reader: bytes.NewReader(largeData)}

	_, err := s.ParseReader(context.Background(), spy, "", "")
	assert.NoError(t, err)

	if spy.ReadBytes > int64(float64(maxBodySize)*1.1) {
		t.Errorf("ParseReader read too much data. Read: %d, Max Expected: %d", spy.ReadBytes, maxBodySize)
	}
}

func TestParseReader_TypedNil_Safety(t *testing.T) {
	s := scraper.New(&mocks.MockFetcher{})

	var buf *bytes.Buffer // nil pointer
	var reader io.Reader = buf

	assert.NotPanics(t, func() {
		doc, err := s.ParseReader(context.Background(), reader, "http://example.com", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must not be nil")
		assert.Nil(t, doc)
	})
}

func TestParseReader_InvalidURL_Robustness(t *testing.T) {
	s := scraper.New(&mocks.MockFetcher{}, scraper.WithMaxResponseBodySize(10*1024*1024))

	htmlContent := `<html><body><a href="/relative">Link</a></body></html>`
	reader := strings.NewReader(htmlContent)

	// Invalid URL containing control character
	invalidURL := "http://example.com/foo\x7fbar"

	doc, err := s.ParseReader(context.Background(), reader, invalidURL, "")

	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, "Link", doc.Find("a").Text())
	assert.Nil(t, doc.Url)
}
