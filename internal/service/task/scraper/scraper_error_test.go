package scraper_test

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFetch_ErrorClassification(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		expectedErrType apperrors.ErrorType
	}{
		{
			name:            "400 Bad Request -> ExecutionFailed",
			statusCode:      400,
			expectedErrType: apperrors.ExecutionFailed,
		},
		{
			name:            "404 Not Found -> ExecutionFailed",
			statusCode:      404,
			expectedErrType: apperrors.ExecutionFailed,
		},
		{
			name:            "429 Too Many Requests -> Unavailable (Retryable)",
			statusCode:      429,
			expectedErrType: apperrors.Unavailable,
		},
		{
			name:            "500 Internal Server Error -> Unavailable",
			statusCode:      500,
			expectedErrType: apperrors.Unavailable,
		},
		{
			name:            "502 Bad Gateway -> Unavailable",
			statusCode:      502,
			expectedErrType: apperrors.Unavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &mocks.MockFetcher{}
			resp := mocks.NewMockResponse("error body", tt.statusCode)
			resp.Status = http.StatusText(tt.statusCode)
			req, _ := http.NewRequest(http.MethodGet, "http://example.com/status", nil)
			resp.Request = req

			mockFetcher.On("Do", mock.Anything).Return(resp, nil)

			s := scraper.New(mockFetcher)

			// Test with FetchHTMLDocument
			doc, err := s.FetchHTMLDocument(context.Background(), "http://example.com/status", nil)

			assert.Error(t, err)
			assert.Nil(t, doc)

			// Check error type
			assert.True(t, apperrors.Is(err, tt.expectedErrType),
				"Expected error type %s for status %d, got err: %v", tt.expectedErrType, tt.statusCode, err)
		})
	}
}

func TestScraper_RetryOn429_408(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"429 Too Many Requests", http.StatusTooManyRequests},
		{"408 Request Timeout", http.StatusRequestTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &mocks.MockFetcher{}
			s := scraper.New(mockFetcher)

			resp := mocks.NewMockResponse("Retry me", tt.statusCode)
			resp.Header.Set("Content-Type", "text/plain")

			mockFetcher.On("Do", mock.Anything).Return(resp, nil)

			_, err := s.FetchHTML(context.Background(), "GET", "http://example.com", nil, nil)

			assert.Error(t, err)
			assert.True(t, apperrors.Is(err, apperrors.Unavailable))
		})
	}
}

func TestScraper_NonRetryOnOther4xx(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	s := scraper.New(mockFetcher)

	resp := mocks.NewMockResponse("Not Found", http.StatusNotFound)
	resp.Header.Set("Content-Type", "text/plain")

	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	_, err := s.FetchHTML(context.Background(), "GET", "http://example.com", nil, nil)

	assert.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.ExecutionFailed))
	assert.False(t, apperrors.Is(err, apperrors.Unavailable))
}

func TestCreateBodyReader_GoroutineLeak(t *testing.T) {
	// 1. Check initial goroutines
	runtime.GC()
	startGoroutines := runtime.NumGoroutine()

	// 2. Scraper with dummy fetcher to exercise createBodyReader via FetchJSON
	ctx := context.Background()
	body := "test body data"

	var result map[string]interface{}
	mockFetcher := &mocks.MockFetcher{}
	mockFetcher.On("Do", mock.Anything).Return(mocks.NewMockResponse("{}", 200), nil)

	s := scraper.New(mockFetcher)
	_ = s.FetchJSON(ctx, "POST", "http://example.com", body, nil, &result)

	// 3. Wait and check
	time.Sleep(10 * time.Millisecond)
	runtime.GC()

	endGoroutines := runtime.NumGoroutine()

	// Relaxed assertion to avoid flakiness
	assert.LessOrEqual(t, endGoroutines, startGoroutines+1, "Goroutine leak detected!")
}

// Slow infinite reader for timeout test
type slowInfiniteReader struct {
	delay time.Duration
}

func (r *slowInfiniteReader) Read(p []byte) (n int, err error) {
	time.Sleep(r.delay)
	if len(p) > 0 {
		p[0] = 'a'
		return 1, nil
	}
	return 0, nil
}

