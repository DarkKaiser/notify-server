package scraper_test

import (
	"context"
	"net/http"
	"runtime"
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