func TestFetchJSON_ContextCancellation_DuringBodyRead(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	slowBody := &slowInfiniteReader{delay: 20 * time.Millisecond}
	s := scraper.New(&mocks.MockFetcher{})

	var result map[string]interface{}
	start := time.Now()
	err := s.FetchJSON(ctx, "POST", "http://example.com", slowBody, nil, &result)
	duration := time.Since(start)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "context canceled"))
	assert.Less(t, duration, 200*time.Millisecond)
}

func TestFetchHTML_ContextCancellation(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	resp := mocks.NewMockResponse(`<html><body>...</body></html>`, 200)
	resp.Header.Set("Content-Type", "text/html")

	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.FetchHTML(ctx, "GET", "http://example.com", nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestBodySizeLimit(t *testing.T) {
	// 11MB dummy content
	largeContent := strings.Repeat("a", 11*1024*1024)

	t.Run("FetchHTMLDocument - Limit Body Size", func(t *testing.T) {
		mockFetcher := &mocks.MockFetcher{}
		resp := mocks.NewMockResponse(largeContent, 200)
		resp.Header.Set("Content-Type", "text/html; charset=utf-8")
		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		resp.Request = req

		mockFetcher.On("Do", mock.Anything).Return(resp, nil)

		s := scraper.New(mockFetcher)
		doc, err := s.FetchHTMLDocument(context.Background(), "http://example.com", nil)

		assert.Error(t, err)
		assert.Nil(t, doc)
		assert.Contains(t, err.Error(), "HTML 처리 중단: 응답 본문의 크기가 허용된 제한")
	})

	t.Run("FetchJSON - Limit Body Size", func(t *testing.T) {
		mockFetcher := &mocks.MockFetcher{}
		// Valid JSON > 10MB
		prefix := `{"key": "`
		suffix := `"}`
		fillerSize := 11*1024*1024 - len(prefix) - len(suffix)
		jsonContent := prefix + strings.Repeat("a", fillerSize) + suffix

		resp := mocks.NewMockResponse(jsonContent, 200)
		mockFetcher.On("Do", mock.Anything).Return(resp, nil)

		s := scraper.New(mockFetcher)
		var result map[string]string
		err := s.FetchJSON(context.Background(), "GET", "http://example.com", nil, nil, &result)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "[JSON 파싱 불가]")
	})
}

func TestWithMaxResponseBodySize(t *testing.T) {
	largeContent := strings.Repeat("a", 5*1024*1024)

	t.Run("Custom MaxBodySize", func(t *testing.T) {
		mockFetcher := &mocks.MockFetcher{}
		resp := mocks.NewMockResponse(largeContent, 200)
		resp.Header.Set("Content-Type", "text/html; charset=utf-8")
		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		resp.Request = req

		mockFetcher.On("Do", mock.Anything).Return(resp, nil)

		limit := int64(1 * 1024 * 1024)
		s := scraper.New(mockFetcher, scraper.WithMaxResponseBodySize(limit))
		doc, err := s.FetchHTMLDocument(context.Background(), "http://example.com", nil)

		assert.Error(t, err)
		assert.Nil(t, doc)
		assert.Contains(t, err.Error(), "HTML 처리 중단: 응답 본문의 크기가 허용된 제한")
	})
}

func TestFetchHTML_ContentTypeValidation_Loose(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}

	// Response with non-HTML content type (e.g. image)
	resp := mocks.NewMockResponse("binary data", 200)
	resp.Header.Set("Content-Type", "image/png")

	url := "http://example.com/image.png"
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	resp.Request = req

	mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == url
	})).Return(resp, nil)

	s := scraper.New(mockFetcher)
	doc, err := s.FetchHTML(context.Background(), http.MethodGet, url, nil, nil)

	// Now it should succeed with warning (loose validation)
	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Contains(t, doc.Text(), "binary data")
}

func TestFetchHTML_ContentTypeSniffingFallback(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}

	// HTML content but with wrong header (text/plain)
	htmlContent := `<html><body><div class="test">Sniffed HTML</div></body></html>`
	resp := mocks.NewMockResponse(htmlContent, 200)
	resp.Header.Set("Content-Type", "text/plain") // Wrong header

	url := "http://example.com/wrong-header"
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	resp.Request = req

	mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == url
	})).Return(resp, nil)

	s := scraper.New(mockFetcher)
	doc, err := s.FetchHTML(context.Background(), http.MethodGet, url, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, "Sniffed HTML", doc.Find(".test").Text())
}
